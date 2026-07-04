// Package mcp implements a Model Context Protocol (MCP) client
// for connecting to external MCP servers via stdio transport.
package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

var rpcID int64
var idMu sync.Mutex

func nextRPCID() int64 {
	idMu.Lock()
	defer idMu.Unlock()
	rpcID++
	return rpcID
}

type rpcMessage struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      *int64    `json:"id,omitempty"`
	Method  string    `json:"method,omitempty"`
	Params  any       `json:"params,omitempty"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Client connects to an MCP server process via stdio.
type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	mu     sync.Mutex

	initialized bool
}

// MCPServerConfig describes how to launch an MCP server.
type MCPServerConfig struct {
	Name    string   `json:"name"`
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}

// ToolDef describes a tool exposed by an MCP server.
type ToolDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema"`
}

// NewClient starts an MCP server process and performs the initialization handshake.
func NewClient(cfg MCPServerConfig) (*Client, error) {
	cmd := exec.Command(cfg.Command, cfg.Args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = nil // let stderr go to parent

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start mcp server %q: %w", cfg.Command, err)
	}

	c := &Client{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
	}

	if err := c.initialize(); err != nil {
		c.Close()
		return nil, err
	}

	return c, nil
}

func (c *Client) initialize() error {
	_, err := c.call("initialize", map[string]any{
		"protocolVersion": "2025-03-26",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "tce",
			"version": "0.1.0",
		},
	})
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	c.initialized = true

	// Send initialized notification (no response expected)
	_ = c.sendNotification("notifications/initialized", map[string]any{})
	return nil
}

// ListTools returns the list of tools from the MCP server.
func (c *Client) ListTools() ([]ToolDef, error) {
	result, err := c.call("tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	resp, ok := result.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected response type for tools/list")
	}
	toolsRaw, ok := resp["tools"]
	if !ok {
		return nil, fmt.Errorf("no tools in tools/list response")
	}
	toolsJSON, err := json.Marshal(toolsRaw)
	if err != nil {
		return nil, fmt.Errorf("marshal tools: %w", err)
	}
	var tools []ToolDef
	if err := json.Unmarshal(toolsJSON, &tools); err != nil {
		return nil, fmt.Errorf("unmarshal tools: %w", err)
	}
	return tools, nil
}

// CallTool executes a tool with the given arguments.
func (c *Client) CallTool(name string, args map[string]any) (string, error) {
	result, err := c.call("tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	})
	if err != nil {
		return "", err
	}
	// tools/call returns {content: [{type: "text", text: "..."}]}
	resp, ok := result.(map[string]any)
	if !ok {
		return fmt.Sprintf("%v", result), nil
	}
	content, _ := resp["content"].([]any)
	var b bytes.Buffer
	for _, c := range content {
		if m, ok := c.(map[string]any); ok {
			if t, _ := m["type"].(string); t == "text" {
				if text, _ := m["text"].(string); text != "" {
					if b.Len() > 0 {
						b.WriteString("\n")
					}
					b.WriteString(text)
				}
			}
		}
	}
	if b.Len() > 0 {
		return b.String(), nil
	}
	return fmt.Sprintf("%v", result), nil
}

// Close terminates the MCP server process.
func (c *Client) Close() error {
	c.stdin.Close()
	return c.cmd.Wait()
}

func (c *Client) call(method string, params any) (any, error) {
	id := nextRPCID()
	msg := rpcMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  method,
		Params:  params,
	}

	if err := c.writeMessage(msg); err != nil {
		return nil, err
	}

	return c.readResponse(id)
}

func (c *Client) sendNotification(method string, params any) error {
	msg := rpcMessage{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	return c.writeMessage(msg)
}

func (c *Client) writeMessage(msg any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// MCP uses Content-Length framing: each message is preceded by
	// "Content-Length: N\r\n\r\n" followed by N bytes of JSON body (ADR-005).
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err := c.stdin.Write([]byte(header)); err != nil {
		return err
	}
	if _, err := c.stdin.Write(data); err != nil {
		return err
	}
	return nil
}

func (c *Client) readResponse(expectedID int64) (any, error) {
	// Read with Content-Length framing
	reader := bufio.NewReader(c.stdout)

	for {
		// Parse headers
		var contentLength int
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return nil, fmt.Errorf("read header: %w", err)
			}
			line = line[:len(line)-1] // trim \n
			// Handle both \r\n and \n
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			if line == "" {
				break
			}
			if bytes.HasPrefix([]byte(line), []byte("Content-Length: ")) {
				_, _ = fmt.Sscanf(line, "Content-Length: %d", &contentLength)
			}
		}

		if contentLength <= 0 {
			return nil, fmt.Errorf("invalid content length: %d", contentLength)
		}

		body := make([]byte, contentLength)
		if _, err := io.ReadFull(reader, body); err != nil {
			return nil, fmt.Errorf("read body: %w", err)
		}

		var resp rpcMessage
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("unmarshal response: %w", err)
		}

		// Skip notifications (no ID)
		if resp.ID == nil {
			continue
		}
		if *resp.ID != expectedID {
			continue
		}

		if resp.Error != nil {
			return nil, fmt.Errorf("RPC error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	}
}
