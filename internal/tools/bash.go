package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var dangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\brm\s+(-rf?|--recursive)\s+(/\s*$|--no-preserve-root)`),
	regexp.MustCompile(`\bdd\s+if=`),
	regexp.MustCompile(`\bmkfs\b`),
	regexp.MustCompile(`\b>+\s+/dev/sd`),
	regexp.MustCompile(`:\(\s*\)\s*\{`),
	regexp.MustCompile(`\bchmod\s+777\s+/`),
	regexp.MustCompile(`\b(wget|curl)\s+.*\|\s*(bash|sh)\b`),
}

type BashTool struct{}

func (t *BashTool) Name() string { return "bash" }
func (t *BashTool) Description() string {
	return "Execute shell commands. Use for running tools, tests, builds, and git operations."
}
func (t *BashTool) ShortDescription() string { return "Run shell commands" }

func (t *BashTool) Schema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The shell command to execute",
			},
			"timeout": map[string]any{
				"type":        "integer",
				"description": "Timeout in seconds (default: 30)",
			},
			"workdir": map[string]any{
				"type":        "string",
				"description": "Working directory relative to project root. Default: project root",
			},
		},
		"required": []string{"command"},
	}
}

type bashInput struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
	Workdir string `json:"workdir,omitempty"`
}

func (t *BashTool) Execute(ctx ExecContext, input json.RawMessage) (string, error) {
	command, timeout, workdir := parseBashInput(input, ctx.ProjectRoot)

	if command == "" {
		return "", fmt.Errorf("%s", fmtErr("missing \"command\"", `{"command": "go test ./..."}`, input))
	}

	for _, pat := range dangerousPatterns {
		if pat.MatchString(command) {
			return fmt.Sprintf("Error: command blocked — matches dangerous pattern: %s", pat), nil
		}
	}

	if ctx.ProjectRoot != "" && !strings.HasPrefix(workdir, ctx.ProjectRoot) {
		msg := fmt.Sprintf("WARNING: workdir %q is outside the project root %q. Running commands outside the project can be dangerous.", workdir, ctx.ProjectRoot)
		if ctx.ReadInput != nil {
			answer, err := ctx.ReadInput(msg + " Continue? (y/N): ")
			if err != nil || (strings.ToLower(answer) != "y" && strings.ToLower(answer) != "yes") {
				return "Command cancelled: workdir is outside project root", nil
			}
		} else {
			return fmt.Sprintf("Error: %s — set workdir inside the project or add a confirmation method", msg), nil
		}
	}

	cmd := exec.CommandContext(ctx.Context, "sh", "-c", command)
	cmd.Dir = workdir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	timer := time.AfterFunc(time.Duration(timeout)*time.Second, func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})
	defer timer.Stop()

	err := cmd.Run()

	output := strings.TrimSpace(stdout.String())
	errOutput := strings.TrimSpace(stderr.String())

	if err != nil {
		if errOutput != "" {
			return "", fmt.Errorf("command failed: %s\n%s", err, errOutput)
		}
		return "", fmt.Errorf("command failed: %s\n%s", err, output)
	}

	maxLen := 4000
	result := output
	if errOutput != "" {
		result = output + "\n" + errOutput
	}
	if len(result) > maxLen {
		result = result[:maxLen] + "\n... (truncated)"
	}

	if result == "" {
		result = "(no output)"
	}

	return fmt.Sprintf("❯ bash(%s)\n── Output ──\n%s\n── End ──", command, result), nil
}

func parseBashInput(input json.RawMessage, projectRoot string) (command string, timeout int, workdir string) {
	timeout = 30
	workdir = projectRoot

	var in bashInput
	if err := json.Unmarshal(input, &in); err == nil && in.Command != "" {
		if in.Timeout > 0 {
			timeout = in.Timeout
		}
		if in.Workdir != "" {
			workdir = filepath.Join(projectRoot, in.Workdir)
		}
		return in.Command, timeout, workdir
	}

	var raw map[string]any
	if err := json.Unmarshal(input, &raw); err != nil {
		if fixed := tryFixJSON(input); fixed != nil {
			return parseBashInput(fixed, projectRoot)
		}
		return "", 0, ""
	}

	command = firstOf(raw, "command", "cmd", "name", "script", "run", "shell")

	if commands, ok := raw["commands"].([]any); ok && len(commands) > 0 {
		if cmdMap, ok := commands[0].(map[string]any); ok {
			if s := firstOf(cmdMap, "command", "cmd"); s != "" {
				command = s
			}
		}
	}

	if args, ok := raw["arguments"]; ok {
		switch v := args.(type) {
		case string:
			var inner map[string]any
			if json.Unmarshal([]byte(v), &inner) == nil {
				if s := firstOf(inner, "command", "cmd"); s != "" {
					command = s
				}
				if commands, ok := inner["commands"].([]any); ok && len(commands) > 0 {
					if cmdMap, ok := commands[0].(map[string]any); ok {
						if s := firstOf(cmdMap, "command", "cmd"); s != "" {
							command = s
						}
					}
				}
			}
		case map[string]any:
			if s := firstOf(v, "command", "cmd"); s != "" {
				command = s
			}
			if cmds, ok := v["commands"].([]any); ok && len(cmds) > 0 {
				if cmdMap, ok := cmds[0].(map[string]any); ok {
					if s := firstOf(cmdMap, "command", "cmd"); s != "" {
						command = s
					}
				}
			}
		}
	}

	if t := intField(raw, "timeout"); t > 0 {
		timeout = t
	} else if t := intField(raw, "time"); t > 0 {
		timeout = t
	}
	if w := strField(raw, "workdir"); w != "" {
		workdir = filepath.Join(projectRoot, w)
	} else if w := firstOf(raw, "dir", "directory", "cwd"); w != "" {
		workdir = filepath.Join(projectRoot, w)
	}

	return
}

func strField(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func intField(m map[string]any, key string) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return 0
}
