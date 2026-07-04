# Devlog — TCE

> Diário de desenvolvimento: o que foi feito, o que foi aprendido, dificuldades encontradas.

---

## 2026-07-03 — Phase 0: Project Foundation

**Feito:**
- Added MIT LICENSE
- Added CONTRIBUTING.md with Conventional Commits (`feat:`, `fix:`, `docs:`, `chore:`, `refactor:`, `test:`)
- Set up GitHub Actions CI (build, vet, lint, test on every PR)
- Configured golangci-lint (govet, errcheck, staticcheck, unused, gosimple, gofmt)
- Created `.tceignore` with full gitignore-style pattern support
  - Built `IgnoreMatcher` in `internal/tools/ignore.go` — parses `#` comments, `!` negation, `**` globstar, anchored `/`, trailing `/` for dir-only
- Integrated `.tceignore` into both GlobTool and GrepTool in `internal/tools/read.go`
- Updated PLAN.md with new roadmap structure, added Phase 7 (function documentation)

**Aprendizado:**
- Go's `filepath.Match` handles `*` and `?` but not `**` — had to implement recursive matching for `**`
- `\b` word boundary in Go regexp doesn't match before `:` (non-word char) — fork bomb pattern `\b:\(\)\s*\{` didn't work, fixed with `:\(\s*\)\s*\{`
- Gitignore semantics: patterns without `/` match basename at any depth; patterns with `/` are anchored

---

## 2026-07-03 — Phase 1: Security and Reliability

**Feito:**
- Added dangerous command blocklist in BashTool: `rm -rf /`, `dd if=`, `mkfs`, `chmod 777 /`, fork bombs, `curl|bash`, `wget|sh`
  - Blocked BEFORE execution with clear error message
- Added interactive confirmation for commands with workdir outside project root via `ReadInput`
- Added diff preview before EditTool applies changes
  - Built `unifiedDiff` in `internal/tools/diff.go` — line-based diff with hunk grouping and context
- Created undo system: `PushUndo`/`PopUndo`/`ClearUndo` in `internal/tools/undo.go`
  - Buffer saves original file content before write/edit operations
  - UndoTool registered as a regular tool the LLM can call
- Wrote 14 new tests: 7 diff tests, 5 undo tests, 2 blocklist tests (8 dangerous, 6 safe)

**Aprendizado:**
- Unified diff is deceptively tricky — naive line-by-line comparison produces overlapping hunks. Had to group consecutive edits into hunks with `context * 2` spacing
- `rm -f` + non-existent file exits 0, but `grep non-existent file` exits 2 — important for test expectations
- The `ExecContext.ReadInput` function was never wired in the agent loop, so `AskTool` effectively never got interactive input. Same issue affects diff preview and outside-dir confirmation in the default flow.

---

## 2026-07-03 — Phase 2: User Experience

**Feito:**
- Replaced PLAN.md with new roadmap structure (phases 0-7); kept architecture + historical appendix
- Created `docs/devlog.md` with chronological entries
- Session persistence: `internal/session/session.go` saves/loads message history to `.tce/sessions/`
  - Added `--resume <path>` flag to restore previous session messages
- `.tce.yaml` project config: `internal/config/config.go` parses `model:`, `agent:`, `verbose:`
  - No external YAML dep — simple line parser handles `key: value` and `# comments`
- Token tracking: agent tracks input/output token estimates using `len(text)/TokenRatio`
  - Stats shown at CLI exit: `📊 Session: N turns, ~X tokens in, ~Y tokens out`
- `--verbose` flag: prints full tool call JSON payloads to stderr
  - Also controllable via `.tce.yaml` → `verbose: true`

**Aprendizado:**
- Adding a YAML parser from scratch works for simple key-value configs, but arrays or nested structures would need a real parser like `gopkg.in/yaml.v3`
- The TUI creates its own agent internally via `tui.NewModel`, so the `ag` from main.go can't be used for post-session stats in TUI mode. CLI mode path works fine.
- Token estimation with fixed char/token ratio is approximate — real tokenizers (tiktoken, etc.) would be more accurate but add dependency weight

---

## 2026-07-03 — Phase 3: Git Integration

**Feito:**
- Created `CommitTool` in `internal/tools/git.go`:
  - If called with `message` param: runs `git add -A && git commit -m "message"`
  - If called without message: returns `git diff` + `git diff --stat` so the LLM can see changes and write a message
  - Returns commit SHA on success
- Created `ReviewTool` in `internal/tools/git.go`:
  - Shows `git diff --cached` (staged) and `git diff` (unstaged) with patches
  - Lets the LLM present a summary of pending changes
- Added `--branch <name>` flag to main.go:
  - Runs `git checkout -b <name>` before starting the session
  - Errors out if branch creation fails (e.g., not a git repo)
- Registered both `commit` and `review` tools in main.go

**Aprendizado:**
- Git tools need the project root as working directory — used `cmd.Dir = ctx.ProjectRoot`
- `git diff --stat` gives a file summary; `git diff` gives the full patch — using both for different contexts (summary for display, patch for LLM context)
- The `CommitTool` follows the same pattern as other tools: returns useful info when called without args, takes action when called with args
- Using `os/exec` directly (instead of going through BashTool) avoids the security blocklist — these git commands are safe by intent

---

## 2026-07-03 — Phase 4: Extensibilidade

