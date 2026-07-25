package result

import (
	"bytes"
	"html/template"
	"time"
)

const reportHTMLTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head><meta charset="UTF-8"><title>X-Mate Test Report</title>
<style>
body { font-family: -apple-system, sans-serif; margin: 20px; }
h1 { border-bottom: 2px solid #333; padding-bottom: 8px; }
.summary { display: flex; gap: 20px; margin: 16px 0; flex-wrap: wrap; }
.stat { padding: 12px 20px; border-radius: 8px; font-size: 18px; font-weight: bold; }
.pass { background: #d4edda; color: #155724; }
.fail { background: #f8d7da; color: #721c24; }
.skip { background: #fff3cd; color: #856404; }
.error { background: #e2e3e5; color: #383d41; }
.case { margin: 8px 0; padding: 8px 12px; border: 1px solid #ddd; border-radius: 4px; }
.case.passed { border-left: 4px solid #28a745; }
.case.failed { border-left: 4px solid #dc3545; }
.case.skipped { border-left: 4px solid #ffc107; }
.case.error { border-left: 4px solid #6c757d; }
.step { margin: 4px 0 4px 20px; padding: 4px 8px; background: #f8f9fa; border-radius: 3px; }
.step.fail { background: #f8d7da; }
</style>
</head><body>
<h1>X-Mate Test Report</h1>
<p>{{.StartTime}} ~ {{.EndTime}} ({{.Duration}})</p>
<div class="summary">
  <div class="stat pass">✅ {{.PassedCases}} Passed</div>
  <div class="stat fail">❌ {{.FailedCases}} Failed</div>
  <div class="stat skip">⏭ {{.SkippedCases}} Skipped</div>
  <div class="stat error">⚠ {{.ErrorCases}} Error</div>
</div>
<p><strong>Total:</strong> {{.TotalCases}} cases</p>
{{range .Results}}
<div class="case {{.StatusClass}}">
  <strong>{{.Name}}</strong> <span>({{.Duration}})</span>
  {{range .Steps}}
  <div class="step{{if not .Pass}} fail{{end}}">
    [{{.Phase}}] {{.Desc}} ({{.Type}}): {{if not .Pass}}<strong>{{.Message}}</strong>{{else}}PASS{{end}}
  </div>
  {{end}}
</div>
{{end}}
</body></html>`

// caseView is the template-friendly view of a case result.
type caseView struct {
	Name        string
	StatusClass string
	Duration    string
	Steps       []stepView
}

// stepView is the template-friendly view of a step result.
type stepView struct {
	Phase   string
	Desc    string
	Type    string
	Pass    bool
	Message string
}

// reportView is the template-friendly view of the full report.
type reportView struct {
	StartTime    string
	EndTime      string
	Duration     string
	TotalCases   int
	PassedCases  int
	FailedCases  int
	SkippedCases int
	ErrorCases   int
	Results      []caseView
}

// statusClass returns the CSS class for a Status.
func statusClass(s Status) string {
	switch s {
	case Passed:
		return "passed"
	case Failed:
		return "failed"
	case Skipped:
		return "skipped"
	case Error:
		return "error"
	default:
		return ""
	}
}

// ToHTML generates a self-contained HTML report.
func (r *Report) ToHTML() ([]byte, error) {
	var cases []caseView
	for _, cr := range r.Results {
		var steps []stepView
		for _, s := range cr.Steps {
			steps = append(steps, stepView{
				Phase:   s.Phase,
				Desc:    s.Desc,
				Type:    s.Type,
				Pass:    s.Pass,
				Message: s.Message,
			})
		}
		cases = append(cases, caseView{
			Name:        cr.Name,
			StatusClass: statusClass(cr.Status),
			Duration:    cr.Duration.String(),
			Steps:       steps,
		})
	}

	view := reportView{
		StartTime:    r.StartTime.Format("2006-01-02 15:04:05"),
		EndTime:      r.EndTime.Format("2006-01-02 15:04:05"),
		Duration:     r.EndTime.Sub(r.StartTime).String(),
		TotalCases:   r.TotalCases,
		PassedCases:  r.PassedCases,
		FailedCases:  r.FailedCases,
		SkippedCases: r.SkippedCases,
		ErrorCases:   r.ErrorCases,
		Results:      cases,
	}

	// Format duration for each result
	for i, cr := range r.Results {
		cases[i].Duration = cr.Duration.String()
	}

	tmpl := template.Must(template.New("report").Parse(reportHTMLTemplate))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, view); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// formatDuration returns a human-readable duration string.
func formatDuration(d time.Duration) string {
	return d.String()
}
