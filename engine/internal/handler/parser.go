package handler

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

// ParseStep parses a raw XML step element string into StepData.
func ParseStep(rawXML string) (*StepData, error) {
	// We parse using encoding/xml with a permissive structure
	type actionElem struct {
		XMLName     xml.Name `xml:"Action"`
		Type        string   `xml:"type,attr"`
		Server      string   `xml:"server,attr"`
		ServerIndex string   `xml:"server_index,attr"`
		TranCode    string   `xml:"trancode,attr"`
		Sleep       string   `xml:"sleep,attr"`
		IP          string   `xml:"ip,attr"`
		Port        string   `xml:"port,attr"`
		API         string   `xml:"api,attr"`
		Method      string   `xml:"method,attr"`
		Key         string   `xml:"key,attr"`
		Value       string   `xml:"value,attr"`
		// Inner text (for SQL value)
		Content string `xml:",chardata"`
	}

	type valueElem struct {
		XMLName xml.Name `xml:"value"`
		Name    string   `xml:"name,attr"`
		Content string   `xml:",chardata"`
	}

	type resultElem struct {
		XMLName    xml.Name `xml:"result"`
		Name       string   `xml:"name,attr"`
		IsHeader   string   `xml:"isHeader,attr"`
		HeaderName string   `xml:"headerName,attr"`
		Content    string   `xml:",chardata"`
	}

	type keyElem struct {
		XMLName  xml.Name `xml:"key"`
		Name     string   `xml:"name,attr"`
		Locator  string   `xml:"locator,attr"`
		Target   string   `xml:"target,attr"`
		Content  string   `xml:",chardata"`
	}

	type saveElem struct {
		XMLName xml.Name  `xml:"save"`
		Keys    []keyElem `xml:"key"`
	}

	type verifyElem struct {
		XMLName  xml.Name    `xml:"Verify"`
		Results  []resultElem `xml:"result"`
		Values   []valueElem  `xml:"value"`
	}

	type headerElem struct {
		XMLName xml.Name `xml:"header"`
		Name    string   `xml:"name,attr"`
		Content string   `xml:",chardata"`
	}

	type queryStringElem struct {
		XMLName xml.Name `xml:"queryString"`
		Name    string   `xml:"name,attr"`
		Content string   `xml:",chardata"`
	}

	type bodyElem struct {
		XMLName xml.Name `xml:"body"`
		Content string   `xml:",chardata"`
	}

	type stepElem struct {
		XMLName     xml.Name       `xml:"step"`
		Desc        string         `xml:"desc,attr"`
		Action      actionElem     `xml:"Action"`
		Values      []valueElem    `xml:"value"`
		Results     []resultElem   `xml:"result"`
		Verify      *verifyElem    `xml:"Verify"`
		Save        *saveElem      `xml:"save"`
		Body        *bodyElem      `xml:"body"`
		Headers     []headerElem   `xml:"header"`
		QueryString []queryStringElem `xml:"queryString"`
	}

	var step stepElem
	if err := xml.Unmarshal([]byte(rawXML), &step); err != nil {
		return nil, err
	}

	data := &StepData{
		StepType: step.Action.Type,
		Desc:     step.Desc,
		Attrs:    make(map[string]string),
	}

	// Parse server (service name) — new, preferred
	data.Server = step.Action.Server

	// Parse server_index — legacy, ignored when Server is set
	if data.Server == "" && step.Action.ServerIndex != "" {
		if idx, err := strconv.Atoi(step.Action.ServerIndex); err == nil {
			data.ServerIndex = idx
		}
	} else {
		data.ServerIndex = 1 // default when no server info provided
	}

	data.TranCode = step.Action.TranCode

	// Parse sleep
	if step.Action.Sleep != "" {
		if ms, err := strconv.Atoi(step.Action.Sleep); err == nil {
			data.Sleep = ms
		}
	}

	// Store all action attrs
	if step.Action.Type != "" {
		data.Attrs["type"] = step.Action.Type
	}
	if step.Action.Server != "" {
		data.Attrs["server"] = step.Action.Server
	}
	if step.Action.ServerIndex != "" {
		data.Attrs["server_index"] = step.Action.ServerIndex
	}
	if step.Action.TranCode != "" {
		data.Attrs["trancode"] = step.Action.TranCode
	}
	if step.Action.IP != "" {
		data.Attrs["ip"] = step.Action.IP
	}
	if step.Action.Port != "" {
		data.Attrs["port"] = step.Action.Port
	}
	if step.Action.API != "" {
		data.Attrs["api"] = step.Action.API
	}
	if step.Action.Method != "" {
		data.Attrs["method"] = step.Action.Method
	}
	if step.Action.Key != "" {
		data.Attrs["key"] = step.Action.Key
	}
	if step.Action.Value != "" {
		data.Attrs["value"] = step.Action.Value
	}
	// SQL content from action chardata
	if strings.TrimSpace(step.Action.Content) != "" {
		data.Attrs["_action_text"] = strings.TrimSpace(step.Action.Content)
	}

	// Parse values (test data)
	for _, v := range step.Values {
		data.Values = append(data.Values, KV{
			Key:   v.Name,
			Value: strings.TrimSpace(v.Content),
		})
	}

	// Pre-check: step-level <result> without <Verify> is unsupported
	if len(step.Results) > 0 {
		return nil, fmt.Errorf("parse error: unsupported: result elements must be inside <Verify>")
	}

	// Parse Verify → unified Assertions
	if step.Verify != nil {
		for _, r := range step.Verify.Results {
			a := Assertion{
				Expected: strings.TrimSpace(r.Content),
			}
			// Determine the locator type from the name attribute
			if strings.EqualFold(r.IsHeader, "true") {
				a.IsHeader = true
				a.HeaderName = r.HeaderName
				a.XPath = r.Name // store name for display/logging
			} else if strings.HasPrefix(r.Name, "$") {
				a.JSONPath = r.Name
			} else if strings.HasPrefix(r.Name, "/") {
				a.XPath = r.Name
			} else {
				// For SQL-style assertions (e.g. "cnt[0]", "status[0]"),
				// or runtime expressions, store in XPath
				a.XPath = r.Name
			}
			data.Assertions = append(data.Assertions, a)
		}
		// <Verify><value> elements: store as assertions with empty XPath (for SQL compatibility)
		for _, v := range step.Verify.Values {
			data.Assertions = append(data.Assertions, Assertion{
				Expected: strings.TrimSpace(v.Content),
			})
		}
	}

	// Parse Save
	if step.Save != nil {
		for _, k := range step.Save.Keys {
			locator := strings.TrimSpace(k.Content)
			if locator == "" {
				locator = k.Locator
			}
			data.Saves = append(data.Saves, SaveEntry{
				Name:    k.Name,
				Locator: locator,
			})
		}
	}

	// HTTP-specific
	if step.Body != nil {
		data.Body = strings.TrimSpace(step.Body.Content)
	}
	for _, h := range step.Headers {
		data.Headers = append(data.Headers, KV{
			Key:   h.Name,
			Value: strings.TrimSpace(h.Content),
		})
	}
	for _, q := range step.QueryString {
		data.QueryString = append(data.QueryString, KV{
			Key:   q.Name,
			Value: strings.TrimSpace(q.Content),
		})
	}

	return data, nil
}

// ValuesToString converts Values to the SRS format: "key1:value1;key2:value2"
func ValuesToString(values []KV) string {
	var parts []string
	for _, v := range values {
		parts = append(parts, v.Key+":"+v.Value)
	}
	return strings.Join(parts, ";")
}

// ResultsToString converts Results to the SRS format: "key1@@@value1;key2@@@value2"
func ResultsToString(results []KV) string {
	var parts []string
	for _, r := range results {
		parts = append(parts, r.Key+"@@@"+r.Value)
	}
	return strings.Join(parts, ";")
}
