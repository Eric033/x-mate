package context

import (
	"testing"
	"time"
)

func TestClone_Empty(t *testing.T) {
	ctx := New()
	clone := ctx.Clone()

	if clone == ctx {
		t.Error("Clone returned same pointer")
	}
	if len(clone.Variables) != 0 {
		t.Errorf("cloned variables = %d, want 0", len(clone.Variables))
	}
}

func TestClone_WithVariables(t *testing.T) {
	ctx := New()
	ctx.Set("key1", "val1")
	ctx.Set("key2", "val2")

	clone := ctx.Clone()

	v, ok := clone.Get("key1")
	if !ok || v != "val1" {
		t.Errorf("key1 = %q", v)
	}
	v, ok = clone.Get("key2")
	if !ok || v != "val2" {
		t.Errorf("key2 = %q", v)
	}

	// Modifying clone should not affect original
	clone.Set("key1", "modified")
	if orig, _ := ctx.Get("key1"); orig != "val1" {
		t.Errorf("original key1 changed to %q", orig)
	}
}

func TestClone_WithServices(t *testing.T) {
	ctx := New()
	ctx.Services = map[string]ServiceDef{
		"MOCK": {Address: "127.0.0.1:8080"},
	}
	ctx.Set("serverIP", "127.0.0.1")

	clone := ctx.Clone()

	// Verify services copied
	svc, ok := clone.Services["MOCK"]
	if !ok {
		t.Fatal("MOCK service not cloned")
	}
	if svc.Address != "127.0.0.1:8080" {
		t.Errorf("service address = %q", svc.Address)
	}

	// Clone should have independent services map
	clone.Services["MOCK"] = ServiceDef{Address: "modified"}
	if ctx.Services["MOCK"].Address != "127.0.0.1:8080" {
		t.Error("original services modified via clone")
	}
}

func TestGenerateSystemVars_NoService(t *testing.T) {
	ctx := New()
	ctx.GenerateSystemVars("")

	// These should be set regardless
	if _, ok := ctx.Get("date_str_6"); !ok {
		t.Error("date_str_6 not set")
	}
	if _, ok := ctx.Get("seq_no"); !ok {
		t.Error("seq_no not set")
	}
	if _, ok := ctx.Get("time_no"); !ok {
		t.Error("time_no not set")
	}

	// serverIP should not be set without service
	if _, ok := ctx.Get("serverIP"); ok {
		t.Error("serverIP should not be set without service")
	}
}

func TestGenerateSystemVars_WithService(t *testing.T) {
	ctx := New()
	ctx.Services = map[string]ServiceDef{
		"MOCK": {Address: "10.0.0.5:5555"},
	}

	ctx.GenerateSystemVars("MOCK")

	ip, ok := ctx.Get("serverIP")
	if !ok || ip != "10.0.0.5" {
		t.Errorf("serverIP = %q, want 10.0.0.5", ip)
	}
	port, ok := ctx.Get("serverPort")
	if !ok || port != "5555" {
		t.Errorf("serverPort = %q, want 5555", port)
	}
}

func TestGenerateSystemVars_SeqNo(t *testing.T) {
	ctx := New()
	ctx.GenerateSystemVars("")

	seqNo, ok := ctx.Get("seq_no")
	if !ok {
		t.Fatal("seq_no not set")
	}
	if len(seqNo) < 10 {
		t.Errorf("seq_no too short: %q", seqNo)
	}

	// Verify format: date_str_6(YMMdd, 5 chars) + 9 digits + 00 = 16 chars
	if len(seqNo) != 16 {
		t.Errorf("seq_no length = %d, want 16", len(seqNo))
	}

	// Last two chars should be 00
	if seqNo[len(seqNo)-2:] != "00" {
		t.Errorf("seq_no suffix = %q, want 00", seqNo[len(seqNo)-2:])
	}

	// First char should be last digit of current year
	yearDigit := time.Now().Year() % 10
	if seqNo[0] != byte('0'+yearDigit) {
		t.Errorf("seq_no[0] = %c, want %d", seqNo[0], yearDigit)
	}
}

func TestGenerateSystemVarsLegacy_WithService(t *testing.T) {
	ctx := New()
	ctx.Services = map[string]ServiceDef{
		"SVC1": {Address: "10.0.0.1:9996"},
		"SVC2": {Address: "10.0.0.2:8080"},
	}

	ctx.GenerateSystemVarsLegacy(2)

	ip, _ := ctx.Get("serverIP")
	if ip != "10.0.0.1" {
		t.Errorf("serverIP = %q, want 10.0.0.1", ip)
	}
	port, _ := ctx.Get("serverPort")
	if port != "9996" {
		t.Errorf("serverPort = %q, want 9996", port)
	}
}

func TestGenerateSystemVarsLegacy_NoService(t *testing.T) {
	ctx := New()

	// Should not set serverIP with no services
	ctx.GenerateSystemVarsLegacy(5)
	if _, ok := ctx.Get("serverIP"); ok {
		t.Error("serverIP should not be set with no services")
	}
}

func TestGenerateRandomVars(t *testing.T) {
	ctx := New()
	ctx.GenerateRandomVars()

	v, ok := ctx.Get("random_8")
	if !ok {
		t.Fatal("random_8 not set")
	}
	if len(v) != 8 {
		t.Errorf("random_8 length = %d, want 8", len(v))
	}
}

