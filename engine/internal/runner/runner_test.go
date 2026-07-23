package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Eric033/x-mate/engine/internal/context"
	"github.com/Eric033/x-mate/engine/internal/handler"
)

// setupTempTestCaseDir creates a temporary testbase with testcase and template directories.
func setupTempTestCaseDir(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()

	// Create template directory
	tmplDir := filepath.Join(tmpDir, "template")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatalf("failed to create template dir: %v", err)
	}

	// Create a sample template
	tmplContent := `<?xml version="1.0" encoding="UTF-8"?>
	<root>
		<TRAN_CODE>T001</TRAN_CODE>
		<STATUS>OK</STATUS>
	</root>`
	if err := os.WriteFile(filepath.Join(tmplDir, "template_T001.xml"), []byte(tmplContent), 0644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	return tmpDir
}

// createTestCase creates a test case XML file in the testbase directory.
func createTestCase(t *testing.T, testBase, caseName, xmlContent string) {
	t.Helper()

	caseDir := filepath.Join(testBase, "testcase", caseName)
	if err := os.MkdirAll(caseDir, 0755); err != nil {
		t.Fatalf("failed to create testcase dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(caseDir, caseName+".xml"), []byte(xmlContent), 0644); err != nil {
		t.Fatalf("failed to write testcase: %v", err)
	}
}

func TestRunner_Run_NoTestCases(t *testing.T) {
	tmpDir := setupTempTestCaseDir(t)

	ctx := context.New()
	ctx.TestBase = tmpDir

	registry := handler.NewRegistry()
	runner := NewRunner(registry)

	report, err := runner.Run(ctx)

	// Directory exists but doesn't have a testcase/ subdirectory.
	// The new Run signature returns an error when it can't read the testcase dir.
	if err == nil {
		t.Error("expected error for missing testcase subdirectory, got nil")
	}
	if report.TotalCases != 0 {
		t.Errorf("expected 0 cases, got %d", report.TotalCases)
	}
}

func TestRunner_Run_DryRun(t *testing.T) {
	tmpDir := setupTempTestCaseDir(t)

	// Create a test case with a registered handler
	caseXML := `<case flags="core" title="TestDryRun">
		<action>
			<step desc="dry run step">
				<Action type="xml_set" server_index="1" trancode="T001"/>
			</step>
		</action>
	</case>`
	createTestCase(t, tmpDir, "case001", caseXML)

	var logBuf strings.Builder

	ctx := context.New()
	ctx.TestBase = tmpDir
	ctx.DryRun = true

	registry := handler.NewRegistry()
	// Register the xml_set handler so dry-run validation passes
	registry.Register("xml_set", &dummyRunnerHandler{success: true})

	runner := NewRunner(registry)
	runner.Logger = func(format string, args ...interface{}) {
		logBuf.WriteString(fmt.Sprintf(format, args...))
		logBuf.WriteString("\n")
	}

	report, _ := runner.Run(ctx)

	// Dry-run should show 1 validated case, 0 errors, 0 total cases in Results
	if report.DryRunValidated != 1 {
		t.Errorf("expected 1 validated case in dry-run, got %d", report.DryRunValidated)
	}
	if report.ErrorCases != 0 {
		t.Errorf("expected 0 errors in dry-run, got %d", report.ErrorCases)
	}
	if report.TotalCases != 0 {
		t.Errorf("expected 0 total (result) cases in dry-run, got %d (validated cases are not in Results)", report.TotalCases)
	}

	logStr := logBuf.String()
	if !strings.Contains(logStr, "DRY-RUN OK") {
		t.Errorf("expected DRY-RUN OK in logs, got: %s", logStr)
	}
}

func TestRunner_Run_SingleCase(t *testing.T) {
	tmpDir := setupTempTestCaseDir(t)

	caseXML := `<case flags="core" title="SimplePassCase">
		<setup>
			<step desc="setup step">
				<Action type="http" server_index="1"/>
			</step>
		</setup>
		<action>
			<step desc="action step">
				<Action type="http" server_index="1" trancode="T001"/>
			</step>
		</action>
		<teardown>
			<step desc="teardown step">
				<Action type="http" server_index="1"/>
			</step>
		</teardown>
	</case>`
	createTestCase(t, tmpDir, "case001", caseXML)

	ctx := context.New()
	ctx.TestBase = tmpDir

	registry := handler.NewRegistry()
	// Register a dummy handler that always succeeds
	dummyHandler := &dummyRunnerHandler{success: true}
	registry.Register("http", dummyHandler)

	runner := NewRunner(registry)
	report, _ := runner.Run(ctx)

	if report.TotalCases != 1 {
		t.Errorf("expected 1 case, got %d", report.TotalCases)
	}
	if report.PassedCases != 1 {
		t.Errorf("expected 1 passed, got %d", report.PassedCases)
	}
	if report.FailedCases != 0 {
		t.Errorf("expected 0 failed, got %d", report.FailedCases)
	}

	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Results))
	}

	result := report.Results[0]
	if result.CaseName != "case001" {
		t.Errorf("expected CaseName 'case001', got '%s'", result.CaseName)
	}
	if result.Status != CasePassed {
		t.Errorf("expected Status 'passed', got '%s'", result.Status)
	}

	// Should have 3 steps: setup, action, teardown
	if len(result.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(result.Steps))
	}

	// Check step phases
	if result.Steps[0].Phase != "setup" {
		t.Errorf("step 0 phase = %q, want 'setup'", result.Steps[0].Phase)
	}
	if result.Steps[1].Phase != "action" {
		t.Errorf("step 1 phase = %q, want 'action'", result.Steps[1].Phase)
	}
	if result.Steps[2].Phase != "teardown" {
		t.Errorf("step 2 phase = %q, want 'teardown'", result.Steps[2].Phase)
	}

	// All steps should pass
	for i, s := range result.Steps {
		if !s.Pass {
			t.Errorf("step %d (%s) failed: %s", i, s.Phase, s.Message)
		}
	}
}

