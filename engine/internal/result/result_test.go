package result

import (
	"strings"
	"testing"
	"time"
)

func TestToJSON(t *testing.T) {
	now := time.Now()
	r := &Report{
		Version:      "1.0",
		StartTime:    now,
		EndTime:      now.Add(5 * time.Second),
		TotalCases:   1,
		PassedCases:  1,
		FailedCases:  0,
		SkippedCases: 0,
		ErrorCases:   0,
		Results: []CaseResult{
			{
				Name:       "test_case",
				Status:     Passed,
				Duration:   5 * time.Second,
				DurationMs: 5000,
				Steps: []StepResult{
					{Phase: "action", Desc: "step1", Type: "HTTP", Pass: true},
				},
			},
		},
	}

	data, err := r.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}

	jsonStr := string(data)
	if !strings.Contains(jsonStr, `"name": "test_case"`) {
		t.Errorf("expected test_case in JSON, got:\n%s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"status": "passed"`) {
		t.Errorf("expected 'passed' status in JSON, got:\n%s", jsonStr)
	}
}

func TestFromJSON(t *testing.T) {
	jsonData := `{
		"version": "1.0",
		"total_cases": 2,
		"passed_cases": 1,
		"failed_cases": 1,
		"results": [
			{"name": "case_a", "status": "passed", "duration_ms": 1000, "steps": []},
			{"name": "case_b", "status": "failed", "duration_ms": 500, "steps": []}
		]
	}`

	r, err := FromJSON([]byte(jsonData))
	if err != nil {
		t.Fatalf("FromJSON: %v", err)
	}

	if r.TotalCases != 2 {
		t.Errorf("expected 2 total cases, got %d", r.TotalCases)
	}
	if len(r.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(r.Results))
	}
	if r.Results[0].Name != "case_a" {
		t.Errorf("expected case_a, got %s", r.Results[0].Name)
	}
	if r.Results[0].Status != Passed {
		t.Errorf("expected passed, got %s", r.Results[0].Status)
	}
	if r.Results[1].Status != Failed {
		t.Errorf("expected failed, got %s", r.Results[1].Status)
	}
}

func TestToJUnitXML(t *testing.T) {
	now := time.Now()
	r := &Report{
		Version:      "1.0",
		StartTime:    now,
		EndTime:      now.Add(3 * time.Second),
		TotalCases:   3,
		PassedCases:  1,
		FailedCases:  1,
		SkippedCases: 1,
		ErrorCases:   0,
		Results: []CaseResult{
			{
				Name:     "pass_case",
				Status:   Passed,
				Duration: 1 * time.Second,
				Steps:    []StepResult{{Phase: "action", Pass: true}},
			},
			{
				Name:     "fail_case",
				Status:   Failed,
				Duration: 1 * time.Second,
				Steps:    []StepResult{{Phase: "action", Pass: false, Message: "assertion failed"}},
			},
			{
				Name:     "skip_case",
				Status:   Skipped,
				Duration: 0,
			},
		},
	}

	data, err := r.ToJUnitXML()
	if err != nil {
		t.Fatalf("ToJUnitXML: %v", err)
	}

	xmlStr := string(data)
	if !strings.Contains(xmlStr, `<testsuites`) {
		t.Errorf("expected testsuites element, got:\n%s", xmlStr)
	}
	if !strings.Contains(xmlStr, `tests="3"`) {
		t.Errorf("expected tests=3, got:\n%s", xmlStr)
	}
	if !strings.Contains(xmlStr, `failures="1"`) {
		t.Errorf("expected failures=1, got:\n%s", xmlStr)
	}
	if !strings.Contains(xmlStr, `<failure`) {
		t.Errorf("expected failure element, got:\n%s", xmlStr)
	}
	if !strings.Contains(xmlStr, `<skipped`) {
		t.Errorf("expected skipped element, got:\n%s", xmlStr)
	}
}

func TestToJUnitXML_ErrorCase(t *testing.T) {
	now := time.Now()
	r := &Report{
		Version:    "1.0",
		StartTime:  now,
		EndTime:    now.Add(1 * time.Second),
		TotalCases: 1,
		ErrorCases: 1,
		Results: []CaseResult{
			{
				Name:   "error_case",
				Status: Error,
				Steps:  nil, // no steps executed
			},
		},
	}

	data, err := r.ToJUnitXML()
	if err != nil {
		t.Fatalf("ToJUnitXML: %v", err)
	}

	xmlStr := string(data)
	if !strings.Contains(xmlStr, `<error`) {
		t.Errorf("expected error element, got:\n%s", xmlStr)
	}
}

func TestToHTML(t *testing.T) {
	now := time.Now()
	r := &Report{
		Version:      "1.0",
		StartTime:    now,
		EndTime:      now.Add(2 * time.Second),
		TotalCases:   2,
		PassedCases:  1,
		FailedCases:  1,
		SkippedCases: 0,
		ErrorCases:   0,
		Results: []CaseResult{
			{
				Name:     "pass_case",
				Status:   Passed,
				Duration: 1 * time.Second,
				Steps:    []StepResult{{Phase: "action", Desc: "ok", Type: "HTTP", Pass: true}},
			},
			{
				Name:     "fail_case",
				Status:   Failed,
				Duration: 500 * time.Millisecond,
				Steps:    []StepResult{{Phase: "action", Desc: "bad", Type: "SQL", Pass: false, Message: "timeout"}},
			},
		},
	}

	data, err := r.ToHTML()
	if err != nil {
		t.Fatalf("ToHTML: %v", err)
	}

	htmlStr := string(data)
	if !strings.Contains(htmlStr, "X-Mate Test Report") {
		t.Errorf("expected title, got:\n%s", htmlStr)
	}
	if !strings.Contains(htmlStr, "pass_case") {
		t.Errorf("expected pass_case, got:\n%s", htmlStr)
	}
	if !strings.Contains(htmlStr, "fail_case") {
		t.Errorf("expected fail_case, got:\n%s", htmlStr)
	}
	if !strings.Contains(htmlStr, "timeout") {
		t.Errorf("expected error message, got:\n%s", htmlStr)
	}
	if !strings.Contains(htmlStr, "PASS") {
		t.Errorf("expected PASS marker, got:\n%s", htmlStr)
	}
}

func TestAppendResult(t *testing.T) {
	r := &Report{}

	r.AppendResult(CaseResult{Name: "c1", Status: Passed})
	r.AppendResult(CaseResult{Name: "c2", Status: Failed})
	r.AppendResult(CaseResult{Name: "c3", Status: Skipped})
	r.AppendResult(CaseResult{Name: "c4", Status: Error})

	if r.TotalCases != 4 {
		t.Errorf("expected 4 total, got %d", r.TotalCases)
	}
	if r.PassedCases != 1 {
		t.Errorf("expected 1 passed, got %d", r.PassedCases)
	}
	if r.FailedCases != 1 {
		t.Errorf("expected 1 failed, got %d", r.FailedCases)
	}
	if r.SkippedCases != 1 {
		t.Errorf("expected 1 skipped, got %d", r.SkippedCases)
	}
	if r.ErrorCases != 1 {
		t.Errorf("expected 1 error, got %d", r.ErrorCases)
	}
	if len(r.Results) != 4 {
		t.Errorf("expected 4 results, got %d", len(r.Results))
	}
}

func TestFromJSON_Invalid(t *testing.T) {
	_, err := FromJSON([]byte(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
