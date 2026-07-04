package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/talen/tce/internal/tools"
)

// ToolAdapter wraps an MCP tool definition as a tools.Tool.
type ToolAdapter struct {
	Def     ToolDef
	Client  *Client
	ToolDesc string
}

func (a *ToolAdapter) Name() string             { return "mcp_" + a.Def.Name }
func (a *ToolAdapter) Description() string      { return a.ToolDesc }
func (a *ToolAdapter) ShortDescription() string { return a.Def.Description }

func (a *ToolAdapter) Schema() any {
	return a.Def.InputSchema
}

func (a *ToolAdapter) Execute(ctx tools.ExecContext, input json.RawMessage) (string, error) {
	var args map[string]any
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	return a.Client.CallTool(a.Def.Name, args)
}
