package sql

import (
	"testing"

	"github.com/Eric033/x-mate/engine/internal/handler"
	"github.com/Eric033/x-mate/engine/internal/sampler"
)

func TestResolveSQLVars(t *testing.T) {
	ctx := setupTestCtx()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "single var",
			input: "SELECT * FROM users WHERE id = {{user_id}}",
			want:  "SELECT * FROM users WHERE id = 12345",
		},
		{
			name:  "multiple vars",
			input: "SELECT {{col}} FROM t WHERE a = {{val_a}} AND b = {{val_b}}",
			want:  "SELECT name FROM t WHERE a = 100 AND b = hello",
		},
		{
			name:  "no vars",
			input: "SELECT 1",
			want:  "SELECT 1",
		},
		{
			name:  "unresolved var stays",
			input: "SELECT {{unknown_var}}",
			want:  "SELECT {{unknown_var}}",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveSQLVars(ctx, tt.input)
			if got != tt.want {
				t.Errorf("resolveSQLVars() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseSQLLocator(t *testing.T) {
	tests := []struct {
		locator string
		wantCol string
		wantRow int
	}{
		{"CUSTOMER_NAME[0]", "CUSTOMER_NAME", 0},
		{"ID[5]", "ID", 5},
		{"NAME[99]", "NAME", 99},
		{"", "", 0},
		{"COL", "COL", 0},
	}

	for _, tt := range tests {
		t.Run(tt.locator, func(t *testing.T) {
			col, row := parseSQLLocator(tt.locator)
			if col != tt.wantCol {
				t.Errorf("col = %q, want %q", col, tt.wantCol)
			}
			if row != tt.wantRow {
				t.Errorf("row = %d, want %d", row, tt.wantRow)
			}
		})
	}
}

func TestSelectHandler_NoSQL(t *testing.T) {
	ctx := setupTestCtx()
	h := &SelectHandler{}
	data := &handler.StepData{
		StepType: "sql_select",
	}
	result := h.Execute(data, ctx)
	if result.Success {
		t.Error("expected failure with no SQL")
	}
}

func TestUpdateHandler_NoSQL(t *testing.T) {
	ctx := setupTestCtx()
	h := &UpdateHandler{}
	data := &handler.StepData{
		StepType: "sql_update",
	}
	result := h.Execute(data, ctx)
	if result.Success {
		t.Error("expected failure with no SQL")
	}
}

func TestSelectHandler_VerifyResultString(t *testing.T) {
	h := &SelectHandler{}
	result := &sampler.QueryResult{
		Columns: []string{"NAME", "STATUS"},
		Rows: []map[string]string{
			{"NAME": "张三", "STATUS": "ACTIVE"},
			{"NAME": "李四", "STATUS": "INACTIVE"},
		},
	}

	// Test exact match
	ok, msg := h.verifyResultString(result, "STATUS[0]@@@ACTIVE")
	if !ok {
		t.Errorf("expected pass, got: %s", msg)
	}

	// Test mismatch
	ok, msg = h.verifyResultString(result, "STATUS[0]@@@INACTIVE")
	if ok {
		t.Error("expected fail for mismatch")
	}
	if msg == "" {
		t.Error("expected failure message")
	}

	// Test wildcard skip
	ok, _ = h.verifyResultString(result, "*")
	if !ok {
		t.Error("expected pass for wildcard")
	}

	// Test empty skip
	ok, _ = h.verifyResultString(result, "")
	if !ok {
		t.Error("expected pass for empty")
	}

	// Test multiple entries
	ok, msg = h.verifyResultString(result, "STATUS[0]@@@ACTIVE;NAME[1]@@@李四")
	if !ok {
		t.Errorf("expected pass for multi-entry, got: %s", msg)
	}

	// Test out of range row
	ok, msg = h.verifyResultString(result, "STATUS[10]@@@ACTIVE")
	if ok {
		t.Error("expected fail for out-of-range row")
	}

	// Test non-existent column
	ok, msg = h.verifyResultString(result, "NONEXISTENT[0]@@@val")
	if ok {
		t.Error("expected fail for non-existent column")
	}
}
