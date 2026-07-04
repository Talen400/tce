package tools

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type ReadTool struct{}

func (t *ReadTool) Name() string { return "read" }
func (t *ReadTool) Description() string {
	return "Read the contents of a file. Include line numbers in the output."
}
func (t *ReadTool) ShortDescription() string { return "Read file contents" }

func (t *ReadTool) Schema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "Path to the file to read (absolute or relative to project root)",
			},
			"offset": map[string]any{
				"type":        "integer",
				"description": "Starting line number (1-indexed). Default: 1",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of lines to read. Default: all",
			},
		},
		"required": []string{"file_path"},
	}
}

type readInput struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

func (t *ReadTool) Execute(ctx ExecContext, input json.RawMessage) (string, error) {
	var in readInput
	if err := json.Unmarshal(input, &in); err != nil || in.FilePath == "" {
		var raw map[string]any
		if err := json.Unmarshal(input, &raw); err != nil {
			if fixed := tryFixJSON(input); fixed != nil {
				return t.Execute(ctx, fixed)
			}
			return "", fmt.Errorf("invalid input: %w", err)
		}
		in.FilePath = firstOf(raw, "file_path", "path", "file", "filename")
		if v, ok := raw["offset"]; ok {
			if f, ok := v.(float64); ok {
				in.Offset = int(f)
			}
		}
		if v, ok := raw["limit"]; ok {
			if f, ok := v.(float64); ok {
				in.Limit = int(f)
			}
		}
	}

	if in.FilePath == "" {
		return "", fmt.Errorf("%s", fmtErr("missing \"file_path\"", `{"file_path": "filename.txt"}`, input))
	}

	path := in.FilePath
	if !filepath.IsAbs(path) {
		path = filepath.Join(ctx.ProjectRoot, path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	start := 1
	if in.Offset > 0 {
		start = in.Offset
	}
	end := len(lines)
	if in.Limit > 0 && start+in.Limit-1 < end {
		end = start + in.Limit - 1
	}

	if start > len(lines) {
		return "", fmt.Errorf("offset %d exceeds file length %d", start, len(lines))
	}
	if start < 1 {
		start = 1
	}
	if end > len(lines) {
		end = len(lines)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("❯ read(%s)\n── Content (lines %d-%d) ──\n", in.FilePath, start, end))
	for i, line := range lines[start-1 : end] {
		b.WriteString(fmt.Sprintf("%d: %s\n", start+i, line))
	}
	b.WriteString("── End ──")

	return b.String(), nil
}

type GrepTool struct{}

func (t *GrepTool) Name() string { return "grep" }
func (t *GrepTool) Description() string {
	return "Search file contents using a regex pattern. Returns matching file paths, line numbers, and content."
}
func (t *GrepTool) ShortDescription() string { return "Search files with regex" }

func (t *GrepTool) Schema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "The regex pattern to search for",
			},
			"include": map[string]any{
				"type":        "string",
				"description": "File glob pattern to filter (e.g. '*.go', '*.{ts,tsx}')",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Subdirectory to search in (relative to project root)",
			},
		},
		"required": []string{"pattern"},
	}
}

type grepInput struct {
	Pattern string `json:"pattern"`
	Include string `json:"include,omitempty"`
	Path    string `json:"path,omitempty"`
}

func (t *GrepTool) Execute(ctx ExecContext, input json.RawMessage) (string, error) {
	var in grepInput
	if err := json.Unmarshal(input, &in); err != nil || in.Pattern == "" {
		var raw map[string]any
		if err := json.Unmarshal(input, &raw); err != nil {
			if fixed := tryFixJSON(input); fixed != nil {
				return t.Execute(ctx, fixed)
			}
			return "", fmt.Errorf("invalid input: %w", err)
		}
		in.Pattern = firstOf(raw, "pattern", "search", "query", "find", "text", "regex")
		in.Include = firstOf(raw, "include", "filter", "glob", "ext")
		in.Path = firstOf(raw, "path", "dir", "directory", "folder")
	}

	if in.Pattern == "" {
		return "", fmt.Errorf("%s", fmtErr("missing \"pattern\"", `{"pattern": "search_term"}`, input))
	}

	return t.executeWith(ctx, in.Pattern, in.Include, in.Path)
}

