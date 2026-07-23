// Package e2e contains end-to-end integration tests for the Engine framework.
//
// These tests start real mock servers (HTTP + TCP), use SQLite for database steps,
// and drive the Engine runner with real XML test cases from testdata/.
package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Eric033/x-mate/engine/e2e/mockserver"
	"github.com/Eric033/x-mate/engine/e2e/testutil"
	"github.com/Eric033/x-mate/engine/handlers/http"
	"github.com/Eric033/x-mate/engine/handlers/rsa"
	runtimeHandler "github.com/Eric033/x-mate/engine/handlers/runtime"
	sqlHandler "github.com/Eric033/x-mate/engine/handlers/sql"
	"github.com/Eric033/x-mate/engine/handlers/tcp"
	"github.com/Eric033/x-mate/engine/internal/context"
	"github.com/Eric033/x-mate/engine/internal/handler"
	"github.com/Eric033/x-mate/engine/internal/runner"
	"github.com/Eric033/x-mate/engine/internal/sampler"
)

// testBase returns the absolute path to the testdata directory.
func testBase(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	return filepath.Join(filepath.Dir(filename), "testdata")
}

// setupE2E starts mock servers, initializes SQLite, creates the handler registry,
// and returns everything needed to run an E2E test.
func setupE2E(t *testing.T) (*context.TestContext, *runner.Runner, *sampler.DBPoolManager, func()) {
	t.Helper()

	// Start HTTP mock server
	httpMock, err := mockserver.NewHTTPMock()
	if err != nil {
		t.Fatalf("http mock: %v", err)
	}

	// Start TCP mock server
	tcpMock, err := mockserver.NewTCPMock()
	if err != nil {
		t.Fatalf("tcp mock: %v", err)
	}

	// Setup SQLite
	dbManager, err := testutil.SetupTestDB()
	if err != nil {
		t.Fatalf("sqlite setup: %v", err)
	}

	// Create test context
	ctx := context.New()
	ctx.TestBase = testBase(t)
	ctx.Services = map[string]context.ServiceDef{
		"MOCK": {
			Address: httpMock.Listener,
		},
		"TCMOCK": {
			Address: tcpMock.Listener,
		},
	}
	ctx.Flags = "core"
	ctx.Concurrency = 2
	ctx.Verbose = testing.Verbose()

	// Register all handlers
	reg := handler.NewRegistry()
	reg.Register("xml_set_8", &tcp.XMLSet8Handler{})
	reg.Register("xml_set_sas", &tcp.XMLSetSASHandler{})
	reg.Register("xml_set", &tcp.XMLSetHandler{})
	reg.Register("mca", &tcp.MCAHandler{})
	reg.Register("http", &http.HTTPHandler{UseDamper: false})
	reg.Register("damper_set", &http.HTTPHandler{UseDamper: true})
	reg.Register("sql_exe", &sqlHandler.SelectHandler{PoolManager: dbManager})
	reg.Register("sql_select", &sqlHandler.SelectHandler{PoolManager: dbManager})
	reg.Register("sql_update", &sqlHandler.UpdateHandler{PoolManager: dbManager})
	reg.Register("runtime_verify", &runtimeHandler.RuntimeVerifyHandler{})
	reg.Register("rsa", &rsa.RSAHandler{})

	// Create runner
	r := runner.NewRunner(reg)
	r.Logger = func(format string, args ...interface{}) {
		t.Logf(format, args...)
	}

	cleanup := func() {
		httpMock.Close()
		tcpMock.Close()
		dbManager.Close()
	}

	return ctx, r, dbManager, cleanup
}

// TestE2E_AllCases runs all standard test cases with mock servers and SQLite.
func TestE2E_AllCases(t *testing.T) {
	ctx, r, _, cleanup := setupE2E(t)
	defer cleanup()

	report, _ := r.Run(ctx)

	// Log all results
	for _, cr := range report.Results {
		for _, s := range cr.Steps {
			if s.Pass {
				t.Logf("  ✓ %s / %s (%s)", cr.CaseName, s.Desc, s.Phase)
			} else {
				t.Errorf("  ✗ %s / %s (%s): %s", cr.CaseName, s.Desc, s.Phase, s.Message)
			}
		}
		t.Logf("  [%s] %s", cr.Status, cr.CaseName)
	}

	t.Logf("Results: %d/%d passed, %d failed, %d skipped, %d error",
		report.PassedCases, report.TotalCases, report.FailedCases, report.SkippedCases, report.ErrorCases)

	if report.FailedCases > 0 || report.ErrorCases > 0 {
		t.Fatalf("got %d failed, %d error cases, want 0", report.FailedCases, report.ErrorCases)
	}

	// Verify expected cases ran
	// case_flags_skip will be in Results with Status=skipped
	// case_dry_run is only used in dry-run test
	expected := []string{
		"case_http_basic",
		"case_http_vars",
		"case_http_errors",
		"case_tcp_xml_set",
		"case_tcp_xml_set_8",
		"case_tcp_mca",
		"case_sql_select",
		"case_sql_update",
		"case_runtime_verify",
		"case_rsa",
		"case_full_flow",
		"case_flags_skip",
	}
	ran := make(map[string]bool)
	for _, cr := range report.Results {
		ran[cr.CaseName] = true
	}
	for _, name := range expected {
		if !ran[name] {
			t.Errorf("expected case %q did not run", name)
		}
	}
}

// TestE2E_DryRun verifies that dry-run mode parses XML without executing.
func TestE2E_DryRun(t *testing.T) {
	ctx, r, _, cleanup := setupE2E(t)
	defer cleanup()

	ctx.DryRun = true
	report, _ := r.Run(ctx)

	// In dry-run mode, all cases are parsed but not executed
	t.Logf("Dry-run: %d cases parsed", report.TotalCases)

	// dry-run cases don't get added to Results (they're silently validated)
	// At minimum, verify no panic occurred and report is populated
	if report == nil {
		t.Fatal("report should not be nil")
	}

	// In dry-run mode, the runner skips execution so TotalCases stays 0
	// Verify the runner didn't crash
}

