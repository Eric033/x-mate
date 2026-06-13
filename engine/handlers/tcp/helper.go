package tcp

import (
	"github.com/Eric033/x-mate/engine/internal/xmlhelper"
)

// xmlhelperGet wraps xmlhelper.Get for MCA handler.
func xmlhelperGet(xpath, xml string) (string, error) {
	return xmlhelper.Get(xpath, xml)
}
