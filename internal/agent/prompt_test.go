package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/talen/tce/internal/llm"
	"github.com/talen/tce/internal/project"
)

func TestBuildSystemPromptContainsIdentity(t *testing.T) {
	TCEVersion = "0.1.0"
	profile := &project.Profile{
		Language:    "Go",
		BuildSystem: "go build",
		TestRunner:  "go test",
		PackageName: "test",
	}
	toolDefs := []llm.ToolDef{
		{Name: "read", Description: "Read a file"},
	}
	prompt := BuildSystemPrompt(profile, toolDefs)
	if !strings.Contains(prompt, "TCE v0.1.0") {
		t.Errorf("expected 'TCE v0.1.0' in prompt, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Terminal Coding Assistant") {
		t.Errorf("expected 'Terminal Coding Assistant' in prompt")
	}
}

func TestBuildSystemPromptSections(t *testing.T) {
	TCEVersion = "test"
	profile := &project.Profile{
		Language:    "Python",
		BuildSystem: "pip",
		TestRunner:  "pytest",
		PackageName: "myapp",
	}
	toolDefs := []llm.ToolDef{
		{Name: "grep", Description: "Search with regex"},
		{Name: "bash", Description: "Run commands"},
	}
	prompt := BuildSystemPrompt(profile, toolDefs)

	sections := []string{
		"## Context",
		"## Tools",
		"## Workflow",
		"## Rules",
		"## Anti-patterns",
		"## Project Rules",
	}
	for _, s := range sections {
		if !strings.Contains(prompt, s) {
			t.Errorf("expected section %q in prompt", s)
		}
	}
}

func TestBuildSystemPromptWorkflowSteps(t *testing.T) {
	TCEVersion = "test"
	profile := &project.Profile{Language: "Go", PackageName: "test"}
	toolDefs := []llm.ToolDef{{Name: "read", Description: "Read"}}
	prompt := BuildSystemPrompt(profile, toolDefs)

	steps := []string{
		"Understand",
		"Gather",
		"Plan",
		"Implement",
		"Verify",
		"Finalize",
	}
	for _, step := range steps {
		if !strings.Contains(prompt, step) {
			t.Errorf("expected workflow step %q in prompt", step)
		}
	}
}

func TestBuildSystemPromptAntiPatterns(t *testing.T) {
	TCEVersion = "test"
	profile := &project.Profile{Language: "C", PackageName: "libft"}
	toolDefs := []llm.ToolDef{{Name: "write", Description: "Write files"}}
	prompt := BuildSystemPrompt(profile, toolDefs)

	if !strings.Contains(prompt, "Anti-patterns") {
		t.Errorf("expected Anti-patterns section")
	}
	if !strings.Contains(prompt, "code blocks in responses") {
		t.Errorf("expected anti-pattern about code blocks")
	}
}

func TestBuildSystemPromptRulesOutputFormat(t *testing.T) {
	TCEVersion = "test"
	profile := &project.Profile{Language: "Rust", PackageName: "app"}
	toolDefs := []llm.ToolDef{{Name: "edit", Description: "Edit files"}}
	prompt := BuildSystemPrompt(profile, toolDefs)

	if !strings.Contains(prompt, `{"name":"tool_name","arguments":`) {
		t.Errorf("expected output format JSON template in prompt")
	}
	if !strings.Contains(prompt, "Brief reasoning") {
		t.Errorf("expected 'Brief reasoning' instruction")
	}
}

func TestBuildSystemPromptMinimalCompact(t *testing.T) {
	TCEVersion = "0.1.0"
	profile := &project.Profile{
		Language:    "C",
		BuildSystem: "make",
		PackageName: "ft_printf",
	}
	toolNames := []string{"read", "grep", "glob", "bash", "search"}
	prompt := BuildSystemPromptMinimal(profile, toolNames)

	if !strings.Contains(prompt, "TCE v0.1.0") {
		t.Errorf("expected version in minimal prompt")
	}
	if !strings.Contains(prompt, "## Workflow") {
		t.Errorf("expected Workflow section in minimal prompt")
	}
	if !strings.Contains(prompt, "## Output") {
		t.Errorf("expected Output section in minimal prompt")
	}
	if !strings.Contains(prompt, "## Rules") {
		t.Errorf("expected Rules section in minimal prompt")
	}
}

func TestBuildSystemPromptMinimalNoWrite(t *testing.T) {
	TCEVersion = "test"
	profile := &project.Profile{Language: "Go", PackageName: "test"}
	toolNames := []string{"read", "grep", "search"}
	prompt := BuildSystemPromptMinimal(profile, toolNames)

	if !strings.Contains(prompt, "I CANNOT write or edit") {
		t.Errorf("expected constraint note when write/edit missing")
	}
}

func TestBuildSystemPromptMinimalNoTask(t *testing.T) {
	TCEVersion = "test"
	profile := &project.Profile{Language: "Go", PackageName: "test"}
	toolNames := []string{"read", "write"}
	prompt := BuildSystemPromptMinimal(profile, toolNames)

	if !strings.Contains(prompt, "I CANNOT launch sub-agents") {
		t.Errorf("expected constraint note when task missing")
	}
}

func TestLoadProjectRulesFileExists(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".tcerules"), []byte("Always use tabs\nPrefer C99\n"), 0644)

	rules := loadProjectRules(dir)
	if !strings.Contains(rules, "Always use tabs") {
		t.Errorf("expected rules content, got %q", rules)
	}
	if !strings.Contains(rules, "Prefer C99") {
		t.Errorf("expected second rule line")
	}
}

