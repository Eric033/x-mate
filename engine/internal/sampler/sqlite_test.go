package sampler

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// setupSQLiteDB creates an in-memory SQLite database for testing.
func setupSQLiteDB(t *testing.T) *DBPoolManager {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	// Create test tables
	_, err = db.Exec(`CREATE TABLE test_users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		email TEXT NOT NULL
	)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	_, err = db.Exec(`INSERT INTO test_users (name, email) VALUES ('Alice', 'alice@test.com')`)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	_, err = db.Exec(`INSERT INTO test_users (name, email) VALUES ('Bob', 'bob@test.com')`)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	mgr := NewDBPoolManager()
	mgr.RegisterPool("TESTDB", db)
	return mgr
}

func TestSelect(t *testing.T) {
	mgr := setupSQLiteDB(t)
	defer mgr.Close()

	result, err := mgr.Select("TESTDB", "SELECT name, email FROM test_users ORDER BY id")
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}

	if len(result.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(result.Rows))
	}
	if len(result.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(result.Columns))
	}

	if result.Rows[0]["name"] != "Alice" {
		t.Errorf("row[0].name = %q, want Alice", result.Rows[0]["name"])
	}
	if result.Rows[0]["email"] != "alice@test.com" {
		t.Errorf("row[0].email = %q", result.Rows[0]["email"])
	}
	if result.Rows[1]["name"] != "Bob" {
		t.Errorf("row[1].name = %q, want Bob", result.Rows[1]["name"])
	}
}

func TestSelect_Filtered(t *testing.T) {
	mgr := setupSQLiteDB(t)
	defer mgr.Close()

	result, err := mgr.Select("TESTDB", "SELECT name FROM test_users WHERE email = ?", "bob@test.com")
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}

	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
	if result.Rows[0]["name"] != "Bob" {
		t.Errorf("name = %q, want Bob", result.Rows[0]["name"])
	}
}

func TestSelect_NoRows(t *testing.T) {
	mgr := setupSQLiteDB(t)
	defer mgr.Close()

	result, err := mgr.Select("TESTDB", "SELECT * FROM test_users WHERE name = 'Nonexistent'")
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}
	if len(result.Rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(result.Rows))
	}
	if len(result.Columns) == 0 {
		t.Error("columns should still be populated for empty result")
	}
}

func TestExec_Insert(t *testing.T) {
	mgr := setupSQLiteDB(t)
	defer mgr.Close()

	affected, err := mgr.Exec("TESTDB", "INSERT INTO test_users (name, email) VALUES ('Charlie', 'charlie@test.com')")
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	if affected != 1 {
		t.Errorf("rows affected = %d, want 1", affected)
	}

	// Verify
	result, _ := mgr.Select("TESTDB", "SELECT COUNT(*) as cnt FROM test_users")
	if result.Rows[0]["cnt"] != "3" {
		t.Errorf("count = %s, want 3", result.Rows[0]["cnt"])
	}
}

func TestExec_Update(t *testing.T) {
	mgr := setupSQLiteDB(t)
	defer mgr.Close()

	affected, err := mgr.Exec("TESTDB", "UPDATE test_users SET email = 'alice_new@test.com' WHERE name = 'Alice'")
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	if affected != 1 {
		t.Errorf("rows affected = %d, want 1", affected)
	}

	result, _ := mgr.Select("TESTDB", "SELECT email FROM test_users WHERE name = 'Alice'")
	if result.Rows[0]["email"] != "alice_new@test.com" {
		t.Errorf("email = %q", result.Rows[0]["email"])
	}
}

func TestExec_Delete(t *testing.T) {
	mgr := setupSQLiteDB(t)
	defer mgr.Close()

	affected, err := mgr.Exec("TESTDB", "DELETE FROM test_users WHERE name = 'Bob'")
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	if affected != 1 {
		t.Errorf("rows affected = %d, want 1", affected)
	}

	result, _ := mgr.Select("TESTDB", "SELECT COUNT(*) as cnt FROM test_users")
	if result.Rows[0]["cnt"] != "1" {
		t.Errorf("count = %s, want 1", result.Rows[0]["cnt"])
	}
}

func TestExec_InsertMulti(t *testing.T) {
	mgr := setupSQLiteDB(t)
	defer mgr.Close()

	affected, err := mgr.Exec("TESTDB", "INSERT INTO test_users (name, email) VALUES ('Multi1', 'm1@t.com'), ('Multi2', 'm2@t.com')")
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	if affected != 2 {
		t.Errorf("rows affected = %d, want 2", affected)
	}

	result, _ := mgr.Select("TESTDB", "SELECT COUNT(*) as cnt FROM test_users")
	if result.Rows[0]["cnt"] != "4" {
		t.Errorf("count = %s, want 4", result.Rows[0]["cnt"])
	}
}

func TestSelect_NoPool(t *testing.T) {
	mgr := NewDBPoolManager()
	_, err := mgr.Select("NONEXISTENT", "SELECT 1")
	if err == nil {
		t.Error("expected error for non-existent pool")
	}
}

func TestExec_NoPool(t *testing.T) {
	mgr := NewDBPoolManager()
	_, err := mgr.Exec("NONEXISTENT", "SELECT 1")
	if err == nil {
		t.Error("expected error for non-existent pool")
	}
}

func TestRegisterPool(t *testing.T) {
	mgr := NewDBPoolManager()
	db, _ := sql.Open("sqlite3", ":memory:")
	mgr.RegisterPool("CUSTOM", db)

	result, err := mgr.Select("CUSTOM", "SELECT 1 as val")
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}
	if result.Rows[0]["val"] != "1" {
		t.Errorf("val = %q", result.Rows[0]["val"])
	}
}

func TestNewDBPoolManager(t *testing.T) {
	mgr := NewDBPoolManager()
	if mgr == nil {
		t.Fatal("NewDBPoolManager returned nil")
	}
}
