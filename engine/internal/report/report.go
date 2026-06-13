package report

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Eric033/x-mate/engine/internal/runner"
)

// PrintReport outputs a human-readable test report to the given writer.
func PrintReport(w io.Writer, r *runner.Report) {
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "========================================\n")
	fmt.Fprintf(w, "          Test Execution Report\n")
	fmt.Fprintf(w, "========================================\n")
	fmt.Fprintf(w, "Start:  %s\n", r.StartTime.Format(time.RFC3339))
	fmt.Fprintf(w, "End:    %s\n", r.EndTime.Format(time.RFC3339))
	fmt.Fprintf(w, "Total:  %s\n", r.EndTime.Sub(r.StartTime))
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "Cases:   %d total, %d passed, %d failed\n",
		r.TotalCases, r.PassedCases, r.FailedCases)
	fmt.Fprintf(w, "\n")

	totalSteps := 0
	passedSteps := 0

	for _, cr := range r.Results {
		status := "✓ PASS"
		hasFail := false
		for _, s := range cr.Steps {
			totalSteps++
			if s.Pass {
				passedSteps++
			} else {
				hasFail = true
			}
		}
		if hasFail {
			status = "✗ FAIL"
		}

		fmt.Fprintf(w, "  %s  %s  (%s)\n", status, cr.CaseName, cr.Duration)
		if hasFail {
			for _, s := range cr.Steps {
				if !s.Pass {
					fmt.Fprintf(w, "         [%s] %s (%s): %s\n", s.Phase, s.Desc, s.Type, s.Message)
				}
			}
		}
	}

	fmt.Fprintf(w, "\n")
	if totalSteps > 0 {
		fmt.Fprintf(w, "Steps:   %d total, %d passed (%.1f%%)\n",
			totalSteps, passedSteps, float64(passedSteps)/float64(totalSteps)*100)
	}
	fmt.Fprintf(w, "========================================\n")
}

// SummaryLine returns a one-line summary.
func SummaryLine(r *runner.Report) string {
	return fmt.Sprintf("%d cases (%d pass, %d fail)",
		r.TotalCases, r.PassedCases, r.FailedCases)
}

// MarkdownReport generates a Markdown-format report string.
func MarkdownReport(r *runner.Report) string {
	var sb strings.Builder
	sb.WriteString("# Test Execution Report\n\n")
	sb.WriteString(fmt.Sprintf("- **Time**: %s ~ %s (%s)\n",
		r.StartTime.Format("15:04:05"), r.EndTime.Format("15:04:05"),
		r.EndTime.Sub(r.StartTime)))
	sb.WriteString(fmt.Sprintf("- **Cases**: %d total, ✅ %d passed, ❌ %d failed\n\n",
		r.TotalCases, r.PassedCases, r.FailedCases))

	sb.WriteString("| Case | Status | Duration |\n")
	sb.WriteString("|------|--------|----------|\n")
	for _, cr := range r.Results {
		allPass := true
		for _, s := range cr.Steps {
			if !s.Pass {
				allPass = false
			}
		}
		status := "✅"
		if !allPass {
			status = "❌"
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", cr.CaseName, status, cr.Duration))
	}

	return sb.String()
}
