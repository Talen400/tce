// Package session provides session persistence: save and resume message history.
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/talen/tce/internal/llm"
)

type sessionData struct {
	Version  int           `json:"version"`
	Created  string        `json:"created"`
	Model    string        `json:"model"`
	Turns    int           `json:"turns"`
	TokenIn  int           `json:"token_in"`
	TokenOut int           `json:"token_out"`
	Messages []llm.Message `json:"messages,omitempty"`
}

func sessionsDir(root string) string {
	return filepath.Join(root, ".tce", "sessions")
}

// Save persists session metadata and message history to .tce/sessions/.
func Save(root, model string, turns, tokenIn, tokenOut int, messages []llm.Message) {
	dir := sessionsDir(root)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}

	now := time.Now().Format("2006-01-02T15-04-05")
	name := fmt.Sprintf("session-%s.json", now)
	path := filepath.Join(dir, name)

	s := sessionData{
		Version:  1,
		Created:  now,
		Model:    model,
		Turns:    turns,
		TokenIn:  tokenIn,
		TokenOut: tokenOut,
		Messages: messages,
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return
	}

	_ = os.WriteFile(path, data, 0644)
}

// Load reads a session file and returns its data.
func Load(path string) (model string, turns, tokenIn, tokenOut int, messages []llm.Message, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0, 0, 0, nil, err
	}

	var s sessionData
	if err := json.Unmarshal(data, &s); err != nil {
		return "", 0, 0, 0, nil, err
	}

	return s.Model, s.Turns, s.TokenIn, s.TokenOut, s.Messages, nil
}

// List returns paths to all session files in the project's .tce/sessions/ directory.
func List(root string) ([]string, error) {
	dir := sessionsDir(root)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var paths []string
	for _, e := range entries {
		if !e.IsDir() {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	return paths, nil
}
