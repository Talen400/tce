# ADR-005: MCP Integration

**Date:** 2026-07-03  
**Status:** Accepted  

## Context

The Model Context Protocol (MCP) is an emerging standard for connecting LLM agents to external tools and data sources. TCE needed a way to integrate MCP servers as tools without coupling to a specific MCP client library or transport.

## Decision

MCP servers are connected via **stdio transport** using a custom JSON-RPC 2.0 client built in `internal/mcp/`.

The integration works as follows:
1. **Config**: MCP servers are defined in `.tce.yaml` under `mcp_servers:` — each specifies a command and args
2. **Connection**: At startup, TCE spawns the server process, connects via stdin/stdout, and performs the `initialize` handshake
3. **Tool discovery**: TCE calls `tools/list` and wraps each returned tool as a `ToolAdapter` implementing the internal `Tool` interface
4. **Registration**: Each adapter is registered in the tool registry with an `mcp_` prefix (e.g., `mcp_read_file`)
5. **Execution**: When the LLM calls `mcp_read_file`, the adapter calls `tools/call` on the MCP server and returns the text result

Key design decisions:
- **Framing**: Uses the MCP spec's `Content-Length` header format, not line-delimited JSON
- **Thread safety**: Write serialization via mutex (JSON-RPC messages must be sent atomically)
- **Graceful failure**: If an MCP server fails to connect, a warning is printed but TCE continues without it
- **No stderr capture**: Server stderr goes to the parent process (null by default), avoiding interference with RPC messages on stdout

## Consequences

**Positive:**
- Any MCP server becomes a TCE tool with zero code changes
- ToolAdapter pattern means MCP tools are indistinguishable from native tools to the agent
- The MCP client is simple (~200 lines) with no external dependencies

**Negative:**
- Stdio transport only — SSE transport is not implemented (servers must be local or proxied)
- No hot-reload — MCP servers are connected at startup only
- MCP tool names are prefixed with `mcp_` to avoid collision with native tools
- Each MCP server is a separate process — resource usage scales with number of servers

## Alternatives Considered

- **Use an existing Go MCP library**: Evaluated but most were pre-1.0 and had different API philosophies
- **Embed MCP server-side logic**: Rejected — the purpose is to connect to EXTERNAL servers, not to host tools
- **HTTP-based plugin system**: Rejected — MCP is becoming a standard; custom protocols would be redundant
