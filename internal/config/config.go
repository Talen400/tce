// Package config loads project-level .tce.yaml configuration.
package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type ProjectConfig struct {
	Model   string
	Agent   string
	verbose bool
}

func (c *ProjectConfig) Verbose() bool { return c.verbose }

func Load(root string) *ProjectConfig {
	cfg := &ProjectConfig{}
	path := filepath.Join(root, ".tce.yaml")
	f, err := os.Open(path)
	if err != nil {
		return cfg
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if val == "" {
			continue
		}
		switch key {
		case "model":
			cfg.Model = val
		case "agent":
			cfg.Agent = val
		case "verbose":
			cfg.verbose = val == "true" || val == "yes" || val == "1"
		}
	}
	return cfg
}
