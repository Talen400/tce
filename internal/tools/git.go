package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/talen/tce/internal/util"
)

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return strings.TrimSpace(stdout.String()), fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimSpace(stdout.String()), nil
}

type CommitTool struct{}

func (t *CommitTool) Name() string { return "commit" }
func (t *CommitTool) Description() string {
	return "Stage all changes and create a git commit. If no message is provided, returns the diff so you can write a commit message."
}
func (t *CommitTool) ShortDescription() string { return "Git commit with message" }

func (t *CommitTool) Schema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"message": map[string]any{
				"type":        "string",
				"description": "Commit message. If empty, returns the diff for you to write one.",
			},
		},
	}
}

func (t *CommitTool) Execute(ctx ExecContext, input json.RawMessage) (string, error) {
	var raw map[string]any
	if err := json.Unmarshal(input, &raw); err != nil {
		if fixed := tryFixJSON(input); fixed != nil {
			return t.Execute(ctx, fixed)
		}
		return "", fmt.Errorf("invalid JSON: %w", err)
	}

	message := firstOf(raw, "message", "msg", "text", "desc", "description")
	root := ctx.ProjectRoot
	if root == "" {
		root = "."
	}

	// Check if we're in a git repo
	if _, err := runGit(root, "rev-parse", "--git-dir"); err != nil {
		return "Not a git repository. Cannot commit.", nil
	}

	if message != "" {
		if _, err := runGit(root, "add", "-A"); err != nil {
			return "", fmt.Errorf("git add failed: %w", err)
		}
		if _, err := runGit(root, "commit", "-m", message); err != nil {
			return "", fmt.Errorf("commit failed: %w", err)
		}
		sha, _ := runGit(root, "rev-parse", "--short", "HEAD")
		return fmt.Sprintf("❯ commit(%s)\n── Committed ──\n%s %s", util.Truncate(message, 60), sha, message), nil
	}

	diff, err := runGit(root, "diff", "--stat")
	if err != nil {
		return "No changes to commit.", nil
	}
	if diff == "" {
		diff, _ = runGit(root, "diff", "--cached", "--stat")
	}
	if diff == "" {
		return "No changes to commit.", nil
	}

	patch, _ := runGit(root, "diff")
	if patch == "" {
		patch, _ = runGit(root, "diff", "--cached")
	}

	return fmt.Sprintf("❯ commit()\n── Uncommitted changes ──\n%s\n\n── Diff preview ──\n%s\n── End ──\nCall commit again with a descriptive message.", diff, util.Truncate(patch, 2000)), nil
}

type ReviewTool struct{}

func (t *ReviewTool) Name() string { return "review" }
func (t *ReviewTool) Description() string {
	return "Show all uncommitted changes (git diff) for review before committing."
}
func (t *ReviewTool) ShortDescription() string { return "Review uncommitted changes" }

func (t *ReviewTool) Schema() any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (t *ReviewTool) Execute(ctx ExecContext, input json.RawMessage) (string, error) {
	root := ctx.ProjectRoot
	if root == "" {
		root = "."
	}

	if _, err := runGit(root, "rev-parse", "--git-dir"); err != nil {
		return "Not a git repository.", nil
	}

	staged, _ := runGit(root, "diff", "--cached", "--stat")
	unstaged, _ := runGit(root, "diff", "--stat")
	stagedPatch, _ := runGit(root, "diff", "--cached")
	unstagedPatch, _ := runGit(root, "diff")

	var b strings.Builder
	b.WriteString("❯ review()\n")

	if staged != "" {
		b.WriteString(fmt.Sprintf("── Staged (%s) ──\n%s\n", staged, util.Truncate(stagedPatch, 2000)))
	} else {
		b.WriteString("── Staged: (none) ──\n")
	}

	if unstaged != "" {
		b.WriteString(fmt.Sprintf("── Unstaged (%s) ──\n%s\n", unstaged, util.Truncate(unstagedPatch, 2000)))
	} else {
		b.WriteString("── Unstaged: (none) ──\n")
	}

	b.WriteString("── End ──")
	return b.String(), nil
}
