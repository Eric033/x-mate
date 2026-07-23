package runtime

import (
	"testing"

	"github.com/Eric033/x-mate/engine/internal/context"
	"github.com/Eric033/x-mate/engine/internal/handler"
)

// ---- evalComparison tests ----

func TestEvalComparison_NumericGreater(t *testing.T) {
	tests := []struct {
		expr     string
		expected bool
	}{
		{"100 > 50", true},
		{"50 > 100", false},
		{"100 > 100", false},
		{"5.5 > 3.2", true},
		{"3.2 > 5.5", false},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got := evalComparison(tt.expr)
			if got != tt.expected {
				t.Errorf("evalComparison(%q) = %v, want %v", tt.expr, got, tt.expected)
			}
		})
	}
}

func TestEvalComparison_NumericLess(t *testing.T) {
	tests := []struct {
		expr     string
		expected bool
	}{
		{"50 < 100", true},
		{"100 < 50", false},
		{"100 < 100", false},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got := evalComparison(tt.expr)
			if got != tt.expected {
				t.Errorf("evalComparison(%q) = %v, want %v", tt.expr, got, tt.expected)
			}
		})
	}
}

func TestEvalComparison_NumericGreaterEqual(t *testing.T) {
	tests := []struct {
		expr     string
		expected bool
	}{
		{"100 >= 50", true},
		{"100 >= 100", true},
		{"50 >= 100", false},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got := evalComparison(tt.expr)
			if got != tt.expected {
				t.Errorf("evalComparison(%q) = %v, want %v", tt.expr, got, tt.expected)
			}
		})
	}
}

func TestEvalComparison_NumericLessEqual(t *testing.T) {
	tests := []struct {
		expr     string
		expected bool
	}{
		{"50 <= 100", true},
		{"100 <= 100", true},
		{"100 <= 50", false},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got := evalComparison(tt.expr)
			if got != tt.expected {
				t.Errorf("evalComparison(%q) = %v, want %v", tt.expr, got, tt.expected)
			}
		})
	}
}

func TestEvalComparison_NumericEqual(t *testing.T) {
	tests := []struct {
		expr     string
		expected bool
	}{
		{"100 == 100", true},
		{"100 == 50", false},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got := evalComparison(tt.expr)
			if got != tt.expected {
				t.Errorf("evalComparison(%q) = %v, want %v", tt.expr, got, tt.expected)
			}
		})
	}
}

func TestEvalComparison_NumericNotEqual(t *testing.T) {
	tests := []struct {
		expr     string
		expected bool
	}{
		{"100 != 50", true},
		{"100 != 100", false},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got := evalComparison(tt.expr)
			if got != tt.expected {
				t.Errorf("evalComparison(%q) = %v, want %v", tt.expr, got, tt.expected)
			}
		})
	}
}

func TestEvalComparison_StringEqual(t *testing.T) {
	tests := []struct {
		expr     string
		expected bool
	}{
		{"hello == hello", true},
		{"hello == world", false},
		{"hello != world", true},
		{"hello != hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got := evalComparison(tt.expr)
			if got != tt.expected {
				t.Errorf("evalComparison(%q) = %v, want %v", tt.expr, got, tt.expected)
			}
		})
	}
}

func TestEvalComparison_BooleanTrue(t *testing.T) {
	tests := []struct {
		expr     string
		expected bool
	}{
		{"true", true},
		{"  true  ", true},
		{"false", false},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got := evalComparison(tt.expr)
			if got != tt.expected {
				t.Errorf("evalComparison(%q) = %v, want %v", tt.expr, got, tt.expected)
			}
		})
	}
}

func TestEvalComparison_InvalidOperator(t *testing.T) {
	// If comparison has an unsupported operator or can't be parsed,
	// evalComparison returns false
	tests := []struct {
		expr     string
		expected bool
	}{
		{"", false},
		{"abc def ghi", false},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got := evalComparison(tt.expr)
			if got != tt.expected {
				t.Errorf("evalComparison(%q) = %v, want %v", tt.expr, got, tt.expected)
			}
		})
	}
}

// ---- jsonpathSimple tests ----