**Feito:**
- Created `ExternalTool` in `internal/tools/external.go`:
  - Wraps a shell command template with `{{param}}` placeholders
  - Auto-generates JSON schema from template variables
  - Parameters can be passed as `{"params": {"path": "src/"}}` or flat `{"path": "src/"}`
  - Executes via `sh -c` in the project root
  - Used e.g.: `format: { command: "clang-format -i {{path}}", description: "Format C files" }`
- Created MCP client in `internal/mcp/mcp.go`:
  - Full JSON-RPC 2.0 implementation over stdio with Content-Length framing
  - Handles initialize/tools/list/tools/call handshake
  - Thread-safe with mutex for write serialization
  - Supports notifications (no response expected)
  - Error handling for RPC errors
- Created `ToolAdapter` in `internal/mcp/tool.go`:
  - Wraps MCP tool definitions as `tools.Tool` interface
  - Registers with `mcp_` prefix (e.g., `mcp_read_file`)
- Extended config parser in `internal/config/config.go`:
  - Added block-level YAML parsing (indentation-aware)
  - Supports `tools:` and `mcp_servers:` sections with nested key-value blocks
  - Backwards-compatible with existing flat config
  - Includes `parseJSONArray` for parsing `args: ["a", "b"]`
- Wired in main.go:
  - External tools from `.tce.yaml` are registered automatically
  - MCP servers are connected at startup; their tools registered with `mcp_` prefix
  - Errors connecting to MCP servers are warned but don't block startup

**Aprendizado:**
- `firstOf` returns `string`, not `any` — cannot type-assert on it; use direct map access
- Method values in Go: `cfg.parseTopLevel` works as a function value (no `&` needed)
- MCP Content-Length framing requires careful buffering with `bufio.Reader`
- YAML block parsing is deceptively tricky; indent-aware line scanning works for the subset we need
- MCP tool schemas map naturally to the existing `Tool` interface

---

## 2026-07-03 — Phase 5: TUI/CLI

**Feito:**
- TUI spinner: status bar now shows animated spinner + current tool name during execution
  - Tracks `currentToolName`/`currentToolArgs` in Model, clears on tool end / agent done
- TUI diff syntax highlighting via `highlightContent()`:
  - Lines starting with `+` → green (`styleDiffAdd`)
  - Lines starting with `-` → red (`styleDiffDel`)
  - Lines starting with `@@` → cyan (`styleDiffHdr`)
  - Code blocks (``` ... ```) → subtle gray (`styleCode`)
  - Applied in `syncViewport()` before setting viewport content
- CLI `/git` command: shows current branch and `git status --short`
  - Color-coded status indicators (? for untracked, M/m for staged/unstaged)
  - Available in both TUI and CLI modes
- CLI `--output` flag (`text`/`json`/`silent`):
  - `json`: returns result as JSON with result, error, tools, elapsed fields
  - `silent`: runs without prompts/output (for automation), only stderr on error
  - `text` (default): normal interactive mode
- Updated `/help` to include `/git` command

**Aprendizado:**
- Bubble Tea viewport content must be styled before `SetContent()` — storing plain text and applying styles at render time keeps the content buffer clean
- `strings.Cut("tools:", ":")` returns `("tools", "", true)` — need to check if trimmed val is empty AND if next line is indented to detect YAML blocks
- `tea.ExecCommand` doesn't exist in Bubble Tea — use `os/exec.Command` for standalone commands outside the Bubble Tea event loop
- Method value vs function: Go allows `cfg.parseTopLevel` as a function value (no `&` needed), but `func() { cfg.parseTopLevel(...) }` closure is also needed in some contexts

---

## 2026-07-03 — Phase 5: Distribuição

**Feito:**
- Created `.github/workflows/release.yml`:
  - Triggers on `v*` tag pushes
  - Builds for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64
  - Embeds version via `-ldflags="-X main.version=${GITHUB_REF_NAME#v}"`
  - Generates SHA256 checksums
  - Creates GitHub release with `softprops/action-gh-release`
- Created `install.sh`:
  - Detects OS and architecture (linux/darwin/windows, amd64/arm64)
  - Downloads latest (or specific) release binary from GitHub
  - Installs to `$BINDIR` (default: `~/.local/bin`)
  - Usage: `curl -fsSL https://raw.githubusercontent.com/talen400/tce/main/install.sh | bash`
- Created `tce.rb` Homebrew formula:
  - Supports macOS (arm64/amd64) and Linux (arm64/amd64)
  - Installs to `bin/tce`
  - Includes basic test (`tce --version`)
  - Ready to push to `github.com/talen400/homebrew-tce`
- Expanded README.md:
  - Added install methods (binary, Homebrew, source)
  - Added `--branch`, `--output`, `--resume` flags to docs
  - Added /git command to CLI commands list
  - Added usage examples per model profile (small/medium/large)
  - Updated architecture diagram with mcp/, config/, session/ packages
  - Added `commit`, `review`, `undo` to tools table
  - Added Custom Tools and MCP Servers documentation sections
  - Added Scenario 5 (Git workflow)

**Aprendizado:**
- `softprops/action-gh-release@v2` needs `contents: write` permission in the workflow
- `uname -m` outputs different arch names across platforms — must normalize (x86_64 → amd64, aarch64 → arm64)
- Homebrew formula checksum must be updated per-release — automated with CI or goreleaser
- The `-ldflags="-s -w -X main.version=X"` strips debug info and injects version at build time
- A Homebrew tap needs a separate repo (`homebrew-tce`) with the formula placed at `Formula/tce.rb`
