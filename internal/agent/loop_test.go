package agent

import (
	"context"
	"os"
	"testing"

	"github.com/talen/tce/internal/llm"
	"github.com/talen/tce/internal/project"
	"github.com/talen/tce/internal/tools"
)

type mockLLM struct {
	responses []mockResponse
	index     int
}

type mockResponse struct {
	content   string
	toolCalls []llm.ToolCall
}

func (m *mockLLM) ModelName() string { return "mock-model" }

func (m *mockLLM) Chat(ctx context.Context, messages []llm.Message, tools []llm.ToolDef) (*llm.Response, error) {
	if m.index >= len(m.responses) {
		return &llm.Response{}, nil
	}
	r := m.responses[m.index]
	m.index++
	return &llm.Response{Content: r.content, ToolCalls: r.toolCalls}, nil
}

func (m *mockLLM) ChatStream(ctx context.Context, messages []llm.Message, tools []llm.ToolDef, onChunk func(string)) (*llm.Response, error) {
	if m.index >= len(m.responses) {
		return &llm.Response{}, nil
	}
	r := m.responses[m.index]
	m.index++
	if onChunk != nil {
		onChunk(r.content)
	}
	return &llm.Response{Content: r.content, ToolCalls: r.toolCalls}, nil
}

func testProfile(root string) *project.Profile {
	return &project.Profile{Root: root, Language: "Go"}
}

func testRegistry() *tools.Registry {
	reg := tools.NewRegistry()
	reg.Register(&tools.ReadTool{})
	reg.Register(&tools.GrepTool{})
	reg.Register(&tools.GlobTool{})
	reg.Register(&tools.BashTool{})
	reg.Register(&tools.WriteTool{})
	reg.Register(&tools.EditTool{})
	reg.Register(&tools.AskTool{})
	reg.Register(&tools.TaskTool{})
	return reg
}

func TestAgentBasicResponse(t *testing.T) {
	agent := New(Config{
		Type:     AgentBuild,
		LLM:      &mockLLM{responses: []mockResponse{{content: "Hello! How can I help?"}}},
		Tools:    testRegistry(),
		Project:  testProfile(t.TempDir()),
		MaxTurns: 5,
	})

	result, err := agent.Run(context.Background(), "hi", nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Hello! How can I help?" {
		t.Errorf("expected 'Hello! How can I help?', got %s", result)
	}
}

