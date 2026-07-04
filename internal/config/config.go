// Package config loads project-level .tce.yaml configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ToolConfig struct {
	Command     string
	Description string
}

type MCPServerConfig struct {
	Command string
	Args    []string
}

type ProjectConfig struct {
	Model   string
	Agent   string
	verbose bool

	Tools      map[string]ToolConfig
	MCPServers map[string]MCPServerConfig
}

func (c *ProjectConfig) Verbose() bool { return c.verbose }

// Load reads .tce.yaml from root and returns a parsed config.
func Load(root string) *ProjectConfig {
	cfg := &ProjectConfig{}
	path := filepath.Join(root, ".tce.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}

	lines := strings.Split(string(data), "\n")
	cfg.parseBlock(lines, 0, cfg.parseTopLevel)

	return cfg
}

// parseBlock processes lines at a given indent level, calling handler for each entry.
func (cfg *ProjectConfig) parseBlock(lines []string, indent int, handler func(key, rawVal string)) {
	var i int
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			i++
			continue
		}

		// Count leading whitespace
		ws := countIndent(line)
		if ws < indent {
			return // end of this block
		}
		if ws > indent {
			i++
			continue // skip continuation lines (shouldn't happen at top level)
		}

		key, val, _ := strings.Cut(trimmed, ":")
		key = strings.TrimSpace(key)
		if key == "" {
			i++
			continue
		}
		val = strings.TrimSpace(val)

		// Check if next lines are indented (block value)
		nextIndented := i+1 < len(lines) && countIndent(lines[i+1]) > ws

		if val == "" && nextIndented {
			// Block value: collect nested lines
			i++
			subLines := collectBlock(lines, &i, ws+2)
			handler(key, subLines)
		} else {
			handler(key, val)
			i++
		}
	}
}

func (cfg *ProjectConfig) parseTopLevel(key, rawVal string) {
	switch key {
	case "model":
		cfg.Model = rawVal
	case "agent":
		cfg.Agent = rawVal
	case "verbose":
		cfg.verbose = rawVal == "true" || rawVal == "yes" || rawVal == "1"
	case "tools":
		cfg.parseToolsBlock(rawVal)
	case "mcp_servers":
		cfg.parseMCPServersBlock(rawVal)
	}
}

func (cfg *ProjectConfig) parseToolsBlock(rawVal string) {
	lines := strings.Split(rawVal, "\n")
	cfg.Tools = make(map[string]ToolConfig, len(lines)/2)
	cfg.parseBlock(lines, 0, func(key, rawVal string) {
		// Tool name key has a block value containing command/description
		subLines := strings.Split(rawVal, "\n")
		tc := ToolConfig{}
		cfg.parseBlock(subLines, 0, func(subKey, subVal string) {
			switch subKey {
			case "command":
				tc.Command = subVal
			case "description":
				tc.Description = subVal
			}
		})
		if tc.Command != "" {
			cfg.Tools[key] = tc
		}
	})
}

func (cfg *ProjectConfig) parseMCPServersBlock(rawVal string) {
	lines := strings.Split(rawVal, "\n")
	cfg.MCPServers = make(map[string]MCPServerConfig, len(lines)/2)
	cfg.parseBlock(lines, 0, func(key, rawVal string) {
		subLines := strings.Split(rawVal, "\n")
		mc := MCPServerConfig{}
		cfg.parseBlock(subLines, 0, func(subKey, subVal string) {
			switch subKey {
			case "command":
				mc.Command = subVal
			case "args":
				mc.Args = parseJSONArray(subVal)
			}
		})
		if mc.Command != "" {
			cfg.MCPServers[key] = mc
		}
	})
}

func countIndent(line string) int {
	n := 0
	for _, ch := range line {
		if ch == ' ' {
			n++
		} else if ch == '\t' {
			n += 2
		} else {
			break
		}
	}
	return n
}

// collectBlock reads lines until indent drops below baseIndent.
func collectBlock(lines []string, idx *int, baseIndent int) string {
	var b strings.Builder
	for *idx < len(lines) {
		line := lines[*idx]
		if strings.TrimSpace(line) == "" {
			*idx++
			continue
		}
		if countIndent(line) < baseIndent {
			break
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(line)
		*idx++
	}
	return b.String()
}

// parseJSONArray parses a JSON-like array of strings: ["a", "b", "c"]
func parseJSONArray(s string) []string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		if s != "" {
			return []string{s}
		}
		return nil
	}
	inner := s[1 : len(s)-1]
	if strings.TrimSpace(inner) == "" {
		return nil
	}
	var items []string
	for {
		inner = strings.TrimSpace(inner)
		if inner == "" {
			break
		}
		var item string
		if inner[0] == '"' {
			end := strings.IndexByte(inner[1:], '"')
			if end < 0 {
				break
			}
			item = inner[1 : 1+end]
			inner = inner[2+end:]
		} else {
			end := strings.IndexByte(inner, ',')
			if end < 0 {
				item = strings.TrimSpace(inner)
				inner = ""
			} else {
				item = strings.TrimSpace(inner[:end])
				inner = inner[end+1:]
			}
		}
		if item != "" {
			items = append(items, item)
		}
		inner = strings.TrimLeft(inner, ",")
	}
	return items
}

// LoadRaw returns the raw file content of .tce.yaml as a string, or empty string if not found.
func LoadRaw(root string) string {
	path := filepath.Join(root, ".tce.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)

}

// FormatYAMLBlock re-indents a block of YAML lines.
func FormatYAMLBlock(block string, indent int) string {
	lines := strings.Split(block, "\n")
	var out []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		out = append(out, fmt.Sprintf("%s%s", strings.Repeat(" ", indent), trimmed))
	}
	return strings.Join(out, "\n")
}
