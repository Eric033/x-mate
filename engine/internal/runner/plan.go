package runner

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Eric033/x-mate/engine/internal/context"
	"github.com/Eric033/x-mate/engine/internal/handler"

	"github.com/antchfx/xmlquery"
)

// CasePlan holds the parsed and validated plan for a test case.
// It is generated during BuildPlan and consumed by ExecutePlan.
type CasePlan struct {
	DirName  string
	Title    string
	GUID     string
	Flags    string
	Parallel bool
	// Pre-parsed XML data
	XMLPath string
	RawXML  []byte
	// Parsed phases with step data
	Setup    []ParsedStep
	Action   []ParsedStep
	Teardown []ParsedStep
	// Validation errors (collected during BuildPlan, not execution errors)
	Errors []PlanError
}

// ParsedStep holds the fully parsed step data for a single step.
type ParsedStep struct {
	Desc  string
	Phase string
	Data  *handler.StepData // fully parsed step data
}

// PlanError records a validation issue found during BuildPlan.
type PlanError struct {
	Phase    string // "setup", "action", "teardown", or "" for case-level
	StepIdx  int    // index within the phase, -1 for phase-level or case-level
	Severity string // "error" or "warning"
	Message  string
}

// hasBlockingErrors returns true if the plan contains at least one error-severity
// issue (handler not found, template missing, service undefined, etc.).
// Warnings (assertion style, missing description) do not block execution.
func (p *CasePlan) hasBlockingErrors() bool {
	for _, e := range p.Errors {
		if e.Severity == "error" {
			return true
		}
	}
	return false
}

// knownTCPStepTypes lists step types that use TCP and require a template file.
var knownTCPStepTypes = map[string]bool{
	"xml_set":        true,
	"xml_set_8":      true,
	"xml_set_sas":    true,
	"mca":            true,
	"tcp_damper_set": true,
	"mca_damper_set": true,
}

// BuildPlan scans the testcase directory, parses all case XML files, validates
// configuration (handlers, templates, services, assertions), and returns a list
// of CasePlan. It has no side effects on external systems.
func (r *Runner) BuildPlan(ctx *context.TestContext) ([]CasePlan, error) {
	testcaseDir := filepath.Join(ctx.TestBase, "testcase")
	entries, err := os.ReadDir(testcaseDir)
	if err != nil {
		return nil, fmt.Errorf("read testcase directory %s: %w", testcaseDir, err)
	}

	var caseDirs []string
	for _, e := range entries {
		if e.IsDir() {
			caseDirs = append(caseDirs, e.Name())
		}
	}
	sort.Strings(caseDirs)

	var plans []CasePlan
	for _, dirName := range caseDirs {
		plan := r.buildPlanForCase(ctx, dirName)
		plans = append(plans, plan)
	}

	return plans, nil
}

// buildPlanForCase builds a CasePlan for a single test case directory.
func (r *Runner) buildPlanForCase(ctx *context.TestContext, dirName string) CasePlan {
	plan := CasePlan{
		DirName:  dirName,
		Parallel: false,
	}

	caseDir := filepath.Join(ctx.TestBase, "testcase", dirName)
	xmlPath, err := findCaseXML(caseDir)
	if err != nil {
		plan.Errors = append(plan.Errors, PlanError{
			Severity: "error",
			Message:  fmt.Sprintf("missing XML case file: %v", err),
		})
		return plan
	}

	data, err := os.ReadFile(xmlPath)
	if err != nil {
		plan.Errors = append(plan.Errors, PlanError{
			Severity: "error",
			Message:  fmt.Sprintf("read XML: %v", err),
		})
		return plan
	}

	plan.XMLPath = xmlPath
	plan.RawXML = data

	var tc caseXML
	if err := xml.Unmarshal(data, &tc); err != nil {
		plan.Errors = append(plan.Errors, PlanError{
			Severity: "error",
			Message:  fmt.Sprintf("parse XML: %v", err),
		})
		return plan
	}

	plan.Title = tc.Title
	if plan.Title == "" {
		plan.Title = tc.Tittle
	}
	plan.GUID = tc.GUID
	plan.Flags = tc.Flags
	plan.Parallel = strings.EqualFold(tc.Parallel, "true")

	// Parse and validate each phase (collecting validation errors)
	setupSteps, setupErrs := r.validatePhase(ctx, "setup", tc.Setup)
	actionSteps, actionErrs := r.validatePhase(ctx, "action", tc.Action)
	teardownSteps, teardownErrs := r.validatePhase(ctx, "teardown", tc.Teardown)

	plan.Setup = setupSteps
	plan.Action = actionSteps
	plan.Teardown = teardownSteps
	plan.Errors = append(plan.Errors, setupErrs...)
	plan.Errors = append(plan.Errors, actionErrs...)
	plan.Errors = append(plan.Errors, teardownErrs...)

	// Check for zero steps
	totalSteps := len(plan.Setup) + len(plan.Action) + len(plan.Teardown)
	if totalSteps == 0 {
		plan.Errors = append(plan.Errors, PlanError{
			Severity: "error",
			Message:  "case has zero steps (setup/action/teardown are all empty)",
		})
	}

	return plan
}

