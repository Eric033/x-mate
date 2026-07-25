package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Eric033/x-mate/engine/internal/context"
)

// writeTempYAML writes content to a temp file and returns its path.
func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "application.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write temp yaml: %v", err)
	}
	return path
}

func TestLoadYAML_NotFound(t *testing.T) {
	cfg := Default()
	err := cfg.LoadYAML()
	// Should error when no config path is provided
	if err == nil {
		t.Error("expected error for missing config path, got nil")
	}
	if err.Error() != "config: no config path provided" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLoadYAML_ExplicitPath_EnvironmentName(t *testing.T) {
	cfg := Default()
	cfg.ConfigPath = writeTempYAML(t, `
system-id: ABCDEF
environment:
  name: TEST_ENV
services:
  MOCK:
    address: "127.0.0.1:19876"
`)
	if err := cfg.LoadYAML(); err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	if cfg.EnvName != "TEST_ENV" {
		t.Errorf("EnvName = %q, want TEST_ENV", cfg.EnvName)
	}
	if cfg.SystemID != "ABCDEF" {
		t.Errorf("SystemID = %q, want ABCDEF", cfg.SystemID)
	}
}

func TestLoadYAML_InvalidSystemID(t *testing.T) {
	cfg := Default()
	cfg.ConfigPath = writeTempYAML(t, "system-id: ABC123\n")
	if err := cfg.LoadYAML(); err == nil {
		t.Fatal("expected invalid system-id error")
	}
}

func TestLoadYAML_WithServices(t *testing.T) {
	cfg := Default()
	cfg.ConfigPath = writeTempYAML(t, `
environment:
  name: QA
services:
  MOCK:
    address: "192.168.1.1:8080"
    tcp-port: 9090
    http-port: 8080
    db:
      address: "10.0.0.1:1521"
      database: ORCL
      username: testuser
      password: testpass
      type: oracle
  DAMPER:
    address: "127.0.0.1:9999"
`)
	if err := cfg.LoadYAML(); err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	if cfg.EnvName != "QA" {
		t.Errorf("EnvName = %q, want QA", cfg.EnvName)
	}
	if cfg.Services == nil {
		t.Fatal("Services is nil")
	}
	mock, ok := cfg.Services["MOCK"]
	if !ok {
		t.Fatal("MOCK service not found")
	}
	if mock.Address != "192.168.1.1:8080" {
		t.Errorf("MOCK address = %q", mock.Address)
	}
	if mock.TCPPort != 9090 {
		t.Errorf("MOCK TCPPort = %d", mock.TCPPort)
	}
	if mock.DB == nil {
		t.Fatal("MOCK DB is nil")
	}
	if mock.DB.Address != "10.0.0.1:1521" {
		t.Errorf("DB address = %q", mock.DB.Address)
	}
	if mock.DB.Database != "ORCL" {
		t.Errorf("DB database = %q", mock.DB.Database)
	}
}

func TestLoadYAML_RejectsEngineKeys(t *testing.T) {
	tests := []string{
		"flags",
		"concurrency",
		"dry-run",
		"verbose",
		"test-base",
		"parent-guid",
	}

	for _, key := range tests {
		t.Run(key, func(t *testing.T) {
			content := key + `: value
services:
  MOCK:
    address: "127.0.0.1:19876"
`
			cfg := Default()
			cfg.ConfigPath = writeTempYAML(t, content)
			err := cfg.LoadYAML()
			if err == nil {
				t.Errorf("expected error for forbidden key %q", key)
			}
		})
	}
}

func TestLoadYAML_RejectsNestedEngineKey(t *testing.T) {
	cfg := Default()
	cfg.ConfigPath = writeTempYAML(t, `
engine:
  flags: smoke
services:
  MOCK:
    address: "127.0.0.1:19876"
`)
	err := cfg.LoadYAML()
	if err == nil {
		t.Fatal("expected error for 'engine' key")
	}
}

func TestSplitHostPort(t *testing.T) {
	ip, port := splitHostPort("10.0.0.1:8080")
	if ip != "10.0.0.1" || port != "8080" {
		t.Errorf("got %q:%q, want 10.0.0.1:8080", ip, port)
	}

	ip2, port2 := splitHostPort("no-port")
	if ip2 != "no-port" || port2 != "" {
		t.Errorf("got %q:%q, want no-port:", ip2, port2)
	}
}

func TestInitContext_MultiService(t *testing.T) {
	cfg := Default()
	cfg.SystemID = "ABCDEF"
	cfg.Services = map[string]context.ServiceDef{
		"ABC": {
			Address:  "10.0.0.1:9996",
			HTTPPort: 9996,
		},
		"XYZ": {
			Address: "10.0.0.2:8080",
			DB: &context.DBConf{
				Address:  "10.0.0.3:1521",
				Database: "TESTDB",
				Username: "user",
				Password: "pass",
			},
		},
	}

	ctx := context.New()
	cfg.InitContext(ctx)

	if ctx.SystemID != "ABCDEF" {
		t.Errorf("ctx.SystemID = %q, want ABCDEF", ctx.SystemID)
	}
	if v, ok := ctx.Get("systemID"); !ok || v != "ABCDEF" {
		t.Errorf("systemID variable = %q, want ABCDEF", v)
	}

	if v, ok := ctx.Get("ABC.address"); !ok || v != "10.0.0.1:9996" {
		t.Errorf("ABC.address = %q", v)
	}
	if v, ok := ctx.Get("ABC.http-port"); !ok || v != "9996" {
		t.Errorf("ABC.http-port = %q", v)
	}
	if v, ok := ctx.Get("XYZ.db.address"); !ok || v != "10.0.0.3:1521" {
		t.Errorf("XYZ.db.address = %q", v)
	}
	if v, ok := ctx.Get("XYZ.db.database"); !ok || v != "TESTDB" {
		t.Errorf("XYZ.db.database = %q", v)
	}

	// The lexicographically first service should deterministically set the
	// backward-compatible vars, regardless of map iteration order.
	for i := 0; i < 100; i++ {
		iterationCtx := context.New()
		cfg.InitContext(iterationCtx)
		if v, ok := iterationCtx.Get("serverIP"); !ok || v != "10.0.0.1" {
			t.Fatalf("iteration %d: serverIP = %q, want 10.0.0.1", i, v)
		}
		if v, ok := iterationCtx.Get("serverPort"); !ok || v != "9996" {
			t.Fatalf("iteration %d: serverPort = %q, want 9996", i, v)
		}
	}
}

func TestInitContext_EnvName(t *testing.T) {
	cfg := Default()
	cfg.EnvName = "PRODUCTION"
	ctx := context.New()
	cfg.InitContext(ctx)

	if ctx.EnvName != "PRODUCTION" {
		t.Errorf("EnvName = %q, want PRODUCTION", ctx.EnvName)
	}
	if v, ok := ctx.Get("envName"); !ok || v != "PRODUCTION" {
		t.Errorf("envName var = %q", v)
	}
}
