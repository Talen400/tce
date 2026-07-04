package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadToolName(t *testing.T) {
	tool := &ReadTool{}
	if tool.Name() != "read" {
		t.Errorf("expected read, got %s", tool.Name())
	}
}

func TestReadToolShortDescription(t *testing.T) {
	tool := &ReadTool{}
	if tool.ShortDescription() == "" {
		t.Error("short description should not be empty")
	}
}

func TestReadExecute(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "read-test-*")
	defer os.RemoveAll(tmpDir)
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("hello\nworld"), 0644)

	tool := &ReadTool{}
	input, _ := json.Marshal(map[string]any{
		"file_path": "test.txt",
	})
	result, err := tool.Execute(ExecContext{Context: context.Background(), ProjectRoot: tmpDir}, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestReadExecuteNotFound(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "read-notfound-*")
	defer os.RemoveAll(tmpDir)
	os.WriteFile(filepath.Join(tmpDir, "existing.txt"), []byte("content"), 0644)

	tool := &ReadTool{}
	input, _ := json.Marshal(map[string]any{
		"file_path": "nonexistent.txt",
	})
	result, err := tool.Execute(ExecContext{Context: context.Background(), ProjectRoot: tmpDir}, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "existing.txt") {
		t.Errorf("expected directory listing to include existing.txt, got %s", result)
	}
}

func TestReadExecuteWithOffsetLimit(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "read-test-*")
	defer os.RemoveAll(tmpDir)

	lines := make([]byte, 0)
	for i := 0; i < 20; i++ {
		lines = append(lines, []byte("line content\n")...)
	}
	os.WriteFile(filepath.Join(tmpDir, "many.txt"), lines, 0644)

	tool := &ReadTool{}
	input, _ := json.Marshal(map[string]any{
		"file_path": "many.txt",
		"offset":    5,
		"limit":     3,
	})
	result, err := tool.Execute(ExecContext{Context: context.Background(), ProjectRoot: tmpDir}, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestGrepToolName(t *testing.T) {
	tool := &GrepTool{}
	if tool.Name() != "grep" {
		t.Errorf("expected grep, got %s", tool.Name())
	}
}

func TestGrepToolShortDescription(t *testing.T) {
	tool := &GrepTool{}
	if tool.ShortDescription() == "" {
		t.Error("short description should not be empty")
	}
}

func TestGrepExecute(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "grep-test-*")
	defer os.RemoveAll(tmpDir)
	os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte("package main\nfunc main() {}\n"), 0644)

	tool := &GrepTool{}
	input, _ := json.Marshal(map[string]any{
		"pattern": "func",
	})
	result, err := tool.Execute(ExecContext{Context: context.Background(), ProjectRoot: tmpDir}, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestGrepExecuteWithRegex(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "grep-test-*")
	defer os.RemoveAll(tmpDir)
	os.WriteFile(filepath.Join(tmpDir, "data.txt"), []byte("abc123\ndef456\nxyz789\n"), 0644)

	tool := &GrepTool{}
	input, _ := json.Marshal(map[string]any{
		"pattern": "[0-9]+",
	})
	result, err := tool.Execute(ExecContext{Context: context.Background(), ProjectRoot: tmpDir}, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestGrepExecuteInvalidRegex(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "grep-test-*")
	defer os.RemoveAll(tmpDir)
	os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte("package main"), 0644)

	tool := &GrepTool{}
	input, _ := json.Marshal(map[string]any{
		"pattern": "[invalid",
	})
	_, err := tool.Execute(ExecContext{Context: context.Background(), ProjectRoot: tmpDir}, input)
	if err == nil {
		t.Error("expected error for invalid regex")
	}
}

func TestGrepExecuteNoMatch(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "grep-test-*")
	defer os.RemoveAll(tmpDir)
	os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte("package main"), 0644)

	tool := &GrepTool{}
	input, _ := json.Marshal(map[string]any{
		"pattern": "ZZZNOTFOUND",
	})
	result, err := tool.Execute(ExecContext{Context: context.Background(), ProjectRoot: tmpDir}, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected 'No matches found' type result")
	}
}

func TestGrepExecuteWithInclude(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "grep-test-*")
	defer os.RemoveAll(tmpDir)
	os.WriteFile(filepath.Join(tmpDir, "a.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("package main"), 0644)

	tool := &GrepTool{}
	input, _ := json.Marshal(map[string]any{
		"pattern": "package",
		"include": "*.go",
	})
	result, err := tool.Execute(ExecContext{Context: context.Background(), ProjectRoot: tmpDir}, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestGlobToolName(t *testing.T) {
	tool := &GlobTool{}
	if tool.Name() != "glob" {
		t.Errorf("expected glob, got %s", tool.Name())
	}
}

func TestGlobToolShortDescription(t *testing.T) {
	tool := &GlobTool{}
	if tool.ShortDescription() == "" {
		t.Error("short description should not be empty")
	}
}

func TestGlobExecute(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "glob-test-*")
	defer os.RemoveAll(tmpDir)
	os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "test.rs"), []byte("fn main() {}"), 0644)

	tool := &GlobTool{}
	input, _ := json.Marshal(map[string]any{
		"pattern": "*.go",
	})
	result, err := tool.Execute(ExecContext{Context: context.Background(), ProjectRoot: tmpDir}, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestGlobExecuteNoMatch(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "glob-test-*")
	defer os.RemoveAll(tmpDir)

	tool := &GlobTool{}
	input, _ := json.Marshal(map[string]any{
		"pattern": "*.py",
	})
	result, err := tool.Execute(ExecContext{Context: context.Background(), ProjectRoot: tmpDir}, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected 'No files matching' type result")
	}
}

func TestGlobExecuteWithPath(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "glob-test-*")
	defer os.RemoveAll(tmpDir)
	os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "subdir", "test.go"), []byte("package main"), 0644)

	tool := &GlobTool{}
	input, _ := json.Marshal(map[string]any{
		"pattern": "*.go",
		"path":    "subdir",
	})
	result, err := tool.Execute(ExecContext{Context: context.Background(), ProjectRoot: tmpDir}, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func BenchmarkGrepSmallFile(b *testing.B) {
	tmpDir := b.TempDir()
	content := strings.Repeat("line of code\n", 50)
	os.WriteFile(filepath.Join(tmpDir, "small.txt"), []byte(content), 0644)

	tool := &GrepTool{}
	input, _ := json.Marshal(map[string]string{"pattern": "code"})
	ctx := ExecContext{Context: context.Background(), ProjectRoot: tmpDir}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tool.Execute(ctx, input)
	}
}

func BenchmarkGrepLargeFile(b *testing.B) {
	tmpDir := b.TempDir()
	content := strings.Repeat("line of code\n", 5000)
	os.WriteFile(filepath.Join(tmpDir, "large.txt"), []byte(content), 0644)

	tool := &GrepTool{}
	input, _ := json.Marshal(map[string]string{"pattern": "code"})
	ctx := ExecContext{Context: context.Background(), ProjectRoot: tmpDir}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tool.Execute(ctx, input)
	}
}

func BenchmarkReadSmallFile(b *testing.B) {
	tmpDir := b.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "small.txt"), []byte("hello world"), 0644)

	tool := &ReadTool{}
	input, _ := json.Marshal(map[string]string{"file_path": "small.txt"})
	ctx := ExecContext{Context: context.Background(), ProjectRoot: tmpDir}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tool.Execute(ctx, input)
	}
}