func TestJsonpathSimple_Simple(t *testing.T) {
	jsonStr := `{"name":"Alice","age":30}`
	tests := []struct {
		path     string
		expected string
	}{
		{"$.name", "Alice"},
		{"$.age", "30"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := jsonpathSimple(tt.path, jsonStr)
			if got != tt.expected {
				t.Errorf("jsonpathSimple(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

func TestJsonpathSimple_Nested(t *testing.T) {
	jsonStr := `{"user":{"name":"Bob","age":25}}`
	got := jsonpathSimple("$.user.name", jsonStr)
	if got != "Bob" {
		t.Errorf("expected 'Bob', got '%s'", got)
	}
}

func TestJsonpathSimple_NotFound(t *testing.T) {
	got := jsonpathSimple("$.nonexistent", `{"name":"Alice"}`)
	if got != "" {
		t.Errorf("expected empty, got '%s'", got)
	}
}

// ---- RuntimeVerifyHandler tests ----

func TestRuntimeVerifyHandler_Execute_TrueExpression(t *testing.T) {
	ctx := context.New()
	ctx.Set("value", "100")

	data := &handler.StepData{
		StepType: "runtime_verify",
		Assertions: []handler.Assertion{
			{Expected: "{{value}} > 50"},
		},
	}

	h := &RuntimeVerifyHandler{}
	result := h.Execute(data, ctx)

	if !result.Success {
		t.Fatalf("expected success, got: %s", result.FailureMessage)
	}
}

func TestRuntimeVerifyHandler_Execute_FalseExpression(t *testing.T) {
	ctx := context.New()
	ctx.Set("value", "10")

	data := &handler.StepData{
		StepType: "runtime_verify",
		Assertions: []handler.Assertion{
			{Expected: "{{value}} > 50"},
		},
	}

	h := &RuntimeVerifyHandler{}
	result := h.Execute(data, ctx)

	if result.Success {
		t.Fatal("expected failure for false expression")
	}
	if result.FailureMessage == "" {
		t.Error("expected non-empty failure message")
	}
}

func TestRuntimeVerifyHandler_Execute_EmptyExpression(t *testing.T) {
	ctx := context.New()

	data := &handler.StepData{
		StepType: "runtime_verify",
		// No assertions
	}

	h := &RuntimeVerifyHandler{}
	result := h.Execute(data, ctx)

	if !result.Success {
		t.Fatalf("expected success for empty expression, got: %s", result.FailureMessage)
	}
}

func TestRuntimeVerifyHandler_Execute_VerifyResults(t *testing.T) {
	ctx := context.New()
	ctx.Set("count", "5")

	data := &handler.StepData{
		StepType: "runtime_verify",
		Assertions: []handler.Assertion{
			{XPath: "count", Expected: "{{count}} == 5"},
		},
	}

	h := &RuntimeVerifyHandler{}
	result := h.Execute(data, ctx)

	if !result.Success {
		t.Fatalf("expected success, got: %s", result.FailureMessage)
	}
}

func TestRuntimeVerifyHandler_Execute_VariableNotFound(t *testing.T) {
	ctx := context.New()
	// Don't set "value" — should default to "0"

	data := &handler.StepData{
		StepType: "runtime_verify",
		Assertions: []handler.Assertion{
			{Expected: "{{value}} > 50"},
		},
	}

	h := &RuntimeVerifyHandler{}
	result := h.Execute(data, ctx)

	if result.Success {
		t.Fatal("expected failure (0 > 50 is false)")
	}
}

func TestRuntimeVerifyHandler_Execute_StringComparison(t *testing.T) {
	ctx := context.New()
	ctx.Set("status", "ok")

	data := &handler.StepData{
		StepType: "runtime_verify",
		Assertions: []handler.Assertion{
			{Expected: "{{status}} == ok"},
		},
	}

	h := &RuntimeVerifyHandler{}
	result := h.Execute(data, ctx)

	if !result.Success {
		t.Fatalf("expected success, got: %s", result.FailureMessage)
	}
}

func TestRuntimeVerifyHandler_Execute_SaveVars(t *testing.T) {
	ctx := context.New()
	ctx.Set("prevResult", `{"result":"hello","count":42}`)

	data := &handler.StepData{
		StepType: "runtime_verify",
		Assertions: []handler.Assertion{
			{Expected: "1 == 1"},
		},
		Saves: []handler.SaveEntry{
			{Name: "saved_result", Locator: "$.result"},
			{Name: "saved_count", Locator: "$.count"},
		},
	}

	h := &RuntimeVerifyHandler{}
	result := h.Execute(data, ctx)

	if !result.Success {
		t.Fatalf("expected success, got: %s", result.FailureMessage)
	}

	if val, ok := ctx.Get("saved_result"); !ok {
		t.Error("expected 'saved_result' to be saved")
	} else if val != "hello" {
		t.Errorf("expected 'hello', got '%s'", val)
	}

	if val, ok := ctx.Get("saved_count"); !ok {
		t.Error("expected 'saved_count' to be saved")
	} else if val != "42" {
		t.Errorf("expected '42', got '%s'", val)
	}
}

func TestRuntimeVerifyHandler_Execute_SaveVarNoPrevResult(t *testing.T) {
	ctx := context.New()
	// No prevResult set

	data := &handler.StepData{
		StepType: "runtime_verify",
		Assertions: []handler.Assertion{
			{Expected: "1 == 1"},
		},
		Saves: []handler.SaveEntry{
			{Name: "saved_result", Locator: "$.result"},
		},
	}

	h := &RuntimeVerifyHandler{}
	result := h.Execute(data, ctx)

	if !result.Success {
		t.Fatalf("expected success, got: %s", result.FailureMessage)
	}

	// Should not crash; variable should not be set
	if _, ok := ctx.Get("saved_result"); ok {
		t.Error("expected 'saved_result' to NOT be saved since prevResult is missing")
	}
}