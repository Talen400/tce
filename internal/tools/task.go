package tools

import (
	"encoding/json"
	"fmt"
	"strings"
)

type TaskTool struct{}

func (t *TaskTool) Name() string { return "task" }

func (t *TaskTool) Description() string {
	return `Launch a sub-agent for specialized tasks. Use "explore" (read-only code search) or "general" (full-access tasks). Provide a clear prompt describing what the sub-agent should do.`
}

func (t *TaskTool) ShortDescription() string { return "Launch a sub-agent" }

func (t *TaskTool) Schema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"subagent": map[string]any{
				"type":        "string",
				"enum":        []string{"explore", "general"},
				"description": "The sub-agent type: explore (read-only code search) or general (full-access)",
			},
			"prompt": map[string]any{
				"type":        "string",
				"description": "What the sub-agent should do",
			},
		},
		"required": []string{"subagent", "prompt"},
	}
}

func (t *TaskTool) Execute(ctx ExecContext, input json.RawMessage) (string, error) {
	var raw map[string]any
	if err := json.Unmarshal(input, &raw); err != nil {
		if fixed := tryFixJSON(input); fixed != nil {
			return t.Execute(ctx, fixed)
		}
		return "", fmt.Errorf("invalid JSON: %w", err)
	}

	subAgent := extractSubAgent(raw)
	prompt := extractPrompt(raw)

	if subAgent == "" {
		return "", fmt.Errorf("%s", fmtErr("missing \"subagent\" (explore or general)", `{"subagent": "explore", "prompt": "find all .go files"}`, input))
	}

	if prompt == "" {
		return "", fmt.Errorf("%s", fmtErr("missing \"prompt\" for sub-agent", `{"subagent": "explore", "prompt": "describe task here"}`, input))
	}

	if ctx.Depth >= MaxSubAgentDepth {
		return "", fmt.Errorf("sub-agent depth limit (%d) exceeded. Cannot nest sub-agents deeper than %d levels.", MaxSubAgentDepth, MaxSubAgentDepth)
	}

	if ctx.SubAgent == nil {
		return "No sub-agent runner configured.", nil
	}

	result, err := ctx.SubAgent.RunSubAgent(ctx.Context, subAgent, prompt)
	if err != nil {
		return "", fmt.Errorf("sub-agent %q error: %v", subAgent, err)
	}

	return fmt.Sprintf("❯ task(%s)\n── Result ──\n%s\n── End ──", subAgent, result), nil
}

func extractSubAgent(raw map[string]any) string {
	for _, key := range []string{"subagent", "type", "mode", "agent"} {
		if s := stringField(raw, key); s != "" {
			s = strings.ToLower(strings.TrimSpace(s))
			if s == "explore" || s == "general" {
				return s
			}
		}
	}
	if s := stringField(raw, "name"); s != "" {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "explore" || s == "general" {
			return s
		}
	}
	return ""
}

func extractPrompt(raw map[string]any) string {
	keys := []string{"prompt", "message", "task", "description", "instruction", "objective", "goal"}
	for _, k := range keys {
		if s := stringField(raw, k); s != "" {
			return s
		}
	}
	if params := stringField(raw, "parameters"); params != "" {
		var inner map[string]any
		if json.Unmarshal([]byte(params), &inner) == nil {
			if s := extractPrompt(inner); s != "" {
				return s
			}
		}
		return params
	}
	return ""
}

func stringField(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return firstOf(m, key)
}
