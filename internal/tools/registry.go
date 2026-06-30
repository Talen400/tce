package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/talen/tce/internal/llm"
)

type SubAgentRunner interface {
	RunSubAgent(ctx context.Context, agentType string, prompt string) (string, error)
}

type ExecContext struct {
	Context     context.Context
	ProjectRoot string
	Stdout      func(string)
	Stderr      func(string)
	ReadInput   func(string) (string, error)
	SubAgent    SubAgentRunner
	Depth       int
}

const MaxSubAgentDepth = 3

type Tool interface {
	Name() string
	Description() string
	ShortDescription() string
	Schema() any
	Execute(ctx ExecContext, input json.RawMessage) (string, error)
}

type Registry struct {
	mu           sync.RWMutex
	tools        map[string]Tool
	toolErrors   map[string]int
	maxToolErrors int

	disabledTools map[string]bool
}

func NewRegistry() *Registry {
	return &Registry{
		tools:         make(map[string]Tool),
		toolErrors:    make(map[string]int),
		maxToolErrors: 3,
		disabledTools: make(map[string]bool),
	}
}

func (r *Registry) SetMaxToolErrors(n int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.maxToolErrors = n
}

func (r *Registry) ResetToolErrors(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.toolErrors, name)
}

func (r *Registry) DisabledTools() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]string, 0, len(r.disabledTools))
	for name := range r.disabledTools {
		list = append(list, name)
	}
	return list
}

func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name()] = t
}

func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		list = append(list, t)
	}
	return list
}

func (r *Registry) LLMDefs() []llm.ToolDef {
	return r.llmDefs(false)
}

func (r *Registry) LLMDefsMinimal() []llm.ToolDef {
	return r.llmDefs(true)
}

func (r *Registry) llmDefs(minimal bool) []llm.ToolDef {
	defs := make([]llm.ToolDef, 0)
	for _, t := range r.List() {
		desc := t.Description()
		if minimal {
			if d := t.ShortDescription(); d != "" {
				desc = d
			}
		}
		defs = append(defs, llm.ToolDef{
			Name:        t.Name(),
			Description: desc,
			Parameters:  t.Schema(),
		})
	}
	return defs
}

func (r *Registry) Execute(ctx ExecContext, call llm.ToolCall) string {
	r.mu.RLock()
	if r.disabledTools[call.Name] {
		r.mu.RUnlock()
		return fmt.Sprintf("Error: tool %q is disabled after too many errors", call.Name)
	}
	r.mu.RUnlock()

	tool, ok := r.Get(call.Name)
	if !ok {
		if matched := fuzzyMatch(call.Name, r.names()); matched != "" {
			call.Name = matched
			tool, _ = r.Get(matched)
		} else {
			return fmt.Sprintf("Error: unknown tool %q\nAvailable: %s", call.Name, strings.Join(r.names(), ", "))
		}
	}

	input := json.RawMessage(call.Arguments)
	if !json.Valid(input) {
		if fixed := tryFixJSON(input); fixed != nil {
			input = fixed
		} else {
			return fmt.Sprintf("Error: invalid JSON for %s\nExpected: {\"field\":\"value\"}\nYou sent: %s", call.Name, call.Arguments)
		}
	}

	result, err := tool.Execute(ctx, input)
	if err != nil {
		r.mu.Lock()
		r.toolErrors[call.Name]++
		if r.toolErrors[call.Name] >= r.maxToolErrors {
			r.disabledTools[call.Name] = true
			r.mu.Unlock()
			return fmt.Sprintf("Error: tool %q has been disabled after %d consecutive errors. Use available tools instead.\nLast error: %v", call.Name, r.maxToolErrors, err)
		}
		r.mu.Unlock()
		return fmt.Sprintf("Error: %v", err)
	}

	r.mu.Lock()
	delete(r.toolErrors, call.Name)
	r.mu.Unlock()
	return result
}

func (r *Registry) names() []string {
	names := make([]string, 0, len(r.tools))
	for _, t := range r.tools {
		names = append(names, t.Name())
	}
	return names
}
