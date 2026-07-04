package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/talen/tce/internal/llm"
	"github.com/talen/tce/internal/tools"
)

// mockLLMServer returns an httptest.Server that responds like an OpenAI-compatible API.
// The responses cycle through a fixed list; after exhausting them, it repeats the last one.
func mockLLMServer(t *testing.T, responses []llm.Response) *httptest.Server {
	t.Helper()

	if len(responses) == 0 {
		responses = []llm.Response{{Content: "ok"}}
	}

	var mu struct {
		idx int
	}
	mu.idx = 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "method not allowed", 405)
			return
		}

		// Decode the request to detect streaming mode
		var rawReq map[string]any
		if err := json.NewDecoder(r.Body).Decode(&rawReq); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}

		model, _ := rawReq["model"].(string)
		if model == "" {
			model = "test-model"
		}
		isStream, _ := rawReq["stream"].(bool)

		mu.idx++
		respIdx := mu.idx - 1
		if respIdx >= len(responses) {
			respIdx = len(responses) - 1
		}
		resp := responses[respIdx]

		// Build tool calls array
		type toolCallJSON struct {
			ID       string `json:"id"`
			Index    int    `json:"index"`
			Type     string `json:"type"`
			Function struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			} `json:"function"`
		}
		var toolCalls []toolCallJSON
		for i, tc := range resp.ToolCalls {
			tcJSON := toolCallJSON{
				ID:    tc.ID,
				Index: i,
				Type:  "function",
			}
			tcJSON.Function.Name = tc.Name
			tcJSON.Function.Arguments = tc.Arguments
			toolCalls = append(toolCalls, tcJSON)
		}

		if isStream {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.WriteHeader(200)

			chunk := map[string]any{
				"choices": []any{
					map[string]any{
						"delta": map[string]any{
							"content":    resp.Content,
							"tool_calls": toolCalls,
						},
						"finish_reason": "stop",
					},
				},
			}
			if len(toolCalls) > 0 {
				chunk["choices"].([]any)[0].(map[string]any)["finish_reason"] = "tool_calls"
			}
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			fmt.Fprintf(w, "data: [DONE]\n\n")
			return
		}

		// Non-streaming response
		finishReason := "stop"
		if len(toolCalls) > 0 {
			finishReason = "tool_calls"
		}

		// Ensure content is null (not empty string) when there are tool calls
		content := resp.Content
		if len(toolCalls) > 0 {
			content = ""
		}

		body := map[string]any{
			"id":      "chatcmpl-mock",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   model,
			"choices": []any{
				map[string]any{
					"index": 0,
					"message": map[string]any{
						"role":       "assistant",
						"content":    content,
						"tool_calls": toolCalls,
					},
					"finish_reason": finishReason,
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     50,
				"completion_tokens": 30,
				"total_tokens":      80,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(body)
	}))

	return srv
}

