package report

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Eric033/x-mate/engine/internal/result"
)

// PrintReport outputs a human-readable test report to the given writer.
func PrintReport(w io.Writer, r *result.Report) {
	fmt.Fprintf(w, "\n")

	// Dry-run or normal header
	if r.ErrorCases > 0 || r.DryRunValidated > 0 {
		// Detect dry-run mode: no passed/failed/skipped, but has validated count
		if r.PassedCases == 0 && r.FailedCases == 0 && r.SkippedCases == 0 && r.DryRunValidated > 0 {
			printDryRunReport(w, r)
			return
		}
	}

	printNormalReport(w, r)
}

// printDryRunReport outputs a report specialized for dry-run validation.
func printDryRunReport(w io.Writer, r *result.Report) {
	fmt.Fprintf(w, "========================================\n")
	fmt.Fprintf(w, "     Test Execution Report (DRY-RUN)\n")
	fmt.Fprintf(w, "========================================\n")
	fmt.Fprintf(w, "Start:  %s\n", r.StartTime.Format(time.RFC3339))
	fmt.Fprintf(w, "End:    %s\n", r.EndTime.Format(time.RFC3339))
	fmt.Fprintf(w, "Total:  %s\n", r.EndTime.Sub(r.StartTime))
	fmt.Fprintf(w, "\n")

	validCount := r.DryRunValidated
	errorCount := r.ErrorCases
	totalCount := validCount + errorCount

	fmt.Fprintf(w, "Cases:  %d total, %d valid, %d errors\n", totalCount, validCount, errorCount)
	fmt.Fprintf(w, "\n")

	// List each case
	for _, cr := range r.Results {
		if cr.Status == result.Error {
			fmt.Fprintf(w, "  ⚠ ERROR  %s\n", cr.Name)
		}
	}

	fmt.Fprintf(w, "\n")
	if errorCount > 0 {
		fmt.Fprintf(w, "⚠ %d case(s) have configuration errors that must be fixed before execution.\n", errorCount)
	}
	fmt.Fprintf(w, "========================================\n")
}

// printNormalReport outputs the standard execution report.
func printNormalReport(w io.Writer, r *result.Report) {
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

		fmt.Fprintf(w, "  %s  %s  (%s)\n", statusSymbol, cr.Name, cr.Duration)
		if cr.Status == result.Failed {
			for _, s := range cr.Steps {
				if !s.Pass {
					fmt.Fprintf(w, "         [%s] %s (%s): %s\n", s.Phase, s.Desc, s.Type, s.Message)
				}
			}
		}
		if cr.Status == result.Error {
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
func statusSymbolFor(status result.Status) string {
	switch status {
	case result.Passed:
		return "✓ PASS"
	case result.Failed:
		return "✗ FAIL"
	case result.Skipped:
		return "— SKIP"
	case result.Error:
		return "⚠ ERROR"
	default:
		return "? UNKN"
	}
}

// SummaryLine returns a one-line summary.
func SummaryLine(r *result.Report) string {
	return fmt.Sprintf("%d cases (%d pass, %d fail, %d skip, %d error)",
		r.TotalCases, r.PassedCases, r.FailedCases, r.SkippedCases, r.ErrorCases)
}

// MarkdownReport generates a Markdown-format report string.
func MarkdownReport(r *result.Report) string {
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
		sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", cr.Name, status, cr.Duration))
	}

	return sb.String()
}

// statusEmojiFor returns an emoji status indicator for a given case status.
func statusEmojiFor(status result.Status) string {
	switch status {
	case result.Passed:
		return "✅"
	case result.Failed:
		return "❌"
	case result.Skipped:
		return "⏭"
	case result.Error:
		return "⚠"
	default:
		return "❓"
	}
}
