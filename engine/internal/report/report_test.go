package report

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/Eric033/x-mate/engine/internal/runner"
)

// ---- PrintReport tests ----

func TestPrintReport_AllPass(t *testing.T) {
	start := time.Now().Add(-2 * time.Second)
	end := time.Now()

	r := &runner.Report{
		StartTime:   start,
		EndTime:     end,
		TotalCases:  2,
		PassedCases: 2,
		FailedCases: 0,
		Results: []runner.CaseResult{
			{
				CaseName: "case_login",
				Duration: 1 * time.Second,
				Steps: []runner.StepReport{
					{Phase: "action", Desc: "login step", Type: "HTTP", Pass: true, Message: ""},
				},
			},
			{
				CaseName: "case_logout",
				Duration: 500 * time.Millisecond,
				Steps: []runner.StepReport{
					{Phase: "action", Desc: "logout step", Type: "HTTP", Pass: true, Message: ""},
				},
			},
		},
	}

	var buf bytes.Buffer
	PrintReport(&buf, r)

	output := buf.String()
	if !strings.Contains(output, "✓ PASS") {
		t.Errorf("expected pass indicator, got:\n%s", output)
	}
	if !strings.Contains(output, "2 total, 2 passed") {
		t.Errorf("expected case summary, got:\n%s", output)
	}
	if !strings.Contains(output, "2 total, 2 passed") {
		t.Errorf("expected step summary, got:\n%s", output)
	}
}

func TestPrintReport_WithFailures(t *testing.T) {
	start := time.Now().Add(-1 * time.Minute)
	end := time.Now()

	r := &runner.Report{
		StartTime:   start,
		EndTime:     end,
		TotalCases:  1,
		PassedCases: 0,
		FailedCases: 1,
		Results: []runner.CaseResult{
			{
				CaseName: "case_fail",
				Duration: 2 * time.Second,
				Steps: []runner.StepReport{
					{Phase: "action", Desc: "step1", Type: "HTTP", Pass: true, Message: ""},
					{Phase: "action", Desc: "step2", Type: "SQL", Pass: false, Message: "connection timeout"},
				},
			},
		},
	}

	var buf bytes.Buffer
	PrintReport(&buf, r)

	output := buf.String()
	if !strings.Contains(output, "✗ FAIL") {
		t.Errorf("expected fail indicator, got:\n%s", output)
	}
	if !strings.Contains(output, "connection timeout") {
		t.Errorf("expected error message in output")
	}
}

func TestPrintReport_EmptyReport(t *testing.T) {
	r := &runner.Report{
		StartTime:   time.Now(),
		EndTime:     time.Now(),
		TotalCases:  0,
		PassedCases: 0,
		FailedCases: 0,
	}

	var buf bytes.Buffer
	PrintReport(&buf, r)

	output := buf.String()
	if !strings.Contains(output, "0 total, 0 passed") {
		t.Errorf("expected zero summary, got:\n%s", output)
	}
}

// ---- SummaryLine tests ----

func TestSummaryLine_Normal(t *testing.T) {
	r := &runner.Report{
		TotalCases:  5,
		PassedCases: 3,
		FailedCases: 2,
	}

	got := SummaryLine(r)
	want := "5 cases (3 pass, 2 fail)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSummaryLine_AllPass(t *testing.T) {
	r := &runner.Report{
		TotalCases:  10,
		PassedCases: 10,
		FailedCases: 0,
	}

	got := SummaryLine(r)
	want := "10 cases (10 pass, 0 fail)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSummaryLine_ZeroCases(t *testing.T) {
	r := &runner.Report{
		TotalCases:  0,
		PassedCases: 0,
		FailedCases: 0,
	}

	got := SummaryLine(r)
	want := "0 cases (0 pass, 0 fail)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// ---- MarkdownReport tests ----

func TestMarkdownReport_Normal(t *testing.T) {
	start := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	end := time.Date(2025, 1, 15, 10, 32, 30, 0, time.UTC)

	r := &runner.Report{
		StartTime:   start,
		EndTime:     end,
		TotalCases:  2,
		PassedCases: 1,
		FailedCases: 1,
		Results: []runner.CaseResult{
			{
				CaseName: "case_a",
				Duration: 1 * time.Minute,
				Steps:    []runner.StepReport{{Phase: "action", Pass: true}},
			},
			{
				CaseName: "case_b",
				Duration: 30 * time.Second,
				Steps:    []runner.StepReport{{Phase: "action", Pass: false, Message: "fail"}},
			},
		},
	}

	md := MarkdownReport(r)

	if !strings.Contains(md, "# Test Execution Report") {
		t.Errorf("expected heading")
	}
	if !strings.Contains(md, "2 total") {
		t.Errorf("expected total count")
	}
	if !strings.Contains(md, "1 passed") {
		t.Errorf("expected passed count")
	}
	if !strings.Contains(md, "1 failed") {
		t.Errorf("expected failed count")
	}
	if !strings.Contains(md, "✅") {
		t.Errorf("expected pass emoji")
	}
	if !strings.Contains(md, "❌") {
		t.Errorf("expected fail emoji")
	}
	if !strings.Contains(md, "| Case |") {
		t.Errorf("expected table header")
	}
	if !strings.Contains(md, "case_a") {
		t.Errorf("expected case_a in table")
	}
	if !strings.Contains(md, "case_b") {
		t.Errorf("expected case_b in table")
	}
}

func TestMarkdownReport_AllPass(t *testing.T) {
	now := time.Now()
	r := &runner.Report{
		StartTime:   now,
		EndTime:     now.Add(5 * time.Second),
		TotalCases:  1,
		PassedCases: 1,
		FailedCases: 0,
		Results: []runner.CaseResult{
			{
				CaseName: "test_ok",
				Duration: 5 * time.Second,
				Steps:    []runner.StepReport{{Phase: "action", Pass: true}},
			},
		},
	}

	md := MarkdownReport(r)

	// The summary always shows both ✅ passed and ❌ failed counts
	if !strings.Contains(md, "✅ 1 passed") {
		t.Errorf("expected pass count in markdown, got:\n%s", md)
	}
	if !strings.Contains(md, "❌ 0 failed") {
		t.Errorf("expected fail count in markdown, got:\n%s", md)
	}
	if !strings.Contains(md, "✅") {
		t.Errorf("expected pass emoji in case row")
	}
}

func TestMarkdownReport_EmptyReport(t *testing.T) {
	now := time.Now()
	r := &runner.Report{
		StartTime:   now,
		EndTime:     now,
		TotalCases:  0,
		PassedCases: 0,
		FailedCases: 0,
	}

	md := MarkdownReport(r)

	if !strings.Contains(md, "0 total") {
		t.Errorf("expected zero in markdown")
	}
}