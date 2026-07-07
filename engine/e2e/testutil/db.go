// Package testutil provides helpers for Engine E2E testing.
package testutil

import (
	"database/sql"
	"fmt"

	"github.com/Eric033/x-mate/engine/internal/sampler"

	// Register SQLite driver
	_ "github.com/mattn/go-sqlite3"
)

// SetupTestDB creates a SQLite in-memory database, initializes tables,
// and returns a DBPoolManager ready for use in E2E tests.
//
// The returned pool manager has a single pool named "TESTDB".
// Test case XML should use server="TESTDB" for SQL steps.
func SetupTestDB() (*sampler.DBPoolManager, error) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Enable WAL mode and foreign keys for realistic testing
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := db.Exec(pragma); err != nil {
			return nil, fmt.Errorf("pragma: %w", err)
		}
	}

	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("create tables: %w", err)
	}

	mgr := sampler.NewDBPoolManager()
	mgr.RegisterPool("TESTDB", db)

	return mgr, nil
}

// createTables creates the test schema and inserts seed data.
func createTables(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'ACTIVE'
		)`,
		`CREATE TABLE IF NOT EXISTS orders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			amount TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'PENDING'
		)`,
		`INSERT INTO users (name, email, status) VALUES ('Alice', 'alice@test.com', 'ACTIVE')`,
		`INSERT INTO users (name, email, status) VALUES ('Bob', 'bob@test.com', 'ACTIVE')`,
		`INSERT INTO users (name, email, status) VALUES ('Charlie', 'charlie@test.com', 'INACTIVE')`,
		`INSERT INTO orders (user_id, amount, status) VALUES (1, '10000', 'PENDING')`,
		`INSERT INTO orders (user_id, amount, status) VALUES (1, '20000', 'COMPLETED')`,
		`INSERT INTO orders (user_id, amount, status) VALUES (2, '15000', 'PENDING')`,
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("sql: %s: %w", stmt[:40], err)
		}
	}

	return nil
}
