# Tool Calling Improvements for Small Models

Implemented July 2026. All 167 tests pass (up from 153).

## Problem

Models with <7B parameters (Qwen 2.5-Coder 1.5B, 3B, 7B) do not reliably use tools:
- Respond with inline text/code instead of JSON tool calls
- Use wrong parameter names or malformed JSON
- Fail to make sequential tool calls
- JSON extraction fallback had low success rate

## Root Causes Identified

1. **Ollama template mismatch**: Qwen2.5-coder template expects `<tool_call>...</tool_call>` XML tags but TCE expected only bare JSON
2. **Temperature 0.2**: Introduced unnecessary variability for small models
3. **"Think step by step"**: Research shows extended CoT (256 tokens) is **worse** than no CoT for function calling (25% vs 44% on Qwen2.5-1.5B). Optimal is 8-16 tokens.
4. **`extractToolCall()` too rigid**: Only matched `{"name"...` pattern, missed single quotes, XML tags, ```json blocks
5. **`ft_` in `codeRequestPatterns`**: False positive blocking search/grep requests mentioning `ft_printf`
6. **`hasHallucinatedCode` too aggressive**: Blocked any response with a code block, even legitimate explanations

## Changes

### Phase 1 — High Impact

#### 1.1 Multi-pattern JSON extraction (`internal/llm/client.go`)

`extractToolCall()` now tries 4 patterns in order:

| Priority | Pattern | Example |
|----------|---------|---------|
| 1 | ` ```json ... ``` ` blocks | Most explicit format |
| 2 | `<tool_call>...</tool_call>` XML tags | Qwen template native format |
| 3 | `{"name"....}` double-quoted JSON | OpenAI standard format |
| 4 | `{'name'....}` single-quoted JSON | Common in small models |
| 5 | Fallback: find any `"name"` + `{...}` in text | Catch-all |

New helper functions:
- `tryParseToolCallJSON(raw)` — unmarshal + fallback to `fixJSONBytes` for malformed JSON
- `fixJSONBytes(raw)` — extracts `{...}` range, converts single quotes → double quotes, escapes literal newlines inside strings
- `convertSingleQuotes(s)` — converts `'key'` to `"key"` outside string contexts

#### 1.2 Temperature 0.0 (`internal/llm/client.go:31`)

```go
Temperature: 0.0, // was 0.2
```

Research consensus (JD Hodges eval, SurePrompts, "Brief Is Better", KATE paper): `temperature=0` is the universal standard for tool calling. Use `top_p=1`, fixed `seed` for reproducibility.

#### 1.3 Brief CoT / FR-CoT (`internal/agent/prompt.go`)

**Full prompt** (line 33):
```
Before: "Think step by step, then use tools or provide the answer."
After:  "Decide briefly (tool? args?), then output ONLY valid JSON for tool calls or plain text for answers. Never mix both."
```

**Minimal prompt** (lines 85-111):
- Restructured from BAD-list format to **conversation examples**:
  ```
  User: read main.c
  Assistant: {"name":"read","arguments":{"file_path":"main.c"}}
  ```
- Removed BAD examples list (negative instructions confuse small models)
- Rule added: `"Briefly decide (which tool? what args?) before outputting JSON"`

Research basis: "Brief Is Better" (arXiv 2604.02155): Qwen2.5-1.5B goes from 44% (no CoT) → 69% (16 token CoT) → 25% (256 token CoT). FR-CoT achieves same accuracy with 0% hallucination.

#### 1.4 Fix false positives (`internal/agent/loop.go`)

**`codeRequestPatterns`**: Removed `"ft_"` — was blocking legitimate search/grep requests mentioning `ft_printf`.

**`hasHallucinatedCode()`**: New heuristic:
```
Before: any ``` block → block
After:  only block if >50% of non-empty lines are code AND ≥3 code lines
```

### Phase 2 — Medium Impact

#### 2.1 Retry with reminder (`internal/agent/loop.go`)

When `len(resp.ToolCalls) == 0` but `looksLikeFailedJSON()` detects a JSON attempt:
1. Add assistant (failed) message to history
2. Inject user reminder with exact format specification
3. Retry without counting the turn

