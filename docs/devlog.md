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
