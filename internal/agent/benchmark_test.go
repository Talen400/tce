package agent

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/talen/tce/internal/llm"
	"github.com/talen/tce/internal/tools"
)

// BenchmarkLatencyReadTool measures the base latency of executing a read tool
// through the mock LLM loop (simulates fast model like Ollama with small context).
func BenchmarkLatencyReadTool(b *testing.B) {
	tmpDir := b.TempDir()
	os.WriteFile(tmpDir+"/file.txt", []byte("content"), 0644)

	responses := []mockResponse{
		{toolCalls: []llm.ToolCall{{ID: "r1", Name: "read", Arguments: `{"file_path":"file.txt"}`}}},
		{content: "done"},
	}

	agent := New(Config{
		Type:     AgentBuild,
		LLM:      &mockLLM{responses: responses},
		Tools:    testRegistry(),
		Project:  testProfile(tmpDir),
		MaxTurns: 5,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		agent.Reset()
		_, err := agent.Run(context.Background(), "read file.txt", nil, nil, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLatencyMultiTool simulates a session with multiple tool calls per turn
// (e.g., read, grep, write sequence).
func BenchmarkLatencyMultiTool(b *testing.B) {
	tmpDir := b.TempDir()
	os.WriteFile(tmpDir+"/main.go", []byte("package main\n\nfunc main() {}"), 0644)

	responses := []mockResponse{
		{
			toolCalls: []llm.ToolCall{
				{ID: "r1", Name: "read", Arguments: `{"file_path":"main.go"}`},
			},
		},
		{
			toolCalls: []llm.ToolCall{
				{ID: "g1", Name: "grep", Arguments: `{"pattern":"func","include":"*.go"}`},
			},
		},
		{
			toolCalls: []llm.ToolCall{
				{ID: "w1", Name: "write", Arguments: `{"file_path":"new.go","content":"package main\n"}`},
			},
		},
		{content: "completed all tools"},
	}

	agent := New(Config{
		Type:     AgentBuild,
		LLM:      &mockLLM{responses: responses},
		Tools:    testRegistry(),
		Project:  testProfile(tmpDir),
		MaxTurns: 10,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		agent.Reset()
		_, err := agent.Run(context.Background(), "refactor main.go", nil, nil, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLatencyWriteTool measures write tool latency (simulates file creation).
func BenchmarkLatencyWriteTool(b *testing.B) {
	tmpDir := b.TempDir()

	responses := []mockResponse{
		{toolCalls: []llm.ToolCall{{ID: "w1", Name: "write", Arguments: `{"file_path":"new.txt","content":"hello world"}`}}},
		{content: "file written"},
	}

	agent := New(Config{
		Type:     AgentBuild,
		LLM:      &mockLLM{responses: responses},
		Tools:    testRegistry(),
		Project:  testProfile(tmpDir),
		MaxTurns: 5,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		agent.Reset()
		_, err := agent.Run(context.Background(), "write new.txt", nil, nil, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLatencyBashTool measures bash command execution latency.
func BenchmarkLatencyBashTool(b *testing.B) {
	tmpDir := b.TempDir()

	responses := []mockResponse{
		{toolCalls: []llm.ToolCall{{ID: "b1", Name: "bash", Arguments: `{"command":"echo 'hello'"}`}}},
		{content: "bash executed"},
	}

	agent := New(Config{
		Type:     AgentBuild,
		LLM:      &mockLLM{responses: responses},
		Tools:    testRegistry(),
		Project:  testProfile(tmpDir),
		MaxTurns: 5,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		agent.Reset()
		_, err := agent.Run(context.Background(), "run echo hello", nil, nil, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLatencySearchTool measures web search latency (uses DuckDuckGo).
func BenchmarkLatencySearchTool(b *testing.B) {
	tmpDir := b.TempDir()

	responses := []mockResponse{
		{toolCalls: []llm.ToolCall{{ID: "s1", Name: "search", Arguments: `{"query":"golang testing"}`}}},
		{content: "search results"},
	}

	reg := tools.NewRegistry()
	reg.Register(&tools.ReadTool{})
	reg.Register(&tools.GrepTool{})
	reg.Register(&tools.GlobTool{})
	reg.Register(&tools.BashTool{})
	reg.Register(&tools.WriteTool{})
	reg.Register(&tools.EditTool{})
	reg.Register(&tools.SearchTool{})

	agent := New(Config{
		Type:     AgentBuild,
		LLM:      &mockLLM{responses: responses},
		Tools:    reg,
		Project:  testProfile(tmpDir),
		MaxTurns: 5,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		agent.Reset()
		_, err := agent.Run(context.Background(), "search golang testing", nil, nil, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLatencyHighConcurrency simulates multiple agent sessions running
// concurrently (like multiple users or parallel tasks).
func BenchmarkLatencyHighConcurrency(b *testing.B) {
	tmpDir := b.TempDir()
	os.WriteFile(tmpDir+"/file.txt", []byte("hello"), 0644)

	responses := []mockResponse{
		{toolCalls: []llm.ToolCall{{ID: "r1", Name: "read", Arguments: `{"file_path":"file.txt"}`}}},
		{content: "done"},
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			agent := New(Config{
				Type:     AgentBuild,
				LLM:      &mockLLM{responses: append([]mockResponse{}, responses...)},
				Tools:    testRegistry(),
				Project:  testProfile(tmpDir),
				MaxTurns: 5,
			})
			_, err := agent.Run(context.Background(), "read file.txt", nil, nil, nil)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkLatencyTurnCount measures how latency scales with turn count
// (simulates long sessions with many back-and-forth turns).
func BenchmarkLatencyTurnCount(b *testing.B) {
	tmpDir := b.TempDir()

	turnCounts := []int{5, 10, 25}

	for _, turns := range turnCounts {
		b.Run(fmt.Sprintf("turns=%d", turns), func(b *testing.B) {
			responses := make([]mockResponse, turns)
			for i := 0; i < turns-1; i++ {
				responses[i] = mockResponse{
					toolCalls: []llm.ToolCall{{ID: "c", Name: "bash", Arguments: `{"command":"echo ok"}`}},
				}
			}
			responses[turns-1] = mockResponse{content: "finished"}

			agent := New(Config{
				Type:          AgentBuild,
				LLM:           &mockLLM{responses: responses},
				Tools:         testRegistry(),
				Project:       testProfile(tmpDir),
				MaxTurns:      turns + 5,
				DisableStream: true,
			})

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				agent.Reset()
				_, err := agent.Run(context.Background(), "do work", nil, nil, nil)
				if err != nil {
					b.Fatalf("error at turns=%d: %v", turns, err)
				}
			}
		})
	}
}
