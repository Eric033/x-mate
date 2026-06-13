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

// StepData holds parsed data from the step XML element.
type StepData struct {
	StepType    string
	Desc        string
	ServerIndex int
	TranCode    string
	Sleep       int
	// Raw action attributes
	Attrs map[string]string
	// Test data pairs from <value> elements: xpath=value
	Values []KV
	// Expected results from <result> elements: xpath@@@expected
	Results []KV
	// Verify element results
	VerifyResults []VerifyEntry
	VerifyValues  []string // raw <value> text inside <Verify>
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

// VerifyEntry represents a <result> inside <Verify> for HTTP steps.
type VerifyEntry struct {
	Name       string // xpath or jsonpath expression
	IsHeader   string // "True" or "False"
	HeaderName string
	Value      string // expected value
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
