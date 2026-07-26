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
// and returns its inner text. For attribute nodes, returns the attribute value.
// Returns ErrXPathNotFound if no node matches.
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

	if node.Type == xmlquery.AttributeNode {
		if node.FirstChild != nil {
			return node.FirstChild.Data, nil
		}
		return "", nil
	}

	return strings.TrimSpace(node.InnerText()), nil
}

// Set finds the first node matching the given XPath in the XML string
// and replaces its text content with the given value. For attribute nodes,
// replaces the attribute value. Returns the modified XML.
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

	if node.Type == xmlquery.AttributeNode {
		// Update the attribute value in the parent element's Attr slice,
		// which is what xmlquery serializes when writing the document.
		attrName := node.Data
		for i := range node.Parent.Attr {
			if node.Parent.Attr[i].Name.Local == attrName {
				node.Parent.Attr[i].Value = value
				break
			}
		}
		return serializeDoc(doc)
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

	return serializeDoc(doc)
}

// serializeDoc serializes an xmlquery document node back to XML string.
func serializeDoc(doc *xmlquery.Node) (string, error) {
	var buf bytes.Buffer
	if err := doc.Write(&buf, false); err != nil {
		return "", fmt.Errorf("xml: output failed: %w", err)
	}
	return buf.String(), nil
}

// QueryAll returns the text content of all nodes matching the XPath expression.
// For element nodes, returns InnerText(); for attribute nodes, returns Data.
// Returns ErrXPathNotFound if no nodes match.
func QueryAll(xpathExpr, xmlStr string) ([]string, error) {
	if xmlStr == "" {
		return nil, fmt.Errorf("%w: empty input", ErrXMLParse)
	}
	doc, err := xmlquery.Parse(strings.NewReader(xmlStr))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrXMLParse, err)
	}
	nodes, err := xmlquery.QueryAll(doc, xpathExpr)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrXPathSyntax, err)
	}
	if len(nodes) == 0 {
		return nil, ErrXPathNotFound
	}
	results := make([]string, len(nodes))
	for i, node := range nodes {
		if node.Type == xmlquery.AttributeNode {
			if node.FirstChild != nil {
				results[i] = node.FirstChild.Data
			}
		} else {
			results[i] = strings.TrimSpace(node.InnerText())
		}
	}
	return results, nil
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
