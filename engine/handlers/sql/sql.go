package sql

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Eric033/x-mate/engine/internal/context"
	"github.com/Eric033/x-mate/engine/internal/handler"
	"github.com/Eric033/x-mate/engine/internal/sampler"
	"github.com/Eric033/x-mate/engine/internal/vars"
)

var templateVarRe = regexp.MustCompile(`\{\{(.+?)\}\}`)

// SelectHandler handles sql_exe and sql_select step types.
type SelectHandler struct {
	PoolManager *sampler.DBPoolManager
	Driver      string
}

func (h *SelectHandler) Execute(data *handler.StepData, ctx *context.TestContext) *handler.StepResult {
	// Extract SQL from action text or values
	sqlText := data.Attrs["_action_text"]
	if sqlText == "" && len(data.Values) > 0 {
		sqlText = data.Values[0].Value
	}

	if sqlText == "" {
		return &handler.StepResult{Success: false, FailureMessage: "no SQL statement found"}
	}

	// Resolve {{var}} placeholders
	sqlText = resolveSQLVars(ctx, sqlText)

	// Also resolve ${var}
	sqlText = vars.PreProcess(ctx, sqlText)

	// Execute query
	result, err := h.PoolManager.Select(data.Server, sqlText)
	if err != nil {
		return &handler.StepResult{Success: false, FailureMessage: err.Error()}
	}

	// Store resultVariable for extraction
	ctx.Set("resultVariable", fmt.Sprintf("%d rows", len(result.Rows)))

	// Verify using unified Assertions
	ok := true
	var failureMsg string
	if len(data.Assertions) > 0 {
		// Split assertions into KV-style (with XPath containing column[row]) and raw value-style (empty XPath)
		var kvResults []handler.KV
		var rawValues []string
		for _, a := range data.Assertions {
			if a.XPath != "" {
				kvResults = append(kvResults, handler.KV{Key: a.XPath, Value: a.Expected})
			} else {
				rawValues = append(rawValues, a.Expected)
			}
		}
		if len(rawValues) > 0 {
			ok, failureMsg = h.verify(result, rawValues)
		} else if len(kvResults) > 0 {
			ok, failureMsg = h.verifyResults(result, kvResults)
		}
	}

	// Extract vars from result set
	h.extractVars(result, data.Saves, ctx)

	return &handler.StepResult{
		Success:        ok,
		FailureMessage: failureMsg,
		ResponseData:   fmt.Sprintf("%d rows", len(result.Rows)),
		RequestData:    sqlText,
	}
}

func (h *SelectHandler) verify(result *sampler.QueryResult, verifyValues []string) (bool, string) {
	// verifyValues are raw text like "COLUMN[0]@@@expected;COLUMN2[1]@@@expected2"
	var allEntries string
	for _, v := range verifyValues {
		allEntries += v
	}
	return h.verifyResultString(result, allEntries)
}

func (h *SelectHandler) verifyResults(result *sampler.QueryResult, results []handler.KV) (bool, string) {
	var parts []string
	for _, r := range results {
		parts = append(parts, r.Key+"@@@"+r.Value)
	}
	return h.verifyResultString(result, strings.Join(parts, ";"))
}

func (h *SelectHandler) verifyResultString(result *sampler.QueryResult, expected string) (bool, string) {
	if expected == "" || expected == "*" {
		return true, ""
	}

	var failures []string
	entries := strings.Split(expected, ";")
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		// Parse COLUMN[rowIndex]@@@expectedValue
		atIdx := strings.Index(entry, "@@@")
		if atIdx < 0 {
			continue
		}
		colExpr := entry[:atIdx]
		expectedVal := entry[atIdx+3:]

		// Parse column and row index
		bracketIdx := strings.Index(colExpr, "[")
		if bracketIdx < 0 {
			continue
		}
		colName := colExpr[:bracketIdx]
		rowIdxStr := colExpr[bracketIdx+1 : strings.Index(colExpr, "]")]

		var rowIdx int
		fmt.Sscanf(rowIdxStr, "%d", &rowIdx)

		if rowIdx < 0 || rowIdx >= len(result.Rows) {
			failures = append(failures, fmt.Sprintf("[ row %d out of range (%d rows) ]", rowIdx, len(result.Rows)))
			continue
		}

		actual, ok := result.Rows[rowIdx][colName]
		if !ok {
			failures = append(failures, fmt.Sprintf("[ column %s not found ]", colName))
			continue
		}

		if actual != expectedVal {
			failures = append(failures,
				fmt.Sprintf("[ %s[%d] mismatch: expected=%s actual=%s ]", colName, rowIdx, expectedVal, actual))
		}
	}

	if len(failures) > 0 {
		return false, strings.Join(failures, "")
	}
	return true, ""
}

