package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteToolName(t *testing.T) {
	tool := &WriteTool{}
	if tool.Name() != "write" {
		t.Errorf("expected write, got %s", tool.Name())
	}
}

func TestWriteToolShortDescription(t *testing.T) {
	tool := &WriteTool{}
	if tool.ShortDescription() == "" {
		t.Error("short description should not be empty")
	}
}

func TestWriteExecute(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "write-test-*")
	defer os.RemoveAll(tmpDir)

	tool := &WriteTool{}
	input, _ := json.Marshal(map[string]any{
		"file_path": "test.txt",
		"content":   "hello world",
	})
	result, err := tool.Execute(ExecContext{Context: context.Background(), ProjectRoot: tmpDir}, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}

	data, _ := os.ReadFile(filepath.Join(tmpDir, "test.txt"))
	if string(data) != "hello world" {
		t.Errorf("expected 'hello world', got %s", string(data))
	}
}

func TestWriteExecuteInvalidPath(t *testing.T) {
	tool := &WriteTool{}
	input, _ := json.Marshal(map[string]any{
		"file_path": "",
		"content":   "hello",
	})
	_, err := tool.Execute(ExecContext{Context: context.Background()}, input)
	if err == nil {
		t.Error("expected error for empty file path")
	}
}

func TestEditToolName(t *testing.T) {
	tool := &EditTool{}
	if tool.Name() != "edit" {
		t.Errorf("expected edit, got %s", tool.Name())
	}
}

func TestEditToolShortDescription(t *testing.T) {
	tool := &EditTool{}
	if tool.ShortDescription() == "" {
		t.Error("short description should not be empty")
	}
}

func TestEditExecute(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "edit-test-*")
	defer os.RemoveAll(tmpDir)

	origPath := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(origPath, []byte("hello world\nfoo bar"), 0644)

	tool := &EditTool{}
	input, _ := json.Marshal(map[string]any{
		"file_path":  "test.txt",
		"old_string": "hello world",
		"new_string": "hi there",
	})
	result, err := tool.Execute(ExecContext{Context: context.Background(), ProjectRoot: tmpDir}, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}

	data, _ := os.ReadFile(origPath)
	if string(data) != "hi there\nfoo bar" {
		t.Errorf("expected 'hi there\\nfoo bar', got %s", string(data))
	}
}

func TestEditExecuteNotFound(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "edit-test-*")
	defer os.RemoveAll(tmpDir)

	origPath := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(origPath, []byte("hello world"), 0644)

	tool := &EditTool{}
	input, _ := json.Marshal(map[string]any{
		"file_path":  "test.txt",
		"old_string": "nonexistent",
		"new_string": "replacement",
	})
	_, err := tool.Execute(ExecContext{Context: context.Background(), ProjectRoot: tmpDir}, input)
	if err == nil {
		t.Error("expected error when old_string not found")
	}
}

func TestEditExecuteEmptyOldString(t *testing.T) {
	tool := &EditTool{}
	_, err := tool.Execute(ExecContext{}, json.RawMessage(`{"file_path":"x","old_string":"","new_string":"y"}`))
	if err == nil {
		t.Error("expected error for empty old_string")
	}
}
