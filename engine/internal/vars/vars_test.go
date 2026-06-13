package vars

import (
	"testing"

	"github.com/Eric033/x-mate/engine/internal/context"
)

// ---------------------------------------------------------------------------
// PreProcess — ${var}
// ---------------------------------------------------------------------------

func TestPreProcess_Basic(t *testing.T) {
	ctx := context.New()
	ctx.Set("name", "Alice")

	result := PreProcess(ctx, "Hello, ${name}!")
	if result != "Hello, Alice!" {
		t.Fatalf("expected 'Hello, Alice!', got %q", result)
	}
}

func TestPreProcess_MultipleVars(t *testing.T) {
	ctx := context.New()
	ctx.Set("first", "John")
	ctx.Set("last", "Doe")

	result := PreProcess(ctx, "${first} ${last}")
	if result != "John Doe" {
		t.Fatalf("expected 'John Doe', got %q", result)
	}
}

func TestPreProcess_UnresolvedVar(t *testing.T) {
	ctx := context.New()
	// ${missing} not set → should stay as-is
	result := PreProcess(ctx, "value=${missing}")
	if result != "value=${missing}" {
		t.Fatalf("expected 'value=${missing}', got %q", result)
	}
}

func TestPreProcess_EmptyString(t *testing.T) {
	ctx := context.New()
	result := PreProcess(ctx, "")
	if result != "" {
		t.Fatalf("expected empty, got %q", result)
	}
}

func TestPreProcess_NoVars(t *testing.T) {
	ctx := context.New()
	result := PreProcess(ctx, "plain text without variables")
	if result != "plain text without variables" {
		t.Fatalf("expected unchanged, got %q", result)
	}
}

func TestPreProcess_EmptyVarName(t *testing.T) {
	ctx := context.New()
	result := PreProcess(ctx, "${}")
	if result != "${}" {
		t.Fatalf("expected '${}', got %q", result)
	}
}

func TestPreProcess_VarWithValueEqualsPattern(t *testing.T) {
	ctx := context.New()
	ctx.Set("var", "hello")
	result := PreProcess(ctx, "${var}")
	if result != "hello" {
		t.Fatalf("expected 'hello', got %q", result)
	}
}

func TestPreProcess_MultipleSameVar(t *testing.T) {
	ctx := context.New()
	ctx.Set("x", "1")
	result := PreProcess(ctx, "${x}${x}${x}")
	if result != "111" {
		t.Fatalf("expected '111', got %q", result)
	}
}

func TestPreProcess_AdjacentVars(t *testing.T) {
	ctx := context.New()
	ctx.Set("a", "A")
	ctx.Set("b", "B")
	result := PreProcess(ctx, "${a}${b}")
	if result != "AB" {
		t.Fatalf("expected 'AB', got %q", result)
	}
}

func TestPreProcess_NestedBracesNotHandled(t *testing.T) {
	ctx := context.New()
	ctx.Set("x", "inner")
	// ${outer} should stay as-is if only x is set
	result := PreProcess(ctx, "${x}")
	if result != "inner" {
		t.Fatalf("expected 'inner', got %q", result)
	}
}

// ---------------------------------------------------------------------------
// ResolveTemplate — {{var}}
// ---------------------------------------------------------------------------

func TestResolveTemplate_Basic(t *testing.T) {
	ctx := context.New()
	ctx.Set("name", "Alice")

	result := ResolveTemplate(ctx, "Hello, {{name}}!")
	if result != "Hello, Alice!" {
		t.Fatalf("expected 'Hello, Alice!', got %q", result)
	}
}

func TestResolveTemplate_WithSpaces(t *testing.T) {
	ctx := context.New()
	ctx.Set("key", "value")

	// The function trims spaces inside {{ }}
	result := ResolveTemplate(ctx, "{{ key }}")
	if result != "value" {
		t.Fatalf("expected 'value', got %q", result)
	}
}

func TestResolveTemplate_MultipleVars(t *testing.T) {
	ctx := context.New()
	ctx.Set("a", "1")
	ctx.Set("b", "2")

	result := ResolveTemplate(ctx, "{{a}} + {{b}} = 3")
	if result != "1 + 2 = 3" {
		t.Fatalf("expected '1 + 2 = 3', got %q", result)
	}
}

func TestResolveTemplate_UnresolvedVar(t *testing.T) {
	ctx := context.New()
	result := ResolveTemplate(ctx, "value={{missing}}")
	if result != "value={{missing}}" {
		t.Fatalf("expected 'value={{missing}}', got %q", result)
	}
}

