package handler

import (
	"testing"

	"github.com/Eric033/x-mate/engine/internal/context"
)

// ---- Registry tests ----

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()

	if h := r.Get("nonexistent"); h != nil {
		t.Fatal("expected nil for unregistered type")
	}

	dummy := &dummyHandler{}
	r.Register("mock", dummy)

	if h := r.Get("mock"); h == nil {
		t.Fatal("expected non-nil handler")
	} else {
		// Execute it and check we get our dummy result back
		res := h.Execute(&StepData{}, context.New())
		if !res.Success {
			t.Fatal("expected success from dummy handler")
		}
	}
}

func TestRegistry_Overwrite(t *testing.T) {
	r := NewRegistry()
	r.Register("dup", &dummyHandler{msg: "first"})
	r.Register("dup", &dummyHandler{msg: "second"})

	h := r.Get("dup")
	res := h.Execute(&StepData{}, context.New())
	if res.FailureMessage != "second" {
		t.Fatalf("expected overwritten handler, got msg=%q", res.FailureMessage)
	}
}

// ---- ParseStep tests ----

func TestParseStep_BasicSQL(t *testing.T) {
	raw := `<step desc="query user">
		<Action type="SQL" server_index="2" trancode="T001" sleep="100">SELECT * FROM users</Action>
	</step>`

	data, err := ParseStep(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if data.StepType != "SQL" {
		t.Errorf("StepType = %q, want %q", data.StepType, "SQL")
	}
	if data.Desc != "query user" {
		t.Errorf("Desc = %q, want %q", data.Desc, "query user")
	}
	if data.ServerIndex != 2 {
		t.Errorf("ServerIndex = %d, want %d", data.ServerIndex, 2)
	}
	if data.TranCode != "T001" {
		t.Errorf("TranCode = %q, want %q", data.TranCode, "T001")
	}
	if data.Sleep != 100 {
		t.Errorf("Sleep = %d, want %d", data.Sleep, 100)
	}
	if data.Attrs["_action_text"] != "SELECT * FROM users" {
		t.Errorf("action text = %q, want %q", data.Attrs["_action_text"], "SELECT * FROM users")
	}
}

func TestParseStep_WithValuesAndResults(t *testing.T) {
	raw := `<step desc="login">
		<Action type="HTTP" server_index="1" trancode="T002"/>
		<value name="username">admin</value>
		<value name="password">secret</value>
		<result name="//code">0</result>
		<result name="//msg">ok</result>
	</step>`

	data, err := ParseStep(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if data.StepType != "HTTP" {
		t.Errorf("StepType = %q", "HTTP")
	}
	if len(data.Values) != 2 {
		t.Fatalf("len(Values) = %d, want 2", len(data.Values))
	}
	if data.Values[0].Key != "username" || data.Values[0].Value != "admin" {
		t.Errorf("Values[0] = %+v", data.Values[0])
	}
	if data.Values[1].Key != "password" || data.Values[1].Value != "secret" {
		t.Errorf("Values[1] = %+v", data.Values[1])
	}
	if len(data.Results) != 2 {
		t.Fatalf("len(Results) = %d, want 2", len(data.Results))
	}
	if data.Results[0].Key != "//code" || data.Results[0].Value != "0" {
		t.Errorf("Results[0] = %+v", data.Results[0])
	}
}

func TestParseStep_WithVerifyAndSave(t *testing.T) {
	raw := `<step desc="verify step">
		<Action type="HTTP"/>
		<Verify>
			<result name="//status" isHeader="False">200</result>
			<value>raw_value_1</value>
		</Verify>
		<save>
			<key name="session_id" locator="//session"/>
		</save>
	</step>`

	data, err := ParseStep(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(data.VerifyResults) != 1 {
		t.Fatalf("len(VerifyResults) = %d, want 1", len(data.VerifyResults))
	}
	if data.VerifyResults[0].Name != "//status" {
		t.Errorf("VerifyResults[0].Name = %q", data.VerifyResults[0].Name)
	}
	if data.VerifyResults[0].Value != "200" {
		t.Errorf("VerifyResults[0].Value = %q", data.VerifyResults[0].Value)
	}
	if len(data.VerifyValues) != 1 || data.VerifyValues[0] != "raw_value_1" {
		t.Errorf("VerifyValues = %v", data.VerifyValues)
	}
	if len(data.Saves) != 1 || data.Saves[0].Name != "session_id" || data.Saves[0].Locator != "//session" {
		t.Errorf("Saves = %+v", data.Saves)
	}
}

func TestParseStep_HTTPWithBodyHeadersQuery(t *testing.T) {
	raw := `<step desc="http post">
		<Action type="HTTP" method="POST" api="/api/login"/>
		<body>{"user":"admin"}</body>
		<header name="Content-Type">application/json</header>
		<queryString name="version">v2</queryString>
	</step>`

	data, err := ParseStep(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if data.Body != `{"user":"admin"}` {
		t.Errorf("Body = %q", data.Body)
	}
	if len(data.Headers) != 1 || data.Headers[0].Key != "Content-Type" || data.Headers[0].Value != "application/json" {
		t.Errorf("Headers = %+v", data.Headers)
	}
	if len(data.QueryString) != 1 || data.QueryString[0].Key != "version" || data.QueryString[0].Value != "v2" {
		t.Errorf("QueryString = %+v", data.QueryString)
	}
	if data.Attrs["method"] != "POST" || data.Attrs["api"] != "/api/login" {
		t.Errorf("attrs method/api = %q, %q", data.Attrs["method"], data.Attrs["api"])
	}
}

func TestParseStep_EmptyXML(t *testing.T) {
	_, err := ParseStep("")
	if err == nil {
		t.Fatal("expected error for empty XML")
	}
}

func TestParseStep_InvalidXML(t *testing.T) {
	_, err := ParseStep("<step><unclosed>")
	if err == nil {
		t.Fatal("expected error for invalid XML")
	}
}

func TestParseStep_DefaultServerIndex(t *testing.T) {
	raw := `<step desc="default">
		<Action type="TCP"/>
	</step>`
	data, err := ParseStep(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.ServerIndex != 1 {
		t.Errorf("default ServerIndex = %d, want 1", data.ServerIndex)
	}
}

// ---- ValuesToString tests ----

func TestValuesToString(t *testing.T) {
	vals := []KV{
		{Key: "username", Value: "admin"},
		{Key: "password", Value: "secret"},
	}
	got := ValuesToString(vals)
	want := "username:admin;password:secret"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestValuesToString_Empty(t *testing.T) {
	got := ValuesToString(nil)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

// ---- ResultsToString tests ----

func TestResultsToString(t *testing.T) {
	results := []KV{
		{Key: "//code", Value: "0"},
		{Key: "//msg", Value: "ok"},
	}
	got := ResultsToString(results)
	want := "//code@@@0;//msg@@@ok"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResultsToString_Empty(t *testing.T) {
	got := ResultsToString(nil)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

// ---- Helpers ----

type dummyHandler struct {
	msg string
}

func (d *dummyHandler) Execute(data *StepData, ctx *context.TestContext) *StepResult {
	if d.msg == "" {
		return &StepResult{Success: true}
	}
	return &StepResult{Success: true, FailureMessage: d.msg}
}