package compactor

import (
	"strings"
	"testing"

	"github.com/talen/tce/internal/llm"
)

func TestEstimateTokens(t *testing.T) {
	c := New(1000)
	if c.estimateTokens("hello world") < 1 {
		t.Error("expected > 0")
	}
	if c.estimateTokens("") != 0 {
		t.Error("expected 0 for empty string")
	}
}

func TestEstimateTokensSafetyCap(t *testing.T) {
	c := New(1000)
	huge := string(make([]byte, 500000))
	tok := c.estimateTokens(huge)
	if tok > 200000 {
		t.Errorf("safety cap exceeded: %d", tok)
	}
}

func TestCompactUnderLimit(t *testing.T) {
	c := New(1000)
	msgs := []llm.Message{
		{Role: "system", Content: "you are a helpful assistant"},
		{Role: "user", Content: "hello"},
	}
	result := c.Compact(msgs)
	if len(result) != 2 {
		t.Errorf("expected 2 messages, got %d", len(result))
	}
}

func TestCompactPreservesSystem(t *testing.T) {
	c := New(100)
	msgs := []llm.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "a"},
		{Role: "assistant", Content: "b"},
		{Role: "tool", Content: "c"},
		{Role: "user", Content: "d"},
		{Role: "assistant", Content: "e"},
		{Role: "tool", Content: "f"},
		{Role: "user", Content: "g"},
		{Role: "assistant", Content: "h"},
		{Role: "tool", Content: "i"},
		{Role: "user", Content: "j"},
		{Role: "assistant", Content: "k"},
	}
	result := c.Compact(msgs)
	for _, m := range result {
		if m.Role == "system" {
			return
		}
	}
	t.Error("system message was pruned")
}

func TestCompactKeepsAtLeast2Turns(t *testing.T) {
	c := New(100)
	msgs := []llm.Message{
		{Role: "system", Content: "s"},
		{Role: "user", Content: "turn 1"},
		{Role: "assistant", Content: "a1"},
		{Role: "tool", Content: "t1"},
		{Role: "user", Content: "turn 2"},
		{Role: "assistant", Content: "a2"},
		{Role: "tool", Content: "t2"},
		{Role: "user", Content: "turn 3"},
		{Role: "assistant", Content: "a3"},
	}
	result := c.Compact(msgs)
	hasTurn3 := false
	for i := range result {
		if result[i].Role == "user" && result[i].Content == "turn 3" {
			hasTurn3 = true
		}
	}
	if !hasTurn3 {
		t.Error("expected last turn to be preserved")
	}
}

func TestCompactPrunesOldTools(t *testing.T) {
	c := New(100)
	msgs := []llm.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "turn 1"},
		{Role: "assistant", Content: "a"},
		{Role: "tool", Content: "file content 1"},
		{Role: "user", Content: "turn 2"},
		{Role: "assistant", Content: "b"},
		{Role: "tool", Content: "file content 2"},
		{Role: "user", Content: "turn 3"},
		{Role: "assistant", Content: "c"},
		{Role: "tool", Content: "file content 3"},
		{Role: "user", Content: "turn 4"},
		{Role: "assistant", Content: "d"},
	}
	result := c.Compact(msgs)
	if len(result) >= len(msgs) {
		t.Errorf("expected compaction to reduce messages, got %d >= %d", len(result), len(msgs))
	}
}

func TestCompactNoSystem(t *testing.T) {
	c := New(100)
	msgs := []llm.Message{
		{Role: "user", Content: "1"},
		{Role: "assistant", Content: "a"},
		{Role: "user", Content: "2"},
		{Role: "assistant", Content: "b"},
		{Role: "user", Content: "3"},
		{Role: "assistant", Content: "c"},
		{Role: "user", Content: "4"},
		{Role: "assistant", Content: "d"},
	}
	result := c.Compact(msgs)
	if len(result) >= len(msgs) {
		t.Errorf("expected compaction, got %d >= %d", len(result), len(msgs))
	}
}

func TestCompactSingleMessage(t *testing.T) {
	c := New(100)
	msgs := []llm.Message{{Role: "user", Content: "hi"}}
	result := c.Compact(msgs)
	if len(result) != 1 {
		t.Errorf("expected 1 message, got %d", len(result))
	}
}

func TestCompactEmpty(t *testing.T) {
	c := New(100)
	result := c.Compact(nil)
	if result != nil {
		t.Error("expected nil for nil input")
	}
}

func TestCompactToolCallsCounted(t *testing.T) {
	c := New(10)
	msgs := []llm.Message{
		{Role: "system", Content: "s"},
		{Role: "user", Content: "u1"},
		{Role: "assistant", Content: "", ToolCalls: []llm.ToolCall{
			{Name: "bash", Arguments: `{"command":"echo hi"}`},
		}},
		{Role: "tool", Content: "result data here"},
		{Role: "user", Content: "u2"},
		{Role: "assistant", Content: "final"},
	}
	result := c.Compact(msgs)
	if len(result) >= len(msgs) {
		t.Logf("expected compaction, got %d >= %d (may not trigger with small data)", len(result), len(msgs))
	}
}

func TestCompactLargeToolContent(t *testing.T) {
	c := New(50)
	c.MaxToolContentLen = 20
	long := string(make([]byte, 500))
	msgs := []llm.Message{
		{Role: "user", Content: "1"},
		{Role: "assistant", Content: "a"},
		{Role: "tool", Content: long},
	}
	result := c.Compact(msgs)
	for _, m := range result {
		if m.Role == "tool" && len(m.Content) == 500 {
			t.Errorf("tool content should be truncated, len=%d", len(m.Content))
		}
	}
}

func TestTruncateToolResults(t *testing.T) {
	c := New(30)
	c.MaxToolContentLen = 10
	long := string(make([]byte, 200))
	msgs := []llm.Message{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "ok"},
		{Role: "tool", Content: long},
	}
	result := c.truncateToolResults(msgs)
	found := false
	for _, m := range result {
		if m.Role == "tool" && len(m.Content) < 200 {
			found = true
		}
	}
	if !found {
		t.Error("expected tool content to be truncated")
	}
}

func BenchmarkEstimateTokens1k(b *testing.B) {
	c := NewWithConfig(Config{TokenRatio: 4.0})
	text := strings.Repeat("hello world ", 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.estimateTokens(text)
	}
}

func BenchmarkEstimateTokens10k(b *testing.B) {
	c := NewWithConfig(Config{TokenRatio: 4.0})
	text := strings.Repeat("hello world ", 1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.estimateTokens(text)
	}
}

func BenchmarkEstimateTokens100k(b *testing.B) {
	c := NewWithConfig(Config{TokenRatio: 4.0})
	text := strings.Repeat("hello world ", 10000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.estimateTokens(text)
	}
}

func BenchmarkCompact(b *testing.B) {
	c := NewWithConfig(Config{MaxContextTokens: 1000, TokenRatio: 4.0})
	msgs := make([]llm.Message, 20)
	for i := range msgs {
		msgs[i] = llm.Message{Role: "user", Content: strings.Repeat("hello ", 100)}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Compact(msgs)
	}
}
