package tools

import (
	"encoding/json"
	"testing"
)

func TestTaskToolName(t *testing.T) {
	tool := &TaskTool{}
	if tool.Name() != "task" {
		t.Errorf("expected task, got %s", tool.Name())
	}
}

func TestTaskToolShortDescription(t *testing.T) {
	tool := &TaskTool{}
	if tool.ShortDescription() == "" {
		t.Error("short description should not be empty")
	}
}

func TestTaskParseStandard(t *testing.T) {
	var raw map[string]any
	json.Unmarshal([]byte(`{"type":"explore","prompt":"find all Go files"}`), &raw)
	agent := extractSubAgent(raw)
	prompt := extractPrompt(raw)
	if agent != "explore" {
		t.Errorf("expected explore, got %s", agent)
	}
	if prompt != "find all Go files" {
		t.Errorf("expected 'find all Go files', got %s", prompt)
	}
}

func TestTaskParseAlternativeKeys(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		agent  string
		prompt string
	}{
		{"subagent+message", `{"subagent":"explore","message":"find files"}`, "explore", "find files"},
		{"agent+task", `{"agent":"general","task":"do work"}`, "general", "do work"},
		{"mode+description", `{"mode":"explore","description":"search code"}`, "explore", "search code"},
		{"name+parameters", `{"name":"general","parameters":"run tests"}`, "general", "run tests"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var raw map[string]any
			if err := json.Unmarshal([]byte(tt.input), &raw); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			agent := extractSubAgent(raw)
			prompt := extractPrompt(raw)
			if agent != tt.agent {
				t.Errorf("expected agent %s, got %s", tt.agent, agent)
			}
			if prompt != tt.prompt {
				t.Errorf("expected prompt %s, got %s", tt.prompt, prompt)
			}
		})
	}
}

func TestTaskParseEmpty(t *testing.T) {
	var raw map[string]any
	json.Unmarshal([]byte(`{}`), &raw)
	agent := extractSubAgent(raw)
	prompt := extractPrompt(raw)
	if agent != "" {
		t.Errorf("expected empty agent, got %s", agent)
	}
	if prompt != "" {
		t.Errorf("expected empty prompt, got %s", prompt)
	}
}

func TestTaskSchemaNotEmpty(t *testing.T) {
	tool := &TaskTool{}
	schema := tool.Schema()
	if schema == nil {
		t.Error("schema should not be nil")
	}
}
