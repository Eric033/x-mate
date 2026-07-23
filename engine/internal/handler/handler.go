package handler

import (
	"github.com/Eric033/x-mate/engine/internal/context"
)

// StepResult holds the result of a single step execution.
type StepResult struct {
	Success       bool
	FailureMessage string
	ResponseData  string
	RequestData   string
	ExtractedVars map[string]string
}

// Assertion represents a single verification assertion parsed from <Verify><result>.
type Assertion struct {
	XPath      string // XML XPath, e.g. "//RESP_CODE"
	JSONPath   string // JSONPath, e.g. "$.ret_code"
	IsHeader   bool   // HTTP header verification
	HeaderName string
	Expected   string // expected value (variable-unresolved raw value)
}

// StepData holds parsed data from the step XML element.
type StepData struct {
	StepType    string
	Desc        string
	Server      string // service name (e.g. "ABC"), overrides ServerIndex
	ServerIndex int    // legacy numeric index, ignored when Server is set
	TranCode    string
	Sleep       int
	// Raw action attributes
	Attrs map[string]string
	// Test data pairs from <value> elements: xpath=value
	Values []KV
	// Unified assertion list from <Verify><result>
	Assertions []Assertion
	// Save keys: name → locator
	Saves []SaveEntry
	// HTTP-specific
	Body        string
	Headers     []KV
	QueryString []KV
}

// KV is a simple key-value pair.
type KV struct {
	Key   string
	Value string
}

// SaveEntry represents a <key> inside <save>.
type SaveEntry struct {
	Name    string
	Locator string
}

// Handler interface that all step type handlers must implement.
type Handler interface {
	Execute(data *StepData, ctx *context.TestContext) *StepResult
}

// Registry holds the mapping of step types to handlers.
type Registry struct {
	handlers map[string]Handler
}

// NewRegistry creates an empty handler registry.
func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]Handler)}
}

// Register associates a step type with a handler.
func (r *Registry) Register(stepType string, h Handler) {
	r.handlers[stepType] = h
}

// Get returns the handler for a step type, or nil if not found.
func (r *Registry) Get(stepType string) Handler {
	return r.handlers[stepType]
}
