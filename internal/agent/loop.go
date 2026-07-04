package agent

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/talen/tce/internal/compactor"
	"github.com/talen/tce/internal/llm"
	"github.com/talen/tce/internal/permission"
	"github.com/talen/tce/internal/project"
	"github.com/talen/tce/internal/tools"
)

type AgentType string

const (
	AgentBuild   AgentType = "build"
	AgentPlan    AgentType = "plan"
	AgentExplore AgentType = "explore"
	AgentGeneral AgentType = "general"
)

var permissionSets = map[AgentType][]permission.Rule{
	AgentBuild:   permission.BuildRules,
	AgentPlan:    permission.PlanRules,
	AgentExplore: permission.ExploreRules,
	AgentGeneral: permission.GeneralRules,
}

var codeRequestPatterns = []string{
	"faz um", "faça um", "faça uma", "faz uma",
	"implement", "write a", "make a", "cria", "crie",
	"desenvolva", "desenvolve", "create a", "build a",
	"crie um", "cria um",
}

var minimalSafeTools = map[string]bool{
	"read": true, "grep": true, "glob": true, "bash": true, "search": true,
}

type LLMClient interface {
	Chat(ctx context.Context, messages []llm.Message, tools []llm.ToolDef) (*llm.Response, error)
	ChatStream(ctx context.Context, messages []llm.Message, tools []llm.ToolDef, onChunk func(string)) (*llm.Response, error)
	ModelName() string
}

type Config struct {
	Type            AgentType
	LLM             LLMClient
	Tools           *tools.Registry
	Project         *project.Profile
	Stdin           io.Reader
	MaxTurns        int
	MinimalMode     bool
	MaxContext      int
	ForceSingleCall bool
	KeepTurns       int
	MaxToolContent  int
	TokenRatio      float64
	DisableStream   bool
	Verbose         bool
}

type Agent struct {
	cfg               Config
	perm              *permission.Checker
	compactor         *compactor.Compactor
	messages          []llm.Message
	cachedToolDefs    []llm.ToolDef
	depth             int
	jsonRetries       int
	turnCount         int
	totalInputTokens  int
	totalOutputTokens int
}

func New(cfg Config) *Agent {
	rules := permissionSets[cfg.Type]
	if rules == nil {
		rules = permission.BuildRules
	}

	maxCtx := cfg.MaxContext
	if maxCtx <= 0 {
		maxCtx = 24000
	}

	keepTurns := cfg.KeepTurns
	if keepTurns <= 0 {
		keepTurns = 2
	}

	maxToolContent := cfg.MaxToolContent
	if maxToolContent <= 0 {
		maxToolContent = 1000
	}

	tokenRatio := cfg.TokenRatio
	if tokenRatio <= 0 {
		tokenRatio = 3.5
	}

	a := &Agent{
		cfg:  cfg,
		perm: permission.NewChecker(rules),
		compactor: compactor.NewWithConfig(compactor.Config{
			MaxContextTokens:  maxCtx,
			KeepTurns:         keepTurns,
			MaxToolContentLen: maxToolContent,
			TokenRatio:        tokenRatio,
		}),
	}
	a.cachedToolDefs = a.allowedToolDefs()
	return a
}

func (a *Agent) Reset() {
	a.messages = nil
}

