package vars

import (
	"regexp"
	"strings"

	"github.com/Eric033/x-mate/engine/internal/context"
)

var (
	// ${variableName} pattern
	dollarBraceRe = regexp.MustCompile(`\$\{([^}]+)\}`)
	// {{variableName}} pattern
	doubleBraceRe = regexp.MustCompile(`\{\{(.+?)\}\}`)
)

// PreProcess replaces all ${var} occurrences in text with their values from ctx.
func PreProcess(ctx *context.TestContext, text string) string {
	return dollarBraceRe.ReplaceAllStringFunc(text, func(match string) string {
		// Extract variable name between ${ and }
		name := match[2 : len(match)-1]
		if v, ok := ctx.Get(name); ok {
			return v
		}
		return match // leave as-is if not found
	})
}

// ResolveTemplate replaces all {{var}} occurrences in text with their values from ctx.
func ResolveTemplate(ctx *context.TestContext, text string) string {
	return doubleBraceRe.ReplaceAllStringFunc(text, func(match string) string {
		// Extract variable name between {{ and }}
		name := strings.TrimSpace(match[2 : len(match)-2])
		if v, ok := ctx.Get(name); ok {
			return v
		}
		return match
	})
}

// ResolveAll replaces both ${var} and {{var}} patterns.
func ResolveAll(ctx *context.TestContext, text string) string {
	text = PreProcess(ctx, text)
	text = ResolveTemplate(ctx, text)
	return text
}