func TestAgentToolCallFlow(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(tmpDir+"/test.txt", []byte("hello world"), 0644)

	agent := New(Config{
		Type: AgentBuild,
		LLM: &mockLLM{
			responses: []mockResponse{
				{
					toolCalls: []llm.ToolCall{
						{ID: "call_1", Name: "read", Arguments: `{"file_path":"test.txt"}`},
					},
				},
				{content: "I read the file: hello world"},
			},
		},
		Tools:    testRegistry(),
		Project:  testProfile(tmpDir),
		MaxTurns: 5,
	})

	result, err := agent.Run(context.Background(), "read test.txt", nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestAgentStallDetection(t *testing.T) {
	agent := New(Config{
		Type: AgentBuild,
		LLM: &mockLLM{
			responses: []mockResponse{
				{toolCalls: []llm.ToolCall{{ID: "c1", Name: "bash", Arguments: `{"command":"true"}`}}},
				{toolCalls: []llm.ToolCall{{ID: "c2", Name: "bash", Arguments: `{"command":"true"}`}}},
				{toolCalls: []llm.ToolCall{{ID: "c3", Name: "bash", Arguments: `{"command":"true"}`}}},
				{toolCalls: []llm.ToolCall{{ID: "c4", Name: "bash", Arguments: `{"command":"true"}`}}},
			},
		},
		Tools:    testRegistry(),
		Project:  testProfile(t.TempDir()),
		MaxTurns: 10,
	})

	_, err := agent.Run(context.Background(), "do something", nil, nil, nil)
	if err == nil {
		t.Error("expected stall error after 3 empty tool calls")
	}
}

func TestAgentMaxTurns(t *testing.T) {
	responses := make([]mockResponse, 30)
	for i := range responses {
		responses[i] = mockResponse{
			toolCalls: []llm.ToolCall{{ID: "c", Name: "bash", Arguments: `{"command":"true"}`}},
		}
	}

	agent := New(Config{
		Type:      AgentBuild,
		LLM:       &mockLLM{responses: responses},
		Tools:     testRegistry(),
		Project:   testProfile(t.TempDir()),
		MaxTurns:  3,
	})

	_, err := agent.Run(context.Background(), "do it", nil, nil, nil)
	if err == nil {
		t.Error("expected max turns error")
	}
}

func TestAgentSubAgent(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(tmpDir+"/main.go", []byte("package main"), 0644)

	agent := New(Config{
		Type: AgentBuild,
		LLM: &mockLLM{
			responses: []mockResponse{
				{
					toolCalls: []llm.ToolCall{
						{ID: "t1", Name: "task", Arguments: `{"subagent":"explore","prompt":"list go files"}`},
					},
				},
				{content: "Sub-agent found some files"},
			},
		},
		Tools:    testRegistry(),
		Project:  testProfile(tmpDir),
		MaxTurns: 5,
	})

	result, err := agent.Run(context.Background(), "find Go files", nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestAgentMinimalMode(t *testing.T) {
	agent := New(Config{
		Type:        AgentBuild,
		LLM:         &mockLLM{responses: []mockResponse{{content: "ok"}}},
		Tools:       testRegistry(),
		Project:     testProfile(t.TempDir()),
		MaxTurns:    3,
		MinimalMode: true,
	})

	result, err := agent.Run(context.Background(), "hi", nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Errorf("expected ok, got %s", result)
	}
}

func TestAgentReset(t *testing.T) {
	agent := New(Config{
		Type:    AgentBuild,
		LLM:     &mockLLM{responses: []mockResponse{{content: "first"}, {content: "second"}}},
		Tools:   tools.NewRegistry(),
		Project: testProfile(t.TempDir()),
	})

	result, err := agent.Run(context.Background(), "msg1", nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "first" {
		t.Errorf("expected first, got %s", result)
	}

	agent.Reset()

	result, err = agent.Run(context.Background(), "msg2", nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "second" {
		t.Errorf("expected second, got %s", result)
	}
}

func TestIsCodeRequestPortuguese(t *testing.T) {
	if !isCodeRequest("faz um ft_printf") {
		t.Error("expected true for 'faz um ft_printf'")
	}
	if !isCodeRequest("Faça uma implementação") {
		t.Error("expected true for 'Faça uma implementação'")
	}
	if !isCodeRequest("cria um projeto em C") {
		t.Error("expected true for 'cria um projeto'")
	}
	if !isCodeRequest("desenvolva uma função") {
		t.Error("expected true for 'desenvolva uma função'")
	}
}

func TestIsCodeRequestEnglish(t *testing.T) {
	if !isCodeRequest("implement a function") {
		t.Error("expected true for 'implement a function'")
	}
	if !isCodeRequest("write a parser") {
		t.Error("expected true for 'write a parser'")
	}
	if !isCodeRequest("make a Makefile") {
		t.Error("expected true for 'make a Makefile'")
	}
}

func TestIsCodeRequestGreeting(t *testing.T) {
	if isCodeRequest("ola") {
		t.Error("expected false for 'ola'")
	}
	if isCodeRequest("hello") {
		t.Error("expected false for 'hello'")
	}
	if isCodeRequest("how are you?") {
		t.Error("expected false for 'how are you?'")
	}
	if isCodeRequest("/help") {
		t.Error("expected false for '/help'")
	}
}

func TestIsCodeRequestFtPrefix(t *testing.T) {
	if isCodeRequest("ft_printf") {
		t.Error("expected false for 'ft_printf' — ft_ prefix is not a code request")
	}
	if isCodeRequest("search to me ft_printf") {
		t.Error("expected false for 'search to me ft_printf' — search requests are not code")
	}
}

func TestHasHallucinatedCodeBlocks(t *testing.T) {
	if !hasHallucinatedCode("Here's the code:\n```c\nint main() {\n    printf(\"hello\");\n    return 0;\n}\n```") {
		t.Error("expected true for code block without write tool")
	}
	if !hasHallucinatedCode("```go\npackage main\n\nfunc main() {\n    fmt.Println(\"hello\")\n}\n```") {
		t.Error("expected true for go code block")
	}
	if hasHallucinatedCode("A single ```c line``` in text") {
		t.Error("expected false for inline code reference")
	}
}

func TestHasHallucinatedCodeWithTool(t *testing.T) {
	if hasHallucinatedCode("❯ write(main.c)\n── Written 27 lines ──\n```c\nint main() {}\n```") {
		t.Error("expected false when write tool was used")
	}
}

func TestHasHallucinatedCodeClean(t *testing.T) {
	if hasHallucinatedCode("Hello! How can I help?") {
		t.Error("expected false for plain text")
	}
	if hasHallucinatedCode("") {
		t.Error("expected false for empty string")
	}
}

func TestLooksLikeFailedJSON(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{`{"name":"read","arguments":{}}`, true},
		{`{ 'name' : 'bash' }`, true},
		{"<tool_call>content</tool_call>", true},
		{"```json\n{\"name\":\"x\"}\n```", true},
		{"Hello, how can I help you today?", false},
		{"I'll search for that information.", false},
	}
	for _, tt := range tests {
		if got := looksLikeFailedJSON(tt.input); got != tt.want {
			t.Errorf("looksLikeFailedJSON(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func BenchmarkAgentSearchWriteFlow(b *testing.B) {
	tmpDir := b.TempDir()
	os.WriteFile(tmpDir+"/main.go", []byte("package main\nfunc main() {\n\tprintln(\"hello\")\n}\n"), 0644)

	responses := make([]mockResponse, 4)
	responses[0] = mockResponse{toolCalls: []llm.ToolCall{{ID: "g1", Name: "glob", Arguments: `{"pattern":"*.go"}`}}}
	responses[1] = mockResponse{toolCalls: []llm.ToolCall{{ID: "r1", Name: "read", Arguments: `{"file_path":"main.go"}`}}}
	responses[2] = mockResponse{toolCalls: []llm.ToolCall{{ID: "b1", Name: "bash", Arguments: `{"command":"echo ok"}`}}}
	responses[3] = mockResponse{content: "Done"}

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
		_, err := agent.Run(context.Background(), "implement ft_printf", nil, nil, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

