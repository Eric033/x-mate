package sampler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ---- DefaultHTTPConfig ----

func TestDefaultHTTPConfig(t *testing.T) {
	cfg := DefaultHTTPConfig()
	if cfg.Timeout != 60*time.Second {
		t.Errorf("Timeout = %v, want 60s", cfg.Timeout)
	}
	if cfg.Headers == nil {
		t.Error("Headers map should be initialized (non-nil)")
	}
	if cfg.Method != "" {
		t.Errorf("Method = %q, want empty", cfg.Method)
	}
}

// ---- HTTPSend tests ----

func TestHTTPSend_GET(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	cfg := HTTPConfig{
		Method:  http.MethodGet,
		URL:     srv.URL + "/test",
		Timeout: 5 * time.Second,
	}

	resp, err := HTTPSend(cfg)
	if err != nil {
		t.Fatalf("HTTPSend failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if resp.Body != `{"status":"ok"}` {
		t.Errorf("Body = %q, want %q", resp.Body, `{"status":"ok"}`)
	}
}

func TestHTTPSend_POSTWithHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Authorization") != "Bearer token123" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}

		// Echo back the body
		var bodyMap map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&bodyMap); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if bodyMap["key"] != "value" {
			t.Errorf("body key = %v", bodyMap["key"])
		}

		w.Header().Set("X-Custom", "yes")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"result":"created"}`))
	}))
	defer srv.Close()

	cfg := HTTPConfig{
		Method: http.MethodPost,
		URL:    srv.URL + "/api/data",
		Headers: map[string]string{
			"Content-Type":  "application/json",
			"Authorization": "Bearer token123",
		},
		Body:    `{"key":"value"}`,
		Timeout: 5 * time.Second,
	}

	resp, err := HTTPSend(cfg)
	if err != nil {
		t.Fatalf("HTTPSend failed: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	if resp.Headers.Get("X-Custom") != "yes" {
		t.Errorf("X-Custom header = %q", resp.Headers.Get("X-Custom"))
	}
	if resp.Body != `{"result":"created"}` {
		t.Errorf("Body = %q, want %q", resp.Body, `{"result":"created"}`)
	}
}

func TestHTTPSend_QueryParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check query params
		if r.URL.Query().Get("q") != "search" {
			t.Errorf("q = %q", r.URL.Query().Get("q"))
		}
		if r.URL.Query().Get("page") != "2" {
			t.Errorf("page = %q", r.URL.Query().Get("page"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()

	cfg := HTTPConfig{
		Method: http.MethodGet,
		URL:    srv.URL + "/search",
		QueryParams: map[string]string{
			"q":    "search",
			"page": "2",
		},
		Timeout: 5 * time.Second,
	}

	resp, err := HTTPSend(cfg)
	if err != nil {
		t.Fatalf("HTTPSend failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d", resp.StatusCode)
	}
}

func TestHTTPSend_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`404`))
	}))
	defer srv.Close()

	cfg := HTTPConfig{
		Method:  http.MethodGet,
		URL:     srv.URL + "/missing",
		Timeout: 5 * time.Second,
	}

	resp, err := HTTPSend(cfg)
	if err != nil {
		t.Fatalf("HTTPSend failed: %v", err)
	}

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestHTTPSend_InvalidURL(t *testing.T) {
	cfg := HTTPConfig{
		Method:  http.MethodGet,
		URL:     "://invalid-url",
		Timeout: 5 * time.Second,
	}

	_, err := HTTPSend(cfg)
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

func TestHTTPSend_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal"}`)
	}))
	defer srv.Close()

	cfg := HTTPConfig{
		Method:  http.MethodGet,
		URL:     srv.URL + "/error",
		Timeout: 5 * time.Second,
	}

	resp, err := HTTPSend(cfg)
	if err != nil {
		t.Fatalf("HTTPSend failed: %v", err)
	}

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d, want 500", resp.StatusCode)
	}
	if resp.Body != `{"error":"internal"}` {
		t.Errorf("Body = %q", resp.Body)
	}
}

func TestHTTPSend_WithExistingQueryParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("existing") != "param" {
			t.Errorf("existing param = %q", r.URL.Query().Get("existing"))
		}
		if r.URL.Query().Get("extra") != "value" {
			t.Errorf("extra param = %q", r.URL.Query().Get("extra"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	cfg := HTTPConfig{
		Method: http.MethodGet,
		URL:    srv.URL + "/path?existing=param",
		QueryParams: map[string]string{
			"extra": "value",
		},
		Timeout: 5 * time.Second,
	}

	resp, err := HTTPSend(cfg)
	if err != nil {
		t.Fatalf("HTTPSend failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d", resp.StatusCode)
	}
}