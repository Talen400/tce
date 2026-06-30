package agent

import (
	"context"
	"fmt"
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
	"ft_", "crie um", "cria um",
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
	MaxTurns        int
	MinimalMode     bool
	MaxContext      int
	ForceSingleCall bool
	KeepTurns       int
	MaxToolContent  int
	TokenRatio      float64
}

type Agent struct {
	cfg            Config
	perm           *permission.Checker
	compactor      *compactor.Compactor
	messages       []llm.Message
	cachedToolDefs []llm.ToolDef
	depth          int
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
	emptyResults := 0
	toolErrors := make(map[string]int)

	for turn < maxTurns {
		turn++

		a.messages = a.compactor.Compact(a.messages)

		var streamedContent string
		resp, err := a.cfg.LLM.ChatStream(ctx, a.messages, a.cachedToolDefs, func(chunk string) {
			streamedContent += chunk
			if onToken != nil {
				onToken(chunk)
			}
		})
		if err != nil {
			return "", fmt.Errorf("LLM call (turn %d): %w", turn, err)
		}

		if len(resp.ToolCalls) == 0 {
			if streamedContent == "" {
				streamedContent = "(no response)"
			}

			if turn == 1 && isCodeRequest(userPrompt) {
				return "", fmt.Errorf("code requests require tool use — implement using write/edit tools, not direct answers")
			}

			if hasHallucinatedCode(streamedContent) {
				return "", fmt.Errorf("response contains code blocks — use write/edit tools instead of direct code output")
			}

			a.messages = append(a.messages, llm.Message{Role: "assistant", Content: streamedContent})
			return streamedContent, nil
		}

		if a.cfg.ForceSingleCall && len(resp.ToolCalls) > 1 {
			resp.ToolCalls = resp.ToolCalls[:1]
		}

		a.messages = append(a.messages, llm.Message{
			Role:      "assistant",
			Content:   streamedContent,
			ToolCalls: resp.ToolCalls,
		})

		hadUsefulResult := false
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
				fmt.Scanln(&answer)
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
				if toolErrors[tc.Name] >= 3 {
					return "", fmt.Errorf("tool %q failed %d times consecutively", tc.Name, toolErrors[tc.Name])
				}
			} else {
				toolErrors[tc.Name] = 0
			}

			if result != "" && !strings.HasPrefix(result, "Error") && !strings.Contains(result, "(no output)") {
				hadUsefulResult = true
			}
		}

		if !hadUsefulResult {
			hasToolErrors := false
			for _, count := range toolErrors {
				if count > 0 {
					hasToolErrors = true
					break
				}
			}
			if hasToolErrors {
				return "", fmt.Errorf("tools are failing — check arguments and available paths")
			}
			emptyResults++
			if emptyResults >= 3 {
				return "Task stalled: repeated tool calls with no useful output. Please rephrase your request or check the available tools.", fmt.Errorf("stalled after %d empty tool calls", emptyResults)
			}
		} else {
			emptyResults = 0
		}
	}

	return "", fmt.Errorf("reached maximum turns (%d) without completing", maxTurns)
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
	return hasCodeBlock && !hasToolMarker
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
