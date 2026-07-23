package runner

import (
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Eric033/x-mate/engine/internal/context"
	"github.com/Eric033/x-mate/engine/internal/handler"
)

// CaseStatus represents the explicit status of a test case.
type CaseStatus string

const (
	CasePassed  CaseStatus = "passed"
	CaseFailed  CaseStatus = "failed"
	CaseSkipped CaseStatus = "skipped"
	CaseError   CaseStatus = "error"
)

// CaseResult holds the execution result of a single test case.
type CaseResult struct {
	CaseName string
	Status   CaseStatus
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
	StartTime    time.Time
	EndTime      time.Time
	TotalCases   int
	PassedCases  int
	FailedCases  int
	SkippedCases int
	ErrorCases   int
	Results      []CaseResult
	mu           sync.Mutex // protects Results for concurrent access
}

// appendResult safely appends a case result to the report, counting by Status.
func (r *Report) appendResult(cr CaseResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Results = append(r.Results, cr)
	r.TotalCases++
	switch cr.Status {
	case CasePassed:
		r.PassedCases++
	case CaseFailed:
		r.FailedCases++
	case CaseSkipped:
		r.SkippedCases++
	case CaseError:
		r.ErrorCases++
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
func (r *Runner) Run(ctx *context.TestContext) (*Report, error) {
	report := &Report{StartTime: time.Now()}
	defer func() {
		report.EndTime = time.Now()
	}()

	// Scan testcase directories
	testcaseDir := filepath.Join(ctx.TestBase, "testcase")
	entries, err := os.ReadDir(testcaseDir)
	if err != nil {
		r.Logger("ERROR: cannot read testcase directory: %v", err)
		return report, fmt.Errorf("read testcase directory %s: %w", testcaseDir, err)
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

	return report, nil
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

// findCaseXML finds the first .xml file in the case directory.
func findCaseXML(caseDir string) (string, error) {
	entries, err := os.ReadDir(caseDir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".xml") {
			return filepath.Join(caseDir, e.Name()), nil
		}
	}
	return "", fmt.Errorf("no XML case file found in %s", caseDir)
}

// generateGUID returns a random UUID v4 string.
func generateGUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// ensureCaseGUID checks if the <case> element has a guid attribute.
// If missing, it generates a GUID, writes it into the XML file, and returns the updated data.
func ensureCaseGUID(data []byte, xmlPath string) ([]byte, string, error) {
	hasGUID, _ := regexp.Match(`(?i)<case\s[^>]*\bguid\s*=`, data)
	if hasGUID {
		re := regexp.MustCompile(`(?i)\bguid\s*=\s*"([^"]*)"`)
		m := re.FindSubmatch(data)
		if len(m) >= 2 {
			return data, string(m[1]), nil
		}
		return data, "", nil
	}

	guid := generateGUID()

	// Find <case ...> opening tag and insert guid attribute
	re := regexp.MustCompile(`(?i)(<case)([^>]*)(>)`)
	newData := re.ReplaceAll(data, []byte(`$1 guid="`+guid+`" $2$3`))

	if string(newData) == string(data) {
		// Fallback: simple replace
		newData = []byte(strings.Replace(string(data), "<case>", `<case guid="`+guid+`">`, 1))
	}

	if err := os.WriteFile(xmlPath, newData, 0644); err != nil {
		return newData, guid, fmt.Errorf("write guid: %w", err)
	}

	return newData, guid, nil
}

// dryRunCase validates a test case XML without executing it.
func (r *Runner) dryRunCase(ctx *context.TestContext, dirName string) {
	caseDir := filepath.Join(ctx.TestBase, "testcase", dirName)
	xmlPath, err := findCaseXML(caseDir)
	if err != nil {
		r.Logger("DRY-RUN ERROR: %s: %v", dirName, err)
		return
	}
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
	GUID     string   `xml:"guid,attr"`
	Parallel string   `xml:"parallel,attr"`
	Flags    string   `xml:"flags,attr"`
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

// shortGUID truncates a full GUID to its first 8 characters for compact logging.
func shortGUID(guid string) string {
	if len(guid) > 8 {
		return guid[:8]
	}
	return guid
}

// indentLines prepends prefix to every line of s.
func indentLines(s, prefix string) string {
	if s == "" {
		return ""
	}
	return prefix + strings.ReplaceAll(s, "\n", "\n"+prefix)
}

// runCase executes a single test case.
func (r *Runner) runCase(ctx *context.TestContext, dirName string) CaseResult {
	start := time.Now()
	result := CaseResult{CaseName: dirName}

	caseDir := filepath.Join(ctx.TestBase, "testcase", dirName)
	xmlPath, err := findCaseXML(caseDir)
	if err != nil {
		r.Logger("ERROR: %s: %v", dirName, err)
		result.Status = CaseError
		result.Duration = time.Since(start)
		return result
	}

	data, err := os.ReadFile(xmlPath)
	if err != nil {
		r.Logger("ERROR: %s: %v", dirName, err)
		result.Status = CaseError
		result.Duration = time.Since(start)
		return result
	}

	// Auto-generate GUID if missing
	data, caseGUID, err := ensureCaseGUID(data, xmlPath)
	if err != nil {
		r.Logger("WARN: %s: guid write: %v", dirName, err)
	}
	ctx.Set("case_guid", caseGUID)

	var tc caseXML
	if err := xml.Unmarshal(data, &tc); err != nil {
		r.Logger("ERROR: %s: parse XML: %v", dirName, err)
		result.Status = CaseError
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

	// Check flags filter for skip logic.
	// Only filter when ctx.Flags is set (non-empty).
	// When ctx.Flags is empty, all cases execute (backward compat).
	if ctx.Flags != "" && (tc.Flags == "" || tc.Flags != ctx.Flags) {
		r.Logger("%s --- SKIPPED: %s (flags=%q, ctx.Flags=%q)", caseGUID, title, tc.Flags, ctx.Flags)
		result.Status = CaseSkipped
		result.Duration = time.Since(start)
		return result
	}

	ctx.GenerateRandomVars()
	r.Logger("%s === CASE: %s ===", caseGUID, title)

	// Execute phases with a sequential step counter
	stepSeq := 0
	if tc.Setup != nil {
		for _, s := range tc.Setup.Steps {
			stepSeq++
			report := r.runStep(ctx, "setup", s, caseGUID, stepSeq)
			result.Steps = append(result.Steps, report)
		}
	}

	if tc.Action != nil {
		for _, s := range tc.Action.Steps {
			stepSeq++
			report := r.runStep(ctx, "action", s, caseGUID, stepSeq)
			result.Steps = append(result.Steps, report)
		}
	}

	if tc.Teardown != nil {
		for _, s := range tc.Teardown.Steps {
			stepSeq++
			report := r.runStep(ctx, "teardown", s, caseGUID, stepSeq)
			result.Steps = append(result.Steps, report)
		}
	}

	// Check for zero steps (error condition)
	if stepSeq == 0 {
		r.Logger("ERROR: %s: case has zero steps (setup/action/teardown are all empty)", title)
		result.Status = CaseError
		result.Duration = time.Since(start)
		return result
	}

	// Cleanup temporary variables
	ctx.CleanupTemporary()

	// Determine pass/fail from steps
	allPass := true
	for _, s := range result.Steps {
		if !s.Pass {
			allPass = false
			break
		}
	}
	if allPass {
		result.Status = CasePassed
	} else {
		result.Status = CaseFailed
	}

	result.Duration = time.Since(start)
	r.Logger("%s === CASE END: %s (%s) ===", caseGUID, title, result.Duration)
	return result
}

// runStep executes a single step.
func (r *Runner) runStep(ctx *context.TestContext, phase string, s stepXML, caseGUID string, stepSeq int) StepReport {
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
	// Log header prefix: shortGUID + stepN
	prefix := fmt.Sprintf("%s STEP%d", shortGUID(caseGUID), stepSeq)

	// Step header
	stepInfo := fmt.Sprintf("type=%s", stepData.StepType)
	if stepData.TranCode != "" {
		stepInfo += fmt.Sprintf(", tranCode=%s", stepData.TranCode)
	}
	r.Logger("%s --- STEP [%s] %s: %s ---", prefix, phase, s.Desc, stepInfo)

	// Log step input data (test values and expected results)
	if len(stepData.Values) > 0 {
		var sb strings.Builder
		for _, v := range stepData.Values {
			sb.WriteString(fmt.Sprintf("      %s = %s\n", v.Key, v.Value))
		}
		r.Logger("%s   Values:\n%s", prefix, sb.String())
	}
	if len(stepData.Assertions) > 0 {
		var sb strings.Builder
		for _, a := range stepData.Assertions {
			name := a.XPath
			if a.JSONPath != "" {
				name = a.JSONPath
			}
			sb.WriteString(fmt.Sprintf("      %s = %s\n", name, a.Expected))
		}
		r.Logger("%s   Assertions:\n%s", prefix, sb.String())
	}

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

	// Log request data (single log call with embedded newlines)
	if result.RequestData != "" {
		r.Logger("%s   Request:\n%s", prefix, indentLines(result.RequestData, "    "))
	}

	// Log response data (single log call with embedded newlines)
	if result.ResponseData != "" {
		r.Logger("%s   Response:\n%s", prefix, indentLines(result.ResponseData, "    "))
	}

	// Log pass/fail with verification details
	if result.Success {
		r.Logger("%s   PASS", prefix)
	} else {
		r.Logger("%s   FAIL: %s", prefix, result.FailureMessage)
	}

	// Log extracted variables
	if len(result.ExtractedVars) > 0 {
		var parts []string
		for k, v := range result.ExtractedVars {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}
		r.Logger("%s   Extracted: %s", prefix, strings.Join(parts, ", "))
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
