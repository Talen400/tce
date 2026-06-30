package tools

import (
	"encoding/json"
	"testing"
)

func TestSearchToolName(t *testing.T) {
	tool := &SearchTool{}
	if tool.Name() != "search" {
		t.Errorf("expected search, got %s", tool.Name())
	}
}

func TestSearchToolShortDescription(t *testing.T) {
	tool := &SearchTool{}
	if tool.ShortDescription() == "" {
		t.Error("short description should not be empty")
	}
}

func TestSearchToolSchema(t *testing.T) {
	tool := &SearchTool{}
	schema := tool.Schema()
	m, ok := schema.(map[string]any)
	if !ok {
		t.Fatal("schema should be a map")
	}
	props, ok := m["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema should have properties")
	}
	if _, ok := props["query"]; !ok {
		t.Error("schema should have query property")
	}
}

func TestSearchToolExecuteEmptyQuery(t *testing.T) {
	tool := &SearchTool{}
	input, _ := json.Marshal(map[string]string{"query": ""})
	_, err := tool.Execute(ExecContext{}, input)
	if err == nil {
		t.Error("expected error for empty query")
	}
}

func TestSearchToolExecuteMissingQuery(t *testing.T) {
	tool := &SearchTool{}
	input, _ := json.Marshal(map[string]string{"wrong": "field"})
	_, err := tool.Execute(ExecContext{}, input)
	if err == nil {
		t.Error("expected error for missing query")
	}
}

func TestSearchToolExecuteInvalidJSON(t *testing.T) {
	tool := &SearchTool{}
	_, err := tool.Execute(ExecContext{}, json.RawMessage(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
