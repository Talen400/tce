# ADR-003: JSON Extraction Fallback

**Date:** 2026-07-03  
**Status:** Accepted  

## Context

Not all LLM backends support native tool calling (OpenAI `tool_calls` in the response). Smaller models (Qwen 2.5-Coder 1.5B, 7B) and some backends (older Ollama versions) return tool call data as inline JSON text instead. TCE needs to handle both cases transparently.

## Decision

TCE supports two code paths for tool calling:

1. **Native Tool Calling** — if the LLM response contains `tool_calls` in the OpenAI format, use them directly
2. **JSON Extraction Fallback** — if no native tool calls exist, scan the text response for `{"name":"tool","arguments":{...}}` patterns

The fallback extraction logic:
1. First checks markdown ` ```json ` blocks for explicit tool call JSON
2. Then checks `<tool_call>` XML tags (Qwen template format)  
3. Then searches for `{"name"` patterns using brace matching
4. Supports single-quoted fields (`{'name': 'read'}`) for models that produce invalid JSON
5. Validates the extracted JSON strictly — rejects false positives

If extraction succeeds but the JSON is malformed, the assistant's message is NOT added to history; instead a correction prompt is sent ("Your response contained a malformed tool call..."). This avoids polluting context with failed attempts.

## Consequences

**Positive:**
- Works with any model that outputs tool schemas as text, even without native function calling support
- Single prompt template regardless of backend capability
- No backend feature detection needed — try native first, fall back to extraction

**Negative:**
- Extraction can produce false positives (e.g., a sentence containing `{"name"}` in a code example)
- Models that OUTPUT tool calls inline but don't ACCEPT tool results in context won't work well
- The JSON retry mechanism adds an extra turn in some failure cases

## Alternatives Considered

- **Require native tool calling always**: Rejected — would exclude most local models < 7B parameters
- **Use a structured output format (e.g., JSON mode)**: Considered but not universally supported by backends
- **Only support markdown code blocks**: Rejected — too restrictive; many models don't use markdown for tool calls
