package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Eric033/x-mate/engine/internal/context"
)

// ---- LoadTemplate tests ----

func TestLoadTemplate_Success(t *testing.T) {
	tmpDir := t.TempDir()
	templateDir := filepath.Join(tmpDir, "template")
	os.MkdirAll(templateDir, 0755)

	templateContent := `<root><value>hello</value></root>`
	os.WriteFile(filepath.Join(templateDir, "template_T001.xml"), []byte(templateContent), 0644)

	content, err := LoadTemplate(tmpDir, "T001")
	if err != nil {
		t.Fatalf("LoadTemplate failed: %v", err)
	}

	if content != templateContent {
		t.Errorf("got %q, want %q", content, templateContent)
	}
}

func TestLoadTemplate_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := LoadTemplate(tmpDir, "NONEXISTENT")
	if err == nil {
		t.Fatal("expected error for missing template, got nil")
	}
}

func TestLoadTemplate_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "template"), 0755)

	_, err := LoadTemplate(tmpDir, "T999")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

// ---- Parametrize tests ----

func TestParametrize_NoPairs(t *testing.T) {
	ctx := context.New()
	template := `<root><name>Alice</name></root>`

	result, err := Parametrize(ctx, template, "")
	if err != nil {
		t.Fatalf("Parametrize failed: %v", err)
	}

	if result != template {
		t.Errorf("expected unchanged template, got %q", result)
	}
}

func TestParametrize_ReplaceValue(t *testing.T) {
	ctx := context.New()
	template := `<root><name>old</name></root>`
	pairs := "name:Bob"

	result, err := Parametrize(ctx, template, pairs)
	if err != nil {
		t.Fatalf("Parametrize failed: %v", err)
	}

	// Strict XML engine preserves original structure without html/body wrappers
	if !strings.Contains(result, "<name>Bob</name>") {
		t.Errorf("expected <name>Bob</name> in result, got %q", result)
	}
	if !strings.Contains(result, "<root>") {
		t.Errorf("expected <root> in result, got %q", result)
	}
}

func TestParametrize_MultiplePairs(t *testing.T) {
	ctx := context.New()
	template := `<data><user>old</user><city>nowhere</city></data>`
	pairs := "user:Alice;city:Beijing"

	result, err := Parametrize(ctx, template, pairs)
	if err != nil {
		t.Fatalf("Parametrize failed: %v", err)
	}

	if !strings.Contains(result, "<user>Alice</user>") {
		t.Errorf("expected <user>Alice</user> in result, got %q", result)
	}
	if !strings.Contains(result, "<city>Beijing</city>") {
		t.Errorf("expected <city>Beijing</city> in result, got %q", result)
	}
}

func TestParametrize_WithVarReference(t *testing.T) {
	ctx := context.New()
	ctx.Set("my_name", "Charlie")
	template := `<root><name>{{placeholder}}</name></root>`
	pairs := "name:Hello {{my_name}}"

	result, err := Parametrize(ctx, template, pairs)
	if err != nil {
		t.Fatalf("Parametrize failed: %v", err)
	}

	if !strings.Contains(result, "<name>Hello Charlie</name>") {
		t.Errorf("expected <name>Hello Charlie</name> in result, got %q", result)
	}
}

func TestParametrize_WithDollarBraceVar(t *testing.T) {
	ctx := context.New()
	ctx.Set("env", "prod")
	template := `<root><env>dev</env></root>`
	pairs := "env:${env}"

	result, err := Parametrize(ctx, template, pairs)
	if err != nil {
		t.Fatalf("Parametrize failed: %v", err)
	}

	if !strings.Contains(result, "<env>prod</env>") {
		t.Errorf("expected <env>prod</env> in result, got %q", result)
	}
}

func TestParametrize_UnresolvedVarLeftAsIs(t *testing.T) {
	ctx := context.New()
	template := `<root><name>old</name></root>`
	pairs := "name:{{undefined_var}}"

	result, err := Parametrize(ctx, template, pairs)
	if err != nil {
		t.Fatalf("Parametrize failed: %v", err)
	}

	// {{undefined_var}} stays as-is because ctx.Get returns ""
	// which leaves the pattern in place (vars.ResolveTemplate leaves it when not found)
	if !strings.Contains(result, "{{undefined_var}}") {
		t.Errorf("expected unresolved var to remain, got %q", result)
	}
}

func TestParametrize_WithAutoDoubleSlash(t *testing.T) {
	ctx := context.New()
	// Use lowercase tag — xmlhelper uses html.Parse which is case-insensitive
	template := `<root><trancode>OLD</trancode></root>`
	pairs := "trancode:NEW"

	result, err := Parametrize(ctx, template, pairs)
	if err != nil {
		t.Fatalf("Parametrize failed: %v", err)
	}

	if !strings.Contains(result, "<trancode>NEW</trancode>") {
		t.Errorf("expected <trancode>NEW</trancode> in result, got %q", result)
	}
	if strings.Contains(result, ">OLD<") {
		t.Errorf("old value should have been replaced, got %q", result)
	}
}

func TestParametrize_InvalidPairFormat(t *testing.T) {
	ctx := context.New()
	template := `<root><name>old</name></root>`

	// No colon means invalid pair, should be skipped
	result, err := Parametrize(ctx, template, "justtext")
	if err != nil {
		t.Fatalf("Parametrize failed: %v", err)
	}

	if result != template {
		t.Errorf("expected unchanged template, got %q", result)
	}
}

func TestParametrize_MultiplePairsOneInvalid(t *testing.T) {
	ctx := context.New()
	template := `<root><name>old</name><city>town</city></root>`
	pairs := "name:Bob;invalidpair;city:Shanghai"

	result, err := Parametrize(ctx, template, pairs)
	if err != nil {
		t.Fatalf("Parametrize failed: %v", err)
	}

	if !strings.Contains(result, "<name>Bob</name>") {
		t.Errorf("expected <name>Bob</name> in result, got %q", result)
	}
	if !strings.Contains(result, "<city>Shanghai</city>") {
		t.Errorf("expected <city>Shanghai</city> in result, got %q", result)
	}
}

func TestParametrize_ValueWithColon(t *testing.T) {
	ctx := context.New()
	template := `<root><time>00:00:00</time></root>`
	pairs := "time:12:30:45"

	result, err := Parametrize(ctx, template, pairs)
	if err != nil {
		t.Fatalf("Parametrize failed: %v", err)
	}

	// time:12:30:45 is parsed as xpath="time", value="12:30:45"
	// The first colon separates key from value
	if !strings.Contains(result, "12:30:45") {
		t.Errorf("expected 12:30:45 in result, got %q", result)
	}
	if strings.Contains(result, ">00:00:00<") {
		t.Errorf("old value should have been replaced, got %q", result)
	}
}