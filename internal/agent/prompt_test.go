package agent

import (
	"testing"

	"github.com/talen/tce/internal/llm"
	"github.com/talen/tce/internal/project"
)

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
		Framework:   "42",
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
		Framework:   "42",
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
