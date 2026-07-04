package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type WriteTool struct{}

func (t *WriteTool) Name() string { return "write" }
func (t *WriteTool) Description() string {
	return "Create a new file or overwrite an existing file with new content."
}
func (t *WriteTool) ShortDescription() string { return "Create/overwrite files" }

func (t *WriteTool) Schema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "Path to the file to write (absolute or relative to project root)",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "The full content to write to the file",
			},
		},
		"required": []string{"file_path", "content"},
	}
}

type writeInput struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

func (t *WriteTool) Execute(ctx ExecContext, input json.RawMessage) (string, error) {
	var in writeInput
	if err := json.Unmarshal(input, &in); err != nil || in.FilePath == "" {
		var raw map[string]any
		if err := json.Unmarshal(input, &raw); err != nil {
			if fixed := tryFixJSON(input); fixed != nil {
				return t.Execute(ctx, fixed)
			}
			return "", fmt.Errorf("invalid input: %w", err)
		}
		in.FilePath = firstOf(raw, "file_path", "path", "file", "filename", "name", "target")
		in.Content = firstOf(raw, "content", "text", "data", "code", "file_content", "body", "source", "code_content")
	}

	if in.FilePath == "" {
		return "", fmt.Errorf("%s", fmtErr("missing \"file_path\"", `{"file_path": "output.txt", "content": "text here"}`, input))
	}
	if in.Content == "" {
		return "", fmt.Errorf("%s", fmtErr("missing \"content\"", `{"file_path": "output.txt", "content": "text here"}`, input))
	}

	path := in.FilePath
	if !filepath.IsAbs(path) {
		path = filepath.Join(ctx.ProjectRoot, path)
	}

	if _, err := os.Stat(path); err == nil {
		PushUndo(path)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create directories: %w", err)
	}

	if err := os.WriteFile(path, []byte(in.Content), 0644); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	rel, _ := filepath.Rel(ctx.ProjectRoot, path)
	lines := strings.Count(in.Content, "\n") + 1
	return fmt.Sprintf("❯ write(%s)\n── Written %d lines ──\n(backup saved — use 'undo' to revert)", rel, lines), nil
}

type EditTool struct{}

func (t *EditTool) Name() string { return "edit" }
func (t *EditTool) Description() string {
	return "Make precise edits to a file using exact find/replace. The old_string must match exactly once in the file."
}
func (t *EditTool) ShortDescription() string { return "Edit files (find/replace)" }

func (t *EditTool) Schema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "Path to the file to edit (absolute or relative to project root)",
			},
			"old_string": map[string]any{
				"type":        "string",
				"description": "The exact text to find (must match exactly once)",
			},
			"new_string": map[string]any{
				"type":        "string",
				"description": "The replacement text",
			},
		},
		"required": []string{"file_path", "old_string", "new_string"},
	}
}

type editInput struct {
	FilePath  string `json:"file_path"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

func (t *EditTool) Execute(ctx ExecContext, input json.RawMessage) (string, error) {
	var in editInput
	if err := json.Unmarshal(input, &in); err != nil || in.FilePath == "" {
		var raw map[string]any
		if err := json.Unmarshal(input, &raw); err != nil {
			if fixed := tryFixJSON(input); fixed != nil {
				return t.Execute(ctx, fixed)
			}
			return "", fmt.Errorf("invalid input: %w", err)
		}
		in.FilePath = firstOf(raw, "file_path", "path", "file")
		in.OldString = firstOf(raw, "old_string", "old", "find", "from")
		in.NewString = firstOf(raw, "new_string", "new", "replace", "to")
	}

	if in.FilePath == "" {
		return "", fmt.Errorf("%s", fmtErr("missing \"file_path\"", `{"file_path": "file.go", "old_string": "old", "new_string": "new"}`, input))
	}

	path := in.FilePath
	if !filepath.IsAbs(path) {
		path = filepath.Join(ctx.ProjectRoot, path)
	}

	// Build files are structurally sensitive — require explicit confirmation
	if ctx.ReadInput != nil && isBuildFile(in.FilePath) {
		answer, err := ctx.ReadInput("⚠️ Editing build file " + in.FilePath + ". Confirm? (Y/n): ")
		if err != nil || strings.ToLower(answer) != "y" {
			return "Edit cancelled — build file requires confirmation", nil
		}
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	text := string(content)

	// Empty old_string = append mode: read file and append new_string at end.
	// Skips diff preview and confirmation since there's no risk of replacement ambiguity.
	if in.OldString == "" {
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		newText := text + in.NewString
		PushUndo(path)
		if err := os.WriteFile(path, []byte(newText), 0644); err != nil {
			return "", fmt.Errorf("write file: %w", err)
		}
		rel, _ := filepath.Rel(ctx.ProjectRoot, path)
		return fmt.Sprintf("Appended %d bytes to %s", len(newText)-len(text), rel), nil
	}
	count := strings.Count(text, in.OldString)

	if count == 0 {
		return "", fmt.Errorf("old_string not found in file %s", path)
	}
	if count > 1 {
		return "", fmt.Errorf("found %d matches for old_string in %s. Provide more surrounding context in old_string to identify the correct match", count, path)
	}

	newText := strings.Replace(text, in.OldString, in.NewString, 1)

	diff := unifiedDiff(text, newText)
	prompt := fmt.Sprintf("Proposed edit to %s:\n%s\nApply? (Y/n): ", in.FilePath, diff)
	if ctx.ReadInput != nil {
		answer, err := ctx.ReadInput(prompt)
		if err != nil || strings.ToLower(answer) == "n" || strings.ToLower(answer) == "no" {
			return "Edit cancelled by user", nil
		}
	}

	PushUndo(path)

	if err := os.WriteFile(path, []byte(newText), 0644); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	rel, _ := filepath.Rel(ctx.ProjectRoot, path)
	oldLines := strings.Count(in.OldString, "\n") + 1
	newLines := strings.Count(in.NewString, "\n") + 1
	return fmt.Sprintf("❯ edit(%s)\n── Replaced %d lines with %d lines ──\n%s\n(backup saved — use 'undo' to revert)", rel, oldLines, newLines, diff), nil
}

func isBuildFile(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	switch base {
	case "makefile", "cmakelists.txt", "dockerfile", "docker-compose.yml",
		"docker-compose.yaml", "composer.json", "package.json", "gemfile",
		"cargo.toml", "build.gradle", "build.gradle.kts", ".gitignore",
		".gitlab-ci.yml", ".github/workflows/ci.yml":
		return true
	}
	return strings.HasSuffix(base, ".mk")
}
