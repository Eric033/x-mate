package template

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Eric033/x-mate/engine/internal/context"
	"github.com/Eric033/x-mate/engine/internal/vars"
	"github.com/Eric033/x-mate/engine/internal/xmlhelper"
)

// LoadTemplate reads a template file by trancode from the test base.
func LoadTemplate(testBase, trancode string) (string, error) {
	path := filepath.Join(testBase, "template", fmt.Sprintf("template_%s.xml", trancode))
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("template file not found: %s", path)
	}
	return string(data), nil
}

// Parametrize applies test data (xpath:value pairs) to the template.
// pairs format: "xpath1:value1;xpath2:value2"
// Each value can contain {{var}} or ${var} references.
func Parametrize(ctx *context.TestContext, templateXML, pairs string) (string, error) {
	result := templateXML
	if pairs == "" {
		return result, nil
	}

	entries := strings.Split(pairs, ";")
	for _, entry := range entries {
		colonIdx := strings.Index(entry, ":")
		if colonIdx < 0 {
			continue
		}
		xpathExpr := strings.TrimSpace(entry[:colonIdx])
		value := entry[colonIdx+1:]

		// Resolve variables in value
		value = vars.ResolveAll(ctx, value)

		// Auto-add // prefix for compatibility
		if !strings.HasPrefix(xpathExpr, "/") {
			xpathExpr = "//" + xpathExpr
		}

		var err error
		result, err = xmlhelper.Set(xpathExpr, value, result)
		if err != nil {
			return result, fmt.Errorf("template parametrize set error: %w", err)
		}
	}
	return result, nil
}
