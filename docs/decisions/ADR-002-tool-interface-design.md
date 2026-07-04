# ADR-002: Tool Interface Design

**Date:** 2026-07-03  
**Status:** Accepted  

## Context

TCE needs a uniform way to define and invoke tools. The LLM (via native tool calling or JSON extraction) produces `{name, arguments}` pairs; the agent must execute them and return results. Tools differ vastly in their inputs (file paths, regex patterns, shell commands) and their execution semantics.

## Decision

Every tool implements the `Tool` interface:

```go
type Tool interface {
    Name() string
    Description() string
    ShortDescription() string
    Schema() any          // JSON Schema for LLM tool definition
    Execute(ctx ExecContext, input json.RawMessage) (string, error)
}
```

Key design choices:
- **`Schema() any`** returns a free-form JSON Schema object, letting each tool define its own parameter structure — no shared type system needed
- **`json.RawMessage` input** — tools parse their own JSON rather than receiving typed structs, allowing flexible field aliasing and error recovery
- **String output** — all tool results are plain text; the LLM receives them as text content, not structured data (simplifies message history serialization)
- **Field aliases** — via `firstOf(raw, "file_path", "path", "file", "filename")`, all tools accept multiple names for the same parameter (resilience against different LLM naming conventions)
- **Structured error format** — `"Error: ...\nExpected: {...}\nYou sent: ..."` gives the LLM immediate, actionable feedback on malformed calls

## Consequences

**Positive:**
- Adding a new tool = writing one struct implementing 5 methods
- The interface is trivially mockable
- External tools (MCP, subprocess) can be wrapped as `Tool` implementations
- The registry can list, filter, and disable tools uniformly

**Negative:**
- String output means structured data (e.g., search results with titles/URLs) must be formatted to text — no structured result passing between tools
- No compile-time parameter validation — each tool must validate its own JSON

## Alternatives Considered

- **Typed parameters via generics**: Rejected — Go generics don't help with runtime JSON parsing from an LLM
- **Single mega-struct with all possible fields**: Rejected — would couple every tool to every other tool's parameters
- **Result as `any`**: Rejected — complicates message history serialization; plain text is always serializable
