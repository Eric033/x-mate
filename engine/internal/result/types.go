package result

import (
	"sync"
	"time"
)

// Status represents the explicit status of a test case.
type Status string

const (
	Passed  Status = "passed"
	Failed  Status = "failed"
	Skipped Status = "skipped"
	Error   Status = "error"
)

// Report is the top-level execution report.
type Report struct {
	Version     string      `json:"version"`
	StartTime   time.Time   `json:"start_time"`
	EndTime     time.Time   `json:"end_time"`
	DurationMs  int64       `json:"duration_ms"`

	TotalCases    int `json:"total_cases"`
	PassedCases   int `json:"passed_cases"`
	FailedCases   int `json:"failed_cases"`
	SkippedCases  int `json:"skipped_cases"`
	ErrorCases    int `json:"error_cases"`

	DryRun         bool `json:"dry_run,omitempty"`
	DryRunValidated int `json:"dry_run_validated,omitempty"`

	Results []CaseResult `json:"results"`

	mu sync.Mutex `json:"-"` // internal, protects Results for concurrent access
}

// AppendResult safely appends a case result to the report, counting by Status.
func (r *Report) AppendResult(cr CaseResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Results = append(r.Results, cr)
	r.TotalCases++
	switch cr.Status {
	case Passed:
		r.PassedCases++
	case Failed:
		r.FailedCases++
	case Skipped:
		r.SkippedCases++
	case Error:
		r.ErrorCases++
	}
}

// CaseResult holds the result of a single test case.
type CaseResult struct {
	Name       string        `json:"name"`
	Status     Status        `json:"status"`
	Duration   time.Duration `json:"-"`
	DurationMs int64         `json:"duration_ms"`
	Steps      []StepResult  `json:"steps"`
}

// StepResult holds the result of a single step within a case.
type StepResult struct {
	Phase      string        `json:"phase"`
	Desc       string        `json:"desc"`
	Type       string        `json:"type"`
	Pass       bool          `json:"pass"`
	Message    string        `json:"message,omitempty"`
	Duration   time.Duration `json:"-"`
	DurationMs int64         `json:"duration_ms"`
}
