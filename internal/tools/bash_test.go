package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBashName(t *testing.T) {
	tool := &BashTool{}
	if tool.Name() != "bash" {
		t.Errorf("expected bash, got %s", tool.Name())
	}
}

func TestBashShortDescription(t *testing.T) {
	tool := &BashTool{}
	if tool.ShortDescription() == "" {
		t.Error("short description should not be empty")
	}
}

func TestBashExecute(t *testing.T) {
	tool := &BashTool{}
	input, _ := json.Marshal(map[string]any{
		"command": "echo hello",
	})
	result, err := tool.Execute(ExecContext{Context: context.Background()}, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestBashExecuteWithTimeout(t *testing.T) {
	tool := &BashTool{}
	input, _ := json.Marshal(map[string]any{
		"command": "echo hello",
		"timeout": 10,
	})
	result, err := tool.Execute(ExecContext{Context: context.Background()}, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestBashExecuteWithWorkdir(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "bash-test-*")
	defer os.RemoveAll(tmpDir)

	tool := &BashTool{}
	input, _ := json.Marshal(map[string]any{
		"command": "pwd",
		"workdir": "",
	})
	result, err := tool.Execute(ExecContext{Context: context.Background(), ProjectRoot: tmpDir}, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestBashExecuteError(t *testing.T) {
	tool := &BashTool{}
	input, _ := json.Marshal(map[string]any{
		"command": "exit 42",
	})
	_, err := tool.Execute(ExecContext{Context: context.Background()}, input)
	if err == nil {
		t.Error("expected error for failing command")
	}
}

func TestBashParseInputStandard(t *testing.T) {
	input := json.RawMessage(`{"command":"ls","timeout":15,"workdir":"src"}`)
	cmd, timeout, workdir := parseBashInput(input, "/project")
	if cmd != "ls" {
		t.Errorf("expected ls, got %s", cmd)
	}
	if timeout != 15 {
		t.Errorf("expected 15, got %d", timeout)
	}
	if filepath.ToSlash(workdir) != "/project/src" {
		t.Errorf("expected /project/src, got %s", workdir)
	}
}

func TestBashParseInputNested(t *testing.T) {
	input := json.RawMessage(`{"arguments":{"commands":[{"command":"ls -la"}]}}`)
	cmd, _, _ := parseBashInput(input, "/project")
	if cmd != "ls -la" {
		t.Errorf("expected ls -la, got %s", cmd)
	}
}

func TestBashParseInputArgumentsString(t *testing.T) {
	input := json.RawMessage(`{"name":"bash","arguments":"{\"command\":\"echo hi\"}"}`)
	cmd, _, _ := parseBashInput(input, "/project")
	if cmd != "echo hi" {
		t.Errorf("expected echo hi, got %s", cmd)
	}
}

func TestBashParseInputEmpty(t *testing.T) {
	input := json.RawMessage(`{}`)
	cmd, timeout, workdir := parseBashInput(input, "/project")
	if cmd != "" {
		t.Errorf("expected empty, got %s", cmd)
	}
	if timeout != 30 {
		t.Errorf("expected default 30, got %d", timeout)
	}
	if workdir != "/project" {
		t.Errorf("expected /project, got %s", workdir)
	}
}

func TestBashExecuteNoCommand(t *testing.T) {
	tool := &BashTool{}
	_, err := tool.Execute(ExecContext{Context: context.Background()}, json.RawMessage(`{}`))
	if err == nil {
		t.Error("expected error for empty input")
	}
}
