package tcp

import (
	"github.com/Eric033/x-mate/engine/internal/context"
	"github.com/Eric033/x-mate/engine/internal/handler"
)

// --- xml_set_8 ---

// XMLSet8Handler: 8-byte BCD length prefix, 8-byte response offset, EOL '>'.
type XMLSet8Handler struct{}

func (h *XMLSet8Handler) Execute(data *handler.StepData, ctx *context.TestContext) *handler.StepResult {
	base := &Base{ResponseOffset: 8, LengthPrefix: 8, EolByte: 0x3E}
	base.Sleep(data)

	request, response, err := base.BuildAndSend(data, ctx)
	if err != nil {
		return &handler.StepResult{Success: false, FailureMessage: err.Error(), RequestData: request}
	}

	// Verify
	ok, msg := base.Verify(response, data.Assertions, ctx)
	base.ExtractVars(response, data.Saves, ctx)

	return &handler.StepResult{
		Success:       ok,
		FailureMessage: msg,
		ResponseData:  response,
		RequestData:   request,
	}
}

// --- xml_set_sas ---

// XMLSetSASHandler: 6-byte response offset, no manual length prefix.
type XMLSetSASHandler struct{}

func (h *XMLSetSASHandler) Execute(data *handler.StepData, ctx *context.TestContext) *handler.StepResult {
	base := &Base{ResponseOffset: 6}
	base.Sleep(data)

	request, response, err := base.BuildAndSend(data, ctx)
	if err != nil {
		return &handler.StepResult{Success: false, FailureMessage: err.Error(), RequestData: request}
	}

	ok, msg := base.Verify(response, data.Assertions, ctx)
	base.ExtractVars(response, data.Saves, ctx)

	return &handler.StepResult{
		Success:       ok,
		FailureMessage: msg,
		ResponseData:  response,
		RequestData:   request,
	}
}

// --- xml_set ---

// XMLSetHandler: standard BCD, 6-byte response offset.
type XMLSetHandler struct{}

func (h *XMLSetHandler) Execute(data *handler.StepData, ctx *context.TestContext) *handler.StepResult {
	base := &Base{ResponseOffset: 6}
	base.Sleep(data)

	request, response, err := base.BuildAndSend(data, ctx)
	if err != nil {
		return &handler.StepResult{Success: false, FailureMessage: err.Error(), RequestData: request}
	}

	ok, msg := base.Verify(response, data.Assertions, ctx)
	base.ExtractVars(response, data.Saves, ctx)

	return &handler.StepResult{
		Success:       ok,
		FailureMessage: msg,
		ResponseData:  response,
		RequestData:   request,
	}
}

// --- mca ---

// MCAHandler: appends \r\n, no length prefix, strips trailing \r\n from response.
type MCAHandler struct{}

func (h *MCAHandler) Execute(data *handler.StepData, ctx *context.TestContext) *handler.StepResult {
	base := &Base{AddCRLF: true}
	base.Sleep(data)

	request, response, err := base.BuildAndSend(data, ctx)
	if err != nil {
		return &handler.StepResult{Success: false, FailureMessage: err.Error(), RequestData: request}
	}

	ok, msg := base.Verify(response, data.Assertions, ctx)
	// MCA: store full response (no offset stripping), no preProcess on extracted values
	ctx.Set("prevResult", response)
	for _, s := range data.Saves {
		val, err := xmlhelperGetNoPreProcess(s.Locator, response)
		if err != nil || val == "" {
			continue
		}
		ctx.Set(s.Name, val)
	}

	return &handler.StepResult{
		Success:       ok,
		FailureMessage: msg,
		ResponseData:  response,
		RequestData:   request,
	}
}

// xmlhelperGetNoPreProcess extracts without preProcess (for MCA compatibility).
func xmlhelperGetNoPreProcess(xpath, xml string) (string, error) {
	// reuse xmlhelper.Get but skip preProcess
	return xmlhelperGet(xpath, xml)
}
