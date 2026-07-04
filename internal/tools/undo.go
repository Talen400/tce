package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

type undoEntry struct {
	filePath string
	content  string
}

var (
	undoMu sync.Mutex
	undos  []undoEntry
)

// PushUndo saves the current content of a file so it can be restored later via PopUndo.
func PushUndo(filePath string) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return
	}
	undoMu.Lock()
	undos = append(undos, undoEntry{filePath: filePath, content: string(data)})
	undoMu.Unlock()
}

// PopUndo restores the most recently saved file content and returns a description.
func PopUndo() string {
	undoMu.Lock()
	if len(undos) == 0 {
		undoMu.Unlock()
		return "Nothing to undo."
	}
	entry := undos[len(undos)-1]
	undos = undos[:len(undos)-1]
	undoMu.Unlock()

	if err := os.WriteFile(entry.filePath, []byte(entry.content), 0644); err != nil {
		return fmt.Sprintf("Undo failed: %v", err)
	}
	return fmt.Sprintf("Undone: restored %s (%d bytes)", entry.filePath, len(entry.content))
}

// ClearUndo empties the undo stack.
func ClearUndo() {
	undoMu.Lock()
	undos = nil
	undoMu.Unlock()
}

type UndoTool struct{}

func (t *UndoTool) Name() string { return "undo" }
func (t *UndoTool) Description() string {
	return "Undo the most recent write or edit operation by restoring the original file content."
}
func (t *UndoTool) ShortDescription() string { return "Undo last write/edit" }

func (t *UndoTool) Schema() any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (t *UndoTool) Execute(ctx ExecContext, input json.RawMessage) (string, error) {
	result := PopUndo()
	return fmt.Sprintf("❯ undo()\n── %s ──", result), nil
}
