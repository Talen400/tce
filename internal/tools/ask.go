package tools

import (
	"encoding/json"
	"fmt"
)

type AskTool struct{}

func (t *AskTool) Name() string        { return "ask" }
func (t *AskTool) Description() string { return "Ask the user a question when you need clarification, confirmation, or additional information." }
func (t *AskTool) ShortDescription() string { return "Ask user a question" }

func (t *AskTool) Schema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"question": map[string]any{
				"type":        "string",
				"description": "The question to ask the user",
			},
		},
		"required": []string{"question"},
	}
}

type askInput struct {
	Question string `json:"question"`
}

func (t *AskTool) Execute(ctx ExecContext, input json.RawMessage) (string, error) {
	var in askInput
	if err := json.Unmarshal(input, &in); err != nil || in.Question == "" {
		return "", fmt.Errorf("%s", fmtErr("missing \"question\"", `{"question": "What should I do?"}`, input))
	}

	if ctx.ReadInput == nil {
		return "No way to read user input. Cannot ask question.", nil
	}

	answer, err := ctx.ReadInput(in.Question)
	if err != nil {
		return "", fmt.Errorf("user input error: %w", err)
	}

	return fmt.Sprintf("❯ ask(%s)\n── Answer ──\n%s\n── End ──", in.Question, answer), nil
}
