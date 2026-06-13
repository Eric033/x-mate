package http

import (
	"fmt"
	"strings"

	"github.com/Eric033/x-mate/engine/internal/context"
	"github.com/Eric033/x-mate/engine/internal/handler"
	"github.com/Eric033/x-mate/engine/internal/sampler"
	"github.com/Eric033/x-mate/engine/internal/vars"
	"github.com/Eric033/x-mate/engine/internal/xmlhelper"
)

// splitHostPort splits "ip:port" into (ip, port).
func splitHostPort(addr string) (string, string) {
	colonIdx := strings.LastIndex(addr, ":")
	if colonIdx < 0 {
		return addr, ""
	}
	return addr[:colonIdx], addr[colonIdx+1:]
}

// jsonpathGet extracts a value from JSON using a simple JSONPath expression.
// Supports $.field.subfield and $[index] patterns.
func jsonpathGet(jsonPath, jsonStr string) string {
	// Simple implementation: split by . and navigate
	path := strings.TrimPrefix(jsonPath, "$")
	path = strings.TrimPrefix(path, ".")

	parts := strings.Split(path, ".")
	current := jsonStr

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// Search for "part": "value" pattern
		searchKey := fmt.Sprintf(`"%s"`, part)
		idx := strings.Index(current, searchKey)
		if idx < 0 {
			return ""
		}
		// Move past the key and find the value
		rest := current[idx+len(searchKey):]
		// Skip whitespace and colon
		rest = strings.TrimLeft(rest, " \t\n\r:")
		// Extract value
		if strings.HasPrefix(rest, `"`) {
			// String value
			rest = rest[1:]
			endIdx := strings.Index(rest, `"`)
			if endIdx < 0 {
				return ""
			}
			current = rest[:endIdx]
		} else {
			// Numeric or other value
			endIdx := strings.IndexAny(rest, ",}\n")
			if endIdx < 0 {
				current = strings.TrimSpace(rest)
			} else {
				current = strings.TrimSpace(rest[:endIdx])
			}
		}
	}
	return current
}

// HTTPHandler handles both "http" and "damper_set" step types.
type HTTPHandler struct {
	UseDamper bool // true for damper_set
}

func (h *HTTPHandler) Execute(data *handler.StepData, ctx *context.TestContext) *handler.StepResult {
	// Resolve server IP/port
	var serverIP, serverPort string

	if h.UseDamper {
		// Damper: look for DAMPER service or fallback to legacy
		if svc, ok := ctx.GetService("DAMPER"); ok && svc.HTTPPort > 0 {
			ip, _ := splitHostPort(svc.Address)
			serverIP = ip
			serverPort = fmt.Sprintf("%d", svc.HTTPPort)
		} else {
			serverIP = ctx.GetOrDefault("httpDamServerIP", "")
			serverPort = ctx.GetOrDefault("httpDamServerPort", "")
		}
	} else if data.Server != "" {
		// New: service name
		if svc, ok := ctx.GetService(data.Server); ok {
			ip, _ := splitHostPort(svc.Address)
			serverIP = ip
			serverPort = ""
			// port is set from the service's address, which will be overridden
			// by the full address or action attrs below
			if addr, ok := ctx.GetServiceAddrForStep(data.Server); ok {
				_, p := splitHostPort(addr)
				serverPort = p
			}
		}
	} else {
		// Legacy fallback
		if ip, ok := ctx.Get("serverIP"); ok {
			serverIP = ip
		}
		if port, ok := ctx.Get("serverPort"); ok {
			serverPort = port
		}
	}

	// Override with Action attrs if specified
	if ip, ok := data.Attrs["ip"]; ok && ip != "" {
		serverIP = ip
	}
	if port, ok := data.Attrs["port"]; ok && port != "" {
		serverPort = port
	}

	// Resolve API path
	apiPath := vars.ResolveAll(ctx, data.Attrs["api"])
	method := data.Attrs["method"]
	if method == "" {
		method = "GET"
	}

	// Build URL
	url := fmt.Sprintf("http://%s:%s%s", serverIP, serverPort, apiPath)

	// Build headers
	headers := make(map[string]string)
	for _, hdr := range data.Headers {
		val := vars.ResolveAll(ctx, hdr.Value)
		val = htmlUnescape(val)
		headers[hdr.Key] = val
	}

	// Build query params
	queryParams := make(map[string]string)
	for _, q := range data.QueryString {
		val := vars.ResolveAll(ctx, q.Value)
		queryParams[q.Key] = val
	}

	// Resolve body
	body := vars.ResolveAll(ctx, data.Body)

	// Send HTTP request
	cfg := sampler.HTTPConfig{
		Method:      method,
		URL:         url,
		Headers:     headers,
		QueryParams: queryParams,
		Body:        body,
	}
	cfg.Timeout = 60 * 1000 * 1000 * 1000 // 60s in ns

	resp, err := sampler.HTTPSend(cfg)
	if err != nil {
		return &handler.StepResult{Success: false, FailureMessage: err.Error()}
	}

	// Verify results
	ok := true
	var failureMsg string
	if len(data.VerifyResults) > 0 {
		ok, failureMsg = h.verify(resp, data.VerifyResults)
	}

	// Extract vars
	ctx.Set("prevResult", resp.Body)
	h.extractVars(resp.Body, data.Saves, ctx)

	return &handler.StepResult{
		Success:        ok,
		FailureMessage: failureMsg,
		ResponseData:   resp.Body,
		RequestData:    fmt.Sprintf("%s %s", method, url),
	}
}

func (h *HTTPHandler) verify(resp *sampler.HTTPResponse, entries []handler.VerifyEntry) (bool, string) {
	var failures []string
	for _, e := range entries {
		var actual string
		if e.IsHeader == "True" {
			actual = resp.Headers.Get(e.HeaderName)
		} else {
			// Body verification: JSONPath or XPath
			if strings.HasPrefix(e.Name, "$") {
				actual = jsonpathGet(e.Name, resp.Body)
			} else if strings.HasPrefix(e.Name, "/") {
				val, err := xmlhelper.Get(e.Name, resp.Body)
				if err == nil {
					actual = val
				}
			}
		}

		if actual != e.Value {
			failures = append(failures,
				fmt.Sprintf("[ tag %s mismatch: expected=%s actual=%s ]", e.Name, e.Value, actual))
		}
	}

	if len(failures) > 0 {
		return false, strings.Join(failures, "")
	}
	return true, ""
}

func (h *HTTPHandler) extractVars(body string, saves []handler.SaveEntry, ctx *context.TestContext) {
	for _, s := range saves {
		var val string
		if s.Locator == "PLAIN_TEXT" {
			val = body
		} else if strings.HasPrefix(s.Locator, "$") {
			val = jsonpathGet(s.Locator, body)
		}
		if val != "" {
			ctx.Set(s.Name, val)
		}
	}
}

// htmlUnescape unescapes HTML entities.
func htmlUnescape(s string) string {
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	s = strings.ReplaceAll(s, "&#39;", "'")
	return s
}
