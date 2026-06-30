package tools

import (
	"context"
	"encoding/json"
	"testing"
)

func TestAskToolName(t *testing.T) {
	tool := &AskTool{}
	if tool.Name() != "ask" {
		t.Errorf("expected ask, got %s", tool.Name())
	}
}

func TestAskToolShortDescription(t *testing.T) {
	tool := &AskTool{}
	if tool.ShortDescription() == "" {
		t.Error("short description should not be empty")
	}
}

func TestAskExecute(t *testing.T) {
	tool := &AskTool{}
	input, _ := json.Marshal(map[string]string{
		"question": "What is your name?",
	})

	answered := false
	ctx := ExecContext{
		Context: context.Background(),
		ReadInput: func(q string) (string, error) {
			answered = true
			if q != "What is your name?" {
				t.Errorf("expected 'What is your name?', got %s", q)
			}
			return "John", nil
		},
	}

	result, err := tool.Execute(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !answered {
		t.Error("ReadInput was not called")
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestAskExecuteNoReadInput(t *testing.T) {
	tool := &AskTool{}
	input, _ := json.Marshal(map[string]string{
		"question": "Are you there?",
	})
	result, err := tool.Execute(ExecContext{Context: context.Background()}, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected fallback response")
	}
}
