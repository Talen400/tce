package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

type Config struct {
	BaseURL        string
	APIKey         string
	Model          string
	Temperature    float32
	MaxTokens      int
	Timeout        time.Duration
	ResponseFormat string
}

var DefaultConfig = Config{
	BaseURL:     "http://localhost:11434/v1",
	APIKey:      "ollama",
	Model:       "qwen3.5:4b",
	Temperature: 0.2,
	MaxTokens:   4096,
	Timeout:     120 * time.Second,
}

type ToolDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

type apiToolCall struct {
	ID       string `json:"id"`
	Index    int    `json:"index"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

func (m Message) MarshalJSON() ([]byte, error) {
	if len(m.ToolCalls) == 0 {
		return json.Marshal(struct {
			Role       string `json:"role"`
			Content    string `json:"content,omitempty"`
			ToolCallID string `json:"tool_call_id,omitempty"`
		}{Role: m.Role, Content: m.Content, ToolCallID: m.ToolCallID})
	}

	tcs := make([]*apiToolCall, 0, len(m.ToolCalls))
	for _, tc := range m.ToolCalls {
		tcs = append(tcs, &apiToolCall{
			ID:    tc.ID,
			Type:  "function",
			Index: 0,
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{Name: tc.Name, Arguments: tc.Arguments},
		})
	}
	return json.Marshal(struct {
		Role      string         `json:"role"`
		Content   string         `json:"content,omitempty"`
		ToolCalls []*apiToolCall `json:"tool_calls"`
	}{Role: m.Role, Content: m.Content, ToolCalls: tcs})
}

func (m *Message) UnmarshalJSON(data []byte) error {
	type Alias struct {
		Role       string         `json:"role"`
		Content    string         `json:"content,omitempty"`
		ToolCallID string         `json:"tool_call_id,omitempty"`
		ToolCalls  []*apiToolCall `json:"tool_calls,omitempty"`
	}
	var a Alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	m.Role = a.Role
	m.Content = a.Content
	m.ToolCallID = a.ToolCallID
	for _, at := range a.ToolCalls {
		m.ToolCalls = append(m.ToolCalls, ToolCall{
			ID:        at.ID,
			Name:      at.Function.Name,
			Arguments: at.Function.Arguments,
		})
	}
	return nil
}

type Response struct {
	Content   string
	ToolCalls []ToolCall
}

type Client struct {
	config Config
	http   *http.Client
}

func NewClient(cfg Config) *Client {
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultConfig.Timeout
	}
	return &Client{
		config: cfg,
		http:   &http.Client{Timeout: cfg.Timeout},
	}
}

func (c *Client) Config() Config {
	return c.config
}

func (c *Client) ModelName() string {
	return c.config.Model
}

type chatRequest struct {
	Model          string      `json:"model"`
	Messages       []Message   `json:"messages"`
	Tools          []ToolDef   `json:"tools,omitempty"`
	Temperature    float32     `json:"temperature"`
	MaxTokens      int         `json:"max_tokens,omitempty"`
	Stream         bool        `json:"stream"`
	ResponseFormat any         `json:"response_format,omitempty"`
}

func (c *Client) Chat(ctx context.Context, messages []Message, tools []ToolDef) (*Response, error) {
	reqBody := chatRequest{
		Model:       c.config.Model,
		Messages:    messages,
		Tools:       tools,
		Temperature: c.config.Temperature,
		MaxTokens:   c.config.MaxTokens,
		Stream:      false,
	}

	if c.config.ResponseFormat != "" {
		reqBody.ResponseFormat = map[string]string{"type": c.config.ResponseFormat}
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	resp, err := c.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResp struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("empty response")
	}

	msg := apiResp.Choices[0].Message
	return &Response{
		Content:   msg.Content,
		ToolCalls: msg.ToolCalls,
	}, nil
}

func (c *Client) ChatStream(ctx context.Context, messages []Message, tools []ToolDef, onChunk func(string)) (*Response, error) {
	reqBody := chatRequest{
		Model:       c.config.Model,
		Messages:    messages,
		Tools:       tools,
		Temperature: c.config.Temperature,
		MaxTokens:   c.config.MaxTokens,
		Stream:      true,
	}

	if c.config.ResponseFormat != "" {
		reqBody.ResponseFormat = map[string]string{"type": c.config.ResponseFormat}
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	resp, err := c.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	result := &Response{}
	parser := newSSEParser(resp.Body)

	for {
		data, ok := parser.next()
		if !ok {
			break
		}

		if string(data) == "[DONE]" {
			break
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content   string        `json:"content"`
					Reasoning string        `json:"reasoning"`
					ToolCalls []*apiToolCall `json:"tool_calls"`
				} `json:"delta"`
				Finish string `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(data, &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta

		if delta.Content != "" {
			result.Content += delta.Content
			if onChunk != nil {
				onChunk(delta.Content)
			}
		}

		if delta.ToolCalls != nil {
			for _, at := range delta.ToolCalls {
				if at.ID == "" && at.Function.Name == "" && at.Function.Arguments == "" {
					continue
				}
				idx := at.Index
				if idx < 0 {
					idx = 0
				}
				if idx < len(result.ToolCalls) {
					tc := &result.ToolCalls[idx]
					if at.ID != "" {
						tc.ID = at.ID
					}
					if at.Function.Name != "" {
						tc.Name = at.Function.Name
					}
					if at.Function.Arguments != "" {
						tc.Arguments += at.Function.Arguments
					}
				} else {
					for len(result.ToolCalls) <= idx {
						result.ToolCalls = append(result.ToolCalls, ToolCall{})
					}
					tc := &result.ToolCalls[idx]
					tc.ID = at.ID
					tc.Name = at.Function.Name
					tc.Arguments = at.Function.Arguments
				}
			}
		}
	}

	return result, parser.err()
}