func (t *GrepTool) executeWith(ctx ExecContext, pattern, include, path string) (string, error) {
	searchPath := ctx.ProjectRoot
	if path != "" {
		searchPath = filepath.Join(ctx.ProjectRoot, path)
	}

	_, err := os.Stat(searchPath)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("path does not exist: %s", path)
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex pattern %q: %w", pattern, err)
	}

	ignoreMatcher, _ := LoadIgnoreFile(filepath.Join(ctx.ProjectRoot, ".tceignore"))

	var b strings.Builder
	b.WriteString(fmt.Sprintf("❯ grep(%q)\n── Matches ──\n", pattern))

	err = filepath.WalkDir(searchPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		relPath, _ := filepath.Rel(ctx.ProjectRoot, path)
		if ignoreMatcher != nil && ignoreMatcher.Match(relPath, d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			base := d.Name()
			if base == ".git" || base == "node_modules" || base == "target" || base == ".venv" || base == "vendor" || base == "__pycache__" || base == ".next" || base == "dist" {
				return filepath.SkipDir
			}
			return nil
		}

		if include != "" {
			match, _ := filepath.Match(include, d.Name())
			if !match {
				return nil
			}
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		lines := strings.Split(string(content), "\n")
		for i, line := range lines {
			if re.MatchString(line) {
				rel, _ := filepath.Rel(ctx.ProjectRoot, path)
				b.WriteString(fmt.Sprintf("%s:%d: %s\n", rel, i+1, strings.TrimSpace(line)))
			}
		}
		return nil
	})

	if err != nil {
		return b.String(), fmt.Errorf("walk error: %w", err)
	}

	if b.Len() == 0 {
		b.WriteString("No matches found.")
	} else {
		b.WriteString("── End ──")
	}

	return b.String(), nil
}

type GlobTool struct{}

func (t *GlobTool) Name() string { return "glob" }
func (t *GlobTool) Description() string {
	return "Find files matching a glob pattern. Example: '**/*.go', 'src/**/*.ts'"
}
func (t *GlobTool) ShortDescription() string { return "Find files by pattern" }

func (t *GlobTool) Schema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "The glob pattern to match files against",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Directory to search in (relative to project root). Default: root",
			},
		},
		"required": []string{"pattern"},
	}
}

type globInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
}

func (t *GlobTool) Execute(ctx ExecContext, input json.RawMessage) (string, error) {
	var in globInput
	if err := json.Unmarshal(input, &in); err != nil || in.Pattern == "" {
		var raw map[string]any
		if err := json.Unmarshal(input, &raw); err != nil {
			if fixed := tryFixJSON(input); fixed != nil {
				return t.Execute(ctx, fixed)
			}
			return "", fmt.Errorf("invalid input: %w", err)
		}
		in.Pattern = firstOf(raw, "pattern", "glob", "search", "query")
		in.Path = firstOf(raw, "path", "dir", "directory")
	}

	if in.Pattern == "" {
		return "", fmt.Errorf("%s", fmtErr("missing \"pattern\"", `{"pattern": "*.go"}`, input))
	}

	searchPath := ctx.ProjectRoot
	if in.Path != "" {
		searchPath = filepath.Join(ctx.ProjectRoot, in.Path)
	}

	ignoreMatcher, _ := LoadIgnoreFile(filepath.Join(ctx.ProjectRoot, ".tceignore"))

	var matches []string

	err := filepath.WalkDir(searchPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		relPath, _ := filepath.Rel(ctx.ProjectRoot, path)
		if ignoreMatcher != nil && ignoreMatcher.Match(relPath, d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			base := d.Name()
			if base == ".git" || base == "node_modules" || base == "target" || base == ".venv" || base == "vendor" || base == "__pycache__" || base == ".next" || base == "dist" {
				return filepath.SkipDir
			}
			return nil
		}

		rel, _ := filepath.Rel(searchPath, path)
		match, _ := filepath.Match(in.Pattern, d.Name())
		if !match {
			match, _ = filepath.Match(in.Pattern, rel)
		}
		if match {
			fullRel, _ := filepath.Rel(ctx.ProjectRoot, path)
			matches = append(matches, fullRel)
		}
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("walk error: %w", err)
	}

	if len(matches) == 0 {
		return fmt.Sprintf("❯ glob(%q)\n── No files found ──", in.Pattern), nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("❯ glob(%q)\n── Files (%d) ──\n", in.Pattern, len(matches)))
	for _, m := range matches {
		b.WriteString(m + "\n")
	}
	b.WriteString("── End ──")

	return b.String(), nil
}
