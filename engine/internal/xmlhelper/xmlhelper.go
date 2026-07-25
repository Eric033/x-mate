package xmlhelper

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/antchfx/xmlquery"
)

var (
	// ErrXPathNotFound is returned when the XPath expression matches no node.
	ErrXPathNotFound = errors.New("xpath: no node matched")
	// ErrXPathSyntax is returned when the XPath expression is invalid.
	ErrXPathSyntax = errors.New("xpath: invalid expression")
	// ErrXMLParse is returned when the input XML cannot be parsed.
	ErrXMLParse = errors.New("xml: parse failed")
)

// Get finds the first node matching the given XPath in the XML string
// and returns its inner text. Returns ErrXPathNotFound if no node matches.
func Get(xpathExpr, xmlStr string) (string, error) {
	if xmlStr == "" {
		return "", fmt.Errorf("%w: empty input", ErrXMLParse)
	}

	doc, err := xmlquery.Parse(strings.NewReader(xmlStr))
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrXMLParse, err)
	}

	node, err := xmlquery.Query(doc, xpathExpr)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrXPathSyntax, err)
	}
	if node == nil {
		return "", ErrXPathNotFound
	}

	return strings.TrimSpace(node.InnerText()), nil
}

// Set finds the first node matching the given XPath in the XML string
// and replaces its text content with the given value. Returns the modified XML.
// Returns ErrXPathNotFound if no node matches.
func Set(xpathExpr, value, xmlStr string) (string, error) {
	if xmlStr == "" {
		return "", fmt.Errorf("%w: empty input", ErrXMLParse)
	}

	doc, err := xmlquery.Parse(strings.NewReader(xmlStr))
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrXMLParse, err)
	}

	node, err := xmlquery.Query(doc, xpathExpr)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrXPathSyntax, err)
	}
	if node == nil {
		return "", ErrXPathNotFound
	}

	// Remove all existing child nodes using RemoveFromTree
	for c := node.FirstChild; c != nil; c = node.FirstChild {
		xmlquery.RemoveFromTree(c)
	}

	// Add a new text node with the value
	textNode := &xmlquery.Node{
		Type: xmlquery.TextNode,
		Data: value,
	}
	xmlquery.AddChild(node, textNode)

	// Serialize back — OutputXML on the document root outputs all children
	var buf bytes.Buffer
	if err := doc.Write(&buf, false); err != nil {
		return "", fmt.Errorf("xml: output failed: %w", err)
	}
	return buf.String(), nil
}

// parseXPathTags converts an XPath expression into a sequence of tag names.
// Kept for backward compatibility with existing test.
// Examples:
//
//	//TRAN_CODE → ["TRAN_CODE"]
//	//Header/TRAN_CODE → ["Header", "TRAN_CODE"]
//	/Root/Header/TRAN_CODE → ["Root", "Header", "TRAN_CODE"]
func parseXPathTags(xpath string) []string {
	// Remove leading /
	cleaned := strings.TrimLeft(xpath, "/")
	parts := strings.Split(cleaned, "/")
	var tags []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// Handle predicates like Tag[1]
		if idx := strings.IndexByte(p, '['); idx >= 0 {
			p = p[:idx]
		}
		tags = append(tags, p)
	}
	return tags
}