func TestRunner_Run_WithFailure(t *testing.T) {
	tmpDir := setupTempTestCaseDir(t)

	caseXML := `<case flags="core" title="FailCase">
		<action>
			<step desc="failing step">
				<Action type="failing_handler" server_index="1"/>
			</step>
		</action>
	</case>`
	createTestCase(t, tmpDir, "fail001", caseXML)

	ctx := context.New()
	ctx.TestBase = tmpDir

	registry := handler.NewRegistry()
	// Register a handler that fails
	failingHandler := &dummyRunnerHandler{success: false, failMsg: "intentional failure"}
	registry.Register("failing_handler", failingHandler)

	runner := NewRunner(registry)
	report, _ := runner.Run(ctx)

	if report.TotalCases != 1 {
		t.Errorf("expected 1 case, got %d", report.TotalCases)
	}
	if report.PassedCases != 0 {
		t.Errorf("expected 0 passed, got %d", report.PassedCases)
	}
	if report.FailedCases != 1 {
		t.Errorf("expected 1 failed, got %d", report.FailedCases)
	}

	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Results))
	}

	result := report.Results[0]
	if result.Status != CaseFailed {
		t.Errorf("expected Status 'failed', got '%s'", result.Status)
	}

	if len(result.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(result.Steps))
	}

	step := result.Steps[0]
	if step.Pass {
		t.Fatal("expected step to fail")
	}
	if step.Message != "intentional failure" {
		t.Errorf("expected message 'intentional failure', got '%s'", step.Message)
	}
}

func TestRunner_Run_UnregisteredHandler(t *testing.T) {
	tmpDir := setupTempTestCaseDir(t)

	caseXML := `<case flags="core" title="UnknownHandler">
		<action>
			<step desc="unknown type">
				<Action type="nonexistent_type" server_index="1"/>
			</step>
		</action>
	</case>`
	createTestCase(t, tmpDir, "unknown001", caseXML)

	ctx := context.New()
	ctx.TestBase = tmpDir

	registry := handler.NewRegistry()
	// Don't register anything for "nonexistent_type"

	runner := NewRunner(registry)
	report, _ := runner.Run(ctx)

	// BuildPlan detects the unregistered handler and reports as Error (not Failed)
	if report.TotalCases != 1 {
		t.Errorf("expected 1 case, got %d", report.TotalCases)
	}
	if report.ErrorCases != 1 {
		t.Errorf("expected 1 error, got %d (failed=%d)", report.ErrorCases, report.FailedCases)
	}
	if report.FailedCases != 0 {
		t.Errorf("expected 0 failed, got %d", report.FailedCases)
	}

	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Results))
	}

	result := report.Results[0]
	if result.Status != CaseError {
		t.Errorf("expected Status 'error', got '%s'", result.Status)
	}
	// The step was excluded from execution by BuildPlan, so there are 0 steps
	if len(result.Steps) != 0 {
		t.Errorf("expected 0 steps (validation error caught in BuildPlan), got %d", len(result.Steps))
	}
}

