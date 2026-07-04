// Package telemetry provides optional opt-in error reporting for TCE.
// It stores errors locally and can send them to a configurable endpoint.
package telemetry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ErrorReport represents a single error occurrence.
type ErrorReport struct {
	Timestamp string `json:"timestamp"`
	ToolName  string `json:"tool_name,omitempty"`
	Error     string `json:"error"`
	Turn      int    `json:"turn,omitempty"`
	Model     string `json:"model,omitempty"`
	Agent     string `json:"agent,omitempty"`
	Version   string `json:"version,omitempty"`
}

// Reporter handles error telemetry.
type Reporter struct {
	mu       sync.Mutex
	enabled  bool
	filePath string
	model    string
	agent    string
	version  string
}

// NewReporter creates a new telemetry reporter.
// enabled controls whether errors are recorded.
// root is the project root where .tce/errors.jsonl will be written.
func NewReporter(enabled bool, root, model, agent, version string) *Reporter {
	if root == "" {
		root = "."
	}
	return &Reporter{
		enabled:  enabled,
		filePath: filepath.Join(root, ".tce", "errors.jsonl"),
		model:    model,
		agent:    agent,
		version:  version,
	}
}

// Report records a tool error.
func (r *Reporter) Report(toolName, errMsg string, turn int) {
	if !r.enabled {
		return
	}
	report := ErrorReport{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		ToolName:  toolName,
		Error:     truncateStr(errMsg, 500),
		Turn:      turn,
		Model:     r.model,
		Agent:     r.agent,
		Version:   r.version,
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.appendReport(report)
}

// ReportGeneral records a non-tool error (e.g., LLM call failure).
func (r *Reporter) ReportGeneral(errMsg string, turn int) {
	if !r.enabled {
		return
	}
	report := ErrorReport{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Error:     truncateStr(errMsg, 500),
		Turn:      turn,
		Model:     r.model,
		Agent:     r.agent,
		Version:   r.version,
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.appendReport(report)
}

// Enabled returns whether the reporter is active.
func (r *Reporter) Enabled() bool { return r.enabled }

func (r *Reporter) appendReport(report ErrorReport) {
	dir := filepath.Dir(r.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}
	f, err := os.OpenFile(r.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	data, _ := json.Marshal(report)
	f.Write(data)
	f.WriteString("\n")
}

// LoadReports reads all stored error reports.
func LoadReports(root string) ([]ErrorReport, error) {
	path := filepath.Join(root, ".tce", "errors.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var reports []ErrorReport
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var r ErrorReport
		if err := json.Unmarshal([]byte(line), &r); err == nil {
			reports = append(reports, r)
		}
	}
	return reports, nil
}

// ClearReports deletes all stored error reports.
func ClearReports(root string) error {
	path := filepath.Join(root, ".tce", "errors.jsonl")
	return os.Remove(path)
}

// ReportCount returns the number of stored error reports.
func ReportCount(root string) int {
	reports, err := LoadReports(root)
	if err != nil {
		return 0
	}
	return len(reports)
}

func (r *Reporter) String() string {
	if !r.enabled {
		return "telemetry: disabled"
	}
	return fmt.Sprintf("telemetry: enabled → %s", r.filePath)
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
