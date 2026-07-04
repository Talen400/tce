package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewClient(t *testing.T) {
	c := NewClient(DefaultConfig)
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestConfig(t *testing.T) {
	c := NewClient(DefaultConfig)
	cfg := c.Config()
	if cfg.Model != DefaultConfig.Model {
		t.Errorf("expected %s, got %s", DefaultConfig.Model, cfg.Model)
	}
}

func TestMessageSerialization(t *testing.T) {
	msg := Message{
		Role:    "assistant",
		Content: "test",
		ToolCalls: []ToolCall{
			{ID: "call1", Name: "bash", Arguments: `{"command":"ls"}`},
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var result map[string]any
	json.Unmarshal(data, &result)
	if result["role"] != "assistant" {
		t.Errorf("expected assistant, got %v", result["role"])
	}

	tcs, ok := result["tool_calls"].([]any)
	if !ok || len(tcs) != 1 {
		t.Fatalf("expected 1 tool_call, got %v", result["tool_calls"])
	}
}

func TestMessageUnmarshal(t *testing.T) {
	data := `{
		"role": "assistant",
		"content": "",
		"tool_calls": [{
			"id": "call_1",
			"type": "function",
			"function": {
				"name": "bash",
				"arguments": "{\"command\":\"ls\"}"
			}
		}]
	}`

	var msg Message
	if err := json.Unmarshal([]byte(data), &msg); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if msg.Role != "assistant" {
		t.Errorf("expected assistant, got %s", msg.Role)
	}
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].Name != "bash" {
		t.Errorf("expected bash, got %s", msg.ToolCalls[0].Name)
	}
	if msg.ToolCalls[0].Arguments != `{"command":"ls"}` {
		t.Errorf("expected arguments, got %s", msg.ToolCalls[0].Arguments)
	}
}

func TestMessageNoToolCalls(t *testing.T) {
	data := `{"role": "user", "content": "hello"}`
	var msg Message
	if err := json.Unmarshal([]byte(data), &msg); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if msg.Role != "user" {
		t.Errorf("expected user, got %s", msg.Role)
	}
	if len(msg.ToolCalls) != 0 {
		t.Errorf("expected 0 tool calls, got %d", len(msg.ToolCalls))
	}
}

func TestChat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"choices": [{
				"message": {
					"role": "assistant",
					"content": "Hello, world!"
				}
			}]
		}`))
	}))
	defer srv.Close()

	cfg := DefaultConfig
	cfg.BaseURL = srv.URL
	client := NewClient(cfg)

	resp, err := client.Chat(context.Background(), []Message{
		{Role: "user", Content: "say hello"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "Hello, world!" {
		t.Errorf("expected 'Hello, world!', got %s", resp.Content)
	}
}

func TestChatWithToolCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"choices": [{
				"message": {
					"role": "assistant",
					"content": "",
					"tool_calls": [{
						"id": "call_1",
						"type": "function",
						"function": {
							"name": "bash",
							"arguments": "{\"command\":\"ls\"}"
						}
					}]
				}
			}]
		}`))
	}))
	defer srv.Close()

	cfg := DefaultConfig
	cfg.BaseURL = srv.URL
	client := NewClient(cfg)

	resp, err := client.Chat(context.Background(), []Message{
		{Role: "user", Content: "run ls"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "bash" {
		t.Errorf("expected bash, got %s", resp.ToolCalls[0].Name)
	}
}

func TestChatAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error": "rate limited"}`))
	}))
	defer srv.Close()

	cfg := DefaultConfig
	cfg.BaseURL = srv.URL
	client := NewClient(cfg)

	_, err := client.Chat(context.Background(), []Message{
		{Role: "user", Content: "hi"},
	}, nil)
	if err == nil {
		t.Error("expected error for 429")
	}
}

func TestChatStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"},\"finish_reason\":null}]}\n\n"))
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\" world\"},\"finish_reason\":null}]}\n\n"))
		w.Write([]byte("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	cfg := DefaultConfig
	cfg.BaseURL = srv.URL
	client := NewClient(cfg)

	var chunks []string
	resp, err := client.ChatStream(context.Background(), []Message{
		{Role: "user", Content: "say hello"},
	}, nil, func(chunk string) {
		chunks = append(chunks, chunk)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "Hello world" {
		t.Errorf("expected 'Hello world', got %s", resp.Content)
	}
	if len(chunks) == 0 {
		t.Error("expected at least 1 chunk")
	}
}

func TestChatStreamToolCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"bash","arguments":""}}]},"finish_reason":null}]}` + "\n\n"))
		w.Write([]byte(`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"com"}}]},"finish_reason":null}]}` + "\n\n"))
		w.Write([]byte(`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"mand\":\"ls\"}"}}]},"finish_reason":null}]}` + "\n\n"))
		w.Write([]byte(`data: {"choices":[{"delta":{},"finish_reason":"stop"}]}` + "\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	cfg := DefaultConfig
	cfg.BaseURL = srv.URL
	client := NewClient(cfg)

	resp, err := client.ChatStream(context.Background(), []Message{
		{Role: "user", Content: "run ls"},
	}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d: %+v", len(resp.ToolCalls), resp.ToolCalls)
	}
	if resp.ToolCalls[0].Name != "bash" {
		t.Errorf("expected bash, got %s", resp.ToolCalls[0].Name)
	}
	if resp.ToolCalls[0].Arguments != `{"command":"ls"}` {
		t.Errorf("expected aggregated arguments, got %s", resp.ToolCalls[0].Arguments)
	}
}

func TestChatStreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal error"}`))
	}))
	defer srv.Close()

	cfg := DefaultConfig
	cfg.BaseURL = srv.URL
	client := NewClient(cfg)

	_, err := client.ChatStream(context.Background(), []Message{
		{Role: "user", Content: "hi"},
	}, nil, nil)
	if err == nil {
		t.Error("expected error for 500")
	}
}