`looksLikeFailedJSON()` triggers on:
- `"name"` + `{` in response
- `'name'` + `{` in response
- `<tool_call>` tags
- ` ```json` blocks

#### 2.2 Circuit breaker: 3→5 (`internal/agent/loop.go:258`)

Small models need more attempts to learn correct format. Changed from 3 to 5 consecutive failures before disabling a tool.

#### 2.3 Remove "42" framework (`internal/project/detect.go`)

Removed `p.Framework = "42"` from C project detection. This leaked into system prompts as meaningless `Framework: 42`. Also removed unused `hasFtPrefix()` function.

#### 2.4 More parameter aliases

| Tool | New aliases |
|------|-------------|
| `bash` | `script`, `run`, `shell` |
| `task` | `instruction`, `objective`, `goal` |
| `search` | `question`, `topic` |

### Phase 3 — Variable Impact

#### 3.1 FR-CoT system prompt (Phase 1.3 combined)

Uses Function-Routing CoT: the model briefly identifies which tool before outputting JSON. Research shows this achieves highest accuracy with zero hallucination rate.

#### 3.2 `TCE_STREAM=false` support (`main.go`, `loop.go`)

When `TCE_STREAM=false`, the agent uses non-streaming `Chat()` instead of `ChatStream()`. Useful for very small models that produce unreliable streaming deltas. Added `DisableStream` field to `Config`.

## New Tests Added

| Test file | Test | What it covers |
|-----------|------|----------------|
| `client_test.go` | `TestExtractToolCallStandardJSON` | Basic `{"name":...}` parsing |
| `client_test.go` | `TestExtractToolCallPrettyJSON` | Pretty-printed JSON |
| `client_test.go` | `TestExtractToolCallSingleQuotes` | `'name':'bash'` format |
| `client_test.go` | `TestExtractToolCallXMLTags` | `<tool_call>...</tool_call>` |
| `client_test.go` | `TestExtractToolCallJSONBlock` | ` ```json ... ``` ` blocks |
| `client_test.go` | `TestExtractToolCallWithSurroundingText` | JSON embedded in text |
| `client_test.go` | `TestExtractToolCallNilForEmpty` | False positives |
| `client_test.go` | `TestFixJSONBytes` | Leading text, trailing garbage, newlines in strings |
| `client_test.go` | `TestExtractToolCallUsesFixJSON` | Malformed JSON recovery |
| `loop_test.go` | `TestLooksLikeFailedJSON` | Retry detection |
| `loop_test.go` | `TestHasHallucinatedCodeBlocks` | Updated: 3+ code lines, >50% ratio |

## Updated Tests

| Test | Change |
|------|--------|
| `TestIsCodeRequestFtPrefix` | Now expects `false` for `ft_printf` (no longer blocked) |
| `TestHasHallucinatedCodeBlocks` | Longer code blocks to meet new 3-line threshold |
| `TestHasHallucinatedCodeWithTool` | Unchanged (write marker still suppresses) |
| `TestDetectCFtPrefix` | Framework check removed (no more "42") |

## Configuration Recommendations

For best results with small models (<7B):

```bash
export TCE_TEMPERATURE=0
export TCE_MODEL=qwen3.5:4b
export TCE_MINIMAL=true
export TCE_STREAM=false    # if model produces broken streaming
```

Or create an Ollama Modelfile:

```dockerfile
FROM qwen2.5-coder:7b
PARAMETER temperature 0
PARAMETER top_p 1
PARAMETER num_ctx 32768
```

## Phase 3 — System Prompt Restructuring (OpenCode/Kilo/OpenClaw patterns)

Informed by analysis of OpenCode, Kilo OpenClaw system prompt architectures.

### Changes

| File | Change |
|------|--------|
| `internal/agent/prompt.go` | Full rewrite of both prompts |
| `internal/agent/prompt.go:13` | New `TCEVersion` package variable |
| `internal/llm/model.go` | All profiles: `Temperature: 0.0` |
| `main.go` | `agent.TCEVersion = version` set at startup |
| `internal/agent/prompt_test.go` | 12 new tests for prompt content + rules loading |

### Full Prompt Structure (from OpenCode writing guide + OpenClaw sections)

```
You are TCE v0.1.0 (Terminal Coding Assistant).