func TestRunner_Run_MultipleCases(t *testing.T) {
	tmpDir := setupTempTestCaseDir(t)

	// Create two test cases
	case1XML := `<case flags="core" title="CaseOne">
		<action>
			<step desc="step one">
				<Action type="mock_handler" server_index="1"/>
			</step>
		</action>
	</case>`
	createTestCase(t, tmpDir, "case001", case1XML)

	case2XML := `<case flags="core" title="CaseTwo">
		<action>
			<step desc="step two">
				<Action type="mock_handler" server_index="1"/>
			</step>
		</action>
	</case>`
	createTestCase(t, tmpDir, "case002", case2XML)

	ctx := context.New()
	ctx.TestBase = tmpDir

	registry := handler.NewRegistry()
	dummyHandler := &dummyRunnerHandler{success: true}
	registry.Register("mock_handler", dummyHandler)

	runner := NewRunner(registry)
	report, _ := runner.Run(ctx)

	if report.TotalCases != 2 {
		t.Errorf("expected 2 cases, got %d", report.TotalCases)
	}
	if report.PassedCases != 2 {
		t.Errorf("expected 2 passed, got %d", report.PassedCases)
	}
	if report.FailedCases != 0 {
		t.Errorf("expected 0 failed, got %d", report.FailedCases)
	}

	if len(report.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(report.Results))
	}
}

func TestRunner_Run_InvalidXML(t *testing.T) {
	tmpDir := setupTempTestCaseDir(t)

	// Create an invalid XML file
	caseDir := filepath.Join(tmpDir, "testcase", "badcase")
	if err := os.MkdirAll(caseDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(caseDir, "badcase.xml"), []byte("not valid xml"), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	ctx := context.New()
	ctx.TestBase = tmpDir

	registry := handler.NewRegistry()
	runner := NewRunner(registry)

	report, _ := runner.Run(ctx)

	// The case should be processed with Error status since XML parsing fails
	if report.TotalCases != 1 {
		t.Errorf("expected 1 case, got %d", report.TotalCases)
	}
	if report.ErrorCases != 1 {
		t.Errorf("expected 1 error, got %d", report.ErrorCases)
	}
	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Results))
	}
	if report.Results[0].Status != CaseError {
		t.Errorf("expected Status 'error', got '%s'", report.Results[0].Status)
	}
}

func TestRunner_Run_CaseNoAction(t *testing.T) {
	tmpDir := setupTempTestCaseDir(t)

	caseXML := `<case flags="core" title="NoActionCase">
		<!-- no action or setup or teardown -->
	</case>`
	createTestCase(t, tmpDir, "empty001", caseXML)

	ctx := context.New()
	ctx.TestBase = tmpDir

	registry := handler.NewRegistry()
	runner := NewRunner(registry)

	report, _ := runner.Run(ctx)

	if report.TotalCases != 1 {
		t.Errorf("expected 1 case, got %d", report.TotalCases)
	}
	if report.ErrorCases != 1 {
		t.Errorf("expected 1 error (zero steps), got %d", report.ErrorCases)
	}

	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Results))
	}

	result := report.Results[0]
	if result.Status != CaseError {
		t.Errorf("expected Status 'error', got '%s'", result.Status)
	}
	if len(result.Steps) != 0 {
		t.Errorf("expected 0 steps, got %d", len(result.Steps))
	}
}

func TestRunner_Run_RequestResponseLogging(t *testing.T) {
	tmpDir := setupTempTestCaseDir(t)

	caseXML := `<case flags="core" title="LoggingCase">
		<action>
			<step desc="logging step">
				<Action type="mock_handler" server_index="1"/>
			</step>
		</action>
	</case>`
	createTestCase(t, tmpDir, "logging001", caseXML)

	var logBuf strings.Builder

	ctx := context.New()
	ctx.TestBase = tmpDir

	registry := handler.NewRegistry()
	dummyHandler := &dummyRunnerHandler{success: true, requestData: "test-request", responseData: "test-response"}
	registry.Register("mock_handler", dummyHandler)

	runner := NewRunner(registry)
	runner.Logger = func(format string, args ...interface{}) {
		logBuf.WriteString(fmt.Sprintf(format, args...))
		logBuf.WriteString("\n")
	}

	report, _ := runner.Run(ctx)

	if report.TotalCases != 1 {
		t.Errorf("expected 1 case, got %d", report.TotalCases)
	}

	logStr := logBuf.String()
	if !strings.Contains(logStr, "PASS") {
		t.Errorf("expected PASS in logs, got: %s", logStr)
	}
	if !strings.Contains(logStr, "Request:") {
		t.Errorf("expected Request: in logs, got: %s", logStr)
	}
	if !strings.Contains(logStr, "test-request") {
		t.Errorf("expected request data in logs, got: %s", logStr)
	}
	if !strings.Contains(logStr, "Response:") {
		t.Errorf("expected Response: in logs, got: %s", logStr)
	}
	if !strings.Contains(logStr, "test-response") {
		t.Errorf("expected response data in logs, got: %s", logStr)
	}
}

