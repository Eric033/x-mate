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
	fmt.Fprintf(w, "Cases:   %d total, %d passed, %d failed, %d skipped, %d error\n",
		r.TotalCases, r.PassedCases, r.FailedCases, r.SkippedCases, r.ErrorCases)
	fmt.Fprintf(w, "\n")

	totalSteps := 0
	passedSteps := 0

	for _, cr := range r.Results {
		statusSymbol := statusSymbolFor(cr.Status)

		for _, s := range cr.Steps {
			totalSteps++
			if s.Pass {
				passedSteps++
			}
		}

		fmt.Fprintf(w, "  %s  %s  (%s)\n", statusSymbol, cr.CaseName, cr.Duration)
		if cr.Status == runner.CaseFailed {
			for _, s := range cr.Steps {
				if !s.Pass {
					fmt.Fprintf(w, "         [%s] %s (%s): %s\n", s.Phase, s.Desc, s.Type, s.Message)
				}
			}
		}
		if cr.Status == runner.CaseError {
			if len(cr.Steps) == 0 {
				fmt.Fprintf(w, "         error: no steps executed\n")
			} else {
				for _, s := range cr.Steps {
					if !s.Pass {
						fmt.Fprintf(w, "         [%s] %s (%s): %s\n", s.Phase, s.Desc, s.Type, s.Message)
					}
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

// statusSymbolFor returns a status symbol for a given case status.
func statusSymbolFor(status runner.CaseStatus) string {
	switch status {
	case runner.CasePassed:
		return "✓ PASS"
	case runner.CaseFailed:
		return "✗ FAIL"
	case runner.CaseSkipped:
		return "— SKIP"
	case runner.CaseError:
		return "⚠ ERROR"
	default:
		return "? UNKN"
	}
}

// SummaryLine returns a one-line summary.
func SummaryLine(r *runner.Report) string {
	return fmt.Sprintf("%d cases (%d pass, %d fail, %d skip, %d error)",
		r.TotalCases, r.PassedCases, r.FailedCases, r.SkippedCases, r.ErrorCases)
}

// MarkdownReport generates a Markdown-format report string.
func MarkdownReport(r *runner.Report) string {
	var sb strings.Builder
	sb.WriteString("# Test Execution Report\n\n")
	sb.WriteString(fmt.Sprintf("- **Time**: %s ~ %s (%s)\n",
		r.StartTime.Format("15:04:05"), r.EndTime.Format("15:04:05"),
		r.EndTime.Sub(r.StartTime)))
	sb.WriteString(fmt.Sprintf("- **Cases**: %d total, ✅ %d passed, ❌ %d failed, ⏭ %d skipped, ⚠ %d error\n\n",
		r.TotalCases, r.PassedCases, r.FailedCases, r.SkippedCases, r.ErrorCases))

	sb.WriteString("| Case | Status | Duration |\n")
	sb.WriteString("|------|--------|----------|\n")
	for _, cr := range r.Results {
		status := statusEmojiFor(cr.Status)
		sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", cr.CaseName, status, cr.Duration))
	}

	return sb.String()
}

// statusEmojiFor returns an emoji status indicator for a given case status.
func statusEmojiFor(status runner.CaseStatus) string {
	switch status {
	case runner.CasePassed:
		return "✅"
	case runner.CaseFailed:
		return "❌"
	case runner.CaseSkipped:
		return "⏭"
	case runner.CaseError:
		return "⚠"
	default:
		return "❓"
	}
}
