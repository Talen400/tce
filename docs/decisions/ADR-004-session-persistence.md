# ADR-004: Session Persistence Format

**Date:** 2026-07-03  
**Status:** Accepted  

## Context

TCE sessions span multiple turns (user prompts → LLM responses → tool calls → results). Users may want to resume interrupted sessions, review past conversations, or share session history for debugging. The persistence format needs to store the full message history faithfully.

## Decision

Sessions are saved as JSON files under `.tce/sessions/` with the filename format `YYYY-MM-DD_HHMMSS.json`.

The format stores:
- **Model name** — which model was used
- **Turn count** — for display/reference
- **Token estimates** — input and output token counts
- **Full message history** — system prompt, user messages, assistant responses (with tool calls), tool results

Key design choices:
- **Entire history is saved**, not just a summary — enabling exact resume
- **JSON format** — trivial to parse, inspect, and debug by hand
- **`.tce/` directory** — respects `.tceignore` patterns by default (if `*` is in `.tceignore`, sessions are excluded from tools)
- **One file per session** — no append-only logs or databases; simple copy/delete/share

The `--resume` flag loads a session file, sets the message history on the agent, and continues from where it left off (system prompt is preserved from the saved session).

## Consequences

**Positive:**
- Exact resume with no loss of context
- Easy to debug: open the JSON file to see exactly what the LLM saw
- No external dependencies (no database, no binary format)
- git-friendly for `.tce/` (can be added to `.gitignore` separately)

**Negative:**
- Can grow large for long sessions (50+ turns with large tool results)
- Token estimates are approximate (character-count-based, not a real tokenizer)
- No compression or pruning of saved sessions

## Alternatives Considered

- **SQLite**: Rejected — adds CGo dependency and complexity for a simple append/read workload
- **Binary encoding (gob/protobuf)**: Rejected — not human-readable, harder to debug
- **Only save session metadata, not messages**: Rejected — makes resume impossible without re-executing tools
