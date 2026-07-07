package config

import (
	"os"
	"path/filepath"
	"testing"
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
	// Should not error when no config file exists
	if err != nil {
		t.Errorf("expected no error for missing config, got: %v", err)
	}
}

func TestLoadYAML_ExplicitPath(t *testing.T) {
	cfg := Default()
	cfg.ConfigPath = writeTempYAML(t, `
engine:
  flags: smoke
  verbose: true
  env-name: TEST_ENV
  concurrency: 3
`)
	if err := cfg.LoadYAML(); err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	if cfg.Flags != "smoke" {
		t.Errorf("Flags = %q, want smoke", cfg.Flags)
	}
	if !cfg.Verbose {
		t.Error("Verbose = false, want true")
	}
	if cfg.EnvName != "TEST_ENV" {
		t.Errorf("EnvName = %q, want TEST_ENV", cfg.EnvName)
	}
	if cfg.Concurrency != 3 {
		t.Errorf("Concurrency = %d, want 3", cfg.Concurrency)
	}
}

func TestLoadYAML_WithServices(t *testing.T) {
	cfg := Default()
	cfg.ConfigPath = writeTempYAML(t, `
engine:
  test-base: /tmp/tests
  flags: core
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
	if cfg.TestBase != "/tmp/tests" {
		t.Errorf("TestBase = %q", cfg.TestBase)
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

func TestLoadYAML_WithProfile(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "application.yaml")
	os.WriteFile(basePath, []byte(`
engine:
  flags: base
  concurrency: 1
services:
  MOCK:
    address: "10.0.0.1:80"
`), 0644)
	profilePath := filepath.Join(dir, "application-default.yaml")
	os.WriteFile(profilePath, []byte(`
engine:
  flags: profile_override
  concurrency: 5
services:
  MOCK:
    address: "10.0.0.2:8080"
`), 0644)

	cfg := Default()
	cfg.ConfigPath = basePath
	if err := cfg.LoadYAML(); err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}
	// Profile should override base
	if cfg.Flags != "profile_override" {
		t.Errorf("Flags = %q, want profile_override", cfg.Flags)
	}
	if cfg.Concurrency != 5 {
		t.Errorf("Concurrency = %d, want 5", cfg.Concurrency)
	}
	mock := cfg.Services["MOCK"]
	if mock.Address != "10.0.0.2:8080" {
		t.Errorf("MOCK address = %q, want 10.0.0.2:8080", mock.Address)
	}
}

func TestParseServer(t *testing.T) {
	tests := []struct {
		input string
		want  int
		ip1   string
		port1 string
	}{
		{"10.0.0.1:9996", 1, "10.0.0.1", "9996"},
		{"ip1:port1@ip2:port2", 2, "ip1", "port1"},
		{"nodots:noport", 1, "nodots", "noport"},
	}
	for _, tt := range tests {
		got := ParseServer(tt.input)
		if len(got) != tt.want {
			t.Errorf("ParseServer(%q) len = %d, want %d", tt.input, len(got), tt.want)
		}
		if len(got) > 0 {
			if got[0].IP != tt.ip1 {
				t.Errorf("ParseServer(%q) ip1 = %q, want %q", tt.input, got[0].IP, tt.ip1)
			}
			if got[0].Port != tt.port1 {
				t.Errorf("ParseServer(%q) port1 = %q, want %q", tt.input, got[0].Port, tt.port1)
			}
		}
	}
}

func TestParseDamper(t *testing.T) {
	tcp, http := ParseDamper("10.0.0.1:9996:12:45")
	if tcp != "10.0.0.1:12" {
		t.Errorf("tcp addr = %q, want 10.0.0.1:12", tcp)
	}
	if http != "10.0.0.1:45" {
		t.Errorf("http addr = %q, want 10.0.0.1:45", http)
	}

	// Short format
	tcp2, http2 := ParseDamper("1.2.3.4:100")
	if tcp2 != "1.2.3.4:100" {
		t.Errorf("short tcp = %q", tcp2)
	}
	if http2 != "1.2.3.4:100" {
		t.Errorf("short http = %q", http2)
	}
}

func TestParseDBInfo(t *testing.T) {
	dbs := ParseDBInfo("10.0.0.1:1521:ORCL:scott:tiger")
	if len(dbs) != 1 {
		t.Fatalf("len = %d", len(dbs))
	}
	if dbs[0].IP != "10.0.0.1" {
		t.Errorf("IP = %q", dbs[0].IP)
	}
	if dbs[0].Name != "ORCL" {
		t.Errorf("Name = %q", dbs[0].Name)
	}
	if dbs[0].User != "scott" {
		t.Errorf("User = %q", dbs[0].User)
	}

	// Multi-db
	dbs2 := ParseDBInfo("ip1:port1:db1:u1:p1@ip2:port2:db2:u2:p2")
	if len(dbs2) != 2 {
		t.Fatalf("multi len = %d", len(dbs2))
	}
	if dbs2[0].IP != "ip1" || dbs2[1].IP != "ip2" {
		t.Errorf("multi ips: %q, %q", dbs2[0].IP, dbs2[1].IP)
	}

	// Empty
	dbs3 := ParseDBInfo("")
	if len(dbs3) != 0 {
		t.Errorf("empty len = %d", len(dbs3))
	}
	dbs4 := ParseDBInfo("UNDEFINED")
	if len(dbs4) != 0 {
		t.Errorf("UNDEFINED len = %d", len(dbs4))
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

	// First service should set backward-compat vars
	if v, ok := ctx.Get("serverIP"); !ok || v != "10.0.0.1" {
		t.Errorf("serverIP = %q", v)
	}
}
