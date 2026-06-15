package runner

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Eric033/x-mate/engine/internal/context"
	"github.com/Eric033/x-mate/engine/internal/handler"
)

// CaseResult holds the execution result of a single test case.
type CaseResult struct {
	CaseName string
	Steps    []StepReport
	Duration time.Duration
}

// StepReport holds the result of a single step.
type StepReport struct {
	Phase   string // setup / action / teardown
	Desc    string
	Type    string
	Pass    bool
	Message string
}

// Report holds the overall test run report.
type Report struct {
	StartTime   time.Time
	EndTime     time.Time
	TotalCases  int
	PassedCases int
	FailedCases int
	Results     []CaseResult
	mu          sync.Mutex // protects Results for concurrent access
}

// appendResult safely appends a case result to the report.
func (r *Report) appendResult(cr CaseResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Results = append(r.Results, cr)
	r.TotalCases++
	allPass := true
	for _, s := range cr.Steps {
		if !s.Pass {
			allPass = false
			break
		}
	}
	if allPass {
		r.PassedCases++
	} else {
		r.FailedCases++
	}
}

// parallelResult holds the result of a parallel case execution.
type parallelResult struct {
	CaseResult CaseResult
	Err        error
}

// Runner orchestrates test case execution.
type Runner struct {
	Registry *handler.Registry
	Logger   func(format string, args ...interface{})
}

// NewRunner creates a Runner with the given handler registry.
func NewRunner(registry *handler.Registry) *Runner {
	return &Runner{
		Registry: registry,
		Logger:   func(format string, args ...interface{}) {}, // no-op default
	}
}

// Run executes all test cases in the testBase directory.
func (r *Runner) Run(ctx *context.TestContext) *Report {
	report := &Report{StartTime: time.Now()}
	defer func() {
		report.EndTime = time.Now()
	}()

	// Scan testcase directories
	testcaseDir := filepath.Join(ctx.TestBase, "testcase")
	entries, err := os.ReadDir(testcaseDir)
	if err != nil {
		r.Logger("ERROR: cannot read testcase directory: %v", err)
		return report
	}

	var caseDirs []string
	for _, e := range entries {
		if e.IsDir() {
			caseDirs = append(caseDirs, e.Name())
		}
	}
	sort.Strings(caseDirs)

	// Determine concurrency
	concurrency := ctx.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}

	// Parallel dispatch helpers
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency) // semaphore for parallel goroutines
	parallelCh := make(chan parallelResult, len(caseDirs))

	for _, dirName := range caseDirs {
		// Dry-run mode: skip execution
		if ctx.DryRun {
			r.dryRunCase(ctx, dirName)
			continue
		}

		// Parse XML to check parallel attribute
		isParallel := r.isCaseParallel(ctx, dirName)

		if isParallel && concurrency > 1 {
			// Parallel case: acquire semaphore slot, launch goroutine
			sem <- struct{}{} // blocks if at capacity
			wg.Add(1)
			go func(name string) {
				defer wg.Done()
				defer func() { <-sem }()

				// Clone context for isolation
				caseCtx := ctx.Clone()
				cr := r.runCase(caseCtx, name)
				parallelCh <- parallelResult{CaseResult: cr}
			}(dirName)
		} else {
			// Serial case: wait for all parallel goroutines to finish first
			wg.Wait()
			// Drain any remaining parallel results
			r.drainParallelResults(report, parallelCh)

			// Execute serially
			caseResult := r.runCase(ctx, dirName)
			report.appendResult(caseResult)
		}
	}

	// Wait for all remaining parallel goroutines
	wg.Wait()
	r.drainParallelResults(report, parallelCh)

	return report
}

// isCaseParallel checks whether a test case has parallel="true" attribute.
func (r *Runner) isCaseParallel(ctx *context.TestContext, dirName string) bool {
	xmlPath := filepath.Join(ctx.TestBase, "testcase", dirName, dirName+".xml")
	data, err := os.ReadFile(xmlPath)
	if err != nil {
		return false
	}

	var tc caseXML
	if err := xml.Unmarshal(data, &tc); err != nil {
		return false
	}

	return strings.EqualFold(tc.Parallel, "true")
}

// drainParallelResults drains all available results from the parallel channel.
func (r *Runner) drainParallelResults(report *Report, ch chan parallelResult) {
	for {
		select {
		case pr := <-ch:
			report.appendResult(pr.CaseResult)
		default:
			return
		}
	}
}

