package xmlhelper

import (
	"errors"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// parseXPathTags
// ---------------------------------------------------------------------------

func TestParseXPathTags_Single(t *testing.T) {
	tags := parseXPathTags("//TRAN_CODE")
	if len(tags) != 1 || tags[0] != "TRAN_CODE" {
		t.Fatalf("expected ['TRAN_CODE'], got %v", tags)
	}
}

func TestParseXPathTags_ChildPath(t *testing.T) {
	tags := parseXPathTags("//Header/TRAN_CODE")
	if len(tags) != 2 || tags[0] != "Header" || tags[1] != "TRAN_CODE" {
		t.Fatalf("expected ['Header', 'TRAN_CODE'], got %v", tags)
	}
}

func TestParseXPathTags_AbsolutePath(t *testing.T) {
	tags := parseXPathTags("/Root/Header/TRAN_CODE")
	if len(tags) != 3 || tags[0] != "Root" || tags[1] != "Header" || tags[2] != "TRAN_CODE" {
		t.Fatalf("expected ['Root', 'Header', 'TRAN_CODE'], got %v", tags)
	}
}

func TestParseXPathTags_WithPredicate(t *testing.T) {
	tags := parseXPathTags("//Tag[1]")
	if len(tags) != 1 || tags[0] != "Tag" {
		t.Fatalf("expected ['Tag'], got %v", tags)
	}
}

func TestParseXPathTags_WithPredicateInMiddle(t *testing.T) {
	tags := parseXPathTags("//Parent[1]/Child")
	if len(tags) != 2 || tags[0] != "Parent" || tags[1] != "Child" {
		t.Fatalf("expected ['Parent', 'Child'], got %v", tags)
	}
}

func TestParseXPathTags_Empty(t *testing.T) {
	tags := parseXPathTags("")
	if len(tags) != 0 {
		t.Fatalf("expected empty, got %v", tags)
	}
}

func TestParseXPathTags_SlashOnly(t *testing.T) {
	tags := parseXPathTags("//")
	if len(tags) != 0 {
		t.Fatalf("expected empty for '//', got %v", tags)
	}
}

func TestParseXPathTags_WithSpaces(t *testing.T) {
	tags := parseXPathTags("// Header / TRAN_CODE ")
	if len(tags) != 2 || tags[0] != "Header" || tags[1] != "TRAN_CODE" {
		t.Fatalf("expected ['Header', 'TRAN_CODE'], got %v", tags)
	}
}

// ---------------------------------------------------------------------------
// Set - New strict XML engine
// ---------------------------------------------------------------------------

func TestSet_Basic(t *testing.T) {
	xmlStr := `<root><name>OldName</name></root>`
	result, err := Set("//name", "NewName", xmlStr)
	if err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	val, err := Get("//name", result)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if val != "NewName" {
		t.Fatalf("expected 'NewName', got %q", val)
	}
}

func TestSet_ChildPath(t *testing.T) {
	xmlStr := `<root><header><code>ABC</code></header></root>`
	result, err := Set("//header/code", "XYZ", xmlStr)
	if err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	val, err := Get("//header/code", result)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if val != "XYZ" {
		t.Fatalf("expected 'XYZ', got %q", val)
	}
}

func TestSet_NonExistentTag(t *testing.T) {
	xmlStr := `<root><item>val</item></root>`
	_, err := Set("//missing", "newval", xmlStr)
	if !errors.Is(err, ErrXPathNotFound) {
		t.Fatalf("expected ErrXPathNotFound, got %v", err)
	}
}

func TestSet_EmptyXML(t *testing.T) {
	_, err := Set("//tag", "val", "")
	if !errors.Is(err, ErrXMLParse) {
		t.Fatalf("expected ErrXMLParse, got %v", err)
	}
}

func TestSet_MultipleNodes(t *testing.T) {
	xmlStr := `<root><item>A</item><item>B</item></root>`
	result, err := Set("//item", "X", xmlStr)
	if err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	val, err := Get("//item", result)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if val != "X" {
		t.Fatalf("expected first item to be 'X', got %q", val)
	}
}

func TestSet_InvalidXML(t *testing.T) {
	_, err := Set("//tag", "val", "<root><unclosed>")
	if !errors.Is(err, ErrXMLParse) {
		t.Fatalf("expected ErrXMLParse for invalid XML, got %v", err)
	}
}

func TestSet_AbsolutePath(t *testing.T) {
	xmlStr := `<root><child>old</child></root>`
	result, err := Set("/root/child", "new", xmlStr)
	if err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	val, err := Get("/root/child", result)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if val != "new" {
		t.Fatalf("expected 'new', got %q", val)
	}
}

func TestSet_NodeWithAttributes(t *testing.T) {
	xmlStr := `<root><elem attr="val">text</elem></root>`
	result, err := Set("//elem", "newtext", xmlStr)
	if err != nil {
		t.Fatalf("Set error: %v", err)
	}

	val, err := Get("//elem", result)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if val != "newtext" {
		t.Fatalf("expected 'newtext', got %q", val)
	}
}

func TestSet_CaseSensitive(t *testing.T) {
	// Strict XML is case-sensitive — uppercase XPath won't match lowercase tag
	xmlStr := `<root><trancode>OLD</trancode></root>`
	_, err := Set("//TRANCODE", "NEW", xmlStr)
	if !errors.Is(err, ErrXPathNotFound) {
		t.Fatalf("expected ErrXPathNotFound for case mismatch, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Get - New strict XML engine
// ---------------------------------------------------------------------------

func TestGet_Basic(t *testing.T) {
	xmlStr := `<root><name>Alice</name></root>`
	val, err := Get("//name", xmlStr)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if val != "Alice" {
		t.Fatalf("expected 'Alice', got %q", val)
	}
}

func TestGet_ChildPath(t *testing.T) {
	xmlStr := `<root><header><code>XYZ</code></header></root>`
	val, err := Get("//header/code", xmlStr)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if val != "XYZ" {
		t.Fatalf("expected 'XYZ', got %q", val)
	}
}

func TestGet_NonExistentTag(t *testing.T) {
	xmlStr := `<root><item>val</item></root>`
	_, err := Get("//missing", xmlStr)
	if !errors.Is(err, ErrXPathNotFound) {
		t.Fatalf("expected ErrXPathNotFound, got %v", err)
	}
}

func TestGet_EmptyXML(t *testing.T) {
	_, err := Get("//tag", "")
	if !errors.Is(err, ErrXMLParse) {
		t.Fatalf("expected ErrXMLParse, got %v", err)
	}
}

func TestGet_AbsolutePath(t *testing.T) {
	xmlStr := `<root><child>data</child></root>`
	val, err := Get("/root/child", xmlStr)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if val != "data" {
		t.Fatalf("expected 'data', got %q", val)
	}
}

func TestGet_DeepNested(t *testing.T) {
	xmlStr := `<root><a><b><c>deep</c></b></a></root>`
	val, err := Get("//a/b/c", xmlStr)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if val != "deep" {
		t.Fatalf("expected 'deep', got %q", val)
	}
}

func TestGet_CaseSensitive(t *testing.T) {
	// Strict XML is case-sensitive; uppercase XPath won't match lowercase tag
	xmlStr := `<Request><Header><TranCode>1001</TranCode></Header></Request>`
	val, err := Get("//Header/TranCode", xmlStr)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if val != "1001" {
		t.Fatalf("expected '1001', got %q", val)
	}
}

func TestGet_CaseSensitiveMismatch(t *testing.T) {
	xmlStr := `<Request><Header><TranCode>1001</TranCode></Header></Request>`
	_, err := Get("//header/trancode", xmlStr)
	if !errors.Is(err, ErrXPathNotFound) {
		t.Fatalf("expected ErrXPathNotFound for case mismatch, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Set + Get integration
// ---------------------------------------------------------------------------

func TestSetThenGet(t *testing.T) {
	xmlStr := `<root><item>original</item></root>`
	modified, err := Set("//item", "updated", xmlStr)
	if err != nil {
		t.Fatalf("Set error: %v", err)
	}

	val, err := Get("//item", modified)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if val != "updated" {
		t.Fatalf("expected 'updated', got %q", val)
	}
}

func TestSetThenGet_ChildPath(t *testing.T) {
	xmlStr := `<Envelope><Body><Data>old</Data></Body></Envelope>`
	modified, err := Set("//Body/Data", "new", xmlStr)
	if err != nil {
		t.Fatalf("Set error: %v", err)
	}

	val, err := Get("//Body/Data", modified)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if val != "new" {
		t.Fatalf("expected 'new', got %q", val)
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestGet_MultipleTextNodes(t *testing.T) {
	xmlStr := `<root><msg>Hello <b>World</b>!</msg></root>`
	val, err := Get("//msg", xmlStr)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	// InnerText concatenates all text content; whitespace may vary
	if !strings.Contains(val, "Hello") || !strings.Contains(val, "World") {
		t.Fatalf("expected msg to contain 'Hello' and 'World', got %q", val)
	}
}

func TestGet_Whitespace(t *testing.T) {
	xmlStr := `<root>  <item>  spaced  </item>  </root>`
	val, err := Get("//item", xmlStr)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	// InnerText does NOT trim whitespace; but we TrimSpace in Get
	if val != "spaced" {
		t.Fatalf("expected 'spaced', got %q", val)
	}
}

func TestSet_NestedSetGetCycle(t *testing.T) {
	xmlStr := `<Request><Body><TranCode>OLD</TranCode></Body></Request>`

	r1, err := Set("//Body/TranCode", "MID", xmlStr)
	if err != nil {
		t.Fatalf("first Set error: %v", err)
	}
	val1, err := Get("//Body/TranCode", r1)
	if err != nil {
		t.Fatalf("first Get error: %v", err)
	}
	if val1 != "MID" {
		t.Fatalf("after first set: expected 'MID', got %q", val1)
	}

	r2, err := Set("//Body/TranCode", "FINAL", r1)
	if err != nil {
		t.Fatalf("second Set error: %v", err)
	}
	val2, err := Get("//Body/TranCode", r2)
	if err != nil {
		t.Fatalf("second Get error: %v", err)
	}
	if val2 != "FINAL" {
		t.Fatalf("after second set: expected 'FINAL', got %q", val2)
	}
}