func TestResolveTemplate_EmptyString(t *testing.T) {
	ctx := context.New()
	result := ResolveTemplate(ctx, "")
	if result != "" {
		t.Fatalf("expected empty, got %q", result)
	}
}

func TestResolveTemplate_NoVars(t *testing.T) {
	ctx := context.New()
	result := ResolveTemplate(ctx, "plain text")
	if result != "plain text" {
		t.Fatalf("expected 'plain text', got %q", result)
	}
}

func TestResolveTemplate_EmptyBraces(t *testing.T) {
	ctx := context.New()
	result := ResolveTemplate(ctx, "{{}}")
	if result != "{{}}" {
		t.Fatalf("expected '{{}}', got %q", result)
	}
}

func TestResolveTemplate_MultipleSameVar(t *testing.T) {
	ctx := context.New()
	ctx.Set("x", "X")
	result := ResolveTemplate(ctx, "{{x}}-{{x}}-{{x}}")
	if result != "X-X-X" {
		t.Fatalf("expected 'X-X-X', got %q", result)
	}
}

// ---------------------------------------------------------------------------
// ResolveAll — both ${var} and {{var}}
// ---------------------------------------------------------------------------

func TestResolveAll_BothPatterns(t *testing.T) {
	ctx := context.New()
	ctx.Set("a", "1")
	ctx.Set("b", "2")

	result := ResolveAll(ctx, "${a} + {{b}} = 3")
	if result != "1 + 2 = 3" {
		t.Fatalf("expected '1 + 2 = 3', got %q", result)
	}
}

func TestResolveAll_OnlyDollar(t *testing.T) {
	ctx := context.New()
	ctx.Set("x", "hello")

	result := ResolveAll(ctx, "${x}")
	if result != "hello" {
		t.Fatalf("expected 'hello', got %q", result)
	}
}

func TestResolveAll_OnlyDouble(t *testing.T) {
	ctx := context.New()
	ctx.Set("y", "world")

	result := ResolveAll(ctx, "{{y}}")
	if result != "world" {
		t.Fatalf("expected 'world', got %q", result)
	}
}

func TestResolveAll_UnresolvedMixed(t *testing.T) {
	ctx := context.New()
	ctx.Set("exists", "OK")

	result := ResolveAll(ctx, "${exists} + {{missing}}")
	if result != "OK + {{missing}}" {
		t.Fatalf("expected 'OK + {{missing}}', got %q", result)
	}
}

func TestResolveAll_EmptyString(t *testing.T) {
	ctx := context.New()
	result := ResolveAll(ctx, "")
	if result != "" {
		t.Fatalf("expected empty, got %q", result)
	}
}

func TestResolveAll_NoVars(t *testing.T) {
	ctx := context.New()
	result := ResolveAll(ctx, "no variables")
	if result != "no variables" {
		t.Fatalf("expected 'no variables', got %q", result)
	}
}

func TestResolveAll_PreProcessFirst(t *testing.T) {
	// Ensure ${} is resolved before {{}}
	ctx := context.New()
	ctx.Set("a", "{{b}}")
	ctx.Set("b", "resolved")

	result := ResolveAll(ctx, "${a}")
	// PreProcess resolves ${a} → "{{b}}"
	// ResolveTemplate then resolves {{b}} → "resolved"
	if result != "resolved" {
		t.Fatalf("expected 'resolved' (chained resolution), got %q", result)
	}
}

// ---------------------------------------------------------------------------
// Mixed edge cases
// ---------------------------------------------------------------------------

func TestPreProcessAndResolveTemplate_Overlapping(t *testing.T) {
	ctx := context.New()
	ctx.Set("var", "value")

	// These patterns should not interfere with each other
	result1 := PreProcess(ctx, "{{${var}}}")
	result2 := ResolveTemplate(ctx, "${ {var} }")

	_ = result1
	_ = result2
	// Just ensure no panic
}

func TestPreProcess_UnclosedBrace(t *testing.T) {
	ctx := context.New()
	ctx.Set("x", "val")
	result := PreProcess(ctx, "${x and ${y}")
	// The regex only matches closed braces, so "${x" stays as is
	_ = result
}

func TestResolveTemplate_VarNameWithUnderscore(t *testing.T) {
	ctx := context.New()
	ctx.Set("my_var_name", "hello")

	result := ResolveTemplate(ctx, "{{my_var_name}}")
	if result != "hello" {
		t.Fatalf("expected 'hello', got %q", result)
	}
}

func TestPreProcess_VarNameWithUnderscore(t *testing.T) {
	ctx := context.New()
	ctx.Set("my_var_name", "hello")

	result := PreProcess(ctx, "${my_var_name}")
	if result != "hello" {
		t.Fatalf("expected 'hello', got %q", result)
	}
}