// dryRunCase validates a test case XML without executing it.
func (r *Runner) dryRunCase(ctx *context.TestContext, dirName string) {
	xmlPath := filepath.Join(ctx.TestBase, "testcase", dirName, dirName+".xml")
	data, err := os.ReadFile(xmlPath)
	if err != nil {
		r.Logger("DRY-RUN ERROR: %s: %v", dirName, err)
		return
	}
	r.Logger("DRY-RUN OK: %s (%d bytes)", dirName, len(data))
}

// caseXML represents the parsed test case XML.
type caseXML struct {
	XMLName  xml.Name `xml:"case"`
	Tittle   string   `xml:"tittle,attr"`
	Title    string   `xml:"title,attr"`
	Parallel string   `xml:"parallel,attr"`
	Setup    *phaseXML `xml:"setup"`
	Action   *phaseXML `xml:"action"`
	Teardown *phaseXML `xml:"teardown"`
}

type phaseXML struct {
	Steps []stepXML `xml:"step"`
}

type stepXML struct {
	Desc  string `xml:"desc,attr"`
	Inner string `xml:",innerxml"`
}

// runCase executes a single test case.
func (r *Runner) runCase(ctx *context.TestContext, dirName string) CaseResult {
	start := time.Now()
	result := CaseResult{CaseName: dirName}

	xmlPath := filepath.Join(ctx.TestBase, "testcase", dirName, dirName+".xml")
	data, err := os.ReadFile(xmlPath)
	if err != nil {
		r.Logger("ERROR: %s: %v", dirName, err)
		result.Duration = time.Since(start)
		return result
	}

	var tc caseXML
	if err := xml.Unmarshal(data, &tc); err != nil {
		r.Logger("ERROR: %s: parse XML: %v", dirName, err)
		result.Duration = time.Since(start)
		return result
	}

	// Determine title
	title := tc.Title
	if title == "" {
		title = tc.Tittle
	}
	if title == "" {
		title = dirName
	}

	ctx.GenerateRandomVars()
	r.Logger("=== CASE: %s ===", title)

	// Execute phases
	if tc.Setup != nil {
		for _, s := range tc.Setup.Steps {
			report := r.runStep(ctx, "setup", s)
			result.Steps = append(result.Steps, report)
		}
	}

	if tc.Action != nil {
		for _, s := range tc.Action.Steps {
			report := r.runStep(ctx, "action", s)
			result.Steps = append(result.Steps, report)
		}
	}

	if tc.Teardown != nil {
		for _, s := range tc.Teardown.Steps {
			report := r.runStep(ctx, "teardown", s)
			result.Steps = append(result.Steps, report)
		}
	}

	// Cleanup temporary variables
	ctx.CleanupTemporary()

	result.Duration = time.Since(start)
	r.Logger("=== CASE END: %s (%s) ===", title, result.Duration)
	return result
}

// runStep executes a single step.
func (r *Runner) runStep(ctx *context.TestContext, phase string, s stepXML) StepReport {
	report := StepReport{
		Phase: phase,
		Desc:  s.Desc,
	}

	// Parse the step XML
	stepData, err := handler.ParseStep("<step desc=\"" + s.Desc + "\">" + s.Inner + "</step>")
	if err != nil {
		report.Pass = false
		report.Message = fmt.Sprintf("parse error: %v", err)
		return report
	}

	report.Type = stepData.StepType
	r.Logger("--- STEP [%s] %s: type=%s ---", phase, s.Desc, stepData.StepType)

	// Generate system variables for this step
	if stepData.Server != "" {
		ctx.GenerateSystemVars(stepData.Server)
	} else {
		ctx.GenerateSystemVarsLegacy(stepData.ServerIndex)
	}

	// Route to handler
	h := r.Registry.Get(stepData.StepType)
	if h == nil {
		report.Pass = false
		report.Message = fmt.Sprintf("no handler for step type: %s", stepData.StepType)
		return report
	}

	// Execute
	result := h.Execute(stepData, ctx)
	report.Pass = result.Success
	report.Message = result.FailureMessage

	if result.RequestData != "" && ctx.Verbose {
		r.Logger("  Request: %s", truncateLog(result.RequestData))
	}
	if result.ResponseData != "" && ctx.Verbose {
		r.Logger("  Response: %s", truncateLog(result.ResponseData))
	}

	if result.Success {
		r.Logger("  ✓ PASS")
	} else {
		r.Logger("  ✗ FAIL: %s", result.FailureMessage)
	}

	return report
}

// truncateLog truncates a string for logging.
func truncateLog(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}