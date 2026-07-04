package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

type ExternalTool struct {
	NameVal        string
	DescVal        string
	ShortDescVal   string
	Command        string
}

func (t *ExternalTool) Name() string             { return t.NameVal }
func (t *ExternalTool) Description() string      { return t.DescVal }
func (t *ExternalTool) ShortDescription() string { return t.ShortDescVal }

var paramRe = regexp.MustCompile(`\{\{(\w+)\}\}`)

func (t *ExternalTool) Schema() any {
	params := paramRe.FindAllStringSubmatch(t.Command, -1)
	props := map[string]any{}
	required := []string{}
	seen := map[string]bool{}
	for _, m := range params {
		name := m[1]
		if seen[name] {
			continue
		}
		seen[name] = true
		props[name] = map[string]any{
			"type":        "string",
			"description": fmt.Sprintf("Value for {{%s}}", name),
		}
		required = append(required, name)
	}
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"params": map[string]any{
				"type":        "object",
				"description": "Parameters for the command template",
				"properties":  props,
				"required":    required,
			},
		},
	}
	return schema
}

func (t *ExternalTool) Execute(ctx ExecContext, input json.RawMessage) (string, error) {
	var raw map[string]any
	if err := json.Unmarshal(input, &raw); err != nil {
		if fixed := tryFixJSON(input); fixed != nil {
			return t.Execute(ctx, fixed)
		}
		return "", fmt.Errorf("invalid JSON: %w", err)
	}

	params := map[string]string{}

	// Try reading from "params" sub-object first
	if p, ok := raw["params"]; ok {
		if pm, ok := p.(map[string]any); ok {
			for k, v := range pm {
				params[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	// Also try reading from "args" or "arguments" sub-object
	for _, key := range []string{"args", "arguments"} {
		if p, ok := raw[key]; ok {
			if pm, ok := p.(map[string]any); ok {
				for k, v := range pm {
					params[k] = fmt.Sprintf("%v", v)
				}
			}
		}
	}

	// Fallback: if params/args/arguments wasn't found, treat all top-level keys as params
	if len(params) == 0 {
		for k, v := range raw {
			params[k] = fmt.Sprintf("%v", v)
		}
	}

	cmd := t.Command
	for k, v := range params {
		cmd = strings.ReplaceAll(cmd, "{{"+k+"}}", v)
	}

	root := ctx.ProjectRoot
	if root == "" {
		root = "."
	}

	shCmd := exec.Command("sh", "-c", cmd)
	shCmd.Dir = root
	var stdout, stderr bytes.Buffer
	shCmd.Stdout = &stdout
	shCmd.Stderr = &stderr
	err := shCmd.Run()
	out := strings.TrimSpace(stdout.String())
	errOut := strings.TrimSpace(stderr.String())
	if err != nil {
		if errOut != "" {
			return out, fmt.Errorf("%s (stderr: %s)", err.Error(), errOut)
		}
		return out, err
	}
	if errOut != "" {
		out += "\nstderr:\n" + errOut
	}
	return out, nil
}
