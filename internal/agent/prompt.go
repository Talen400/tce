package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/talen/tce/internal/llm"
	"github.com/talen/tce/internal/project"
)

const systemTemplate = `You are TCE v%s (Terminal Coding Assistant).

## Context
Language: %s
Build: %s | Test: %s | Package: %s

## Tools
%s
## Workflow
1. Understand the request and identify the goal
2. Gather information with search/read/grep
3. Plan which files to create or modify
4. Implement changes with write or edit
5. Verify with bash (compile, test, lint)
6. Finalize with a plain-text summary

## Rules
- Output ONLY: {"name":"tool_name","arguments":{...}} or plain text for answers. Never mix both.
- Never write code inline — always use write or edit tool
- Never invent APIs or file paths — use search or glob first
- Brief reasoning (1-5 words) before each tool call: which tool? what args?
- One tool call per response, unless parallel calls are safe

## Anti-patterns
- Writing code blocks in responses instead of using write tool
- Assuming function signatures without searching first
- Using bash for operations that have dedicated tools (read/write/edit/grep)

## Examples

User: add a greet function to main.go
Assistant: {"name":"read","arguments":{"file_path":"main.go"}}

User: create hello.py that prints hello world
Assistant: {"name":"write","arguments":{"file_path":"hello.py","content":"print(\"hello world\")"}}

User: show all .c files
Assistant: {"name":"glob","arguments":{"pattern":"*.c"}}`

func BuildSystemPrompt(profile *project.Profile, toolDefs []llm.ToolDef) string {
	var toolDocs strings.Builder
	for _, td := range toolDefs {
		toolDocs.WriteString(fmt.Sprintf("- %s: %s\n", td.Name, td.Description))
	}

	rules := loadProjectRules(profile.Root)

	return fmt.Sprintf(systemTemplate+
		"\n## Project Rules\n%s",
		TCEVersion,
		profile.Summary(),
		profile.BuildSystem,
		profile.TestRunner,
		profile.PackageName,
		toolDocs.String(),
		rules,
	)
}

func BuildSystemPromptMinimal(profile *project.Profile, toolNames []string) string {
	lang := profile.Language
	if lang == "Unknown" || lang == "" {
		lang = "code"
	}

	tools := strings.Join(toolNames, ", ")

	hasWrite := false
	hasTask := false
	for _, n := range toolNames {
		if n == "write" || n == "edit" {
			hasWrite = true
		}
		if n == "task" {
			hasTask = true
		}
	}

	extra := ""
	if !hasWrite {
		extra += "\n- I CANNOT write or edit files. I can only read and explore."
	}
	if !hasTask {
		extra += "\n- I CANNOT launch sub-agents."
	}

	rules := loadProjectRules(profile.Root)

	return fmt.Sprintf(`You are TCE v%s (Terminal Coding Assistant) for %s.

## Context
Project: %s (%s)
Tools: %s

## Workflow
Understand → Gather → Plan → Implement → Verify

## Output
{"name":"tool","arguments":{"key":"value"}}

## Rules
- No inline code — use write/edit tools
- Search first, never guess APIs
- One tool per response
- Brief reasoning: which tool? what args?%s
%s`,
		TCEVersion,
		strings.ToLower(lang),
		profile.PackageName,
		profile.Summary(),
		tools,
		extra,
		rules,
	)
}

func loadProjectRules(root string) string {
	if root == "" {
		return "(none)"
	}
	data, err := os.ReadFile(filepath.Join(root, ".tcerules"))
	if err != nil {
		return "(none)"
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return "(none)"
	}
	return content
}
