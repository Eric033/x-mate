package config

import (
	"testing"

	"github.com/Eric033/x-mate/engine/internal/context"
)

// ---------------------------------------------------------------------------
// Default
// ---------------------------------------------------------------------------

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg == nil {
		t.Fatal("Default() returned nil")
	}
	if cfg.Flags != "core" {
		t.Fatalf("Flags: expected 'core', got %q", cfg.Flags)
	}
	if cfg.EnvName != "UNDEFINED" {
		t.Fatalf("EnvName: expected 'UNDEFINED', got %q", cfg.EnvName)
	}
	if cfg.ParentGUID != "92508788-4c1c-11e9-808b-005056a01111" {
		t.Fatalf("ParentGUID mismatch: got %q", cfg.ParentGUID)
	}
	if cfg.TestBase != "" {
		t.Fatalf("TestBase: expected empty, got %q", cfg.TestBase)
	}
	if cfg.Server != "" {
		t.Fatalf("Server: expected empty, got %q", cfg.Server)
	}
	if cfg.DBInfo != "" {
		t.Fatalf("DBInfo: expected empty, got %q", cfg.DBInfo)
	}
	if cfg.DamperServer != "" {
		t.Fatalf("DamperServer: expected empty, got %q", cfg.DamperServer)
	}
	if cfg.Verbose != false {
		t.Fatal("Verbose: expected false")
	}
	if cfg.DryRun != false {
		t.Fatal("DryRun: expected false")
	}
}

// ---------------------------------------------------------------------------
// ParseServer
// ---------------------------------------------------------------------------

func TestParseServer_Single(t *testing.T) {
	servers := ParseServer("10.0.0.1:8080")
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	if servers[0].IP != "10.0.0.1" {
		t.Fatalf("IP: expected '10.0.0.1', got %q", servers[0].IP)
	}
	if servers[0].Port != "8080" {
		t.Fatalf("Port: expected '8080', got %q", servers[0].Port)
	}
}

func TestParseServer_Multi(t *testing.T) {
	servers := ParseServer("1.2.3.4:1111@5.6.7.8:2222")
	if len(servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(servers))
	}
	if servers[0].IP != "1.2.3.4" || servers[0].Port != "1111" {
		t.Fatalf("server1: expected 1.2.3.4:1111, got %s:%s", servers[0].IP, servers[0].Port)
	}
	if servers[1].IP != "5.6.7.8" || servers[1].Port != "2222" {
		t.Fatalf("server2: expected 5.6.7.8:2222, got %s:%s", servers[1].IP, servers[1].Port)
	}
}

func TestParseServer_NoPort(t *testing.T) {
	servers := ParseServer("10.0.0.1")
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	if servers[0].IP != "10.0.0.1" {
		t.Fatalf("IP: expected '10.0.0.1', got %q", servers[0].IP)
	}
	if servers[0].Port != "" {
		t.Fatalf("Port: expected empty, got %q", servers[0].Port)
	}
}

func TestParseServer_IPv6WithPort(t *testing.T) {
	// Note: Uses LastIndex of ":", so IPv6 will be tricky
	servers := ParseServer("::1:8080")
	// LastIndex splits at the last ":"
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	// IP = "::1", Port = "8080" — this is the intended behavior
	if servers[0].Port != "8080" {
		t.Fatalf("Port: expected '8080', got %q", servers[0].Port)
	}
}

func TestParseServer_EmptyString(t *testing.T) {
	servers := ParseServer("")
	if len(servers) != 1 {
		t.Fatalf("expected 1 server (empty string), got %d", len(servers))
	}
	if servers[0].IP != "" {
		t.Fatalf("IP: expected empty, got %q", servers[0].IP)
	}
}

func TestParseServer_MultiWithEmptyPort(t *testing.T) {
	servers := ParseServer("host1@host2:8080")
	if len(servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(servers))
	}
	if servers[0].IP != "host1" || servers[0].Port != "" {
		t.Fatalf("server1: expected host1 with empty port, got %s:%s", servers[0].IP, servers[0].Port)
	}
	if servers[1].IP != "host2" || servers[1].Port != "8080" {
		t.Fatalf("server2: expected host2:8080, got %s:%s", servers[1].IP, servers[1].Port)
	}
}

