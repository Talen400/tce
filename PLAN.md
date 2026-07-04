# TCE — Roadmap

> Projeto: `github.com/talen/tce`
> Atualizado em: 2026-07-03

---

## Legenda

- `[x]` Concluído
- `[ ]` Pendente

---

## Fase 0 — Fundação (curto prazo)

- [x] Adicionar `LICENSE` (MIT)
- [x] Adicionar `CONTRIBUTING.md` com instruções de setup, build e padrão de commits (Conventional Commits)
- [x] Criar suíte de testes unitários mínima — 180+ testes cobrindo parsing, perfis de modelo, tools, agente, compactor, LLM client
- [x] Configurar GitHub Actions: build + test + `go vet` + lint (`golangci-lint`) em cada PR
- [x] Adicionar `.tceignore` (padrão gitignore-style) para excluir diretórios de `glob`/`grep`

## Fase 1 — Segurança e confiabilidade

- [x] Blocklist de comandos perigosos na tool `bash` (regex para padrões destrutivos conhecidos)
- [x] Timeout configurável por execução de comando
- [x] Confirmação interativa para comandos fora do diretório do projeto
- [x] Diff preview (unified diff) antes de aplicar mudanças via tool `edit`
- [x] Comando `tce undo` para reverter a última edição de arquivo (buffer local, sem depender de git)
- [x] Retry com backoff exponencial para chamadas de API (timeout, rate limit, 5xx)

## Fase 2 — Experiência do usuário

- [x] Streaming de respostas do modelo no terminal (token a token)
- [x] Persistência de sessão (`.tce/sessions/*.json`) com `tce --resume`
- [x] Arquivo de configuração por projeto (`.tce.yaml`): modelo padrão, agente padrão, verbose
- [x] Contagem de tokens e custo estimado por sessão (exibido ao final da execução)
- [x] Modo verboso/debug (`--verbose`) mostrando payloads de tool calls

## Fase 3 — Integração com git

- [x] Geração automática de mensagens de commit a partir do diff das mudanças aplicadas
- [x] Comando para revisar (`tce review`) mudanças pendentes antes de commitar
- [x] Flag para criar branch automaticamente antes de uma sessão de edição

## Fase 4 — Extensibilidade

- [x] Abstrair interface `Tool` para permitir tools nativas e tools externas
- [x] Cliente MCP (stdio/SSE) para conectar servidores MCP externos como tools
- [x] Sistema de plugins simples (Go plugins ou subprocessos) para tools customizadas por usuário

## Fase 5 — Distribuição

- [x] Publicar releases no GitHub com binários pré-compilados (Linux/macOS/Windows, amd64/arm64)
- [x] Ajustar `install.sh` para baixar binário da release ao invés de exigir Go instalado
- [x] Publicar fórmula Homebrew (`brew install talen400/tce/tce`)
- [x] Página de documentação (GitHub Pages ou README expandido) com exemplos de uso por perfil de modelo

## Fase 6 — Qualidade contínua

- [ ] Testes de integração end-to-end (simulando sessão real com mock de API)
- [ ] Benchmark de latência por provedor (Ollama local vs APIs remotas)
- [ ] Telemetria opcional (opt-in) de erros para priorizar correções

## Fase 7 — Documentação e acompanhamento de estudo

- [ ] Criar `docs/decisions/` com ADRs (Architecture Decision Records) — um arquivo curto por decisão técnica relevante (contexto, decisão, alternativas consideradas, consequências)
- [ ] Criar `docs/devlog.md` — diário de desenvolvimento com entradas cronológicas curtas: o que foi feito, o que foi aprendido, dificuldades encontradas
- [ ] Criar `docs/glossario.md` — conceitos técnicos aprendidos durante o projeto (Go, arquitetura de agentes, LLM tooling, MCP, etc.), 2-3 linhas por termo
- [ ] Padronizar comentários de código para explicar o "porquê" das decisões não óbvias, referenciando o ADR correspondente quando existir
- [ ] Adicionar diagramas versionados em Mermaid dentro dos `.md` (fluxo de tool calling, arquitetura geral, ciclo de vida de uma sessão)
- [ ] Ao final de cada fase do roadmap, escrever um resumo "o que eu não sabia antes e sei agora" (formato flashcard, revisão rápida)
- [ ] Manter mensagens de commit descritivas o suficiente para servirem como changelog de aprendizado (não só "fix bug")

