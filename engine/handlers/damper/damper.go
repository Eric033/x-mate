package damper

import (
	"fmt"
	"strings"
	"time"

	"github.com/Eric033/x-mate/engine/internal/context"
	"github.com/Eric033/x-mate/engine/internal/handler"
	"github.com/Eric033/x-mate/engine/internal/sampler"
	"github.com/Eric033/x-mate/engine/internal/template"
	"github.com/Eric033/x-mate/engine/internal/xmlhelper"
)

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

	// Add @ prefix to TRAN_CODE
	parametrized, _ = xmlhelper.Set("//TRAN_CODE", "@"+getTranCode(parametrized), parametrized)

	// Send to damper TCP server
	addr := ctx.DamperTCP
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

	// Verify
	ok := true
	var failureMsg string
	for _, r := range data.Results {
		actual, _ := xmlhelper.Get(r.Key, response)
		if actual != r.Value {
			ok = false
			failureMsg += fmt.Sprintf("[ tag %s mismatch: expected=%s actual=%s ]", r.Key, r.Value, actual)
		}
	}

	// Extract
	ctx.Set("prevResult", response)
	for _, s := range data.Saves {
		val, _ := xmlhelper.Get(s.Locator, response)
		if val != "" {
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

	// Add @ prefix to _TransactionId
	tid, _ := xmlhelper.Get("//_TransactionId", parametrized)
	parametrized, _ = xmlhelper.Set("//_TransactionId", "@"+tid, parametrized)

	// Append \r\n
	payload := append([]byte(parametrized), '\r', '\n')

	// Send to damper TCP server
	addr := ctx.DamperTCP
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

	// Verify
	ok := true
	var failureMsg string
	for _, r := range data.Results {
		actual, _ := xmlhelper.Get(r.Key, response)
		if actual != r.Value {
			ok = false
			failureMsg += fmt.Sprintf("[ tag %s mismatch: expected=%s actual=%s ]", r.Key, r.Value, actual)
		}
	}

	// Extract
	ctx.Set("prevResult", response)
	for _, s := range data.Saves {
		val, _ := xmlhelper.Get(s.Locator, response)
		if val != "" {
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
	val, _ := xmlhelper.Get("//TRAN_CODE", xml)
	return val
}

var _ = strings.TrimSpace