func (h *SelectHandler) extractVars(result *sampler.QueryResult, saves []handler.SaveEntry, ctx *context.TestContext) {
	for _, s := range saves {
		// Parse COLUMN[rowIndex] locator
		if s.Locator == "" {
			// Default: first row, first column
			if len(result.Rows) > 0 && len(result.Columns) > 0 {
				ctx.Set(s.Name, result.Rows[0][result.Columns[0]])
			}
			continue
		}

		col, rowIdx := parseSQLLocator(s.Locator)
		if col != "" && rowIdx >= 0 && rowIdx < len(result.Rows) {
			ctx.Set(s.Name, result.Rows[rowIdx][col])
		}
	}
}

// UpdateHandler handles sql_update step type.
type UpdateHandler struct {
	PoolManager *sampler.DBPoolManager
	Driver      string
}

func (h *UpdateHandler) Execute(data *handler.StepData, ctx *context.TestContext) *handler.StepResult {
	// Extract SQL
	sqlText := data.Attrs["_action_text"]
	if sqlText == "" && len(data.Values) > 0 {
		sqlText = data.Values[0].Value
	}

	if sqlText == "" {
		return &handler.StepResult{Success: false, FailureMessage: "no SQL statement found"}
	}

	// Resolve variables
	sqlText = resolveSQLVars(ctx, sqlText)
	sqlText = vars.PreProcess(ctx, sqlText)

	// Execute
	affected, err := h.PoolManager.Exec(data.Server, sqlText)
	if err != nil {
		return &handler.StepResult{Success: false, FailureMessage: err.Error()}
	}

	affectedStr := fmt.Sprintf("%d", affected)
	ctx.Set("sqlActualResult_1", affectedStr)

	// Verify using unified Assertions
	ok := true
	var failureMsg string
	for _, a := range data.Assertions {
		v := strings.TrimSpace(a.Expected)
		if v == "*" || v == "" {
			continue
		}
		if affectedStr != v {
			ok = false
			failureMsg = fmt.Sprintf("[ sql_update mismatch: expected=%s actual=%s ]", v, affectedStr)
		}
	}

	// Extract: default to sqlActualResult_1
	for _, s := range data.Saves {
		ctx.Set(s.Name, affectedStr)
	}

	return &handler.StepResult{
		Success:        ok,
		FailureMessage: failureMsg,
		ResponseData:   affectedStr,
		RequestData:    sqlText,
	}
}

// resolveSQLVars replaces {{var}} placeholders in SQL text.
func resolveSQLVars(ctx *context.TestContext, text string) string {
	return templateVarRe.ReplaceAllStringFunc(text, func(match string) string {
		name := strings.TrimSpace(match[2 : len(match)-2])
		if v, ok := ctx.Get(name); ok {
			return v
		}
		return match
	})
}

// parseSQLLocator parses "COLUMN[rowIndex]" format.
func parseSQLLocator(locator string) (col string, row int) {
	bracketIdx := strings.Index(locator, "[")
	if bracketIdx < 0 {
		return locator, 0
	}
	col = locator[:bracketIdx]
	rowStr := locator[bracketIdx+1 : strings.Index(locator, "]")]
	fmt.Sscanf(rowStr, "%d", &row)
	return
}