func TestChatStreamNonSSELines(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// Non-SSE lines should be ignored
		w.Write([]byte(":\n\n"))
		w.Write([]byte("event: ping\n"))
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hi\"},\"finish_reason\":null}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	cfg := DefaultConfig
	cfg.BaseURL = srv.URL
	client := NewClient(cfg)

	resp, err := client.ChatStream(context.Background(), []Message{
		{Role: "user", Content: "hi"},
	}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(resp.Content, "hi") {
		t.Errorf("expected 'hi' in content, got %s", resp.Content)
	}
}

func TestChatStreamEmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {}\n\n"))
		w.Write([]byte("data: {\"choices\":[]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	cfg := DefaultConfig
	cfg.BaseURL = srv.URL
	client := NewClient(cfg)

	_, err := client.ChatStream(context.Background(), []Message{
		{Role: "user", Content: "hi"},
	}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExtractToolCallStandardJSON(t *testing.T) {
	tc := extractToolCall(`{"name":"read","arguments":{"file_path":"main.c"}}`)
	if tc == nil {
		t.Fatal("expected tool call")
	}
	if tc.Name != "read" {
		t.Errorf("expected read, got %s", tc.Name)
	}
	if tc.Arguments != `{"file_path":"main.c"}` {
		t.Errorf("unexpected args: %s", tc.Arguments)
	}
}

func TestExtractToolCallPrettyJSON(t *testing.T) {
	tc := extractToolCall(`{
		"name": "grep",
		"arguments": {
			"pattern": "printf",
			"include": "*.c"
		}
	}`)
	if tc == nil {
		t.Fatal("expected tool call")
	}
	if tc.Name != "grep" {
		t.Errorf("expected grep, got %s", tc.Name)
	}
}

func TestExtractToolCallSingleQuotes(t *testing.T) {
	tc := extractToolCall(`{'name':'bash','arguments':{'command':'ls -la'}}`)
	if tc == nil {
		t.Fatal("expected tool call from single-quoted JSON")
	}
	if tc.Name != "bash" {
		t.Errorf("expected bash, got %s", tc.Name)
	}
}

func TestExtractToolCallXMLTags(t *testing.T) {
	tc := extractToolCall(`<tool_call>{"name":"read","arguments":{"file_path":"main.c"}}</tool_call>`)
	if tc == nil {
		t.Fatal("expected tool call from XML tags")
	}
	if tc.Name != "read" {
		t.Errorf("expected read, got %s", tc.Name)
	}
}

func TestExtractToolCallJSONBlock(t *testing.T) {
	tc := extractToolCall("Here's the result:\n```json\n{\"name\":\"search\",\"arguments\":{\"query\":\"qsort\"}}\n```")
	if tc == nil {
		t.Fatal("expected tool call from json block")
	}
	if tc.Name != "search" {
		t.Errorf("expected search, got %s", tc.Name)
	}
}

func TestExtractToolCallWithSurroundingText(t *testing.T) {
	tc := extractToolCall(`Let me look that up. {"name":"read","arguments":{"file_path":"main.c"}} Here's what I found.`)
	if tc == nil {
		t.Fatal("expected tool call from text with surrounding text")
	}
	if tc.Name != "read" {
		t.Errorf("expected read, got %s", tc.Name)
	}
}

func TestExtractToolCallNilForEmpty(t *testing.T) {
	if tc := extractToolCall(""); tc != nil {
		t.Error("expected nil for empty")
	}
	if tc := extractToolCall("Hello, how can I help?"); tc != nil {
		t.Error("expected nil for plain text")
	}
	if tc := extractToolCall("```python\nprint('hello')\n```"); tc != nil {
		t.Error("expected nil for non-JSON code block")
	}
}

func TestFixJSONBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantOK   bool
		wantName string
	}{
		{"valid JSON", `{"name":"read","arguments":{"file_path":"x"}}`, true, "read"},
		{"leading text", `some text {"name":"write","arguments":{}}`, true, "write"},
		{"trailing garbage", `{"name":"bash","arguments":{}}
and more text`, true, "bash"},
		{"newlines in string", `{"name":"grep","arguments":{"pattern":"hello\nworld"}}`, true, "grep"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixed := fixJSONBytes([]byte(tt.input))
			if tt.wantOK && fixed == nil {
				t.Fatal("expected fixed JSON, got nil")
			}
			if !tt.wantOK && fixed != nil {
				t.Fatal("expected nil, got fixed JSON")
			}
			if fixed != nil {
				var result struct {
					Name string `json:"name"`
				}
				if err := json.Unmarshal(fixed, &result); err != nil {
					t.Fatalf("unmarshal error: %v", err)
				}
				if result.Name != tt.wantName {
					t.Errorf("expected name %s, got %s", tt.wantName, result.Name)
				}
			}
		})
	}
}

func TestExtractToolCallUsesFixJSON(t *testing.T) {
	// JSON with unescaped newline — fixJSONBytes should repair it
	tc := extractToolCall(`{"name":"write","arguments":{"file_path":"test.txt","content":"line1
line2"}}`)
	if tc == nil {
		t.Fatal("expected tool call from malformed JSON (unescaped newline)")
	}
	if tc.Name != "write" {
		t.Errorf("expected write, got %s", tc.Name)
	}
}
