package permission

import (
	"fmt"
	"strings"
)

type Action string

const (
	Allow Action = "allow"
	Deny  Action = "deny"
	Ask   Action = "ask"
)

type Rule struct {
	Pattern string `json:"pattern"`
	Action  Action `json:"action"`
	Message string `json:"message,omitempty"`
}

type Checker struct {
	rules []Rule
}

func NewChecker(rules []Rule) *Checker {
	return &Checker{rules: rules}
}

var (
	BuildRules = []Rule{
		{Pattern: "*", Action: Allow},
	}

	PlanRules = []Rule{
		{Pattern: "task", Action: Allow},
		{Pattern: "read", Action: Allow},
		{Pattern: "grep", Action: Allow},
		{Pattern: "glob", Action: Allow},
		{Pattern: "bash", Action: Ask, Message: "Run this command?"},
		{Pattern: "ask", Action: Allow},
		{Pattern: "write", Action: Deny, Message: "Plan agent cannot modify files. Use Build agent instead."},
		{Pattern: "edit", Action: Deny, Message: "Plan agent cannot modify files. Use Build agent instead."},
	}

	ExploreRules = []Rule{
		{Pattern: "read", Action: Allow},
		{Pattern: "grep", Action: Allow},
		{Pattern: "glob", Action: Allow},
		{Pattern: "*", Action: Deny, Message: "Explore agent is read-only."},
	}

	GeneralRules = []Rule{
		{Pattern: "*", Action: Allow},
	}
)

func (c *Checker) Check(toolName string) (Action, string) {
	for _, r := range c.rules {
		if matched(toolName, r.Pattern) {
			return r.Action, r.Message
		}
	}
	return Deny, fmt.Sprintf("Tool %q not allowed by any rule", toolName)
}

func matched(name, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if pattern == name {
		return true
	}
	if strings.HasSuffix(pattern, "*") && strings.HasPrefix(name, strings.TrimSuffix(pattern, "*")) {
		return true
	}
	return false
}

func (c *Checker) AllowedTools(all []string) []string {
	result := make([]string, 0)
	for _, name := range all {
		action, _ := c.Check(name)
		if action != Deny {
			result = append(result, name)
		}
	}
	return result
}
