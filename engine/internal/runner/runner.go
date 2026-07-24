package runner

import (
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

	// DryRunValidated tracks the number of valid cases in dry-run mode.
	DryRunValidated int

	mu sync.Mutex // protects Results for concurrent access
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

// Run executes all test cases. In dry-run mode, it validates configuration
// without making any network calls or modifying files. In normal mode, it
// builds a plan and then executes each case.
func (r *Runner) Run(ctx *context.TestContext) (*Report, error) {
	if ctx.DryRun {
		return r.dryRunCases(ctx), nil
	}

	// Build plan first
	plans, err := r.BuildPlan(ctx)
	if err != nil {
		report := &Report{StartTime: time.Now(), EndTime: time.Now()}
		return report, err
	}

	// Execute plan
	return r.executePlans(ctx, plans), nil
}

// ---------------------------------------------------------------------------
// Dry-run
// ---------------------------------------------------------------------------

// dryRunCases validates all test case configurations without executing them.
func (r *Runner) dryRunCases(ctx *context.TestContext) *Report {
	report := &Report{StartTime: time.Now()}
	defer func() { report.EndTime = time.Now() }()

	plans, err := r.BuildPlan(ctx)
	if err != nil {
		r.Logger("DRY-RUN ERROR: %v", err)
		return report
	}

	for _, plan := range plans {
		if plan.hasBlockingErrors() {
			// Case has blocking validation errors — report each one
			for _, pe := range plan.Errors {
				if pe.Severity == "error" {
					loc := plan.DirName
					if pe.Phase != "" {
						loc += " [" + pe.Phase + "]"
					}
					r.Logger("DRY-RUN ERROR: %s: %s", loc, pe.Message)
				}
			}
			// Report as an Error case
			cr := CaseResult{
				CaseName: plan.DirName,
				Status:   CaseError,
				Duration: 0,
			}
			report.appendResult(cr)
		} else {
			// Case is valid (may have non-blocking warnings)
			stepCount := len(plan.Setup) + len(plan.Action) + len(plan.Teardown)
			r.Logger("DRY-RUN OK: %s (%d steps)", plan.DirName, stepCount)
			// Log any non-blocking warnings
			for _, pe := range plan.Errors {
				loc := plan.DirName
				if pe.Phase != "" {
					loc += " [" + pe.Phase + "]"
				}
				r.Logger("DRY-RUN WARN: %s: %s", loc, pe.Message)
			}
			report.DryRunValidated++
		}
	}

	return report
}

// ---------------------------------------------------------------------------
// Execute plan
// ---------------------------------------------------------------------------

// executePlans runs all test cases from their pre-built plans, handling
// parallel/serial dispatch.
func (r *Runner) executePlans(ctx *context.TestContext, plans []CasePlan) *Report {
	report := &Report{StartTime: time.Now()}
	defer func() { report.EndTime = time.Now() }()

	concurrency := ctx.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	parallelCh := make(chan parallelResult, len(plans))

	for _, plan := range plans {
		if plan.Parallel && concurrency > 1 {
			// Parallel case: acquire semaphore slot, launch goroutine
			sem <- struct{}{}
			wg.Add(1)
			go func(p CasePlan) {
				defer wg.Done()
				defer func() { <-sem }()
				caseCtx := ctx.Clone()
				cr := r.executePlan(caseCtx, p)
				parallelCh <- parallelResult{CaseResult: cr}
			}(plan)
		} else {
			// Serial case: wait for all parallel goroutines first
			wg.Wait()
			r.drainParallelResults(report, parallelCh)
			caseResult := r.executePlan(ctx, plan)
			report.appendResult(caseResult)
		}
	}

	wg.Wait()
	r.drainParallelResults(report, parallelCh)

	return report
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

// executePlan executes a single test case from its pre-built CasePlan.
func (r *Runner) executePlan(ctx *context.TestContext, plan CasePlan) CaseResult {
	start := time.Now()
	result := CaseResult{CaseName: plan.DirName}

	// If the plan has blocking validation errors from BuildPlan, report immediately.
	// The faulty steps were excluded from Setup/Action/Teardown during BuildPlan.
	// Non-blocking warnings are reported by the logger but do not block execution.
	if plan.hasBlockingErrors() {
		result.Status = CaseError
		result.Duration = time.Since(start)
		return result
	}

	// Ensure GUID (write to XML file for persistent tracking)
	data, caseGUID, err := ensureCaseGUID(plan.RawXML, plan.XMLPath)
	if err != nil {
		r.Logger("WARN: %s: guid write: %v", plan.DirName, err)
	}
	_ = data // data is used by ensureCaseGUID to write to file; we keep the plan.RawXML reference
	ctx.Set("case_guid", caseGUID)

	// Determine title
	title := plan.Title
	if title == "" {
		title = plan.DirName
	}

	// Check flags filter — multi-tag, case-insensitive, run-all support
	if !ctx.RunAll && ctx.Flags != "" {
		caseFlags := strings.Fields(plan.Flags)
		cmdFlags := strings.Fields(ctx.Flags)
		matched := false
		for _, cf := range caseFlags {
			for _, cli := range cmdFlags {
				if strings.EqualFold(cf, cli) {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			r.Logger("%s --- SKIPPED: %s (flags=%q, ctx.Flags=%q)", caseGUID, title, plan.Flags, ctx.Flags)
			result.Status = CaseSkipped
			result.Duration = time.Since(start)
			return result
		}
	}

	ctx.GenerateRandomVars()
	r.Logger("%s === CASE: %s ===", caseGUID, title)

	// Execute phases with a sequential step counter
	stepSeq := 0
	for _, ps := range plan.Setup {
		stepSeq++
		report := r.runPlanStep(ctx, "setup", ps, caseGUID, stepSeq)
		result.Steps = append(result.Steps, report)
	}
	for _, ps := range plan.Action {
		stepSeq++
		report := r.runPlanStep(ctx, "action", ps, caseGUID, stepSeq)
		result.Steps = append(result.Steps, report)
	}
	for _, ps := range plan.Teardown {
		stepSeq++
		report := r.runPlanStep(ctx, "teardown", ps, caseGUID, stepSeq)
		result.Steps = append(result.Steps, report)
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

// runPlanStep executes a single step using pre-parsed step data from a CasePlan.
func (r *Runner) runPlanStep(ctx *context.TestContext, phase string, ps ParsedStep, caseGUID string, stepSeq int) StepReport {
	report := StepReport{
		Phase: phase,
		Desc:  ps.Desc,
	}

	stepData := ps.Data
	report.Type = stepData.StepType

	// Log header prefix: shortGUID + stepN
	prefix := fmt.Sprintf("%s STEP%d", shortGUID(caseGUID), stepSeq)

	// Step header
	stepInfo := fmt.Sprintf("type=%s", stepData.StepType)
	if stepData.TranCode != "" {
		stepInfo += fmt.Sprintf(", tranCode=%s", stepData.TranCode)
	}
	r.Logger("%s --- STEP [%s] %s: %s ---", prefix, phase, ps.Desc, stepInfo)

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
	var systemVarsErr error
	if stepData.Server != "" {
		systemVarsErr = ctx.GenerateSystemVars(stepData.Server)
	} else {
		systemVarsErr = ctx.GenerateSystemVarsLegacy(stepData.ServerIndex)
	}
	if systemVarsErr != nil {
		report.Pass = false
		report.Message = fmt.Sprintf("generate system variables: %v", systemVarsErr)
		return report
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

	// Log request data
	if result.RequestData != "" {
		r.Logger("%s   Request:\n%s", prefix, indentLines(result.RequestData, "    "))
	}

	// Log response data
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

// ---------------------------------------------------------------------------
// Internal helpers (shared by BuildPlan and execution)
// ---------------------------------------------------------------------------

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

// caseXML represents the parsed test case XML.
type caseXML struct {
	XMLName  xml.Name  `xml:"case"`
	Tittle   string    `xml:"tittle,attr"`
	Title    string    `xml:"title,attr"`
	GUID     string    `xml:"guid,attr"`
	Parallel string    `xml:"parallel,attr"`
	Flags    string    `xml:"flags,attr"`
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

// truncateLog truncates a string for logging.
func truncateLog(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}

// ---------------------------------------------------------------------------
// Legacy runCase / runStep (kept for reference, no longer used by Run)
// ---------------------------------------------------------------------------

// runCase executes a single test case by re-reading its XML file.
// It is kept for backward compatibility but is no longer called by Run().
// The new execution path uses executePlan() with a CasePlan.
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

	// Check flags filter — multi-tag, case-insensitive, run-all support
	if !ctx.RunAll && ctx.Flags != "" {
		caseFlags := strings.Fields(tc.Flags)
		cmdFlags := strings.Fields(ctx.Flags)
		matched := false
		for _, cf := range caseFlags {
			for _, cli := range cmdFlags {
				if strings.EqualFold(cf, cli) {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			r.Logger("%s --- SKIPPED: %s (flags=%q, ctx.Flags=%q)", caseGUID, title, tc.Flags, ctx.Flags)
			result.Status = CaseSkipped
			result.Duration = time.Since(start)
			return result
		}
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

// runStep executes a single step by parsing its XML at runtime.
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
	var systemVarsErr error
	if stepData.Server != "" {
		systemVarsErr = ctx.GenerateSystemVars(stepData.Server)
	} else {
		systemVarsErr = ctx.GenerateSystemVarsLegacy(stepData.ServerIndex)
	}
	if systemVarsErr != nil {
		report.Pass = false
		report.Message = fmt.Sprintf("generate system variables: %v", systemVarsErr)
		return report
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