func (a *Agent) Run(ctx context.Context, userPrompt string, onToken func(string), onToolStart func(name, args string), onToolEnd func(name, result string)) (string, error) {
	if len(a.messages) == 0 {
		if a.cfg.MinimalMode {
			toolNames := a.allowedToolNames()
			sysPrompt := BuildSystemPromptMinimal(a.cfg.Project, toolNames)
			a.messages = append(a.messages, llm.Message{Role: "system", Content: sysPrompt})
		} else {
			toolDefs := a.cachedToolDefs
			sysPrompt := BuildSystemPrompt(a.cfg.Project, toolDefs)
			a.messages = append(a.messages, llm.Message{Role: "system", Content: sysPrompt})
		}
		a.autoBootstrap(ctx)
	}

	a.messages = append(a.messages, llm.Message{Role: "user", Content: userPrompt})

	maxTurns := a.cfg.MaxTurns
	if maxTurns == 0 {
		maxTurns = 25
	}

	turn := 0
	toolErrors := make(map[string]int)
	fileEditFails := make(map[string]int)

	// Outer loop: each iteration is one user-facing "turn".
	// Inner loop: each iteration is one LLM call + tool execution round.
	// The inner loop lets the model self-correct tool errors without
	// consuming extra user turns.
	for turn < maxTurns {
		turn++
		a.turnCount++

		for inner := 0; inner < 12; inner++ {
			a.messages = a.compactor.Compact(a.messages)

			var streamedContent string
			var resp *llm.Response
			var err error

			// Estimate input tokens
			for _, m := range a.messages {
				a.totalInputTokens += estimateTokens(m.Content, a.cfg.TokenRatio)
			}

			if a.cfg.DisableStream {
				resp, err = a.cfg.LLM.Chat(ctx, a.messages, a.cachedToolDefs)
				if err == nil {
					streamedContent = resp.Content
					a.totalOutputTokens += estimateTokens(resp.Content, a.cfg.TokenRatio)
					if onToken != nil {
						onToken(resp.Content)
					}
				}
			} else {
				resp, err = a.cfg.LLM.ChatStream(ctx, a.messages, a.cachedToolDefs, func(chunk string) {
					streamedContent += chunk
					if onToken != nil {
						onToken(chunk)
					}
				})
				if err == nil {
					a.totalOutputTokens += estimateTokens(streamedContent, a.cfg.TokenRatio)
				}
			}
			if err != nil {
				return "", fmt.Errorf("LLM call (turn %d): %w", turn, err)
			}

			// Verbose: print tool call payloads
			if a.cfg.Verbose && len(resp.ToolCalls) > 0 {
				for _, tc := range resp.ToolCalls {
					fmt.Fprintf(os.Stderr, "[verbose] tool call: %s(%s)\n", tc.Name, tc.Arguments)
				}
			}

			if len(resp.ToolCalls) == 0 {
				if streamedContent == "" {
					streamedContent = "(no response)"
				}

				// Retry with reminder if the model attempted JSON but failed format
				if a.jsonRetries < 1 && looksLikeFailedJSON(streamedContent) {
					a.jsonRetries++
					a.messages = append(a.messages, llm.Message{Role: "assistant", Content: streamedContent})
					a.messages = append(a.messages, llm.Message{
						Role: "user",
						Content: "Your response contained a malformed tool call JSON. " +
							"Follow this EXACT format with no surrounding text:\n" +
							`{"name":"tool_name","arguments":{"param":"value"}}`,
					})
					continue
				}

				if turn == 1 && isCodeRequest(userPrompt) {
					return "", fmt.Errorf("code requests require tool use — implement using write/edit tools, not direct answers")
				}

				if hasHallucinatedCode(streamedContent) {
					// Feed inline code back to model instead of aborting — model retries with write tool
					a.messages = append(a.messages, llm.Message{Role: "assistant", Content: streamedContent})
					a.messages = append(a.messages, llm.Message{
						Role:    "user",
						Content: "You wrote code inline instead of using write/edit. Retry using the write tool with the same content.",
					})
					continue
				}

				a.messages = append(a.messages, llm.Message{Role: "assistant", Content: streamedContent})
				return streamedContent, nil
			}

			if a.cfg.ForceSingleCall && len(resp.ToolCalls) > 1 {
				resp.ToolCalls = resp.ToolCalls[:1]
			}

			// If tool calls were extracted from text (JSON fallback), clear streamed content
			// to avoid adding raw JSON as assistant content
			if len(resp.ToolCalls) > 0 && strings.HasPrefix(strings.TrimSpace(streamedContent), "{") && strings.Contains(streamedContent, `"name"`) {
				streamedContent = ""
			}

			a.messages = append(a.messages, llm.Message{
				Role:      "assistant",
				Content:   streamedContent,
				ToolCalls: resp.ToolCalls,
			})

			for _, tc := range resp.ToolCalls {
				action, msg := a.perm.Check(tc.Name)

				if action == permission.Deny {
					errMsg := fmt.Sprintf("Tool %q denied: %s", tc.Name, msg)
					if onToolEnd != nil {
						onToolEnd(tc.Name, errMsg)
					}
					a.messages = append(a.messages, llm.Message{
						Role:       "tool",
						ToolCallID: tc.ID,
						Content:    errMsg,
					})
					continue
				}

				if action == permission.Ask {
					fmt.Printf("\n⚠️  %s (y/N): ", msg)
					var answer string
					_, _ = fmt.Scanln(&answer)
					answer = strings.TrimSpace(strings.ToLower(answer))
					if answer != "y" && answer != "yes" {
						errMsg := fmt.Sprintf("User denied tool %q", tc.Name)
						a.messages = append(a.messages, llm.Message{
							Role:       "tool",
							ToolCallID: tc.ID,
							Content:    errMsg,
						})
						continue
					}
				}

				if onToolStart != nil {
					onToolStart(tc.Name, tc.Arguments)
				}

				execCtx := tools.ExecContext{
					Context:     ctx,
					ProjectRoot: a.cfg.Project.Root,
					Stdout:      func(s string) {},
					Stderr:      func(s string) {},
					ReadInput:   a.stdinReader(),
					SubAgent:    a,
					Depth:       a.depth,
				}

				result := a.cfg.Tools.Execute(execCtx, tc)

				if onToolEnd != nil {
					onToolEnd(tc.Name, result)
				}

				a.messages = append(a.messages, llm.Message{
					Role:       "tool",
					ToolCallID: tc.ID,
					Content:    result,
				})

				if strings.HasPrefix(result, "Error") {
					toolErrors[tc.Name]++
					if toolErrors[tc.Name] >= 5 {
						return "", fmt.Errorf("tool %q failed %d times consecutively", tc.Name, toolErrors[tc.Name])
					}
				} else {
					toolErrors[tc.Name] = 0
				}

				// Auto-read file when edit fails with "old_string not found"
				if tc.Name == "edit" && strings.Contains(result, "old_string not found in file") {
					if filePath := extractFilePath(result); filePath != "" {
						fileEditFails[filePath]++
						if fileEditFails[filePath] >= 2 {
							return "", fmt.Errorf("edit failed twice on %s — file state changed. Use read to see current content", filePath)
						}
						readResult := a.cfg.Tools.Execute(execCtx, llm.ToolCall{
							Name:      "read",
							Arguments: fmt.Sprintf(`{"file_path":"%s"}`, filePath),
						})
						a.messages = append(a.messages, llm.Message{
							Role:    "tool",
							Content: "❯ auto-read (current state after failed edit):\n" + readResult,
						})
					}
				}
			}
		}

		return "", fmt.Errorf("tool call loop exceeded %d iterations — model may be stuck in a loop", 12)
	}

	return "", fmt.Errorf("reached maximum turns (%d) without completing", maxTurns)
}

