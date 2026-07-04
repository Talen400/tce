# ADR-001: Client-Only Architecture

**Date:** 2026-07-03  
**Status:** Accepted  

## Context

TCE needed to decide how to handle LLM inference. Two approaches were considered:
1. Bundle a model runner (llama.cpp, vLLM) inside the tool — "batteries included"
2. Be a pure client that connects to any OpenAI-compatible API — "bring your own backend"

The original prototype (`serve.py`) bundled a Python/llama-cpp backend, which worked but added significant complexity: Python dependency, GPU/CPU config, model downloads, version conflicts between the Go client and Python server.

## Decision

TCE is **client-only**. It does not bundle, download, or spawn any LLM backend. Users bring their own backend via the OpenAI-compatible chat completions API.

The only requirement is `POST /v1/chat/completions` with standard OpenAI request/response format, which works with:
- Ollama (local)
- OpenAI API (remote)
- Groq, Together, Fireworks, vLLM (remote)
- Any proxy implementing the OpenAI spec

## Consequences

**Positive:**
- No Python/llama-cpp dependencies in the Go codebase
- Users choose their own backend, provider, and model
- TCE can target any model scale (0.5B local → 400B API)
- Simpler distribution: single Go binary

**Negative:**
- Users must set up and configure a separate backend
- Latency depends entirely on the backend, not under TCE's control
- No built-in model download or management

## Alternatives Considered

- **Bundle llama.cpp via CGo**: Rejected — adds cross-compilation pain and doubles binary size
- **Keep Python backend**: Rejected — Python dependency out of scope for a Go CLI tool
- **Use Ollama Go bindings**: Rejected — locks users into Ollama; the OpenAI API is more universal
