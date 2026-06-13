package tcp

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Eric033/x-mate/engine/internal/context"
	"github.com/Eric033/x-mate/engine/internal/handler"
)

// startMockTCPServer starts a goroutine TCP listener that echoes back the received data,
// optionally applying a BCD-length prefix offset and trailing EOL byte behavior.
func startMockTCPServer(t *testing.T, responsePrefix string, eolByte byte) (net.Listener, string) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock TCP server: %v", err)
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 4096)
				n, err := c.Read(buf)
				if err != nil {
					return
				}
				received := buf[:n]

				var response []byte
				if responsePrefix != "" {
					response = append(response, []byte(responsePrefix)...)
				}
				payloadStart := 0
				if len(received) >= 8 && allDigits(string(received[:8])) {
					payloadStart = 8
				}
				response = append(response, received[payloadStart:]...)

				if eolByte != 0 {
					response = append(response, eolByte)
				}

				c.SetWriteDeadline(time.Now().Add(time.Second))
				c.Write(response)
			}(conn)
		}
	}()

	return listener, listener.Addr().String()
}

func allDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// setupTempTestBase creates a temporary directory with template files for testing.
// NOTE: xmlhelper uses html.Parse which lowercases all tag names.
// So XPath expressions must use lowercase tags.
func setupTempTestBase(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir := t.TempDir()

	tmplDir := filepath.Join(tmpDir, "template")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatalf("failed to create template dir: %v", err)
	}

	tmplContent := `<?xml version="1.0" encoding="UTF-8"?>
<root>
	<tran_code>T001</tran_code>
	<username>__USERNAME__</username>
	<password>__PASSWORD__</password>
</root>`
	if err := os.WriteFile(filepath.Join(tmplDir, "template_T001.xml"), []byte(tmplContent), 0644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

// Test XMLSet8Handler - uses 8-byte BCD prefix, 8-byte response offset, EOL '>'
func TestXMLSet8Handler_Execute_Success(t *testing.T) {
	tmpDir, cleanup := setupTempTestBase(t)
	defer cleanup()

	listener, addr := startMockTCPServer(t, "00000042", 0x3E)
	defer listener.Close()

	parts := strings.Split(addr, ":")
	serverIP := parts[0]
	serverPort := parts[1]

	ctx := context.New()
	ctx.Set("serverIP", serverIP)
	ctx.Set("serverPort", serverPort)
	ctx.TestBase = tmpDir

	data := &handler.StepData{
		StepType: "xml_set_8",
		TranCode: "T001",
		Values: []handler.KV{
			{Key: "//username", Value: "admin"},
			{Key: "//password", Value: "secret"},
		},
		Results: []handler.KV{
			{Key: "//username", Value: "admin"},
			{Key: "//password", Value: "secret"},
		},
	}

	h := &XMLSet8Handler{}
	result := h.Execute(data, ctx)

	if !result.Success {
		t.Fatalf("expected success, got failure: %s", result.FailureMessage)
	}
	if result.RequestData == "" {
		t.Error("expected non-empty RequestData")
	}
	if result.ResponseData == "" {
		t.Error("expected non-empty ResponseData")
	}
}

func TestXMLSet8Handler_Execute_ConnectionRefused(t *testing.T) {
	tmpDir, cleanup := setupTempTestBase(t)
	defer cleanup()

	ctx := context.New()
	ctx.Set("serverIP", "127.0.0.1")
	ctx.Set("serverPort", "19999")
	ctx.TestBase = tmpDir

	data := &handler.StepData{
		StepType: "xml_set_8",
		TranCode: "T001",
		Values: []handler.KV{
			{Key: "//username", Value: "admin"},
		},
	}

	h := &XMLSet8Handler{}
	result := h.Execute(data, ctx)

	if result.Success {
		t.Fatal("expected failure due to connection refused")
	}
	if result.FailureMessage == "" {
		t.Error("expected non-empty failure message")
	}
}

func TestXMLSet8Handler_Execute_TemplateNotFound(t *testing.T) {
	ctx := context.New()
	ctx.Set("serverIP", "127.0.0.1")
	ctx.Set("serverPort", "19999")
	ctx.TestBase = t.TempDir()

	data := &handler.StepData{
		StepType: "xml_set_8",
		TranCode: "NONEXISTENT",
	}

	h := &XMLSet8Handler{}
	result := h.Execute(data, ctx)

	if result.Success {
		t.Fatal("expected failure due to missing template")
	}
	if result.FailureMessage == "" {
		t.Error("expected non-empty failure message")
	}
}

// Test XMLSetSASHandler - 6-byte response offset, no manual length prefix
func TestXMLSetSASHandler_Execute_Success(t *testing.T) {
	tmpDir, cleanup := setupTempTestBase(t)
	defer cleanup()

	listener, addr := startMockTCPServer(t, "000000", 0)
	defer listener.Close()

	parts := strings.Split(addr, ":")
	serverIP := parts[0]
	serverPort := parts[1]

	ctx := context.New()
	ctx.Set("serverIP", serverIP)
	ctx.Set("serverPort", serverPort)
	ctx.TestBase = tmpDir

	data := &handler.StepData{
		StepType: "xml_set_sas",
		TranCode: "T001",
		Values: []handler.KV{
			{Key: "//username", Value: "sas_user"},
		},
		Results: []handler.KV{
			{Key: "//username", Value: "sas_user"},
		},
	}

	h := &XMLSetSASHandler{}
	result := h.Execute(data, ctx)

	if !result.Success {
		t.Fatalf("expected success, got failure: %s", result.FailureMessage)
	}
}

func TestXMLSetSASHandler_Execute_VerifyMismatch(t *testing.T) {
	tmpDir, cleanup := setupTempTestBase(t)
	defer cleanup()

	listener, addr := startMockTCPServer(t, "000000", 0)
	defer listener.Close()

	parts := strings.Split(addr, ":")
	serverIP := parts[0]
	serverPort := parts[1]

	ctx := context.New()
	ctx.Set("serverIP", serverIP)
	ctx.Set("serverPort", serverPort)
	ctx.TestBase = tmpDir

	data := &handler.StepData{
		StepType: "xml_set_sas",
		TranCode: "T001",
		Values: []handler.KV{
			{Key: "//username", Value: "admin"},
		},
		Results: []handler.KV{
			{Key: "//username", Value: "WRONG_VALUE"},
		},
	}

	h := &XMLSetSASHandler{}
	result := h.Execute(data, ctx)

	if result.Success {
		t.Fatal("expected failure due to verify mismatch")
	}
	if !strings.Contains(result.FailureMessage, "mismatch") {
		t.Errorf("failure message should contain 'mismatch', got: %s", result.FailureMessage)
	}
}

// Test XMLSetHandler - standard BCD, 6-byte response offset
func TestXMLSetHandler_Execute_Success(t *testing.T) {
	tmpDir, cleanup := setupTempTestBase(t)
	defer cleanup()

	listener, addr := startMockTCPServer(t, "000000", 0)
	defer listener.Close()

	parts := strings.Split(addr, ":")
	serverIP := parts[0]
	serverPort := parts[1]

	ctx := context.New()
	ctx.Set("serverIP", serverIP)
	ctx.Set("serverPort", serverPort)
	ctx.TestBase = tmpDir

	data := &handler.StepData{
		StepType: "xml_set",
		TranCode: "T001",
		Values: []handler.KV{
			{Key: "//username", Value: "test_user"},
		},
	}

	h := &XMLSetHandler{}
	result := h.Execute(data, ctx)

	if !result.Success {
		t.Fatalf("expected success, got failure: %s", result.FailureMessage)
	}
}

func TestXMLSetHandler_Execute_SaveVar(t *testing.T) {
	tmpDir, cleanup := setupTempTestBase(t)
	defer cleanup()

	listener, addr := startMockTCPServer(t, "000000", 0)
	defer listener.Close()

	parts := strings.Split(addr, ":")
	serverIP := parts[0]
	serverPort := parts[1]

	ctx := context.New()
	ctx.Set("serverIP", serverIP)
	ctx.Set("serverPort", serverPort)
	ctx.TestBase = tmpDir

	data := &handler.StepData{
		StepType: "xml_set",
		TranCode: "T001",
		Values: []handler.KV{
			{Key: "//username", Value: "saved_user"},
		},
		Saves: []handler.SaveEntry{
			{Name: "extracted_user", Locator: "//username"},
		},
	}

	h := &XMLSetHandler{}
	result := h.Execute(data, ctx)

	if !result.Success {
		t.Fatalf("expected success, got failure: %s", result.FailureMessage)
	}

	if val, ok := ctx.Get("extracted_user"); !ok {
		t.Error("expected 'extracted_user' to be saved in context")
	} else if val != "saved_user" {
		t.Errorf("expected extracted_user='saved_user', got '%s'", val)
	}

	if _, ok := ctx.Get("prevResult"); !ok {
		t.Error("expected 'prevResult' to be set in context")
	}
}

// Test MCAHandler - appends \r\n, no length prefix, strips trailing \r\n from response
func TestMCAHandler_Execute_Success(t *testing.T) {
	tmpDir, cleanup := setupTempTestBase(t)
	defer cleanup()

	listener, addr := startMockTCPServer(t, "", 0)
	defer listener.Close()

	parts := strings.Split(addr, ":")
	serverIP := parts[0]
	serverPort := parts[1]

	ctx := context.New()
	ctx.Set("serverIP", serverIP)
	ctx.Set("serverPort", serverPort)
	ctx.TestBase = tmpDir

	data := &handler.StepData{
		StepType: "mca",
		TranCode: "T001",
		Values: []handler.KV{
			{Key: "//username", Value: "mca_user"},
		},
	}

	h := &MCAHandler{}
	result := h.Execute(data, ctx)

	if !result.Success {
		t.Fatalf("expected success, got failure: %s", result.FailureMessage)
	}
	if result.RequestData == "" {
		t.Error("expected non-empty RequestData")
	}
}

func TestMCAHandler_Execute_VerifyMismatch(t *testing.T) {
	tmpDir, cleanup := setupTempTestBase(t)
	defer cleanup()

	listener, addr := startMockTCPServer(t, "", 0)
	defer listener.Close()

	parts := strings.Split(addr, ":")
	serverIP := parts[0]
	serverPort := parts[1]

	ctx := context.New()
	ctx.Set("serverIP", serverIP)
	ctx.Set("serverPort", serverPort)
	ctx.TestBase = tmpDir

	data := &handler.StepData{
		StepType: "mca",
		TranCode: "T001",
		Values: []handler.KV{
			{Key: "//username", Value: "admin"},
		},
		Results: []handler.KV{
			{Key: "//username", Value: "WRONG"},
		},
	}

	h := &MCAHandler{}
	result := h.Execute(data, ctx)

	if result.Success {
		t.Fatal("expected failure due to verify mismatch")
	}
}

func TestMCAHandler_Execute_SaveVar(t *testing.T) {
	tmpDir, cleanup := setupTempTestBase(t)
	defer cleanup()

	listener, addr := startMockTCPServer(t, "", 0)
	defer listener.Close()

	parts := strings.Split(addr, ":")
	serverIP := parts[0]
	serverPort := parts[1]

	ctx := context.New()
	ctx.Set("serverIP", serverIP)
	ctx.Set("serverPort", serverPort)
	ctx.TestBase = tmpDir

	data := &handler.StepData{
		StepType: "mca",
		TranCode: "T001",
		Values: []handler.KV{
			{Key: "//username", Value: "mca_extract"},
		},
		Saves: []handler.SaveEntry{
			{Name: "mca_user", Locator: "//username"},
		},
	}

	h := &MCAHandler{}
	result := h.Execute(data, ctx)

	if !result.Success {
		t.Fatalf("expected success, got failure: %s", result.FailureMessage)
	}

	if val, ok := ctx.Get("mca_user"); !ok {
		t.Error("expected 'mca_user' to be saved in context")
	} else if val != "mca_extract" {
		t.Errorf("expected mca_user='mca_extract', got '%s'", val)
	}

	if _, ok := ctx.Get("prevResult"); !ok {
		t.Error("expected 'prevResult' to be set in context")
	}
}