func estimateTokens(text string, ratio float64) int {
	if ratio <= 0 {
		ratio = 3.5
	}
	return int(float64(len(text)) / ratio)
}

func (a *Agent) Stats() (turns int, tokenIn int, tokenOut int) {
	return a.turnCount, a.totalInputTokens, a.totalOutputTokens
}

func (a *Agent) SetMessages(msgs []llm.Message) {
	a.messages = msgs
}

func (a *Agent) GetMessages() []llm.Message {
	return a.messages
}

func (a *Agent) RunSubAgent(ctx context.Context, agentType string, prompt string) (string, error) {
	at := AgentType(agentType)
	if at != AgentExplore && at != AgentGeneral {
		return "", fmt.Errorf("unknown sub-agent type %q", agentType)
	}

	subCfg := Config{
		Type:       at,
		LLM:        a.cfg.LLM,
		Tools:      a.cfg.Tools,
		Project:    a.cfg.Project,
		Stdin:      a.cfg.Stdin,
		MaxTurns:   15,
		MaxContext: a.cfg.MaxContext,
	}

	sub := New(subCfg)
	sub.depth = a.depth + 1
	result, err := sub.Run(ctx, prompt, nil, nil, nil)
	if err != nil {
		return "", fmt.Errorf("sub-agent %s error: %w", agentType, err)
	}
	return result, nil
}

func (a *Agent) allowedToolDefs() []llm.ToolDef {
	var all []llm.ToolDef
	if a.cfg.MinimalMode {
		all = a.cfg.Tools.LLMDefsMinimal()
	} else {
		all = a.cfg.Tools.LLMDefs()
	}
	allowed := a.perm.AllowedTools(toolNamesFromDefs(all))
	filtered := make([]llm.ToolDef, 0, len(allowed))
	for _, td := range all {
		for _, name := range allowed {
			if td.Name == name {
				if a.cfg.MinimalMode && !minimalSafeTools[name] {
					continue
				}
				filtered = append(filtered, td)
				break
			}
		}
	}
	return filtered
}

func (a *Agent) allowedToolNames() []string {
	all := a.perm.AllowedTools(toolNamesFromDefs(a.cfg.Tools.LLMDefs()))
	if a.cfg.MinimalMode {
		filtered := make([]string, 0, len(all))
		for _, name := range all {
			if minimalSafeTools[name] {
				filtered = append(filtered, name)
			}
		}
		return filtered
	}
	return all
}

func (a *Agent) LLMDefs() []llm.ToolDef {
	return a.allowedToolDefs()
}

func toolNamesFromDefs(defs []llm.ToolDef) []string {
	names := make([]string, len(defs))
	for i, d := range defs {
		names[i] = d.Name
	}
	return names
}

