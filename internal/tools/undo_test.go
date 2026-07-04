package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUndoPushAndPop(t *testing.T) {
	ClearUndo()
	defer ClearUndo()

	dir, _ := os.MkdirTemp("", "undo-test-*")
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("original"), 0644)

	PushUndo(path)

	os.WriteFile(path, []byte("modified"), 0644)

	result := PopUndo()
	if result == "" || result == "Nothing to undo." {
		t.Fatalf("expected undo to succeed, got: %s", result)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "original" {
		t.Errorf("expected file content 'original', got %q", string(data))
	}
}

func TestUndoEmptyStack(t *testing.T) {
	ClearUndo()
	defer ClearUndo()

	result := PopUndo()
	if result != "Nothing to undo." {
		t.Errorf("expected 'Nothing to undo.', got %q", result)
	}
}

func TestUndoMultiple(t *testing.T) {
	ClearUndo()
	defer ClearUndo()

	dir, _ := os.MkdirTemp("", "undo-test-*")
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("v1"), 0644)
	PushUndo(path)

	os.WriteFile(path, []byte("v2"), 0644)
	PushUndo(path)

	os.WriteFile(path, []byte("v3"), 0644)

	PopUndo()
	data, _ := os.ReadFile(path)
	if string(data) != "v2" {
		t.Errorf("expected v2 after first undo, got %q", string(data))
	}

	PopUndo()
	data, _ = os.ReadFile(path)
	if string(data) != "v1" {
		t.Errorf("expected v1 after second undo, got %q", string(data))
	}
}

func TestUndoNonexistentFile(t *testing.T) {
	ClearUndo()
	defer ClearUndo()

	PushUndo("/nonexistent/path/file.txt")

	result := PopUndo()
	if result != "Nothing to undo." {
		t.Errorf("expected 'Nothing to undo.', got %q", result)
	}
}

func TestUndoToolName(t *testing.T) {
	tool := &UndoTool{}
	if tool.Name() != "undo" {
		t.Errorf("expected undo, got %s", tool.Name())
	}
}

func TestUndoToolShortDescription(t *testing.T) {
	tool := &UndoTool{}
	if tool.ShortDescription() == "" {
		t.Error("short description should not be empty")
	}
}
