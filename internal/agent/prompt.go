package agent

import (
	"fmt"
	"strings"

	"github.com/talen/tce/internal/llm"
	"github.com/talen/tce/internal/project"
)

const systemTemplate = `You are tce, a terminal coding assistant.

<project>
Language: %s
Framework: %s
Build: %s
Test: %s
Package: %s
</project>

<rules>
- Write production-ready, idiomatic %s code
- When searching, prefer grep and glob before reading files
- Use bash for running commands, tests, and git operations
- If unsure, use the ask tool to get clarification
- When the task is complete, provide a summary
</rules>

<tools>
%s
</tools>

Think step by step, then use tools or provide the answer.
`

func BuildSystemPrompt(profile *project.Profile, toolDefs []llm.ToolDef) string {
	var toolDocs strings.Builder
	for _, td := range toolDefs {
		toolDocs.WriteString(fmt.Sprintf("- %s: %s\n", td.Name, td.Description))
	}

	lang := profile.Language
	if lang == "Unknown" || lang == "" {
		lang = "the project's"
	}

	return fmt.Sprintf(systemTemplate,
		profile.Summary(),
		profile.Framework,
		profile.BuildSystem,
		profile.TestRunner,
		profile.PackageName,
		strings.ToLower(lang),
		toolDocs.String(),
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
		extra += "- I CANNOT write or edit files. I can only read and explore.\n"
	}
	if !hasTask {
		extra += "- I CANNOT launch sub-agents.\n"
	}

	return fmt.Sprintf(`You are tce, a coding assistant for %s.

Project: %s (%s)
Tools: %s

GOOD EXAMPLES (use tools):
- Read file: {"name":"read","arguments":{"file_path":"main.c"}}
- Search code: {"name":"grep","arguments":{"pattern":"ft_printf"}}
- Find files: {"name":"glob","arguments":{"pattern":"*.c"}}
- Write file: {"name":"write","arguments":{"file_path":"ft_printf.c","content":"#include ..."}}
- Web search: {"name":"search","arguments":{"query":"ft_printf implementation C"}}

BAD EXAMPLES (never do these):
- BAD: writing code directly in your response
- BAD: inventing function signatures or APIs
- BAD: answering with code instead of using write tool
- BAD: guessing an API — use search tool before writing code

RULES:
- NEVER write code directly. Always use write/edit tools.
- NEVER invent function signatures. Use search tool to discover them.
- If you don't know a function or library: search first, then write.
- Only answer directly for greetings or simple questions.
- At most ONE tool call per response.
- Output ONLY valid JSON or plain text, never both.
%s
User: `,
		strings.ToLower(lang),
		profile.PackageName,
		profile.Summary(),
		tools,
		extra,
	)
}