// apiToolCall mirrors the OpenAI API tool call format for the mock server.
type apiToolCall struct {
	ID       string `json:"id"`
	Index    int    `json:"index,omitempty"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func testLLMClient(serverURL string) *llm.Client {
	cfg := llm.DefaultConfig
	cfg.BaseURL = serverURL
	cfg.Model = "test-model"
	cfg.APIKey = "test-key"
	cfg.Timeout = 5 * time.Second
	return llm.NewClient(cfg)
}

func fullTestRegistry() *tools.Registry {
	reg := tools.NewRegistry()
	reg.Register(&tools.ReadTool{})
	reg.Register(&tools.GrepTool{})
	reg.Register(&tools.GlobTool{})
	reg.Register(&tools.BashTool{})
	reg.Register(&tools.WriteTool{})
	reg.Register(&tools.EditTool{})
	reg.Register(&tools.SearchTool{})
	return reg
}

// TestE2EHttpMockServer tests the full agent loop against a real HTTP mock.
func TestE2EHttpMockServer(t *testing.T) {
	srv := mockLLMServer(t, []llm.Response{
		{ToolCalls: []llm.ToolCall{
			{ID: "c1", Name: "bash", Arguments: `{"command":"echo hello"}`},
		}},
		{Content: "Task completed successfully."},
	})
	defer srv.Close()

	client := testLLMClient(srv.URL)
	agent := New(Config{
		Type:      AgentBuild,
		LLM:       client,
		Tools:     fullTestRegistry(),
		Project:   testProfile(t.TempDir()),
		MaxTurns:  5,
		MaxContext: 16000,
	})

	errCh := make(chan error, 1)

	go func() {
		_, err := agent.Run(context.Background(), "run echo hello",
			func(token string) {},
			func(name, args string) {},
			func(name, result string) {},
		)
		errCh <- err
	}()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for agent to finish")
	}

	// Verify the agent made progress by checking message history
	msgs := agent.GetMessages()
	if len(msgs) < 2 {
		t.Fatal("expected at least 2 messages (system + user + tool)")
	}
}

// TestE2EStreamingHttpMock tests streaming mode.
func TestE2EStreamingHttpMock(t *testing.T) {
	srv := mockLLMServer(t, []llm.Response{
		{ToolCalls: []llm.ToolCall{
			{ID: "c1", Name: "glob", Arguments: `{"pattern":"*.go"}`},
		}},
		{Content: "Here are the Go files."},
	})
	defer srv.Close()

	client := testLLMClient(srv.URL)
	agent := New(Config{
		Type:          AgentBuild,
		LLM:           client,
		Tools:         fullTestRegistry(),
		Project:       testProfile(t.TempDir()),
		MaxTurns:      5,
		MaxContext:    16000,
		DisableStream: false,
	})

	toolNames := make([]string, 0, 2)
	_, err := agent.Run(context.Background(), "find go files",
		func(token string) {},
		func(name, args string) {
			toolNames = append(toolNames, name)
		},
		func(name, result string) {},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(toolNames) == 0 {
		t.Error("expected at least one tool call")
	}
	if toolNames[0] != "glob" {
		t.Errorf("expected glob, got %s", toolNames[0])
	}
}

// TestE2EMultiToolCall tests multiple tool calls in a single turn.
func TestE2EMultiToolCall(t *testing.T) {
	srv := mockLLMServer(t, []llm.Response{
		{
			ToolCalls: []llm.ToolCall{
				{ID: "c1", Name: "glob", Arguments: `{"pattern":"*.go"}`},
				{ID: "c2", Name: "grep", Arguments: `{"pattern":"func","include":"*.go"}`},
			},
		},
		{Content: "Found files and functions."},
	})
	defer srv.Close()

	client := testLLMClient(srv.URL)
	agent := New(Config{
		Type:      AgentBuild,
		LLM:       client,
		Tools:     fullTestRegistry(),
		Project:   testProfile(t.TempDir()),
		MaxTurns:  5,
		MaxContext: 16000,
	})

	toolNames := make([]string, 0, 2)
	_, err := agent.Run(context.Background(), "find go files and functions",
		func(token string) {},
		func(name, args string) {
			toolNames = append(toolNames, name)
		},
		func(name, result string) {},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(toolNames) < 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(toolNames))
	}
	if toolNames[0] != "glob" {
		t.Errorf("expected glob first, got %s", toolNames[0])
	}
	if toolNames[1] != "grep" {
		t.Errorf("expected grep second, got %s", toolNames[1])
	}
}

// TestE2EFileReadWrite tests a real file read and write through the agent.
func TestE2EFileReadWrite(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(tmpDir+"/hello.txt", []byte("Hello, World!"), 0644)

	srv := mockLLMServer(t, []llm.Response{
		{
			ToolCalls: []llm.ToolCall{
				{ID: "r1", Name: "read", Arguments: `{"file_path":"hello.txt"}`},
			},
		},
		{Content: "File content read successfully."},
	})
	defer srv.Close()

	client := testLLMClient(srv.URL)
	agent := New(Config{
		Type:      AgentBuild,
		LLM:       client,
		Tools:     fullTestRegistry(),
		Project:   testProfile(tmpDir),
		MaxTurns:  5,
		MaxContext: 16000,
	})

	result, err := agent.Run(context.Background(), "read hello.txt", nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}
