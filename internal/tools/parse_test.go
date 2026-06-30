package tools

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFirstOfFound(t *testing.T) {
	m := map[string]any{"name": "test", "path": "main.go"}
	if s := firstOf(m, "name", "path"); s != "test" {
		t.Errorf("expected test, got %s", s)
	}
}

func TestFirstOfFallback(t *testing.T) {
	m := map[string]any{"path": "main.go"}
	if s := firstOf(m, "name", "path"); s != "main.go" {
		t.Errorf("expected main.go, got %s", s)
	}
}

func TestFirstOfNotFound(t *testing.T) {
	m := map[string]any{}
	if s := firstOf(m, "name", "path"); s != "" {
		t.Errorf("expected empty, got %s", s)
	}
}

func TestFirstOfEmptyValue(t *testing.T) {
	m := map[string]any{"name": "", "path": "main.go"}
	if s := firstOf(m, "name", "path"); s != "main.go" {
		t.Errorf("expected main.go, got %s", s)
	}
}

func TestFmtErrFull(t *testing.T) {
	input := json.RawMessage(`{"wrong":"field"}`)
	result := fmtErr("missing file_path", `{"file_path":"x.txt"}`, input)
	if !strings.Contains(result, "Error: missing file_path") {
		t.Errorf("should contain Error message, got: %s", result)
	}
	if !strings.Contains(result, "Expected:") {
		t.Errorf("should contain Expected, got: %s", result)
	}
	if !strings.Contains(result, "You sent:") {
		t.Errorf("should contain You sent, got: %s", result)
	}
}

func TestFmtErrNoInput(t *testing.T) {
	result := fmtErr("missing file_path", `{"file_path":"x.txt"}`, nil)
	if strings.Contains(result, "You sent:") {
		t.Errorf("should not contain 'You sent' when input is nil, got: %s", result)
	}
}

func TestFmtErrNullInput(t *testing.T) {
	result := fmtErr("missing file_path", `{"file_path":"x.txt"}`, json.RawMessage("null"))
	if strings.Contains(result, "You sent:") {
		t.Errorf("should not contain 'You sent' when input is null, got: %s", result)
	}
}

func TestTryFixJSONAlreadyValid(t *testing.T) {
	input := json.RawMessage(`{"name":"test"}`)
	fixed := tryFixJSON(input)
	if fixed == nil {
		t.Fatal("expected non-nil")
	}
	if string(fixed) != `{"name":"test"}` {
		t.Errorf("expected unchanged, got %s", string(fixed))
	}
}

func TestTryFixJSONLeadingGarbage(t *testing.T) {
	input := json.RawMessage(`Here is the result: {"name":"test"} okay`)
	fixed := tryFixJSON(input)
	if fixed == nil {
		t.Fatal("expected non-nil")
	}
	if string(fixed) != `{"name":"test"}` {
		t.Errorf("expected cleaned JSON, got %s", string(fixed))
	}
}

func TestTryFixJSONNewlines(t *testing.T) {
	input := json.RawMessage("{\"name\":\n\"test\"}")
	fixed := tryFixJSON(input)
	if fixed == nil {
		t.Fatal("expected non-nil for newline inside JSON")
	}
}

func TestTryFixJSONEmpty(t *testing.T) {
	if fixed := tryFixJSON(json.RawMessage("")); fixed != nil {
		t.Error("expected nil for empty input")
	}
	if fixed := tryFixJSON(json.RawMessage("   ")); fixed != nil {
		t.Error("expected nil for whitespace input")
	}
}

func TestTryFixJSONNoBraces(t *testing.T) {
	if fixed := tryFixJSON(json.RawMessage("hello world")); fixed != nil {
		t.Error("expected nil for input without braces")
	}
}

func TestFuzzyMatchExact(t *testing.T) {
	names := []string{"read", "write", "edit", "grep"}
	if got := fuzzyMatch("read", names); got != "read" {
		t.Errorf("expected read, got %s", got)
	}
}

func TestFuzzyMatchCaseInsensitive(t *testing.T) {
	names := []string{"ReadTool", "WriteTool"}
	if got := fuzzyMatch("readtool", names); got != "ReadTool" {
		t.Errorf("expected ReadTool, got %s", got)
	}
}

func TestFuzzyMatchPrefix(t *testing.T) {
	names := []string{"bash", "read", "write"}
	if got := fuzzyMatch("ba", names); got != "bash" {
		t.Errorf("expected bash, got %s", got)
	}
}

func TestFuzzyMatchAmbiguous(t *testing.T) {
	names := []string{"read", "write", "remove"}
	if got := fuzzyMatch("re", names); got != "" {
		t.Errorf("expected empty for ambiguous prefix, got %s", got)
	}
}

func TestFuzzyMatchNoMatch(t *testing.T) {
	names := []string{"read", "write"}
	if got := fuzzyMatch("zzz", names); got != "" {
		t.Errorf("expected empty, got %s", got)
	}
}

func TestFuzzyMatchEmpty(t *testing.T) {
	names := []string{"read", "write"}
	if got := fuzzyMatch("", names); got != "" {
		t.Errorf("expected empty for empty input, got %s", got)
	}
}

func BenchmarkTryFixJSONValid(b *testing.B) {
	input := json.RawMessage(`{"command":"go test ./...","timeout":30}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tryFixJSON(input)
	}
}

func BenchmarkTryFixJSONGarbage(b *testing.B) {
	input := json.RawMessage(`Here is: {"command":"go test ./...","timeout":30} with trailing text`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tryFixJSON(input)
	}
}

func BenchmarkTryFixJSONLarge(b *testing.B) {
	input := json.RawMessage(`{"content":"` + strings.Repeat("a", 10000) + `"}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tryFixJSON(input)
	}
}

func BenchmarkFuzzyMatch(b *testing.B) {
	names := []string{"read", "write", "edit", "grep", "glob", "bash", "ask", "task", "search"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fuzzyMatch("re", names)
	}
}
