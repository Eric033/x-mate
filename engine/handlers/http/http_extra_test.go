package http

import (
	"testing"
)

func TestSplitHostPort(t *testing.T) {
	tests := []struct {
		input    string
		wantIP   string
		wantPort string
	}{
		{"10.0.0.1:8080", "10.0.0.1", "8080"},
		{"192.168.1.1:9999", "192.168.1.1", "9999"},
		{"no-port", "no-port", ""},
		{"", "", ""},
		{"host:0", "host", "0"},
		{"::1:8080", "::1", "8080"},
		{"[::1]:8080", "[::1]", "8080"},
	}
	for _, tt := range tests {
		ip, port := splitHostPort(tt.input)
		if ip != tt.wantIP || port != tt.wantPort {
			t.Errorf("splitHostPort(%q) = (%q, %q), want (%q, %q)",
				tt.input, ip, port, tt.wantIP, tt.wantPort)
		}
	}
}

func TestJsonpathGet(t *testing.T) {
	json := `{"ret_code":"000000","ret_msg":"success","data":{"user_id":"U001","amount":"5000"},"items":[{"id":1}]}`

	tests := []struct {
		path string
		want string
	}{
		{"$.ret_code", "000000"},
		{"$.ret_msg", "success"},
		{"$.data.user_id", "U001"},
		{"$.data.amount", "5000"},
		{"$.nonexistent", ""},
	}

	for _, tt := range tests {
		got := jsonpathGet(tt.path, json)
		if got != tt.want {
			t.Errorf("jsonpathGet(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestJsonpathGet_NumericValue(t *testing.T) {
	json := `{"count":42,"price":9.99,"active":true}`
	tests := []struct {
		path string
		want string
	}{
		{"$.count", "42"},
		{"$.price", "9.99"},
		{"$.active", "true"},
	}
	for _, tt := range tests {
		got := jsonpathGet(tt.path, json)
		if got != tt.want {
			t.Errorf("jsonpathGet(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestJsonpathGet_EmptyJson(t *testing.T) {
	if got := jsonpathGet("$.key", ""); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestHtmlUnescape(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"&amp;", "&"},
		{"&lt;", "<"},
		{"&gt;", ">"},
		{"&quot;", `"`},
		{"&#39;", "'"},
		{"no entities", "no entities"},
		{"mixed &amp; &lt; stuff", "mixed & < stuff"},
	}
	for _, tt := range tests {
		got := htmlUnescape(tt.input)
		if got != tt.want {
			t.Errorf("htmlUnescape(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