func TestRunner_Run_TitleFallback(t *testing.T) {
	tmpDir := setupTempTestCaseDir(t)

	// Use tittle attribute (old format)
	caseXML := `<case flags="core" tittle="OldTitle">
		<action>
			<step desc="test step">
				<Action type="mock_handler" server_index="1"/>
			</step>
		</action>
	</case>`
	createTestCase(t, tmpDir, "oldtitle001", caseXML)

	ctx := context.New()
	ctx.TestBase = tmpDir

	registry := handler.NewRegistry()
	dummyHandler := &dummyRunnerHandler{success: true}
	registry.Register("mock_handler", dummyHandler)

	runner := NewRunner(registry)
	report, _ := runner.Run(ctx)

	if report.TotalCases != 1 {
		t.Errorf("expected 1 case, got %d", report.TotalCases)
	}
	if report.PassedCases != 1 {
		t.Errorf("expected 1 passed, got %d", report.PassedCases)
	}
}

func TestRunner_Run_DryRunError(t *testing.T) {
	tmpDir := setupTempTestCaseDir(t)

	// Create a case directory without an XML file
	caseDir := filepath.Join(tmpDir, "testcase", "emptycase")
	if err := os.MkdirAll(caseDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	// Don't create the XML file

	var logBuf strings.Builder

	ctx := context.New()
	ctx.TestBase = tmpDir
	ctx.DryRun = true

	registry := handler.NewRegistry()
	runner := NewRunner(registry)
	runner.Logger = func(format string, args ...interface{}) {
		logBuf.WriteString(fmt.Sprintf(format, args...))
		logBuf.WriteString("\n")
	}

	report, _ := runner.Run(ctx)

	// Dry-run with a missing XML file: 1 error case, 0 validated
	if report.TotalCases != 1 {
		t.Errorf("expected 1 error case in dry-run, got %d", report.TotalCases)
	}
	if report.ErrorCases != 1 {
		t.Errorf("expected 1 error in dry-run, got %d", report.ErrorCases)
	}
	if report.DryRunValidated != 0 {
		t.Errorf("expected 0 validated in dry-run, got %d", report.DryRunValidated)
	}

	logStr := logBuf.String()
	if !strings.Contains(logStr, "DRY-RUN ERROR") {
		t.Errorf("expected DRY-RUN ERROR in logs, got: %s", logStr)
	}
}

func TestRunner_Run_ReadDirError(t *testing.T) {
	ctx := context.New()
	ctx.TestBase = "/nonexistent/path"

	registry := handler.NewRegistry()
	runner := NewRunner(registry)

	report, err := runner.Run(ctx)

	if err == nil {
		t.Error("expected error for nonexistent directory, got nil")
	}

	if report.TotalCases != 0 {
		t.Errorf("expected 0 cases, got %d", report.TotalCases)
	}
}

func TestRunner_Run_StepHandlerRouting(t *testing.T) {
	tmpDir := setupTempTestCaseDir(t)

	// Test that the handler routing works for different step types
	caseXML := `<case flags="core" title="RoutingTest">
		<action>
			<step desc="tcp step">
				<Action type="xml_set" server_index="1" trancode="T001"/>
			</step>
			<step desc="http step">
				<Action type="http" server_index="1"/>
			</step>
			<step desc="mca step">
				<Action type="mca" server_index="1"/>
			</step>
		</action>
	</case>`
	createTestCase(t, tmpDir, "routing001", caseXML)

	ctx := context.New()
	ctx.TestBase = tmpDir

	registry := handler.NewRegistry()
	// Register different handlers for each type
	registry.Register("xml_set", &dummyRunnerHandler{success: true})
	registry.Register("http", &dummyRunnerHandler{success: true})
	registry.Register("mca", &dummyRunnerHandler{success: true})

	runner := NewRunner(registry)
	report, _ := runner.Run(ctx)

	if report.TotalCases != 1 {
		t.Errorf("expected 1 case, got %d", report.TotalCases)
	}
	if report.PassedCases != 1 {
		t.Errorf("expected 1 passed, got %d", report.PassedCases)
	}

	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Results))
	}

	steps := report.Results[0].Steps
	if len(steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(steps))
	}

	// Check step types
	expectedTypes := []string{"xml_set", "http", "mca"}
	for i, s := range steps {
		if s.Type != expectedTypes[i] {
			t.Errorf("step %d type = %q, want %q", i, s.Type, expectedTypes[i])
		}
	}
}

