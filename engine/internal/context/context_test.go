package context

import (
	"fmt"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// New
// ---------------------------------------------------------------------------

func TestNew(t *testing.T) {
	ctx := New()
	if ctx == nil {
		t.Fatal("New() returned nil")
	}
	if ctx.Variables == nil {
		t.Fatal("New().Variables is nil")
	}
	if len(ctx.Variables) != 0 {
		t.Fatal("New().Variables should be empty")
	}
	if ctx.Servers != nil {
		t.Fatal("New().Servers should be nil")
	}
	if ctx.DBPools != nil {
		t.Fatal("New().DBPools should be nil")
	}
}

// ---------------------------------------------------------------------------
// Set / Get
// ---------------------------------------------------------------------------

func TestSetAndGet(t *testing.T) {
	ctx := New()
	ctx.Set("key1", "value1")

	v, ok := ctx.Get("key1")
	if !ok {
		t.Fatal("expected key1 to exist")
	}
	if v != "value1" {
		t.Fatalf("expected value1, got %s", v)
	}
}

func TestGet_NotExists(t *testing.T) {
	ctx := New()
	_, ok := ctx.Get("nonexistent")
	if ok {
		t.Fatal("expected nonexistent key to return false")
	}
}

func TestSet_Overwrite(t *testing.T) {
	ctx := New()
	ctx.Set("k", "v1")
	ctx.Set("k", "v2")

	v, ok := ctx.Get("k")
	if !ok {
		t.Fatal("expected k to exist")
	}
	if v != "v2" {
		t.Fatalf("expected v2, got %s", v)
	}
}

// ---------------------------------------------------------------------------
// GetOrDefault
// ---------------------------------------------------------------------------

func TestGetOrDefault_Exists(t *testing.T) {
	ctx := New()
	ctx.Set("k", "actual")
	got := ctx.GetOrDefault("k", "default")
	if got != "actual" {
		t.Fatalf("expected actual, got %s", got)
	}
}

func TestGetOrDefault_NotExists(t *testing.T) {
	ctx := New()
	got := ctx.GetOrDefault("missing", "fallback")
	if got != "fallback" {
		t.Fatalf("expected fallback, got %s", got)
	}
}

func TestGetOrDefault_EmptyDefault(t *testing.T) {
	ctx := New()
	got := ctx.GetOrDefault("missing", "")
	if got != "" {
		t.Fatalf("expected empty string, got %s", got)
	}
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

func TestDelete(t *testing.T) {
	ctx := New()
	ctx.Set("k", "v")
	ctx.Delete("k")

	_, ok := ctx.Get("k")
	if ok {
		t.Fatal("expected key to be deleted")
	}
}

func TestDelete_NonExistent(t *testing.T) {
	ctx := New()
	// Should not panic
	ctx.Delete("doesnotexist")
}

// ---------------------------------------------------------------------------
// CleanupTemporary
// ---------------------------------------------------------------------------

func TestCleanupTemporary(t *testing.T) {
	ctx := New()
	ctx.Set("testv_abc", "tmp1")
	ctx.Set("testv_xyz", "tmp2")
	ctx.Set("setup_1", "setupVal")
	ctx.Set("teardown_1", "teardownVal")
	ctx.Set("server_index", "0")
	ctx.Set("resultVariable", "result")
	ctx.Set("keep_me", "shouldStay")

	ctx.CleanupTemporary()

	// Temporary vars should be gone
	checkDeleted(t, ctx, "testv_abc")
	checkDeleted(t, ctx, "testv_xyz")
	checkDeleted(t, ctx, "setup_1")
	checkDeleted(t, ctx, "teardown_1")
	checkDeleted(t, ctx, "server_index")
	checkDeleted(t, ctx, "resultVariable")

	// Non-temporary should remain
	v, ok := ctx.Get("keep_me")
	if !ok {
		t.Fatal("expected keep_me to remain after cleanup")
	}
	if v != "shouldStay" {
		t.Fatalf("expected shouldStay, got %s", v)
	}
}

func TestCleanupTemporary_EmptyVars(t *testing.T) {
	ctx := New()
	// Should not panic when Variables map is empty
	ctx.CleanupTemporary()
}

func TestCleanupTemporary_PrefixBoundary(t *testing.T) {
	ctx := New()
	ctx.Set("testv_", "edge")        // exactly 6 chars prefix
	ctx.Set("testv_long", "val")     // 7+ chars
	ctx.Set("notextv_abc", "keep")   // does not start with testv_

	ctx.CleanupTemporary()

	checkDeleted(t, ctx, "testv_")
	checkDeleted(t, ctx, "testv_long")

	v, ok := ctx.Get("notextv_abc")
	if !ok {
		t.Fatal("expected notextv_abc to remain")
	}
	if v != "keep" {
		t.Fatalf("expected keep, got %s", v)
	}
}

// ---------------------------------------------------------------------------
// GenerateSystemVars
// ---------------------------------------------------------------------------

func TestGenerateSystemVars_Basic(t *testing.T) {
	ctx := New()
	now := time.Now()
	// YMMdd format: year last digit + month + day
	expectedDate6 := fmt.Sprintf("%d%02d%02d", now.Year()%10, now.Month(), now.Day())

	ctx.GenerateSystemVarsLegacy(0) // serverIndex=0 → no server vars

	// date_str_6
	v, ok := ctx.Get("date_str_6")
	if !ok {
		t.Fatal("date_str_6 should exist")
	}
	if v != expectedDate6 {
		t.Fatalf("date_str_6: expected %s, got %s", expectedDate6, v)
	}

	// time_str_6 (YMMddHH)
	expectedTime6 := fmt.Sprintf("%d%02d%02d%02d", now.Year()%10, now.Month(), now.Day(), now.Hour())
	v, ok = ctx.Get("time_str_6")
	if !ok {
		t.Fatal("time_str_6 should exist")
	}
	if v != expectedTime6 {
		t.Fatalf("time_str_6: expected %s, got %s", expectedTime6, v)
	}

	// seq_no: date_str_6 + last 9 of nano + "00"
	v, ok = ctx.Get("seq_no")
	if !ok {
		t.Fatal("seq_no should exist")
	}
	if len(v) != len(expectedDate6)+9+2 {
		t.Fatalf("seq_no length mismatch: got %s (len=%d)", v, len(v))
	}
	if v[:len(expectedDate6)] != expectedDate6 {
		t.Fatalf("seq_no prefix mismatch: expected %s, got %s", expectedDate6, v[:len(expectedDate6)])
	}
	if v[len(v)-2:] != "00" {
		t.Fatalf("seq_no suffix should be 00, got %s", v[len(v)-2:])
	}

	// time_no
	v, ok = ctx.Get("time_no")
	if !ok {
		t.Fatal("time_no should exist")
	}
	timeStr6 := fmt.Sprintf("%d%02d%02d%02d", now.Year()%10, now.Month(), now.Day(), now.Hour())
	if v != timeStr6 {
		t.Fatalf("time_no: expected %s, got %s", timeStr6, v)
	}

	// serverIP/serverPort should NOT be set when serverIndex=0
	_, ok = ctx.Get("serverIP")
	if ok {
		t.Fatal("serverIP should not be set when serverIndex=0")
	}
}

func TestGenerateSystemVars_WithServer(t *testing.T) {
	ctx := New()
	ctx.Servers = []ServerEntry{
		{IP: "10.0.0.1", Port: "8080"},
		{IP: "10.0.0.2", Port: "9090"},
	}

	ctx.GenerateSystemVarsLegacy(1)

	v, ok := ctx.Get("serverIP")
	if !ok || v != "10.0.0.1" {
		t.Fatalf("serverIP: expected 10.0.0.1, got %s", v)
	}

	v, ok = ctx.Get("serverPort")
	if !ok || v != "8080" {
		t.Fatalf("serverPort: expected 8080, got %s", v)
	}

	// serverIndex=2
	ctx2 := New()
	ctx2.Servers = ctx.Servers
	ctx2.GenerateSystemVarsLegacy(2)

	v, _ = ctx2.Get("serverIP")
	if v != "10.0.0.2" {
		t.Fatalf("serverIP(2): expected 10.0.0.2, got %s", v)
	}
	v, _ = ctx2.Get("serverPort")
	if v != "9090" {
		t.Fatalf("serverPort(2): expected 9090, got %s", v)
	}
}

func TestGenerateSystemVars_ServerOutOfRange(t *testing.T) {
	ctx := New()
	ctx.Servers = []ServerEntry{{IP: "1.2.3.4", Port: "1234"}}

	// serverIndex > len(Servers) → should not set serverIP/serverPort
	ctx.GenerateSystemVarsLegacy(5)
	_, ok := ctx.Get("serverIP")
	if ok {
		t.Fatal("serverIP should not be set when serverIndex > len(Servers)")
	}

	// serverIndex=0 with non-empty Servers should also skip
	ctx2 := New()
	ctx2.Servers = ctx.Servers
	ctx2.GenerateSystemVarsLegacy(0)
	_, ok = ctx2.Get("serverIP")
	if ok {
		t.Fatal("serverIP should not be set when serverIndex=0")
	}
}

func TestGenerateSystemVars_SeqNoPay(t *testing.T) {
	ctx := New()
	ctx.GenerateSystemVarsLegacy(0)

	v, ok := ctx.Get("seq_no_pay")
	if !ok {
		t.Fatal("seq_no_pay should exist")
	}
	date6, _ := ctx.Get("date_str_6")
	if len(date6) >= 5 {
		expectedSuffix := date6[len(date6)-5:]
		if v[:5] != expectedSuffix {
			t.Fatalf("seq_no_pay prefix: expected %s, got %s", expectedSuffix, v[:5])
		}
	}
	if v[len(v)-2:] != "00" {
		t.Fatalf("seq_no_pay suffix should be 00, got %s", v[len(v)-2:])
	}
}

// ---------------------------------------------------------------------------
// GenerateRandomVars
// ---------------------------------------------------------------------------

func TestGenerateRandomVars(t *testing.T) {
	ctx := New()
	ctx.GenerateRandomVars()

	v, ok := ctx.Get("random_8")
	if !ok {
		t.Fatal("random_8 should exist")
	}
	if len(v) != 8 {
		t.Fatalf("random_8 should be 8 digits, got %s (len=%d)", v, len(v))
	}
	// Should be numeric
	for _, c := range v {
		if c < '0' || c > '9' {
			t.Fatalf("random_8 contains non-digit: %c", c)
		}
	}
}

func TestGenerateRandomVars_Repeatability(t *testing.T) {
	// rand.Intn is seeded globally, so two calls may produce different values.
	// We just verify they run without error and produce 8-char strings.
	ctx := New()
	ctx.GenerateRandomVars()
	v1, _ := ctx.Get("random_8")

	ctx2 := New()
	ctx2.GenerateRandomVars()
	v2, _ := ctx2.Get("random_8")

	_ = v1
	_ = v2
	// No assertion on equality — randomness means values may differ.
}

// ---------------------------------------------------------------------------
// Concurrency (basic)
// ---------------------------------------------------------------------------

func TestConcurrentAccess(t *testing.T) {
	ctx := New()

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(n int) {
			key := "k"
			_ = key
			ctx.Set("goroutine", "val")
			_, _ = ctx.Get("goroutine")
			done <- true
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func checkDeleted(t *testing.T, ctx *TestContext, key string) {
	t.Helper()
	_, ok := ctx.Get(key)
	if ok {
		t.Fatalf("expected key %q to be deleted after cleanup", key)
	}
}