# TCE — Terminal Coding Assistant

TCE is a CLI agent that helps you code by connecting to any OpenAI-compatible LLM backend (Ollama, OpenAI, vLLM, Groq, etc.). It reads, writes, edits, searches files and runs commands through tool calling — all from your terminal.

## Table of Contents

- [How it Works](#how-it-works)
- [Install](#install)
- [Quick Start](#quick-start)
- [Usage](#usage)
- [Agent Types](#agent-types)
- [Tools](#tools)
- [Model Profiles](#model-profiles)
- [Environment Variables](#environment-variables)
- [Custom Tools](#custom-tools)
- [MCP Servers](#mcp-servers)
- [How Tool Calling Works](#how-tool-calling-works)
- [Architecture](#architecture)
- [Scenarios](#scenarios)
- [FAQ](#faq)

---

## How it Works

1. You ask something in natural language (e.g., "implement a linked list")
2. TCE sends your request + available tools to the LLM
3. The LLM responds with tool calls (`read`, `write`, `bash`, `grep`, `search`, etc.)
4. TCE executes the tool and sends the result back to the LLM
5. The LLM uses the results to make progress (read more, write code, run tests)
6. This loop repeats until the task is done

LLMs that do **not** support native tool calling (like Qwen 2.5-Coder 1.5B/7B) can still work — TCE has a **JSON extraction fallback** that parses `{"name":"tool","arguments":{...}}` from the model's text output.

## Install

### Option 1: Quick install (binary) — **recommended**

```bash
curl -fsSL https://raw.githubusercontent.com/talen400/tce/main/install.sh | bash

# Or with custom path:
BINDIR=/usr/local/bin curl -fsSL https://raw.githubusercontent.com/talen400/tce/main/install.sh | bash
```

Downloads a pre-compiled binary for Linux/macOS (amd64/arm64).

### Option 2: Homebrew

```bash
brew install talen400/tce/tce
```

### Option 3: Build from source

```bash
git clone https://github.com/talen400/tce && cd tce
go build -o ~/.local/bin/tce .
```

Requires Go 1.24+.

## Quick Start

```bash
# 1. Make sure you have Ollama (or any OpenAI-compatible API) running

# 2. Pull a model
ollama pull qwen2.5-coder:7b

# 3. Run TCE
tce --model qwen2.5-coder:7b

# 4. Start coding
> read the main file
> implement a function that parses command-line arguments
> write a Makefile for this project
> search for how to use signals in C
```

## Usage

### Basic

```bash
# TUI mode (default)
tce

# CLI mode
tce --cli

# Specify model
tce --model qwen2.5-coder:7b

# Specify API endpoint
tce --base-url http://localhost:11434/v1

# Project directory
tce --dir /path/to/project
```

### Advanced

```bash
# Minimal prompt (better for models < 4B parameters)
tce --minimal

# Agent type
tce --agent build        # full access (default)
tce --agent plan         # read-only + bash with confirmation
tce --agent explore      # read-only

# Context size override
tce --context-size 8192

# API key for remote services (OpenAI, Groq, etc.)
tce --base-url https://api.openai.com/v1 --api-key sk-...

# Create a branch before starting
tce --branch feat/my-feature

# Silent mode (no output, for automation)
tce --cli --output silent < prompt.txt

# JSON output (machine-readable)
tce --cli --output json --model gpt-4o-mini -dir /tmp/test

# Config via environment
export TCE_API_URL=http://localhost:11434/v1
export TCE_MODEL=qwen2.5-coder:7b
export TCE_API_KEY=sk-...
tce
```

### Inline commands

```
/help     → Show help
/exit     → Exit
/clear    → Clear terminal
/project  → Show detected project info
/git      → Show current branch and git status
```

### Resume a session

```bash
tce --resume .tce/sessions/2026-07-03_143022.json
```

### Configuration file (`.tce.yaml`)

```yaml
model: qwen2.5-coder:7b
agent: build
verbose: true
```

See [Custom Tools](#custom-tools) and [MCP Servers](#mcp-servers) for extended configuration.

## Agent Types

| Type | Allowed Tools | Use Case |
|------|--------------|----------|
| **build** | All tools | General development |
| **plan** | read, grep, glob, bash (with confirmation), task | Architecture & planning |
| **explore** | read, grep, glob (read-only) | Code exploration |

## Tools

| Tool | Description | Fields |
|------|-------------|--------|
| `read` | Read a file with line numbers | `file_path`, `offset`, `limit` |
| `grep` | Regex search across files | `pattern`, `path`, `include` |
| `glob` | Find files by glob pattern | `pattern`, `path` |
| `write` | Create or overwrite a file | `file_path`, `content` |
| `edit` | Exact string replacement | `file_path`, `old_string`, `new_string` |
| `bash` | Run a shell command | `command`, `timeout`, `workdir` |
| `ask` | Ask the user a question | `question` |
| `task` | Launch a sub-agent (explore/general) | `description`, `subagent_type` |
| `search` | Search the web via DuckDuckGo | `query` |
| `commit` | Stage files and create a git commit | `message` |
| `review` | Show staged/unstaged git diff | *(none)* |
| `undo` | Revert the last file edit | *(none)* |

All tools accept multiple field name aliases for resilience:
- `file_path` also accepts `path`, `file`, `filename`
- `pattern` also accepts `search`, `query`, `find`, `text`, `regex`
- `content` also accepts `text`, `data`, `code`
- `command` also accepts `cmd`, `run`

## Model Profiles

TCE auto-adjusts settings based on the model name.

| Model | Max Context | Max Turns | Temperature | Minimal Mode |
|-------|-------------|-----------|-------------|--------------|
| qwen3.5:0.8b | 10K | 15 | 0.2 | yes |
| qwen3.5:2b | 16K | 20 | 0.15 | yes |
| qwen3.5:4b+ | 20K | 25 | 0.2 | no |
| default | 24K | 25 | 0.2 | no |

Flags like `--minimal` and `--context-size` override the profile.

### Usage by model profile

**Small models (0.8B–2B)** — use `--minimal` for best results:
```bash
tce --model qwen3.5:0.8b --minimal --context-size 8192
```

**Medium models (4B–7B)** — good balance:
```bash
tce --model qwen3.5:4b
```

**Large models via API** — turn off minimal mode for full capability:
```bash
tce --base-url https://api.openai.com/v1 --api-key sk-... --model gpt-4o --agent build
```

**Local large models (7B+)** — use Ollama:
```bash
tce --model qwen2.5-coder:7b --context-size 16384
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `TCE_API_URL` | `http://localhost:11434/v1` | API base URL |
| `TCE_API_KEY` | `ollama` | API key |
| `TCE_MODEL` | `qwen3.5:4b` | Model name |
| `TCE_STREAM` | `true` | Enable/disable streaming (`false` to disable) |

## Custom Tools

Define custom shell commands as tools in `.tce.yaml`:

```yaml
tools:
  format:
    command: clang-format -i {{path}}
    description: Format C source files with clang-format
  lint:
    command: golangci-lint run ./...
    description: Run Go linter on the project
```

Custom tools use `{{param}}` syntax for arguments. The LLM will pass parameters
automatically based on the template variables found in the command string.

## MCP Servers

Connect external MCP (Model Context Protocol) servers as tools by adding them
to `.tce.yaml`:

```yaml
mcp_servers:
  filesystem:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "."]
```

MCP tools are registered with a `mcp_` prefix (e.g., `mcp_read_file`).

## How Tool Calling Works

TCE supports two mechanisms for tool calling:

### 1. Native Tool Calling (OpenAI format)

If the LLM backend supports `tools` in the chat request and returns `tool_calls` in the response, TCE uses them directly. This works with:
- OpenAI API (GPT-4o, GPT-4o-mini)
- Ollama ≥ 0.5.x (Llama 3.2, Mistral, Qwen 2.5 7B+)
- vLLM
- Groq, Together, Fireworks, etc.

### 2. JSON Extraction Fallback

If the LLM does **not** return native `tool_calls` but generates JSON inline in its text response (common with smaller models like Qwen 2.5-Coder 1.5B/7B), TCE parses it:

```
Model response: {"name": "read", "arguments": {"file_path": "main.c"}}
                       ↓
TCE extracts → ToolCall{Name: "read", Arguments: `{"file_path": "main.c"}`}
                       ↓
TCE executes read("main.c") → returns file content
                       ↓
Model receives result and continues
```

This works with any model that outputs `{"name":"...","arguments":{...}}` in its text, even without native function calling support. The extraction logic:
1. Searches for `{"name"` (handling whitespace/newlines between `{` and `"name"`)
2. Also checks inside markdown ` ```json ` blocks
3. Validates that `name` is a non-empty string and `arguments` is a valid object
4. Rejects false positives via strict JSON unmarshalling

## Architecture

```
main.go                     → CLI + TUI entry point
internal/
├── agent/
│   ├── loop.go             → Run(), RunSubAgent(), stall detection
│   └── prompt.go           → BuildSystemPrompt / Minimal prompt
├── compactor/
│   └── compactor.go        → Context compaction, pruning, truncation
├── config/
│   └── config.go           → .tce.yaml parser (model, agent, tools, mcp)
├── llm/
│   ├── client.go           → Chat(), ChatStream(), SSE parser, retry, JSON extraction fallback
│   └── model.go            → Model profiles (context, temperature, mode)
├── mcp/
│   ├── mcp.go              → MCP client (JSON-RPC 2.0 over stdio)
│   └── tool.go             → ToolAdapter: wraps MCP tools as Tool interface
├── tools/
│   ├── parse.go            → firstOf, tryFixJSON, fuzzyMatch
│   ├── registry.go         → Tool interface, Registry, Execute()
│   ├── external.go         → ExternalTool (shell commands as tools)
│   ├── git.go              → CommitTool, ReviewTool
│   ├── ignore.go           → IgnoreMatcher gitignore-style
│   ├── diff.go             → unifiedDiff helper
│   ├── undo.go             → PushUndo/PopUndo/ClearUndo + UndoTool
│   ├── read.go             → ReadTool, GrepTool, GlobTool
│   ├── write.go            → WriteTool, EditTool
│   ├── bash.go             → BashTool
│   ├── ask.go              → AskTool
│   ├── task.go             → TaskTool (sub-agent)
│   └── search.go           → SearchTool (DuckDuckGo)
├── permission/             → Access control rules
├── project/                → Language/framework detection
│   └── detect.go           → Makefile, go.mod, Cargo.toml, etc.
├── tui/                    → Bubbletea terminal UI
├── session/                → Session persistence
└── util/                   → Truncate, helpers
```

### Key Design Decisions

**Client-only.** TCE is purely a client. It does not bundle or spawn any LLM backend. You bring your own backend (Ollama, OpenAI, vLLM, etc.) via the OpenAI-compatible API.

**No Python dependency.** The Python backend (`serve.py`) was removed in favor of the simpler client-only approach. Use Ollama or any OpenAI-compatible API.

**Universal parsing.** All tools accept multiple field name aliases and have structured error feedback (`Error: ...\nExpected: ...\nYou sent: ...`). Malformed JSON is fixed via `tryFixJSON()`.

**Circuit breaker.** If a tool fails 3 consecutive times, it is automatically disabled for the rest of the session to prevent infinite error loops.

**Per-turn context.** Ctrl+C creates a new context per turn — it cancels the current LLM request without killing the entire session.

**Sub-agents.** The `task` tool can launch child agents (`explore`, `general`) with a fresh context, up to 3 levels of nesting.

## Scenarios

### Scenario 1: Implementing a Feature

```
> implement a function that reads a CSV file

🔧 read({"file_path": "main.c"})
✅ read reads main.c → shows existing code

🔧 write({"file_path": "main.c", "content": "..."})
✅ write creates main.c with CSV parsing

🔧 bash({"command": "gcc -o test main.c && ./test"})
✅ bash compiles and runs → success
```

### Scenario 2: Debugging

```
> my program crashes, find the bug

🔧 bash({"command": "gcc -g -o test main.c 2>&1"})
✅ bash → compilation errors shown

🔧 read({"file_path": "main.c"})
✅ read shows code

🔧 edit({"file_path": "main.c", "old_string": "...", "new_string": "..."})
✅ edit fixes the bug

🔧 bash({"command": "gcc -o test main.c && ./test"})
✅ bash → runs without errors
```

### Scenario 3: Researching a Library

```
> search for how to use signals in C

🔧 query("C signal handling tutorial sa_handler sigaction")
✅ search returns top results

🔧 read({"file_path": "main.c"})
✅ read checks existing code

🔧 write({"file_path": "main.c", "content": "..."})
✅ write implements signal handlers
```

### Scenario 4: Exploring a New Codebase

```
> what does this project do?

🔧 glob({"pattern": "*.c"})
✅ glob → lists all C files

🔧 read({"file_path": "main.c"})
✅ read → shows entry point

🔧 grep({"pattern": "TODO|FIXME"})
✅ grep → finds all TODOs
```

### Scenario 5: Git workflow

```
> review my changes

🔧 review({})
✅ review → shows staged/unstaged diff

> commit this

🔧 commit({"message": "feat: add CSV parsing"})
✅ commit → 3 files changed, 42 insertions(+)
```

## FAQ

### Why does TCE show `❌ Error: code requests require tool use`?

This happens when:
1. You ask to write code (e.g., "implement X", "write a Y")
2. The model responds with text/code directly instead of using `write` or `edit` tools

The model must use `write`/`edit` tools to produce code, not output it inline. If you see this error with a legitimate `search` or `read` request, the `isCodeRequest` heuristic may have matched a false positive.

### The model keeps calling `search` with `pattern` instead of `query`

The `search` tool expects `query` as the search term. Some models confuse the `search` schema with the `grep` schema (which uses `pattern`). The structured error message shows the correct format.

### Why does TCE use DuckDuckGo for search?

DuckDuckGo Lite (`lite.duckduckgo.com`) requires no API key and returns clean HTML results. It's the simplest zero-config web search option for a CLI tool.

### My model doesn't support tool calling. Can I still use TCE?

Yes! TCE has a JSON extraction fallback for models that output `{"name":"toolname","arguments":{...}}` as text. This works with Qwen 2.5-Coder 1.5B/7B and similar models that understand tool schemas but don't implement native function calling.

### Can I use TCE with OpenAI / Groq / other APIs?

Yes, any OpenAI-compatible API works:

```bash
# OpenAI
tce --base-url https://api.openai.com/v1 --api-key sk-... --model gpt-4o-mini

# Groq (free)
tce --base-url https://api.groq.com/openai/v1 --api-key gsk-... --model llama3.3-70b

# Local Ollama
tce --base-url http://localhost:11434/v1 --model qwen2.5-coder:7b
```

### Why was `serve.py` removed?

TCE is a client-only agent. The Python backend (`serve.py`) bundled a model runner, which added Python/llama-cpp dependencies and was out of scope for a Go CLI tool. Use Ollama or any OpenAI-compatible API instead.
