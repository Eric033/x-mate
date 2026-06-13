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

	report := runner.Run(ctx)

	if report.TotalCases != 0 {
		t.Errorf("expected 0 cases, got %d", report.TotalCases)
	}
}

func TestRunner_Run_DryRun(t *testing.T) {
	tmpDir := setupTempTestCaseDir(t)

	// Create a test case
	caseXML := `<case title="TestDryRun">
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
	runner := NewRunner(registry)
	runner.Logger = func(format string, args ...interface{}) {
		logBuf.WriteString(fmt.Sprintf(format, args...))
		logBuf.WriteString("\n")
	}

	report := runner.Run(ctx)

	if report.TotalCases != 0 {
		t.Errorf("expected 0 cases in dry-run mode, got %d", report.TotalCases)
	}

	logStr := logBuf.String()
	if !strings.Contains(logStr, "DRY-RUN OK") {
		t.Errorf("expected DRY-RUN OK in logs, got: %s", logStr)
	}
}

func TestRunner_Run_SingleCase(t *testing.T) {
	tmpDir := setupTempTestCaseDir(t)

	caseXML := `<case title="SimplePassCase">
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
	report := runner.Run(ctx)

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

	caseXML := `<case title="FailCase">
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
	report := runner.Run(ctx)

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

	if len(report.Results[0].Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(report.Results[0].Steps))
	}

	step := report.Results[0].Steps[0]
	if step.Pass {
		t.Fatal("expected step to fail")
	}
	if step.Message != "intentional failure" {
		t.Errorf("expected message 'intentional failure', got '%s'", step.Message)
	}
}

func TestRunner_Run_UnregisteredHandler(t *testing.T) {
	tmpDir := setupTempTestCaseDir(t)

	caseXML := `<case title="UnknownHandler">
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
	report := runner.Run(ctx)

	if report.TotalCases != 1 {
		t.Errorf("expected 1 case, got %d", report.TotalCases)
	}
	if report.FailedCases != 1 {
		t.Errorf("expected 1 failed, got %d", report.FailedCases)
	}

	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Results))
	}

	step := report.Results[0].Steps[0]
	if step.Pass {
		t.Fatal("expected step to fail")
	}
	if !strings.Contains(step.Message, "no handler") {
		t.Errorf("expected 'no handler' in message, got '%s'", step.Message)
	}
}

func TestRunner_Run_MultipleCases(t *testing.T) {
	tmpDir := setupTempTestCaseDir(t)

	// Create two test cases
	case1XML := `<case title="CaseOne">
	<action>
		<step desc="step one">
			<Action type="mock_handler" server_index="1"/>
		</step>
	</action>
</case>`
	createTestCase(t, tmpDir, "case001", case1XML)

	case2XML := `<case title="CaseTwo">
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
	report := runner.Run(ctx)

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

	report := runner.Run(ctx)

	// The case should be processed but steps will be empty since parsing fails
	if report.TotalCases != 1 {
		t.Errorf("expected 1 case, got %d", report.TotalCases)
	}
}

func TestRunner_Run_CaseNoAction(t *testing.T) {
	tmpDir := setupTempTestCaseDir(t)

	caseXML := `<case title="NoActionCase">
	<!-- no action or setup or teardown -->
</case>`
	createTestCase(t, tmpDir, "empty001", caseXML)

	ctx := context.New()
	ctx.TestBase = tmpDir

	registry := handler.NewRegistry()
	runner := NewRunner(registry)

	report := runner.Run(ctx)

	if report.TotalCases != 1 {
		t.Errorf("expected 1 case, got %d", report.TotalCases)
	}
	if report.PassedCases != 1 {
		t.Errorf("expected 1 passed, got %d", report.PassedCases)
	}

	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Results))
	}

	// No steps, so it should pass
	if len(report.Results[0].Steps) != 0 {
		t.Errorf("expected 0 steps, got %d", len(report.Results[0].Steps))
	}
}

func TestRunner_Run_VerboseLogging(t *testing.T) {
	tmpDir := setupTempTestCaseDir(t)

	caseXML := `<case title="VerboseCase">
	<action>
		<step desc="verbose step">
			<Action type="mock_handler" server_index="1"/>
		</step>
	</action>
</case>`
	createTestCase(t, tmpDir, "verbose001", caseXML)

	var logBuf strings.Builder

	ctx := context.New()
	ctx.TestBase = tmpDir
	ctx.Verbose = true

	registry := handler.NewRegistry()
	dummyHandler := &dummyRunnerHandler{success: true, requestData: "test-request", responseData: "test-response"}
	registry.Register("mock_handler", dummyHandler)

	runner := NewRunner(registry)
	runner.Logger = func(format string, args ...interface{}) {
		logBuf.WriteString(fmt.Sprintf(format, args...))
		logBuf.WriteString("\n")
	}

	report := runner.Run(ctx)

	if report.TotalCases != 1 {
		t.Errorf("expected 1 case, got %d", report.TotalCases)
	}

	logStr := logBuf.String()
	if !strings.Contains(logStr, "PASS") {
		t.Errorf("expected PASS in logs, got: %s", logStr)
	}
	if !strings.Contains(logStr, "test-request") {
		t.Errorf("expected request data in logs, got: %s", logStr)
	}
	if !strings.Contains(logStr, "test-response") {
		t.Errorf("expected response data in logs, got: %s", logStr)
	}
}

func TestRunner_Run_TitleFallback(t *testing.T) {
	tmpDir := setupTempTestCaseDir(t)

	// Use tittle attribute (old format)
	caseXML := `<case tittle="OldTitle">
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
	report := runner.Run(ctx)

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

	report := runner.Run(ctx)

	if report.TotalCases != 0 {
		t.Errorf("expected 0 cases in dry-run, got %d", report.TotalCases)
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

	report := runner.Run(ctx)

	if report.TotalCases != 0 {
		t.Errorf("expected 0 cases, got %d", report.TotalCases)
	}
}

func TestRunner_Run_StepHandlerRouting(t *testing.T) {
	tmpDir := setupTempTestCaseDir(t)

	// Test that the handler routing works for different step types
	caseXML := `<case title="RoutingTest">
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
	report := runner.Run(ctx)

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

// ---- TruncateLog tests ----

func TestTruncateLog_Short(t *testing.T) {
	s := "short string"
	got := truncateLog(s)
	if got != s {
		t.Errorf("expected '%s', got '%s'", s, got)
	}
}

func TestTruncateLog_Long(t *testing.T) {
	s := ""
	for i := 0; i < 300; i++ {
		s += "x"
	}
	got := truncateLog(s)
	if len(got) != 203 { // 200 + "..."
		t.Errorf("expected 203 chars, got %d", len(got))
	}
	if got[200:] != "..." {
		t.Errorf("expected trailing '...', got '%s'", got[200:])
	}
}

func TestTruncateLog_Newlines(t *testing.T) {
	s := "line1\nline2\nline3"
	got := truncateLog(s)
	if strings.Contains(got, "\n") {
		t.Errorf("newlines should be replaced with spaces, got: %s", got)
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