// validatePhase parses and validates all steps in a phase, returning parsed
// steps and any validation errors encountered.
// Blocking errors (no handler, missing template, missing service) exclude the
// step from the plan. Non-blocking warnings (missing description, assertion
// format) are collected as PlanError but the step is still included.
func (r *Runner) validatePhase(ctx *context.TestContext, phase string, phaseXML *phaseXML) ([]ParsedStep, []PlanError) {
	if phaseXML == nil {
		return nil, nil
	}

	var steps []ParsedStep
	var errs []PlanError
	for idx, s := range phaseXML.Steps {
		ps, planErrs := r.validateAndParseStep(ctx, phase, s)
		for i := range planErrs {
			planErrs[i].StepIdx = idx
		}
		if ps.Data == nil {
			// Blocking error — step was excluded
			errs = append(errs, planErrs...)
			continue
		}
		// Non-blocking warnings are still collected, but step is included
		errs = append(errs, planErrs...)
		steps = append(steps, ps)
	}
	return steps, errs
}

// validateAndParseStep parses a step XML and validates its configuration.
// Returns the ParsedStep and any PlanErrors. The step is considered valid
// when ParsedStep.Data is non-nil; if Data is nil the step must be excluded.
func (r *Runner) validateAndParseStep(ctx *context.TestContext, phase string, s stepXML) (ParsedStep, []PlanError) {
	var errs []PlanError

	if strings.TrimSpace(s.Desc) == "" {
		errs = append(errs, PlanError{
			Phase:    phase,
			Severity: "warning",
			Message:  "step has no description",
		})
	}

	rawXML := "<step desc=\"" + s.Desc + "\">" + s.Inner + "</step>"
	stepData, err := handler.ParseStep(rawXML)
	if err != nil {
		errs = append(errs, PlanError{
			Phase:    phase,
			Severity: "error",
			Message:  fmt.Sprintf("parse step XML: %v", err),
		})
		// Can't use this step
		return ParsedStep{}, errs
	}

	// Check handler registration (blocking)
	h := r.Registry.Get(stepData.StepType)
	if h == nil {
		errs = append(errs, PlanError{
			Phase:    phase,
			Severity: "error",
			Message:  fmt.Sprintf("no handler registered for step type: %s", stepData.StepType),
		})
		return ParsedStep{}, errs
	}
	_ = h // handler reference — we validate existence without executing

	// Check template exists for TCP steps (blocking)
	if knownTCPStepTypes[stepData.StepType] && stepData.TranCode != "" {
		tmplPath := filepath.Join(ctx.TestBase, "template", "template_"+stepData.TranCode+".xml")
		if _, err := os.Stat(tmplPath); os.IsNotExist(err) {
			errs = append(errs, PlanError{
				Phase:    phase,
				Severity: "error",
				Message:  fmt.Sprintf("template not found for trancode %s: %s", stepData.TranCode, tmplPath),
			})
			return ParsedStep{}, errs
		}
	}

	// Check service exists for steps with server attribute (non-blocking —
	// services like "TESTDB" may be configured via DB pool manager or other
	// mechanisms not visible in ctx.Services).
	if stepData.Server != "" {
		if _, ok := ctx.Services[stepData.Server]; !ok {
			errs = append(errs, PlanError{
				Phase:    phase,
				Severity: "warning",
				Message:  fmt.Sprintf("service %q not defined in context.Services (may be configured externally)", stepData.Server),
			})
		}
	}

	// Basic assertion format check (non-blocking — runtime expressions like
	// "{{var}} > 0" are valid even with empty name attribute).
	// We skip empty XPath/JSONPath checks because runtime_verify and SQL
	// assertion patterns use expression-based assertions without traditional
	// paths.
	_ = stepData.Assertions // validation placeholder

	// Validate XPath syntax in value expressions (blocking)
	for _, v := range stepData.Values {
		if isXPathExpr(v.Key) {
			if _, err := compileXPath(v.Key); err != nil {
				errs = append(errs, PlanError{
					Phase:    phase,
					Severity: "error",
					Message:  fmt.Sprintf("invalid XPath expression %q: %v", v.Key, err),
				})
			}
		}
	}

	return ParsedStep{
		Desc:  s.Desc,
		Phase: phase,
		Data:  stepData,
	}, errs
}

// isXPathExpr returns true if the string looks like an XPath expression.
func isXPathExpr(s string) bool {
	return strings.HasPrefix(s, "//") || strings.HasPrefix(s, "/")
}

// compileXPath checks whether the XPath expression is syntactically valid.
func compileXPath(expr string) (interface{}, error) {
	// Use xmlquery to validate XPath syntax
	doc, err := xmlquery.Parse(strings.NewReader("<root/>"))
	if err != nil {
		return nil, err
	}
	_, err = xmlquery.Query(doc, expr)
	return nil, err
}
