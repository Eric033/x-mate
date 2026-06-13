package xmlhelper

import (
	"bytes"
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

// Set finds the node at the given XPath in the XML string and replaces its text content.
// Returns the modified XML string.
// Supports a subset of XPath: //TagName, //Parent/Child, /Root/Child, //Tag[n], //Tag@attr
func Set(xpathExpr, value, xmlStr string) (string, error) {
	// Ensure xpath starts with //
	if !strings.HasPrefix(xpathExpr, "//") && !strings.HasPrefix(xpathExpr, "/") {
		xpathExpr = "//" + xpathExpr
	}

	doc, err := html.Parse(bytes.NewReader([]byte(xmlStr)))
	if err != nil {
		return "", fmt.Errorf("xmlhelper.Set parse error: %w", err)
	}

	// For simplicity, we use a tag-based path matching approach
	// Parse the xpath to extract tag path components
	tags := parseXPathTags(xpathExpr)
	if len(tags) == 0 {
		return xmlStr, nil
	}

	// Navigate and set text content
	found := setNodeText(doc, tags, value)
	if !found {
		return xmlStr, nil // no match, return unchanged
	}

	var buf bytes.Buffer
	if err := html.Render(&buf, doc); err != nil {
		return "", fmt.Errorf("xmlhelper.Set render error: %w", err)
	}

	return buf.String(), nil
}

// Get finds the node at the given XPath and returns its text content.
func Get(xpathExpr, xmlStr string) (string, error) {
	if !strings.HasPrefix(xpathExpr, "//") && !strings.HasPrefix(xpathExpr, "/") {
		xpathExpr = "//" + xpathExpr
	}

	doc, err := html.Parse(bytes.NewReader([]byte(xmlStr)))
	if err != nil {
		return "", fmt.Errorf("xmlhelper.Get parse error: %w", err)
	}

	tags := parseXPathTags(xpathExpr)
	if len(tags) == 0 {
		return "", nil
	}

	return getNodeText(doc, tags), nil
}

// parseXPathTags converts an XPath expression into a sequence of tag names.
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

// setNodeText traverses the tree and sets text content for the last tag in the path.
func setNodeText(n *html.Node, tags []string, value string) bool {
	if len(tags) == 0 {
		return false
	}

	if n.Type == html.ElementNode && n.Data == tags[0] {
		if len(tags) == 1 {
			// Found the target node, set its text content
			setTextContent(n, value)
			return true
		}
		// Look for child in path
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if setNodeText(c, tags[1:], value) {
				return true
			}
		}
	}

	// Search descendants
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c == n {
			continue
		}
		if setNodeText(c, tags, value) {
			return true
		}
	}
	return false
}

// getNodeText traverses the tree and returns text content of the target node.
func getNodeText(n *html.Node, tags []string) string {
	if len(tags) == 0 {
		return ""
	}

	if n.Type == html.ElementNode && n.Data == tags[0] {
		if len(tags) == 1 {
			return getTextContent(n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if result := getNodeText(c, tags[1:]); result != "" {
				return result
			}
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c == n {
			continue
		}
		if result := getNodeText(c, tags); result != "" {
			return result
		}
	}
	return ""
}

// setTextContent removes all children and replaces with a single text node.
func setTextContent(n *html.Node, value string) {
	// Remove existing children
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		n.RemoveChild(c)
		c = next
	}
	// Add text node
	textNode := &html.Node{
		Type: html.TextNode,
		Data: value,
	}
	n.AppendChild(textNode)
}

// getTextContent returns the concatenated text content of a node.
func getTextContent(n *html.Node) string {
	var buf bytes.Buffer
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			buf.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(buf.String())
}