---

## Apêndice — Histórico de Melhorias

### Melhorias da fase de otimização (antiga Fase 2)
- [x] Tokenizer Real: `estimateTokens` usa `TokenRatio` configurável (padrão 3.5)
- [x] Tool Content Truncation: `MaxToolContentLen` configurável por perfil

### Melhorias de teste (antiga Fase 3)
- [x] Compactor Tests
- [x] Tool Tests
- [x] Agent Loop Tests
- [x] LLM Client Tests
- [x] Permission Tests
- [x] Benchmarks (10 benchmarks em compactor, parse, read, agent, prompt, search)

### Melhorias de performance (antiga Fase 4)
- [x] Glob Walk Optimization (WalkDir + SkipDir)
- [x] SSE Parser stateful (múltiplas linhas `data:`, keep-alives, `[DONE]`)
- [x] Dedup `truncate` → `internal/util/strings.go`
- [x] Dead code removido

### Melhorias de UX (antiga Fase 5)
- [x] Retry em LLM Calls (backoff 100ms → 300ms → 900ms)
- [x] Subagent Recursion Limit (MaxSubAgentDepth = 3)
- [x] Context Cancellation no TUI (Ctrl+C)
- [x] TUI: `/clear` Command
- [x] TUI: Limitar Histórico do Viewport (5000 linhas)
- [x] TUI: Spinner + tool name na status bar durante tool calls
- [x] TUI: Syntax highlighting em diffs (+ verde, - vermelho, @@ azul)
- [x] TUI+CLI: `/git` command (branch atual + status)
- [x] CLI: `--output text|json|silent` para controlar formato da resposta

### Melhorias Implementadas (fora do plano original)

- [x] Parsing Universal — todas as tools aceitam múltiplos nomes de campo via `firstOf()`
- [x] Recuperação de JSON Malformado — `tryFixJSON()` extrai JSON de markdown, converte single quotes
- [x] Fuzzy Match de Tool Name — prefix match automático se o LLM errar o nome
- [x] Validação Pré-Execução — valida JSON antes de executar
- [x] Feedback Estruturado — formato consistente `❯ tool(args)\n── Section ──\n...\n── End ──`
- [x] Configuração Parametrizada por Modelo — `MatchProfile()` por nome exato > prefixo > fallback
- [x] Forçar 1 Tool Call (ForceSingleCall) — para perfis que não suportam paralelo
- [x] Recovery de Erros Consecutivos — tool desabilitada após N falhas seguidas

---

## Arquitetura

```
main.go                     → CLI + TUI entrypoint
internal/
├── agent/                  → loop principal, subagentes, prompts
│   ├── loop.go             → Run(), RunSubAgent(), stall detection
│   └── prompt.go           → BuildSystemPrompt / Minimal
├── compactor/              → compressão de contexto
│   └── compactor.go        → Compact(), prune, truncate, estimate
├── llm/                    → cliente HTTP OpenAI-compatível
│   ├── client.go           → Chat(), ChatStream(), SSE parser, retry
│   └── model.go            → perfis de configuração por modelo
├── tools/                  → implementações de ferramentas
│   ├── parse.go            → firstOf, tryFixJSON, fuzzyMatch
│   ├── registry.go         → Tool interface, Registry, Execute()
│   ├── ignore.go           → IgnoreMatcher gitignore-style
│   ├── diff.go             → unifiedDiff helper
│   ├── undo.go             → PushUndo/PopUndo/ClearUndo + UndoTool
│   ├── read.go             → ReadTool, GrepTool, GlobTool
│   ├── write.go            → WriteTool, EditTool (diff preview)
│   ├── bash.go             → BashTool (blocklist + dir check)
│   ├── ask.go              → AskTool
│   ├── task.go             → TaskTool (subagente)
│   └── search.go           → SearchTool (DuckDuckGo)
├── permission/             → regras de controle de acesso
├── project/                → detecção de linguagem
│   └── detect.go           → Makefile, go.mod, Cargo.toml, etc.
├── tui/                    → interface Bubbletea
└── util/                   → funções utilitárias (Truncate)
```