// ---------------------------------------------------------------------------
// ParseDamper
// ---------------------------------------------------------------------------

func TestParseDamper_Full(t *testing.T) {
	tcpAddr, httpAddr := ParseDamper("10.0.0.1:1234:5678:9012")
	if tcpAddr != "10.0.0.1:5678" {
		t.Fatalf("tcpAddr: expected '10.0.0.1:5678', got %q", tcpAddr)
	}
	if httpAddr != "10.0.0.1:9012" {
		t.Fatalf("httpAddr: expected '10.0.0.1:9012', got %q", httpAddr)
	}
}

func TestParseDamper_Partial(t *testing.T) {
	tcpAddr, httpAddr := ParseDamper("10.0.0.1:8080")
	if tcpAddr != "10.0.0.1:8080" {
		t.Fatalf("tcpAddr: expected '10.0.0.1:8080', got %q", tcpAddr)
	}
	if httpAddr != "10.0.0.1:8080" {
		t.Fatalf("httpAddr: expected '10.0.0.1:8080', got %q", httpAddr)
	}
}

func TestParseDamper_SinglePart(t *testing.T) {
	tcpAddr, httpAddr := ParseDamper("justhost")
	if tcpAddr != "" {
		t.Fatalf("tcpAddr: expected empty, got %q", tcpAddr)
	}
	if httpAddr != "" {
		t.Fatalf("httpAddr: expected empty, got %q", httpAddr)
	}
}

func TestParseDamper_Empty(t *testing.T) {
	tcpAddr, httpAddr := ParseDamper("")
	if tcpAddr != "" {
		t.Fatalf("tcpAddr: expected empty, got %q", tcpAddr)
	}
	if httpAddr != "" {
		t.Fatalf("httpAddr: expected empty, got %q", httpAddr)
	}
}

func TestParseDamper_ThreeParts(t *testing.T) {
	// ip:port:tcpPort — only 3 parts, falls to len >= 2 branch
	// parts[0]=host, parts[1]=1000 → tcpAddr=httpAddr="host:1000"
	tcpAddr, httpAddr := ParseDamper("host:1000:2000")
	if tcpAddr != "host:1000" {
		t.Fatalf("tcpAddr: expected 'host:1000', got %q", tcpAddr)
	}
	if httpAddr != "host:1000" {
		t.Fatalf("httpAddr: expected 'host:1000', got %q", httpAddr)
	}
}

// ---------------------------------------------------------------------------
// ParseDBInfo
// ---------------------------------------------------------------------------

func TestParseDBInfo_Single(t *testing.T) {
	dbs := ParseDBInfo("10.0.0.1:3306:mydb:admin:secret")
	if len(dbs) != 1 {
		t.Fatalf("expected 1 db, got %d", len(dbs))
	}
	db := dbs[0]
	if db.IP != "10.0.0.1" {
		t.Fatalf("IP: expected '10.0.0.1', got %q", db.IP)
	}
	if db.Port != "3306" {
		t.Fatalf("Port: expected '3306', got %q", db.Port)
	}
	if db.Name != "mydb" {
		t.Fatalf("Name: expected 'mydb', got %q", db.Name)
	}
	if db.User != "admin" {
		t.Fatalf("User: expected 'admin', got %q", db.User)
	}
	if db.Passwd != "secret" {
		t.Fatalf("Passwd: expected 'secret', got %q", db.Passwd)
	}
	if db.Type != "UNDEFINED" {
		t.Fatalf("Type: expected 'UNDEFINED', got %q", db.Type)
	}
}

func TestParseDBInfo_Multi(t *testing.T) {
	dbs := ParseDBInfo("10.0.0.1:3306:db1:user1:pwd1@10.0.0.2:3307:db2:user2:pwd2")
	if len(dbs) != 2 {
		t.Fatalf("expected 2 dbs, got %d", len(dbs))
	}

	db0 := dbs[0]
	if db0.IP != "10.0.0.1" || db0.Port != "3306" || db0.Name != "db1" || db0.User != "user1" || db0.Passwd != "pwd1" {
		t.Fatalf("db0 mismatch: got %+v", db0)
	}

	db1 := dbs[1]
	if db1.IP != "10.0.0.2" || db1.Port != "3307" || db1.Name != "db2" || db1.User != "user2" || db1.Passwd != "pwd2" {
		t.Fatalf("db1 mismatch: got %+v", db1)
	}
}