func isCodeRequest(input string) bool {
	lower := strings.ToLower(input)
	for _, p := range codeRequestPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

func hasHallucinatedCode(text string) bool {
	if text == "" {
		return false
	}
	hasCodeBlock := strings.Contains(text, "```c") || strings.Contains(text, "```go") ||
		strings.Contains(text, "```C") || strings.Contains(text, "```")
	hasToolMarker := strings.Contains(text, "❯ write(") || strings.Contains(text, "❯ edit(")
	if !hasCodeBlock || hasToolMarker {
		return false
	}
	// Only block if code lines predominate (> 50% of non-empty lines)
	lines := strings.Split(text, "\n")
	totalNonEmpty := 0
	codeLines := 0
	inCode := false
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" {
			continue
		}
		totalNonEmpty++
		if strings.HasPrefix(trimmed, "```") {
			inCode = !inCode
			continue
		}
		if inCode {
			codeLines++
		}
	}
	if totalNonEmpty == 0 {
		return false
	}
	return codeLines > 0 && float64(codeLines)/float64(totalNonEmpty) > 0.5 && codeLines >= 3
}

func looksLikeFailedJSON(text string) bool {
	lower := strings.ToLower(text)
	return (strings.Contains(lower, `"name"`) && strings.Contains(lower, `{`)) ||
		(strings.Contains(lower, `'name'`) && strings.Contains(lower, `{`)) ||
		strings.Contains(lower, "<tool_call>") ||
		strings.Contains(lower, "```json")
}

// stdinReader returns a ReadInput function for tools that need user interaction
// (ask, edit confirmation, bash workdir check). Reads from a.cfg.Stdin (set to os.Stdin in main.go).
// extractFilePath parses the file path from an edit tool error like
// "old_string not found in file /abs/path/file.c"
func extractFilePath(errMsg string) string {
	prefix := "old_string not found in file "
	if idx := strings.Index(errMsg, prefix); idx >= 0 {
		return strings.TrimSpace(errMsg[idx+len(prefix):])
	}
	return ""
}

func (a *Agent) stdinReader() func(string) (string, error) {
	return func(question string) (string, error) {
		if a.cfg.Stdin == nil {
			return "", fmt.Errorf("no stdin available")
		}
		fmt.Fprint(os.Stderr, "\n❓ "+question+"\n> ")
		reader := bufio.NewReader(a.cfg.Stdin)
		answer, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("read input: %w", err)
		}
		return strings.TrimSpace(answer), nil
	}
}

func (a *Agent) autoBootstrap(ctx context.Context) {
	root := a.cfg.Project.Root
	if root == "" {
		return
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}

	hasCFiles := false
	hasHFiles := false
	hasGoFiles := false
	hasMakefile := false
	hasFtPrefix := false

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		switch {
		case strings.HasSuffix(e.Name(), ".c"):
			hasCFiles = true
		case strings.HasSuffix(e.Name(), ".h"):
			hasHFiles = true
		case strings.HasSuffix(e.Name(), ".go"):
			hasGoFiles = true
		case e.Name() == "Makefile":
			hasMakefile = true
		}
		if strings.HasPrefix(e.Name(), "ft_") {
			hasFtPrefix = true
		}
	}

	execCtx := tools.ExecContext{
		Context:     ctx,
		ProjectRoot: root,
		Stdout:      func(s string) {},
		Stderr:      func(s string) {},
		ReadInput:   a.stdinReader(),
		SubAgent:    a,
		Depth:       a.depth,
	}

	if hasMakefile {
		result := a.cfg.Tools.Execute(execCtx, llm.ToolCall{Name: "read", Arguments: `{"file_path":"Makefile"}`})
		if !strings.HasPrefix(result, "Error") {
			a.messages = append(a.messages, llm.Message{Role: "tool", Content: "❯ auto-bootstrap: Makefile\n" + result})
		}
	}

	if hasCFiles || hasFtPrefix {
		result := a.cfg.Tools.Execute(execCtx, llm.ToolCall{Name: "glob", Arguments: `{"pattern":"*.c"}`})
		if !strings.HasPrefix(result, "Error") {
			a.messages = append(a.messages, llm.Message{Role: "tool", Content: "❯ auto-bootstrap: C source files\n" + result})
		}
	}
	if hasHFiles || hasFtPrefix {
		result := a.cfg.Tools.Execute(execCtx, llm.ToolCall{Name: "glob", Arguments: `{"pattern":"*.h"}`})
		if !strings.HasPrefix(result, "Error") {
			a.messages = append(a.messages, llm.Message{Role: "tool", Content: "❯ auto-bootstrap: C headers\n" + result})
		}
	}

	if hasGoFiles {
		result := a.cfg.Tools.Execute(execCtx, llm.ToolCall{Name: "glob", Arguments: `{"pattern":"*.go"}`})
		if !strings.HasPrefix(result, "Error") {
			a.messages = append(a.messages, llm.Message{Role: "tool", Content: "❯ auto-bootstrap: Go files\n" + result})
		}
	}
}