## Context
Language: C
Build: make | Test: make test | Package: ft_printf

## Tools
- read: Read file contents
- write: Create/overwrite files
- bash: Run shell commands

## Workflow
1. Understand the request and identify the goal
2. Gather information with search/read/grep
3. Plan which files to create or modify
4. Implement changes with write or edit
5. Verify with bash (compile, test, lint)
6. Finalize with a plain-text summary

## Rules
- Output ONLY: {"name":"tool_name","arguments":{...}} or plain text
- Never write code inline — always use write or edit tool
- Never invent APIs or file paths — use search or glob first
- Brief reasoning (1-5 words) before each tool call: which tool? what args?
- One tool call per response, unless parallel calls are safe

## Anti-patterns
- Writing code blocks in responses instead of using write tool
- Assuming function signatures without searching first
- Using bash for operations that have dedicated tools

## Project Rules
(from .tcerules file — optional)
```

Key design decisions:
- **Identity injection** (OpenCode pattern): `"You are TCE v0.1.0 (Terminal Coding Assistant)"` prevents model self-identification errors
- **Seções nomeadas** (OpenClaw pattern): `## Tools`, `## Workflow`, `## Rules`, `## Anti-patterns` — clear section headers optimize for model attention
- **Workflow em 6 passos** (OpenCode guide): sequential, actionable, maps to available tools
- **Anti-patterns section**: more effective than scattered rules (OpenCode finding)
- **Brief CoT integrado**: "Brief reasoning (1-5 words) before each tool call" — implements FR-CoT from "Brief Is Better" paper

### Minimal Prompt Structure (ultra-compact for <4B models)

```
You are TCE v0.1.0 (Terminal Coding Assistant) for c.

## Context
Project: ft_printf (C)
Tools: read, grep, glob, bash, search

## Workflow
Understand → Gather → Plan → Implement → Verify

## Output
{"name":"tool","arguments":{"key":"value"}}

## Rules
- No inline code — use write/edit tools
- Search first, never guess APIs
- One tool per response
- Brief reasoning: which tool? what args?
(none)
```

Reduced from 62 lines to ~20 lines (67% reduction). Removed conversation examples — research shows direct instructions outperform examples for models <4B.

### `.tcerules` Support (Kilo pattern)

Users can create a `.tcerules` file in the project root:

```
Always compile with -Wall -Wextra -Werror
Use C99 standard
Never use printf without bounds checking
```

Content is appended to the `## Project Rules` section of both full and minimal prompts.

### Cache Optimization (OpenClaw pattern)

Sections ordered by stability (top = most cached):
1. Identity + Rules — never changes
2. Tools — stable within session
3. Context/Project — changes per project
4. Project Rules — user-controlled, at the bottom

Maximizes KV-cache hit rate on Ollama/vLLM backends.

### Test Count

180 tests total (up from 153, +27 new tests):
- 12 new prompt content tests (sections, version, rules loading)
- 9 new extraction/parsing tests
- 3 updated tests
- 3 new utility tests (looksLikeFailedJSON, fixJSONBytes, convertSingleQuotes)

### Updated Model Profiles

All model profiles now use `Temperature: 0.0` for deterministic tool calling:

| Profile | Old Temp | New Temp |
|---------|----------|----------|
| qwen3.5:0.8b | 0.2 | 0.0 |
| qwen3.5:2b | 0.15 | 0.0 |
| qwen3.5 (prefix) | 0.2 | 0.0 |
| default | 0.2 | 0.0 |

## Research References

- "Brief Is Better: Non-Monotonic Chain-of-Thought Budget Effects in Function-Calling Language Agents" (arXiv 2604.02155, Apr 2026)
- "Don't Adapt Small Language Models for Tools; Adapt Tool Schemas to the Models" — PA-Tool (2025)
- Meta-Tool: Efficient Few-Shot Tool Adaptation for Small Language Models (GitHub, 2025)
- OpenInterpreter parsing architecture (3-tier: tool calling → function calling → text-based)
- JD Hodges Local LLM Tool Calling Eval (Mar 2026): Qwen3.5 4B achieves 97.5% pass rate
