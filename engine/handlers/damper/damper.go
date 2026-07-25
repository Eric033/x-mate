package damper

import (
	"fmt"
	"strings"
	"time"

	"github.com/Eric033/x-mate/engine/internal/context"
	"github.com/Eric033/x-mate/engine/internal/handler"
	"github.com/Eric033/x-mate/engine/internal/sampler"
	"github.com/Eric033/x-mate/engine/internal/template"
	"github.com/Eric033/x-mate/engine/internal/vars"
	"github.com/Eric033/x-mate/engine/internal/xmlhelper"
)

// verifyAssertions performs XPath-based verification for damper handlers.
func verifyAssertions(response string, assertions []handler.Assertion, ctx *context.TestContext) []string {
	var failures []string
	for _, a := range assertions {
		if a.XPath == "" {
			continue
		}
		actual, err := xmlhelper.Get(a.XPath, response)
		if err != nil {
			failures = append(failures,
				fmt.Sprintf("[ tag %s mismatch: %v ]", a.XPath, err))
			continue
		}
		expected := vars.ResolveAll(ctx, a.Expected)
		if actual != expected {
			failures = append(failures,
				fmt.Sprintf("[ tag %s mismatch: expected=%s actual=%s ]", a.XPath, expected, actual))
		}
	}
	return failures
}

// TCPDamperSetHandler handles tcp_damper_set step type.
type TCPDamperSetHandler struct{}

func (h *TCPDamperSetHandler) Execute(data *handler.StepData, ctx *context.TestContext) *handler.StepResult {
	if data.Sleep > 0 {
		time.Sleep(time.Duration(data.Sleep) * time.Millisecond)
	}

	// Load and parametrize template
	tmplXML, err := template.LoadTemplate(ctx.TestBase, data.TranCode)
	if err != nil {
		return &handler.StepResult{Success: false, FailureMessage: err.Error()}
	}

	valuesStr := handler.ValuesToString(data.Values)
	parametrized, err := template.Parametrize(ctx, tmplXML, valuesStr)
	if err != nil {
		return &handler.StepResult{Success: false, FailureMessage: err.Error()}
	}

	// Add @ prefix to TRAN_CODE (if present)
	tid, err := xmlhelper.Get("//TRAN_CODE", parametrized)
	if err == nil {
		parametrized, err = xmlhelper.Set("//TRAN_CODE", "@"+tid, parametrized)
		if err != nil {
			return &handler.StepResult{Success: false, FailureMessage: err.Error(), RequestData: parametrized}
		}
	}

	// Send to damper TCP server (service name or legacy)
	addr := ""
	if svc, ok := ctx.GetService("DAMPER"); ok {
		if svc.TCPPort > 0 {
			ip, _ := splitHostPort(svc.Address)
			addr = fmt.Sprintf("%s:%d", ip, svc.TCPPort)
		} else {
			addr = svc.Address
		}
	}
	if addr == "" {
		addr = ctx.GetOrDefault("tcpDamServerIP", "") + ":" + ctx.GetOrDefault("tcpDamServerPort", "")
	}

	cfg := sampler.DefaultTCPConfig()
	cfg.CloseConnection = true

	respBytes, err := sampler.TCPSend(addr, []byte(parametrized), cfg)
	if err != nil {
		return &handler.StepResult{Success: false, FailureMessage: err.Error(), RequestData: parametrized}
	}

	response := string(respBytes)
	// Strip first 6 bytes (BCD prefix)
	if len(response) > 6 {
		response = response[6:]
	}

	// Verify using unified Assertions
	ok := true
	var failureMsg string
	if len(data.Assertions) > 0 {
		failures := verifyAssertions(response, data.Assertions, ctx)
		if len(failures) > 0 {
			ok = false
			failureMsg = strings.Join(failures, "")
		}
	}

	// Extract
	ctx.Set("prevResult", response)
	for _, s := range data.Saves {
		val, err := xmlhelper.Get(s.Locator, response)
		if err == nil && val != "" {
			ctx.Set(s.Name, val)
		}
	}

	return &handler.StepResult{
		Success:        ok,
		FailureMessage: failureMsg,
		ResponseData:   response,
		RequestData:    parametrized,
	}
}

// MCADamperSetHandler handles mca_damper_set step type.
type MCADamperSetHandler struct{}

func (h *MCADamperSetHandler) Execute(data *handler.StepData, ctx *context.TestContext) *handler.StepResult {
	if data.Sleep > 0 {
		time.Sleep(time.Duration(data.Sleep) * time.Millisecond)
	}

	// Load and parametrize template
	tmplXML, err := template.LoadTemplate(ctx.TestBase, data.TranCode)
	if err != nil {
		return &handler.StepResult{Success: false, FailureMessage: err.Error()}
	}

	valuesStr := handler.ValuesToString(data.Values)
	parametrized, err := template.Parametrize(ctx, tmplXML, valuesStr)
	if err != nil {
		return &handler.StepResult{Success: false, FailureMessage: err.Error()}
	}

	// Add @ prefix to _TransactionId (if present)
	tid, err := xmlhelper.Get("//_TransactionId", parametrized)
	if err == nil {
		parametrized, err = xmlhelper.Set("//_TransactionId", "@"+tid, parametrized)
		if err != nil {
			return &handler.StepResult{Success: false, FailureMessage: err.Error(), RequestData: parametrized}
		}
	}

	// Append \r\n
	payload := append([]byte(parametrized), '\r', '\n')

	// Send to damper TCP server (service name or legacy)
	addr := ""
	if svc, ok := ctx.GetService("DAMPER"); ok {
		if svc.TCPPort > 0 {
			ip, _ := splitHostPort(svc.Address)
			addr = fmt.Sprintf("%s:%d", ip, svc.TCPPort)
		} else {
			addr = svc.Address
		}
	}
	if addr == "" {
		addr = ctx.GetOrDefault("tcpDamServerIP", "") + ":" + ctx.GetOrDefault("tcpDamServerPort", "")
	}

	cfg := sampler.DefaultTCPConfig()
	cfg.CloseConnection = true

	respBytes, err := sampler.TCPSend(addr, payload, cfg)
	if err != nil {
		return &handler.StepResult{Success: false, FailureMessage: err.Error(), RequestData: parametrized}
	}

	response := string(respBytes)
	// Strip trailing \r\n
	if len(response) >= 2 {
		response = response[:len(response)-2]
	}

	// Verify using unified Assertions
	ok := true
	var failureMsg string
	if len(data.Assertions) > 0 {
		failures := verifyAssertions(response, data.Assertions, ctx)
		if len(failures) > 0 {
			ok = false
			failureMsg = strings.Join(failures, "")
		}
	}

	// Extract
	ctx.Set("prevResult", response)
	for _, s := range data.Saves {
		val, err := xmlhelper.Get(s.Locator, response)
		if err == nil && val != "" {
			ctx.Set(s.Name, val)
		}
	}

	return &handler.StepResult{
		Success:        ok,
		FailureMessage: failureMsg,
		ResponseData:   response,
		RequestData:    parametrized,
	}
}

// getTranCode extracts //TRAN_CODE text from XML.
func getTranCode(xml string) string {
	val, err := xmlhelper.Get("//TRAN_CODE", xml)
	if err != nil {
		return ""
	}
	return val
}

// splitHostPort splits "ip:port" into (ip, port).
func splitHostPort(addr string) (string, string) {
	colonIdx := strings.LastIndex(addr, ":")
	if colonIdx < 0 {
		return addr, ""
	}
	return addr[:colonIdx], addr[colonIdx+1:]
}