func TestParseDBInfo_PartialFields(t *testing.T) {
	dbs := ParseDBInfo("10.0.0.1:3306")
	if len(dbs) != 1 {
		t.Fatalf("expected 1 db, got %d", len(dbs))
	}
	db := dbs[0]
	if db.IP != "10.0.0.1" || db.Port != "3306" || db.Name != "" {
		t.Fatalf("expected IP+Port only, got %+v", db)
	}
	if db.User != "readonly" {
		t.Fatalf("User: expected default 'readonly', got %q", db.User)
	}
	if db.Passwd != "readonly" {
		t.Fatalf("Passwd: expected default 'readonly', got %q", db.Passwd)
	}
}

func TestParseDBInfo_Empty(t *testing.T) {
	dbs := ParseDBInfo("")
	if len(dbs) != 0 {
		t.Fatalf("expected 0 dbs for empty string, got %d", len(dbs))
	}
}

func TestParseDBInfo_Undefined(t *testing.T) {
	dbs := ParseDBInfo("UNDEFINED")
	if len(dbs) != 0 {
		t.Fatalf("expected 0 dbs for 'UNDEFINED', got %d", len(dbs))
	}
}

func TestParseDBInfo_SingleField(t *testing.T) {
	dbs := ParseDBInfo("10.0.0.1")
	if len(dbs) != 1 {
		t.Fatalf("expected 1 db, got %d", len(dbs))
	}
	db := dbs[0]
	if db.IP != "10.0.0.1" || db.Port != "" {
		t.Fatalf("expected IP only, got %+v", db)
	}
}

// ---------------------------------------------------------------------------
// InitContext
// ---------------------------------------------------------------------------

func TestInitContext_Full(t *testing.T) {
	cfg := &Config{
		TestBase:     "/tmp/test",
		Server:       "10.0.0.1:8080@10.0.0.2:9090",
		Flags:        "core,extended",
		Verbose:      true,
		DryRun:       false,
		DBInfo:       "1.2.3.4:3306:mydb:admin:pass",
		DamperServer: "192.168.1.1:1000:2000:3000",
		EnvName:      "prod",
		ParentGUID:   "abc-123",
	}

	ctx := context.New()
	cfg.InitContext(ctx)

	// Basic fields
	if ctx.TestBase != "/tmp/test" {
		t.Fatalf("TestBase: expected '/tmp/test', got %q", ctx.TestBase)
	}
	if ctx.Flags != "core,extended" {
		t.Fatalf("Flags: expected 'core,extended', got %q", ctx.Flags)
	}
	if ctx.Verbose != true {
		t.Fatal("Verbose: expected true")
	}
	if ctx.DryRun != false {
		t.Fatal("DryRun: expected false")
	}
	if ctx.EnvName != "prod" {
		t.Fatalf("EnvName: expected 'prod', got %q", ctx.EnvName)
	}
	if ctx.ParentGUID != "abc-123" {
		t.Fatalf("ParentGUID: expected 'abc-123', got %q", ctx.ParentGUID)
	}

	// Servers
	if len(ctx.Servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(ctx.Servers))
	}
	if ctx.Servers[0].IP != "10.0.0.1" || ctx.Servers[0].Port != "8080" {
		t.Fatalf("server0: expected 10.0.0.1:8080, got %s:%s", ctx.Servers[0].IP, ctx.Servers[0].Port)
	}
	if ctx.Servers[1].IP != "10.0.0.2" || ctx.Servers[1].Port != "9090" {
		t.Fatalf("server1: expected 10.0.0.2:9090, got %s:%s", ctx.Servers[1].IP, ctx.Servers[1].Port)
	}

	// Damper
	if ctx.DamperTCP != "192.168.1.1:2000" {
		t.Fatalf("DamperTCP: expected '192.168.1.1:2000', got %q", ctx.DamperTCP)
	}
	if ctx.DamperHTTP != "192.168.1.1:3000" {
		t.Fatalf("DamperHTTP: expected '192.168.1.1:3000', got %q", ctx.DamperHTTP)
	}

	// DBPools
	if len(ctx.DBPools) != 1 {
		t.Fatalf("expected 1 dbpool, got %d", len(ctx.DBPools))
	}
	if ctx.DBPools[0].IP != "1.2.3.4" || ctx.DBPools[0].Port != "3306" || ctx.DBPools[0].Name != "mydb" {
		t.Fatalf("dbpool0 mismatch: got %+v", ctx.DBPools[0])
	}

	// Server variables
	checkCtxVar(t, ctx, "server_1", "10.0.0.1")
	checkCtxVar(t, ctx, "port_1", "8080")
	checkCtxVar(t, ctx, "server_2", "10.0.0.2")
	checkCtxVar(t, ctx, "port_2", "9090")

	// Damper variables
	checkCtxVar(t, ctx, "tcpDamServerIP", "192.168.1.1")
	checkCtxVar(t, ctx, "tcpDamServerPort", "2000")
	checkCtxVar(t, ctx, "httpDamServerIP", "192.168.1.1")
	checkCtxVar(t, ctx, "httpDamServerPort", "3000")

	// DB variables
	checkCtxVar(t, ctx, "dbips_1", "1.2.3.4")
	checkCtxVar(t, ctx, "dbports_1", "3306")
	checkCtxVar(t, ctx, "dbnames_1", "mydb")

	// Raw config vars
	checkCtxVar(t, ctx, "testBase", "/tmp/test")
	checkCtxVar(t, ctx, "flags", "core,extended")
	checkCtxVar(t, ctx, "G_server", "10.0.0.1:8080@10.0.0.2:9090")
	checkCtxVar(t, ctx, "G_DamplerServer", "192.168.1.1:1000:2000:3000")
	checkCtxVar(t, ctx, "parent_guid", "abc-123")
	checkCtxVar(t, ctx, "DB_info", "1.2.3.4:3306:mydb:admin:pass")
	checkCtxVar(t, ctx, "envName", "prod")

	// First DB shorthands
	checkCtxVar(t, ctx, "DB_IP", "1.2.3.4")
	checkCtxVar(t, ctx, "DB_port", "3306")
	checkCtxVar(t, ctx, "DB_name", "mydb")
	checkCtxVar(t, ctx, "DB_user", "admin")
	checkCtxVar(t, ctx, "DB_passwd", "pass")
	checkCtxVar(t, ctx, "DB_type", "UNDEFINED")
}

