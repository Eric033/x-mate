package tcp

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

// Base contains common TCP handler logic.
type Base struct {
	ResponseOffset  int    // bytes to skip from response start (e.g., 6 or 8)
	LengthPrefix    int    // bytes for length prefix (0 = none, 8 = manual 8-byte)
	EolByte         byte   // EOL byte for TCP reading (0x3E = '>')
	AddCRLF         bool   // append \r\n to payload
	PrefixTransform string // "" | "bcd"
}

// BuildAndSend loads template, parametrizes, sends TCP, returns response and request.
func (b *Base) BuildAndSend(data *handler.StepData, ctx *context.TestContext) (request, response string, err error) {
	// Load template
	tmplXML, err := template.LoadTemplate(ctx.TestBase, data.TranCode)
	if err != nil {
		return "", "", err
	}

	// Parametrize
	valuesStr := handler.ValuesToString(data.Values)
	parametrized, err := template.Parametrize(ctx, tmplXML, valuesStr)
	if err != nil {
		return "", "", err
	}

	request = parametrized

	// Build payload
	payload := []byte(parametrized)

	if b.AddCRLF {
		payload = append(payload, '\r', '\n')
	}

	if b.LengthPrefix > 0 {
		prefix := sampler.BuildBCDLengthPrefix(len(payload), b.LengthPrefix)
		payload = append(prefix, payload...)
	}

	// Determine target address
	serverIP := ctx.GetOrDefault("serverIP", "")
	serverPort := ctx.GetOrDefault("serverPort", "")
	addr := fmt.Sprintf("%s:%s", serverIP, serverPort)

	// Send TCP
	cfg := sampler.DefaultTCPConfig()
	cfg.EolByte = b.EolByte

	respBytes, err := sampler.TCPSend(addr, payload, cfg)
	if err != nil {
		return request, "", err
	}

	response = string(respBytes)

	// Apply response offset
	if b.ResponseOffset > 0 && len(response) > b.ResponseOffset {
		response = response[b.ResponseOffset:]
	}

	// For MCA: strip trailing \r\n
	if b.AddCRLF && len(response) >= 2 {
		response = response[:len(response)-2]
	}

	return request, response, nil
}

// Verify performs XPath-based verification against the response.
func (b *Base) Verify(response string, results []handler.KV) (bool, string) {
	if len(results) == 0 {
		return true, ""
	}

	var failures []string
	for _, r := range results {
		actual, err := xmlhelper.Get(r.Key, response)
		if err != nil {
			failures = append(failures, fmt.Sprintf("[ tag %s mismatch: %v ]", r.Key, err))
			continue
		}
		expected := r.Value
		if actual != expected {
			failures = append(failures,
				fmt.Sprintf("[ tag %s mismatch: expected=%s actual=%s ]", r.Key, expected, actual))
		}
	}

	if len(failures) > 0 {
		return false, strings.Join(failures, "")
	}
	return true, ""
}

// ExtractVars extracts variables from the response using SaveEntry locators (XPath).
func (b *Base) ExtractVars(response string, saves []handler.SaveEntry, ctx *context.TestContext) {
	ctx.Set("prevResult", response)
	for _, s := range saves {
		val, err := xmlhelper.Get(s.Locator, response)
		if err != nil || val == "" {
			continue
		}
		val = vars.PreProcess(ctx, val)
		ctx.Set(s.Name, val)
	}
}

// Sleep waits for the configured milliseconds.
func (b *Base) Sleep(data *handler.StepData) {
	if data.Sleep > 0 {
		time.Sleep(time.Duration(data.Sleep) * time.Millisecond)
	}
}