// TestRunner_Run_SkippedCase — case without flags should be Skipped
func TestRunner_Run_SkippedCase(t *testing.T) {
	tmpDir := setupTempTestCaseDir(t)

	// No flags attribute → should be skipped
	caseXML := `<case title="NoFlagsCase">
		<action>
			<step desc="should not run">
				<Action type="mock_handler" server_index="1"/>
			</step>
		</action>
	</case>`
	createTestCase(t, tmpDir, "skip001", caseXML)

	ctx := context.New()
	ctx.TestBase = tmpDir
	ctx.Flags = "core"

	registry := handler.NewRegistry()
	dummyHandler := &dummyRunnerHandler{success: true}
	registry.Register("mock_handler", dummyHandler)

	runner := NewRunner(registry)
	report, _ := runner.Run(ctx)

	if report.TotalCases != 1 {
		t.Errorf("expected 1 case, got %d", report.TotalCases)
	}
	if report.SkippedCases != 1 {
		t.Errorf("expected 1 skipped, got %d", report.SkippedCases)
	}
	if report.PassedCases != 0 {
		t.Errorf("expected 0 passed, got %d", report.PassedCases)
	}

	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Results))
	}
	if report.Results[0].Status != CaseSkipped {
		t.Errorf("expected Status 'skipped', got '%s'", report.Results[0].Status)
	}
}

// TestRunner_Run_FlagsMismatch — flags mismatch should be Skipped
func TestRunner_Run_FlagsMismatch(t *testing.T) {
	tmpDir := setupTempTestCaseDir(t)

	caseXML := `<case flags="extended" title="ExtendedOnlyCase">
		<action>
			<step desc="should not run">
				<Action type="mock_handler" server_index="1"/>
			</step>
		</action>
	</case>`
	createTestCase(t, tmpDir, "extended001", caseXML)

	ctx := context.New()
	ctx.TestBase = tmpDir
	ctx.Flags = "core"

	registry := handler.NewRegistry()
	dummyHandler := &dummyRunnerHandler{success: true}
	registry.Register("mock_handler", dummyHandler)

	runner := NewRunner(registry)
	report, _ := runner.Run(ctx)

	if report.TotalCases != 1 {
		t.Errorf("expected 1 case, got %d", report.TotalCases)
	}
	if report.SkippedCases != 1 {
		t.Errorf("expected 1 skipped, got %d", report.SkippedCases)
	}
	if report.PassedCases != 0 {
		t.Errorf("expected 0 passed, got %d", report.PassedCases)
	}

	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Results))
	}
	if report.Results[0].Status != CaseSkipped {
		t.Errorf("expected Status 'skipped', got '%s'", report.Results[0].Status)
	}
}

// TestRunner_Run_FlagsMatch — flags matching should execute normally
func TestRunner_Run_FlagsMatch(t *testing.T) {
	tmpDir := setupTempTestCaseDir(t)

	caseXML := `<case flags="core" title="CoreCase">
		<action>
			<step desc="should run">
				<Action type="mock_handler" server_index="1"/>
			</step>
		</action>
	</case>`
	createTestCase(t, tmpDir, "core001", caseXML)

	ctx := context.New()
	ctx.TestBase = tmpDir
	ctx.Flags = "core"

	registry := handler.NewRegistry()
	dummyHandler := &dummyRunnerHandler{success: true}
	registry.Register("mock_handler", dummyHandler)

	runner := NewRunner(registry)
	report, _ := runner.Run(ctx)

	if report.TotalCases != 1 {
		t.Errorf("expected 1 case, got %d", report.TotalCases)
	}
	if report.PassedCases != 1 {
		t.Errorf("expected 1 passed, got %d", report.PassedCases)
	}
	if report.SkippedCases != 0 {
		t.Errorf("expected 0 skipped, got %d", report.SkippedCases)
	}

	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Results))
	}
	if report.Results[0].Status != CasePassed {
		t.Errorf("expected Status 'passed', got '%s'", report.Results[0].Status)
	}
}

// ---- dummyRunnerHandler - a simple handler for testing the runner ----

type dummyRunnerHandler struct {
	success      bool
	failMsg      string
	requestData  string
	responseData string
}

func (d *dummyRunnerHandler) Execute(data *handler.StepData, ctx *context.TestContext) *handler.StepResult {
	return &handler.StepResult{
		Success:        d.success,
		FailureMessage: d.failMsg,
		RequestData:    d.requestData,
		ResponseData:   d.responseData,
	}
}