func TestLoadProjectRulesFileEmpty(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".tcerules"), []byte("  \n  \n"), 0644)

	rules := loadProjectRules(dir)
	if rules != "(none)" {
		t.Errorf("expected '(none)' for empty file, got %q", rules)
	}
}

func TestLoadProjectRulesNoFile(t *testing.T) {
	dir := t.TempDir()
	rules := loadProjectRules(dir)
	if rules != "(none)" {
		t.Errorf("expected '(none)' when no .tcerules, got %q", rules)
	}
}

func TestLoadProjectRulesEmptyRoot(t *testing.T) {
	rules := loadProjectRules("")
	if rules != "(none)" {
		t.Errorf("expected '(none)' for empty root, got %q", rules)
	}
}

func TestBuildSystemPromptWithProjectRules(t *testing.T) {
	TCEVersion = "test"
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".tcerules"), []byte("Always compile with -Wall -Wextra -Werror"), 0644)

	profile := &project.Profile{Root: dir, Language: "C", PackageName: "libft"}
	toolDefs := []llm.ToolDef{{Name: "bash", Description: "Run commands"}}
	prompt := BuildSystemPrompt(profile, toolDefs)

	if !strings.Contains(prompt, "-Wall -Wextra -Werror") {
		t.Errorf("expected project rules content in prompt, got:\n%s", prompt)
	}
}

func BenchmarkBuildSystemPrompt(b *testing.B) {
	profile := &project.Profile{
		Language:    "Go",
		Framework:   "",
		BuildSystem: "go build",
		TestRunner:  "go test",
		PackageName: "test",
	}
	toolDefs := []llm.ToolDef{
		{Name: "read", Description: "Read a file"},
		{Name: "write", Description: "Write a file"},
		{Name: "bash", Description: "Run a command"},
		{Name: "grep", Description: "Search with regex"},
		{Name: "glob", Description: "Find files"},
		{Name: "search", Description: "Search the web"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildSystemPrompt(profile, toolDefs)
	}
}

func BenchmarkBuildSystemPromptMinimal(b *testing.B) {
	profile := &project.Profile{
		Language:    "C",
		Framework:   "",
		BuildSystem: "make",
		PackageName: "ft_printf",
	}
	toolNames := []string{"read", "grep", "glob", "bash", "search"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildSystemPromptMinimal(profile, toolNames)
	}
}

func BenchmarkBuildSystemPrompt9Tools(b *testing.B) {
	profile := &project.Profile{
		Language:    "C",
		Framework:   "",
		BuildSystem: "make",
		PackageName: "ft_printf",
	}
	toolDefs := []llm.ToolDef{
		{Name: "read", Description: "Read a file"},
		{Name: "write", Description: "Write a file"},
		{Name: "edit", Description: "Edit a file"},
		{Name: "grep", Description: "Search with regex"},
		{Name: "glob", Description: "Find files"},
		{Name: "bash", Description: "Run commands"},
		{Name: "ask", Description: "Ask the user"},
		{Name: "task", Description: "Launch sub-agent"},
		{Name: "search", Description: "Search the web"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildSystemPrompt(profile, toolDefs)
	}
}
