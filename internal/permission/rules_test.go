package permission

import (
	"testing"
)

func TestBuildRules(t *testing.T) {
	c := NewChecker(BuildRules)
	allowed := c.AllowedTools([]string{"bash", "read", "write", "edit", "grep", "glob", "ask", "task"})
	if len(allowed) != 8 {
		t.Errorf("build should allow all 8 tools, got %d: %v", len(allowed), allowed)
	}
}

func TestBuildAllowsEverything(t *testing.T) {
	c := NewChecker(BuildRules)
	action, _ := c.Check("bash")
	if action != Allow {
		t.Errorf("build should allow bash, got %v", action)
	}
	action, _ = c.Check("unknown_tool")
	if action != Allow {
		t.Errorf("build wildcard should allow unknown tool, got %v", action)
	}
}

func TestPlanRules(t *testing.T) {
	c := NewChecker(PlanRules)

	tests := []struct {
		tool   string
		action Action
	}{
		{"task", Allow},
		{"read", Allow},
		{"grep", Allow},
		{"glob", Allow},
		{"ask", Allow},
		{"bash", Ask},
		{"write", Deny},
		{"edit", Deny},
		{"unknown", Deny},
	}

	for _, tt := range tests {
		action, _ := c.Check(tt.tool)
		if action != tt.action {
			t.Errorf("plan %s: expected %v, got %v", tt.tool, tt.action, action)
		}
	}
}

func TestExploreRules(t *testing.T) {
	c := NewChecker(ExploreRules)

	tests := []struct {
		tool   string
		action Action
	}{
		{"read", Allow},
		{"grep", Allow},
		{"glob", Allow},
		{"bash", Deny},
		{"write", Deny},
		{"edit", Deny},
		{"task", Deny},
		{"ask", Deny},
		{"unknown", Deny},
	}

	for _, tt := range tests {
		action, msg := c.Check(tt.tool)
		if action != tt.action {
			t.Errorf("explore %s: expected %v, got %v (msg: %s)", tt.tool, tt.action, action, msg)
		}
		if tt.action == Deny && msg == "" {
			t.Errorf("explore %s: deny should include a message", tt.tool)
		}
	}
}

func TestGeneralRules(t *testing.T) {
	c := NewChecker(GeneralRules)
	action, _ := c.Check("bash")
	if action != Allow {
		t.Errorf("general should allow bash, got %v", action)
	}
	action, _ = c.Check("unknown")
	if action != Allow {
		t.Errorf("general wildcard should allow unknown, got %v", action)
	}
}

func TestFirstMatchWins(t *testing.T) {
	rules := []Rule{
		{Pattern: "write*", Action: Deny, Message: "writing denied"},
		{Pattern: "*", Action: Allow},
	}
	c := NewChecker(rules)

	action, msg := c.Check("write")
	if action != Deny || msg != "writing denied" {
		t.Errorf("write should match first rule (deny), got %v %s", action, msg)
	}

	action, _ = c.Check("bash")
	if action != Allow {
		t.Errorf("bash should match second rule (allow), got %v", action)
	}
}

func TestNoMatchDefaultsDeny(t *testing.T) {
	rules := []Rule{
		{Pattern: "read", Action: Allow},
	}
	c := NewChecker(rules)

	action, msg := c.Check("bash")
	if action != Deny || msg == "" {
		t.Errorf("unmatched tool should deny with message, got %v %s", action, msg)
	}
}

func TestEmptyRules(t *testing.T) {
	c := NewChecker([]Rule{})
	action, msg := c.Check("anything")
	if action != Deny || msg == "" {
		t.Errorf("empty rules should deny everything, got %v %s", action, msg)
	}
}

func TestAllowedToolsFiltering(t *testing.T) {
	c := NewChecker(PlanRules)
	all := []string{"bash", "read", "write", "edit", "grep", "glob", "ask", "task"}
	allowed := c.AllowedTools(all)

	expected := map[string]bool{"read": true, "grep": true, "glob": true, "ask": true, "task": true, "bash": true}
	for _, name := range allowed {
		if !expected[name] {
			t.Errorf("unexpected allowed tool: %s", name)
		}
	}
	if len(allowed) != 6 {
		t.Errorf("expected 6 allowed tools (bash=ask is included), got %d: %v", len(allowed), allowed)
	}
}

func TestWildcardPattern(t *testing.T) {
	if !matched("write_anything", "write*") {
		t.Error("write* should match write_anything")
	}
	if matched("read", "write*") {
		t.Error("write* should not match read")
	}
	if !matched("anything", "*") {
		t.Error("* should match anything")
	}
	if !matched("bash", "bash") {
		t.Error("exact match should work")
	}
}