type sseParser struct {
	scanner *bufio.Scanner
	buf     strings.Builder
	inData  bool
	lastErr error
}

func newSSEParser(r io.Reader) *sseParser {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	return &sseParser{scanner: s}
}

func (p *sseParser) next() ([]byte, bool) {
	for p.scanner.Scan() {
		line := p.scanner.Text()

		if strings.HasPrefix(line, "data: ") {
			payload := strings.TrimPrefix(line, "data: ")
			if payload == "[DONE]" {
				return []byte("[DONE]"), true
			}
			p.buf.WriteString(payload)
			p.inData = true
			continue
		}

		if line == "" && p.inData {
			data := []byte(p.buf.String())
			p.buf.Reset()
			p.inData = false
			return data, true
		}

		if strings.HasPrefix(line, "event: ") || strings.HasPrefix(line, "retry: ") {
			continue
		}

		if line == "data:" && !p.inData {
			continue
		}
	}

	if p.inData && p.buf.Len() > 0 {
		data := []byte(p.buf.String())
		p.buf.Reset()
		p.inData = false
		return data, true
	}

	p.lastErr = p.scanner.Err()
	return nil, false
}

func (p *sseParser) err() error {
	return p.lastErr
}

func (c *Client) doRequest(ctx context.Context, body []byte) (*http.Response, error) {
	var lastErr error
	backoff := 100 * time.Millisecond
	maxRetries := 2

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			jitter := time.Duration(rand.Int63n(int64(backoff)))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff + jitter):
			}
			backoff *= 3
		}

		req, err := http.NewRequestWithContext(ctx, "POST", c.config.BaseURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("http request (attempt %d): %w", attempt+1, err)
			continue
		}

		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = fmt.Errorf("API error (attempt %d): %d", attempt+1, resp.StatusCode)
			continue
		}

		if resp.StatusCode != 200 {
			respBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
		}

		return resp, nil
	}

	return nil, lastErr
}