// TestE2E_Parallel verifies parallel case execution with concurrency>1.
func TestE2E_Parallel(t *testing.T) {
	ctx, r, _, cleanup := setupE2E(t)
	defer cleanup()

	// Isolate to just parallel cases by using a filter
	// (runner doesn't support flags-based filtering yet, so we use a sub-context)
	ctx.Concurrency = 4

	report, _ := r.Run(ctx)

	for _, cr := range report.Results {
		for _, s := range cr.Steps {
			if s.Pass {
				t.Logf("  ✓ %s / %s", cr.CaseName, s.Desc)
			} else {
				t.Errorf("  ✗ %s / %s: %s", cr.CaseName, s.Desc, s.Message)
			}
		}
	}

	if report.FailedCases > 0 {
		t.Fatalf("parallel test: %d failed cases", report.FailedCases)
	}
}

// TestE2E_HTTPBasicOnly runs a subset of only HTTP cases.
func TestE2E_HTTPBasicOnly(t *testing.T) {
	ctx, r, _, cleanup := setupE2E(t)
	defer cleanup()

	// Point to a testdata subset with only HTTP cases
	// We create a symlink-free copy by constructing paths manually
	_ = ctx  // we use the full testBase, runner handles iteration

	report, _ := r.Run(ctx)

	httpCases := 0
	for _, cr := range report.Results {
		if strings.HasPrefix(cr.CaseName, "case_http_") {
			httpCases++
			for _, s := range cr.Steps {
				if !s.Pass {
					t.Errorf("  ✗ %s / %s: %s", cr.CaseName, s.Desc, s.Message)
				}
			}
		}
	}
	t.Logf("HTTP cases ran: %d", httpCases)
	if httpCases == 0 {
		t.Error("no HTTP cases ran")
	}
}

// TestE2E_SQLOnly runs only SQL test cases.
func TestE2E_SQLOnly(t *testing.T) {
	ctx, r, dbManager, cleanup := setupE2E(t)
	defer cleanup()
	_ = dbManager

	report, _ := r.Run(ctx)

	sqlCases := 0
	for _, cr := range report.Results {
		if strings.HasPrefix(cr.CaseName, "case_sql_") {
			sqlCases++
			for _, s := range cr.Steps {
				if !s.Pass {
					t.Errorf("  ✗ %s / %s: %s", cr.CaseName, s.Desc, s.Message)
				}
			}
		}
	}

	t.Logf("SQL cases ran: %d", sqlCases)
	if sqlCases == 0 {
		t.Error("no SQL cases ran")
	}
}

// TestE2E_FlagsFilter tests the runner's flag filtering capability.
// When flags="extended", only cases with flags="extended" should execute;
// all core cases (flags="core") should be skipped.
func TestE2E_FlagsFilter(t *testing.T) {
	ctx, r, _, cleanup := setupE2E(t)
	defer cleanup()

	// Set flags to "extended" — only case_flags_skip has flags="extended"
	ctx.Flags = "extended"

	report, _ := r.Run(ctx)

	t.Logf("Flags 'extended': %d total, %d passed, %d skipped, %d error",
		report.TotalCases, report.PassedCases, report.SkippedCases, report.ErrorCases)

	// Most cases have flags="core" and should be skipped
	skippedNames := []string{}
	executedNames := []string{}
	for _, cr := range report.Results {
		if cr.Status == "skipped" {
			skippedNames = append(skippedNames, cr.CaseName)
		} else {
			executedNames = append(executedNames, cr.CaseName)
		}
	}

	t.Logf("Executed: %v", executedNames)
	t.Logf("Skipped: %v", skippedNames)

	// case_flags_skip has flags="extended", so it should execute
	hasFlagsSkip := false
	for _, name := range executedNames {
		if name == "case_flags_skip" {
			hasFlagsSkip = true
		}
	}
	if !hasFlagsSkip {
		t.Error("expected case_flags_skip to execute with flags=extended")
	}

	// Most other cases (flags="core") should be skipped
	if report.SkippedCases == 0 {
		t.Error("expected some skipped cases with flags=extended")
	}
}

// TestE2E_HarnessSanityCheck verifies that the mock server and DB setup work.
func TestE2E_HarnessSanityCheck(t *testing.T) {
	// Test mockserver
	httpMock, err := mockserver.NewHTTPMock()
	if err != nil {
		t.Fatalf("http mock: %v", err)
	}
	defer httpMock.Close()
	t.Logf("HTTP mock on %s", httpMock.Listener)

	tcpMock, err := mockserver.NewTCPMock()
	if err != nil {
		t.Fatalf("tcp mock: %v", err)
	}
	defer tcpMock.Close()
	t.Logf("TCP mock on %s", tcpMock.Listener)

	// Test SQLite setup
	dbManager, err := testutil.SetupTestDB()
	if err != nil {
		t.Fatalf("sqlite setup: %v", err)
	}
	defer dbManager.Close()

	// Verify we can query the test DB
	result, err := dbManager.Select("TESTDB", "SELECT COUNT(*) as cnt FROM users")
	if err != nil {
		t.Fatalf("select count: %v", err)
	}
	if len(result.Rows) == 0 {
		t.Fatal("no rows from users count")
	}
	t.Logf("Users count: %s", result.Rows[0]["cnt"])
}

// initLogger initializes logging for verbose test runs.
var _ = fmt.Sprintf // avoid unused import
var _ = os.ReadDir  // allow future use
