package sql

import (
	"database/sql"
	"testing"

	"github.com/Eric033/x-mate/engine/internal/context"
	"github.com/Eric033/x-mate/engine/internal/handler"
	"github.com/Eric033/x-mate/engine/internal/sampler"

	_ "github.com/mattn/go-sqlite3"
)

// setupTestDB creates a SQLite in-memory test database.
func setupTestDB(t *testing.T) *sampler.DBPoolManager {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	_, err = db.Exec(`
		CREATE TABLE test_users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT NOT NULL,
			status TEXT DEFAULT 'ACTIVE'
		)
	`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	_, err = db.Exec(`INSERT INTO test_users (name, email, status) VALUES ('Alice', 'alice@t.com', 'ACTIVE')`)
	_, err = db.Exec(`INSERT INTO test_users (name, email, status) VALUES ('Bob', 'bob@t.com', 'ACTIVE')`)
	_, err = db.Exec(`INSERT INTO test_users (name, email, status) VALUES ('Charlie', 'c@t.com', 'INACTIVE')`)
	if err != nil {
		t.Fatalf("seed data: %v", err)
	}

	mgr := sampler.NewDBPoolManager()
	mgr.RegisterPool("TESTDB", db)
	return mgr
}

func TestSelectHandler_Execute(t *testing.T) {
	mgr := setupTestDB(t)
	defer mgr.Close()

	h := &SelectHandler{PoolManager: mgr}
	ctx := context.New()

	// Use a simple step that selects all active users
	data := &handler.StepData{
		StepType: "sql_select",
		Server:   "TESTDB",
		Attrs: map[string]string{
			"_action_text": "SELECT name, email FROM test_users WHERE status = 'ACTIVE' ORDER BY id",
		},
		Saves: []handler.SaveEntry{
			{Name: "first_user", Locator: "name[0]"},
		},
	}

	result := h.Execute(data, ctx)
	if !result.Success {
		t.Fatalf("Execute failed: %s", result.FailureMessage)
	}

	// Verify extracted var
	if v, ok := ctx.Get("first_user"); !ok || v != "Alice" {
		t.Errorf("first_user = %q, want Alice", v)
	}
}

func TestSelectHandler_VerifyResults(t *testing.T) {
	mgr := setupTestDB(t)
	defer mgr.Close()

	h := &SelectHandler{PoolManager: mgr}
	ctx := context.New()

	data := &handler.StepData{
		StepType: "sql_select",
		Server:   "TESTDB",
		Attrs: map[string]string{
			"_action_text": "SELECT name, email, status FROM test_users WHERE status = 'ACTIVE' ORDER BY id",
		},
		Assertions: []handler.Assertion{
			{XPath: "name[0]", Expected: "Alice"},
			{XPath: "name[1]", Expected: "Bob"},
			{XPath: "email[0]", Expected: "alice@t.com"},
		},
	}

	result := h.Execute(data, ctx)
	if !result.Success {
		t.Fatalf("Execute with verify failed: %s", result.FailureMessage)
	}
}

func TestSelectHandler_VerifyFailure(t *testing.T) {
	mgr := setupTestDB(t)
	defer mgr.Close()

	h := &SelectHandler{PoolManager: mgr}
	ctx := context.New()

	data := &handler.StepData{
		StepType: "sql_select",
		Server:   "TESTDB",
		Attrs: map[string]string{
			"_action_text": "SELECT name, email, status FROM test_users WHERE status = 'ACTIVE' ORDER BY id",
		},
		Assertions: []handler.Assertion{
			{XPath: "name[0]", Expected: "WRONG_NAME"},
		},
	}

	result := h.Execute(data, ctx)
	if result.Success {
		t.Fatal("expected failure for mismatched verify, got success")
	}
}

func TestSelectHandler_NoRows(t *testing.T) {
	mgr := setupTestDB(t)
	defer mgr.Close()

	h := &SelectHandler{PoolManager: mgr}
	ctx := context.New()

	data := &handler.StepData{
		StepType: "sql_select",
		Server:   "TESTDB",
		Attrs: map[string]string{
			"_action_text": "SELECT * FROM test_users WHERE name = 'NONEXISTENT'",
		},
		Saves: []handler.SaveEntry{
			{Name: "result", Locator: "name[0]"},
		},
	}

	result := h.Execute(data, ctx)
	if !result.Success {
		t.Fatalf("Execute failed: %s", result.FailureMessage)
	}

	// resultVariable should be set
	if v, ok := ctx.Get("resultVariable"); !ok || v != "0 rows" {
		t.Errorf("resultVariable = %q, want '0 rows'", v)
	}
}

func TestUpdateHandler_Insert(t *testing.T) {
	mgr := setupTestDB(t)
	defer mgr.Close()

	h := &UpdateHandler{PoolManager: mgr}
	ctx := context.New()

	data := &handler.StepData{
		StepType: "sql_update",
		Server:   "TESTDB",
		Attrs: map[string]string{
			"_action_text": "INSERT INTO test_users (name, email) VALUES ('Dave', 'dave@t.com')",
		},
	}

	result := h.Execute(data, ctx)
	if !result.Success {
		t.Fatalf("Execute insert failed: %s", result.FailureMessage)
	}

	if v, ok := ctx.Get("sqlActualResult_1"); !ok || v != "1" {
		t.Errorf("sqlActualResult_1 = %q, want 1", v)
	}
}

func TestUpdateHandler_Update(t *testing.T) {
	mgr := setupTestDB(t)
	defer mgr.Close()

	h := &UpdateHandler{PoolManager: mgr}
	ctx := context.New()

	data := &handler.StepData{
		StepType: "sql_update",
		Server:   "TESTDB",
		Attrs: map[string]string{
			"_action_text": "UPDATE test_users SET status = 'INACTIVE' WHERE name = 'Alice'",
		},
	}

	result := h.Execute(data, ctx)
	if !result.Success {
		t.Fatalf("Execute update failed: %s", result.FailureMessage)
	}
}

func TestUpdateHandler_Verify(t *testing.T) {
	mgr := setupTestDB(t)
	defer mgr.Close()

	h := &UpdateHandler{PoolManager: mgr}
	ctx := context.New()

	data := &handler.StepData{
		StepType: "sql_update",
		Server:   "TESTDB",
		Attrs: map[string]string{
			"_action_text": "UPDATE test_users SET status = 'INACTIVE' WHERE name = 'Alice'",
		},
		Assertions: []handler.Assertion{{Expected: "1"}},
	}

	result := h.Execute(data, ctx)
	if !result.Success {
		t.Fatalf("Execute with verify failed: %s", result.FailureMessage)
	}
}

func TestUpdateHandler_VerifyFailure(t *testing.T) {
	mgr := setupTestDB(t)
	defer mgr.Close()

	h := &UpdateHandler{PoolManager: mgr}
	ctx := context.New()

	data := &handler.StepData{
		StepType: "sql_update",
		Server:   "TESTDB",
		Attrs: map[string]string{
			"_action_text": "UPDATE test_users SET status = 'INACTIVE' WHERE name = 'Alice'",
		},
		Assertions: []handler.Assertion{{Expected: "999"}},
	}

	result := h.Execute(data, ctx)
	if result.Success {
		t.Fatal("expected failure for mismatched verify, got success")
	}
}

func TestSelectHandler_SaveExtract(t *testing.T) {
	mgr := setupTestDB(t)
	defer mgr.Close()

	h := &SelectHandler{PoolManager: mgr}
	ctx := context.New()

	data := &handler.StepData{
		StepType: "sql_select",
		Server:   "TESTDB",
		Attrs: map[string]string{
			"_action_text": "SELECT name, email, status FROM test_users ORDER BY id",
		},
		Saves: []handler.SaveEntry{
			{Name: "user1_name", Locator: "name[0]"},
			{Name: "user1_email", Locator: "email[0]"},
			{Name: "user2_name", Locator: "name[1]"},
			{Name: "user2_status", Locator: "status[1]"},
		},
	}

	result := h.Execute(data, ctx)
	if !result.Success {
		t.Fatalf("Execute failed: %s", result.FailureMessage)
	}

	checkVar(t, ctx, "user1_name", "Alice")
	checkVar(t, ctx, "user1_email", "alice@t.com")
	checkVar(t, ctx, "user2_name", "Bob")
	checkVar(t, ctx, "user2_status", "ACTIVE")
}

func TestSelectHandler_EmptySQL(t *testing.T) {
	mgr := setupTestDB(t)
	defer mgr.Close()

	h := &SelectHandler{PoolManager: mgr}
	ctx := context.New()

	data := &handler.StepData{
		StepType: "sql_select",
		Server:   "TESTDB",
	}

	result := h.Execute(data, ctx)
	if result.Success {
		t.Fatal("expected failure for empty SQL")
	}
}

func TestSelectHandler_SQLTemplateVar(t *testing.T) {
	mgr := setupTestDB(t)
	defer mgr.Close()

	h := &SelectHandler{PoolManager: mgr}
	ctx := context.New()
	ctx.Set("target_name", "Bob")

	data := &handler.StepData{
		StepType: "sql_select",
		Server:   "TESTDB",
		Attrs: map[string]string{
			"_action_text": "SELECT email FROM test_users WHERE name = '{{target_name}}'",
		},
		Assertions: []handler.Assertion{
			{XPath: "email[0]", Expected: "bob@t.com"},
		},
	}

	result := h.Execute(data, ctx)
	if !result.Success {
		t.Fatalf("Execute with template var failed: %s", result.FailureMessage)
	}
}

func checkVar(t *testing.T, ctx *context.TestContext, key, expected string) {
	t.Helper()
	v, ok := ctx.Get(key)
	if !ok {
		t.Errorf("variable %q not set", key)
		return
	}
	if v != expected {
		t.Errorf("variable %q = %q, want %q", key, v, expected)
	}
}
