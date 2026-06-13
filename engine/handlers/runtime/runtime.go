package runtime

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Eric033/x-mate/engine/internal/context"
	"github.com/Eric033/x-mate/engine/internal/handler"
)

var (
	exprVarRe = regexp.MustCompile(`\{\{(.+?)\}\}`)
	cmpRe     = regexp.MustCompile(`^(.+?)(>=|<=|==|!=|>|<)(.+)$`)
)

// RuntimeVerifyHandler evaluates expressions against context variables.
type RuntimeVerifyHandler struct{}

func (h *RuntimeVerifyHandler) Execute(data *handler.StepData, ctx *context.TestContext) *handler.StepResult {
	// Get expression from Verify results
	var expr string
	for _, r := range data.VerifyResults {
		expr += r.Value
	}
	for _, v := range data.VerifyValues {
		expr += v
	}

	if expr == "" {
		return &handler.StepResult{Success: true}
	}

	// Replace {{var}} with values
	resolved := exprVarRe.ReplaceAllStringFunc(expr, func(match string) string {
		name := strings.TrimSpace(match[2 : len(match)-2])
		if v, ok := ctx.Get(name); ok {
			return v
		}
		return "0"
	})

	// Evaluate simple comparison expressions
	result := evalComparison(resolved)

	// Extract vars
	for _, s := range data.Saves {
		// JSONPath or XPath extraction from context vars
		if strings.HasPrefix(s.Locator, "$") {
			// JSONPath from prevResult
			if prev, ok := ctx.Get("prevResult"); ok {
				val := jsonpathSimple(s.Locator, prev)
				if val != "" {
					ctx.Set(s.Name, val)
				}
			}
		}
	}

	return &handler.StepResult{
		Success:       result,
		FailureMessage: func() string {
			if !result {
				return fmt.Sprintf("[ expression '%s' evaluated to false ]", resolved)
			}
			return ""
		}(),
	}
}

// evalComparison evaluates a simple comparison expression like "100 > 50".
func evalComparison(expr string) bool {
	expr = strings.TrimSpace(expr)
	matches := cmpRe.FindStringSubmatch(expr)
	if len(matches) != 4 {
		// Try as boolean literal
		return expr == "true"
	}

	left := strings.TrimSpace(matches[1])
	op := matches[2]
	right := strings.TrimSpace(matches[3])

	// Try numeric comparison
	var l, r float64
	_, err1 := fmt.Sscanf(left, "%f", &l)
	_, err2 := fmt.Sscanf(right, "%f", &r)

	if err1 == nil && err2 == nil {
		switch op {
		case ">":
			return l > r
		case "<":
			return l < r
		case ">=":
			return l >= r
		case "<=":
			return l <= r
		case "==":
			return l == r
		case "!=":
			return l != r
		}
	}

	// String comparison
	switch op {
	case "==":
		return left == right
	case "!=":
		return left != right
	}

	return false
}

// jsonpathSimple does a basic JSONPath lookup.
func jsonpathSimple(jsonPath, jsonStr string) string {
	path := strings.TrimPrefix(jsonPath, "$")
	path = strings.TrimPrefix(path, ".")
	parts := strings.Split(path, ".")
	current := jsonStr

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		searchKey := fmt.Sprintf(`"%s"`, part)
		idx := strings.Index(current, searchKey)
		if idx < 0 {
			return ""
		}
		rest := current[idx+len(searchKey):]
		rest = strings.TrimLeft(rest, " \t\n\r:")
		if strings.HasPrefix(rest, `"`) {
			rest = rest[1:]
			endIdx := strings.Index(rest, `"`)
			if endIdx < 0 {
				return ""
			}
			current = rest[:endIdx]
		} else {
			endIdx := strings.IndexAny(rest, ",}\n")
			if endIdx < 0 {
				current = strings.TrimSpace(rest)
			} else {
				current = strings.TrimSpace(rest[:endIdx])
			}
		}
	}
	return current
}
