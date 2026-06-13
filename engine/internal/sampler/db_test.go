package sampler

import (
	"testing"
)

func TestNewDBPoolManager(t *testing.T) {
	m := NewDBPoolManager()
	if m == nil {
		t.Fatal("NewDBPoolManager returned nil")
	}
	if len(m.pools) != 0 {
		t.Errorf("expected 0 pools, got %d", len(m.pools))
	}
}

func TestDBPoolManager_Get_NotFound(t *testing.T) {
	m := NewDBPoolManager()
	_, err := m.Get("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent pool")
	}
}

func TestDBPoolManager_Close(t *testing.T) {
	m := NewDBPoolManager()
	m.Close() // should not panic
}

func TestBuildDSN_MySQL(t *testing.T) {
	cfg := DBConfig{
		IP:     "127.0.0.1",
		Port:   "3306",
		Name:   "testdb",
		User:   "root",
		Passwd: "secret",
	}
	dsn := buildDSN(cfg, "mysql")
	want := "root:secret@tcp(127.0.0.1:3306)/testdb"
	if dsn != want {
		t.Errorf("mysql DSN = %q, want %q", dsn, want)
	}
}

func TestBuildDSN_Oracle(t *testing.T) {
	cfg := DBConfig{
		IP:     "10.0.0.1",
		Port:   "1521",
		Name:   "ORCL",
		User:   "scott",
		Passwd: "tiger",
	}
	dsn := buildDSN(cfg, "oracle")
	want := "oracle://scott:tiger@10.0.0.1:1521/ORCL"
	if dsn != want {
		t.Errorf("oracle DSN = %q, want %q", dsn, want)
	}
}

func TestBuildDSN_Default(t *testing.T) {
	cfg := DBConfig{
		IP:     "host",
		Port:   "1234",
		Name:   "db",
		User:   "user",
		Passwd: "pass",
	}
	dsn := buildDSN(cfg, "unknown")
	want := "user:pass@host:1234/db"
	if dsn != want {
		t.Errorf("default DSN = %q, want %q", dsn, want)
	}
}