func TestCleanupTemporary(t *testing.T) {
	ctx := New()
	ctx.Set("testv_1", "temp1")
	ctx.Set("testv_2", "temp2")
	ctx.Set("setup_1", "setup")
	ctx.Set("teardown_1", "teardown")
	ctx.Set("server_index", "1")
	ctx.Set("resultVariable", "some")
	ctx.Set("persistent_var", "keep")

	ctx.CleanupTemporary()

	// Temporary vars should be removed
	if _, ok := ctx.Get("testv_1"); ok {
		t.Error("testv_1 should be removed")
	}
	if _, ok := ctx.Get("setup_1"); ok {
		t.Error("setup_1 should be removed")
	}
	if _, ok := ctx.Get("teardown_1"); ok {
		t.Error("teardown_1 should be removed")
	}
	if _, ok := ctx.Get("server_index"); ok {
		t.Error("server_index should be removed")
	}
	if _, ok := ctx.Get("resultVariable"); ok {
		t.Error("resultVariable should be removed")
	}

	// Persistent var should remain
	if v, ok := ctx.Get("persistent_var"); !ok || v != "keep" {
		t.Error("persistent_var should remain")
	}
}

func TestGetSetDelete(t *testing.T) {
	ctx := New()

	ctx.Set("key", "value")
	v, ok := ctx.Get("key")
	if !ok || v != "value" {
		t.Errorf("Get = %q, %v", v, ok)
	}

	ctx.Delete("key")
	if _, ok := ctx.Get("key"); ok {
		t.Error("key should be deleted")
	}
}

func TestGetOrDefault(t *testing.T) {
	ctx := New()
	ctx.Set("exists", "yes")

	if v := ctx.GetOrDefault("exists", "no"); v != "yes" {
		t.Errorf("GetOrDefault existing = %q", v)
	}
	if v := ctx.GetOrDefault("missing", "default"); v != "default" {
		t.Errorf("GetOrDefault missing = %q", v)
	}
}

func TestGetServiceAddr(t *testing.T) {
	ctx := New()
	ctx.Services = map[string]ServiceDef{
		"MOCK": {Address: "10.0.0.1:8080"},
	}

	addr, ok := ctx.GetServiceAddr("MOCK")
	if !ok || addr != "10.0.0.1:8080" {
		t.Errorf("GetServiceAddr = %q, %v", addr, ok)
	}

	_, ok = ctx.GetServiceAddr("NONEXISTENT")
	if ok {
		t.Error("GetServiceAddr for non-existent should return false")
	}
}

func TestGetService(t *testing.T) {
	ctx := New()
	ctx.Services = map[string]ServiceDef{
		"MOCK": {Address: "10.0.0.1:8080", HTTPPort: 8080},
	}

	svc, ok := ctx.GetService("MOCK")
	if !ok || svc.Address != "10.0.0.1:8080" || svc.HTTPPort != 8080 {
		t.Errorf("GetService = %+v, %v", svc, ok)
	}
}

func TestGetServiceDB(t *testing.T) {
	ctx := New()
	ctx.Services = map[string]ServiceDef{
		"MOCK": {
			Address: "10.0.0.1:8080",
			DB: &DBConf{Address: "10.0.0.2:1521", Database: "ORCL"},
		},
	}

	db, ok := ctx.GetServiceDB("MOCK")
	if !ok || db.Address != "10.0.0.2:1521" || db.Database != "ORCL" {
		t.Errorf("GetServiceDB = %+v, %v", db, ok)
	}

	_, ok = ctx.GetServiceDB("NONEXISTENT")
	if ok {
		t.Error("GetServiceDB for non-existent should return false")
	}

	// Services without DB should return false
	ctx2 := New()
	ctx2.Services = map[string]ServiceDef{
		"NO_DB": {Address: "10.0.0.1:8080"},
	}
	_, ok = ctx2.GetServiceDB("NO_DB")
	if ok {
		t.Error("GetServiceDB for service without DB should return false")
	}
}

func TestGetServiceAddrForStep(t *testing.T) {
	ctx := New()
	ctx.Services = map[string]ServiceDef{
		"MOCK": {Address: "10.0.0.1:8080"},
	}

	addr, ok := ctx.GetServiceAddrForStep("MOCK")
	if !ok || addr != "10.0.0.1:8080" {
		t.Errorf("GetServiceAddrForStep(MOCK) = %q, %v", addr, ok)
	}

	// Literal ip:port
	addr, ok = ctx.GetServiceAddrForStep("1.2.3.4:5678")
	if !ok || addr != "1.2.3.4:5678" {
		t.Errorf("GetServiceAddrForStep(ip:port) = %q, %v", addr, ok)
	}

	// Fallback to serverIP/serverPort
	ctx.Set("serverIP", "192.168.1.1")
	ctx.Set("serverPort", "9999")
	addr, ok = ctx.GetServiceAddrForStep("")
	if !ok || addr != "192.168.1.1:9999" {
		t.Errorf("GetServiceAddrForStep(fallback) = %q, %v", addr, ok)
	}
}