func TestInitContext_EmptyServer(t *testing.T) {
	cfg := &Config{}
	ctx := context.New()
	cfg.InitContext(ctx)

	// Empty server should produce 0 entries
	if len(ctx.Servers) != 0 {
		t.Fatalf("expected 0 server entries from empty string, got %d", len(ctx.Servers))
	}
}

func TestInitContext_NoDB(t *testing.T) {
	cfg := &Config{
		TestBase: "/tmp",
		Flags:    "core",
	}
	ctx := context.New()
	cfg.InitContext(ctx)

	if len(ctx.DBPools) != 0 {
		t.Fatalf("expected 0 dbpools, got %d", len(ctx.DBPools))
	}

	// No services configured, so DB_* vars are not set
	// (they are only set when services have DB config or DBInfo is provided)
	if len(ctx.DBPools) != 0 {
		t.Fatalf("expected 0 dbpools, got %d", len(ctx.DBPools))
	}
}

func TestInitContext_DamperMinimal(t *testing.T) {
	cfg := &Config{
		DamperServer: "10.0.0.1:5555",
	}
	ctx := context.New()
	cfg.InitContext(ctx)

	if ctx.DamperTCP != "10.0.0.1:5555" {
		t.Fatalf("DamperTCP: expected '10.0.0.1:5555', got %q", ctx.DamperTCP)
	}
	if ctx.DamperHTTP != "10.0.0.1:5555" {
		t.Fatalf("DamperHTTP: expected '10.0.0.1:5555', got %q", ctx.DamperHTTP)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func checkCtxVar(t *testing.T, ctx *context.TestContext, key, expected string) {
	t.Helper()
	v, ok := ctx.Get(key)
	if !ok {
		t.Fatalf("variable %q not set", key)
	}
	if v != expected {
		t.Fatalf("variable %q: expected %q, got %q", key, expected, v)
	}
}