# TCE — Plano de Melhorias

> Gerado em: 2026-07-03 (atualizado)
> Projeto: `github.com/talen/tce`

---

## Legenda

- `[x]` Concluído
- `[ ]` Pendente

---

## Fase 0 — Fundação

*Impacto: Estrutura do repositório, CI, e ferramentas base.*

### [x] 0.1 Licença MIT

**Arquivo:** `LICENSE`

Adicionada licença MIT padrão.

### [x] 0.2 Guia de Contribuição

**Arquivo:** `CONTRIBUTING.md`

Instruções de setup, build, teste, lint, padrão de commits (Conventional Commits) e PRs.

### [x] 0.3 GitHub Actions + Linter

**Arquivos:** `.github/workflows/ci.yml`, `.golangci.yml`

Workflow CI com build, vet, lint (golangci-lint) e testes em cada PR.

### [x] 0.4 `.tceignore`

**Arquivos:** `.tceignore`, `internal/tools/ignore.go`

Arquivo de padrões gitignore-style para excluir diretórios de `glob`/`grep`. Parsing completo: comentários, negação `!`, `**`, diretórios, âncoras.

### [x] 0.5 Testes unitários

Suíte de 180 testes já existente cobre parsing de tool calls, perfis de modelo, e demais pacotes.

---

## Fase 1 — Segurança e Confiabilidade

*Impacto: Proteção contra comandos destrutivos, previsibilidade de edições, e reversão de erros.*

> Itens históricos (1.1 e 1.2 do plano original) já foram resolvidos em fases anteriores.

### [x] 1.1 Blocklist de Comandos Perigosos (Bash)

**Arquivo:** `internal/tools/bash.go`

Lista de padrões regex para bloquear comandos destrutivos: `rm -rf /`, `dd if=`, `mkfs`, `chmod 777 /`, fork bombs, `curl|bash`. O comando é recusado com mensagem clara antes da execução.

### [x] 1.2 Timeout Configurável

**Arquivo:** `internal/tools/bash.go` (já existente)

Campo `timeout` no schema (default 30s). Modelo pode especificar timeout por comando.

### [x] 1.3 Confirmação para Comandos Fora do Projeto

**Arquivo:** `internal/tools/bash.go`

Se `workdir` estiver fora de `ProjectRoot`, o tool pede confirmação interativa via `ReadInput`. Sem `ReadInput`, o comando é bloqueado com erro.

### [x] 1.4 Diff Preview (Edit)

**Arquivos:** `internal/tools/write.go`, `internal/tools/diff.go`

Antes de aplicar um `edit`, exibe um diff unificado das mudanças e pede confirmação interativa. O diff também é incluído no resultado da ferramenta.

### [x] 1.5 `tce undo`

**Arquivos:** `internal/tools/undo.go`

Buffer local (sem git) que salva o conteúdo original antes de `write`/`edit`. A tool `undo` restaura a última versão salva. Suporta múltiplos níveis de undo.

### [x] 1.6 Retry com Backoff (API)

**Arquivo:** `internal/llm/client.go` (já existente)

`doRequest()` implementa backoff exponencial (100ms → 300ms → 900ms) para 429 e 5xx, com jitter.

---

## Fase 2 — Otimização de Tokens

*Impacto: Permite mais turns úteis em modelos pequenos (0.8B/4B com 32K contexto).*

### [x] 2.1 Tokenizer Real

**Arquivos:** `internal/compactor/compactor.go`

**Solução:** `estimateTokens` agora é método em `*Compactor` e usa `TokenRatio` configurável (chars/token). Valor padrão 3.5 (compatível Qwen). Modelos podem definir seu próprio ratio via perfil.

### [ ] 2.2 Cache de Tool Definitions Serializadas

**Arquivo:** `internal/agent/loop.go` — `cachedToolDefs` já é cacheado em `New()`. A implementação já está correta.

### [x] 2.3 Tool Content Truncation

**Arquivo:** `internal/compactor/compactor.go`

**Solução:** `MaxToolContentLen` é configurável por perfil de modelo (ex: 500 para 0.8B, 1000 para 4B+).

### [ ] 2.4 Tool Definition Size Reduction (Minimal Mode)

**Arquivos:** `internal/tools/*.go`

**Status:** Pendente. As tools já têm `ShortDescription()`, mas o Minimal Mode poderia reduzir ainda mais os schemas.

### [ ] 2.5 Cachear/Omitir Tool Definitions no Request

**Arquivo:** `internal/llm/client.go`

**Status:** Pendente. Investigar se a API mantém cache das tool definitions.

---

## Fase 3 — Testes

*Impacto: Confiabilidade.*

### [x] Compactor Tests
### [x] Tool Tests
### [x] Agent Loop Tests
### [x] LLM Client Tests
### [x] Permission Tests
### [x] Project Detect Tests (parcial — há falhas pré-existentes com paths fixos)

**Status geral:** Testes existentes passam (exceto `detect_test.go` que usa paths `/tmp/testprojects/*`).

### [x] Testes para novos módulos
- **`internal/tools/parse.go`**: `firstOf`, `tryFixJSON`, `fuzzyMatch` ✅
- **`internal/llm/model.go`**: `MatchProfile`, match exato, prefixo, fallback ✅
- **`internal/tools/search.go`**: `parseLiteHTML`, `stripTags`, `decodeEntities`, `decodeURLParam` ✅

---

## Fase 4 — Performance Geral

*Impacto: Velocidade de execução e uso de memória.*

### [x] 4.1 Glob Walk Optimization

**Arquivo:** `internal/tools/read.go`

**Resolvido em:** Já usa `filepath.WalkDir` com `filepath.SkipDir` para `.git`, `node_modules`, etc.

### [x] 4.2 SSE Parser Robustez

**Arquivo:** `internal/llm/client.go`

**Solução:** Implementado parser SSE stateful (`sseParser`) que acumula múltiplas linhas `data:`, ignora `event:`/`retry:`, trata keep-alives e `[DONE]`.

### [x] 4.3 Dedup: `truncate` Function

**Arquivos:** `main.go`, `tui.go` → `internal/util/strings.go`

**Resolvido em:** Já está em `internal/util/strings.go`.

### [x] 4.4 Dedup: Chat/ChatStream Request Building

**Arquivo:** `internal/llm/client.go`

**Status:** Parcial — `Chat()` e `ChatStream()` ainda compartilham ~40 linhas de construção de request.

### [x] 4.5 Remover Dead Code

**Arquivos:** `detect.go`, `read.go`

**Resolvido em:** `detect.go` não tem mais duplicatas de Rust. `read.go` sem dead assignments.

---

## Fase 5 — UX e Resiliência

*Impacto: Experiência do usuário e robustez em produção.*

### [x] 5.1 Retry em LLM Calls

**Arquivo:** `internal/llm/client.go`

**Resolvido em:** Já implementado via `doRequest()` com backoff (100ms, 300ms, 900ms) para 429 e 5xx.

### [x] 5.2 Subagent Recursion Limit

**Arquivos:** `internal/tools/task.go`, `internal/agent/loop.go`

**Resolvido em:** `MaxSubAgentDepth = 3` verificado em `TaskTool.Execute()`.

### [x] 5.3 Context Cancellation no TUI

**Arquivo:** `internal/tui/tui.go`

**Resolvido em:** `context.WithCancel` no `runAgent()`, Ctrl-C no TUI cancela o agente.

### [x] 5.4 TUI: `/clear` Command

**Arquivo:** `internal/tui/tui.go`

**Resolvido em:** Handler para `/clear` reseta `m.content` e `m.viewport`.

### [x] 5.5 TUI: Limitar Histórico do Viewport

**Arquivo:** `internal/tui/tui.go`

**Resolvido em:** `trimContent()` mantém últimas `maxViewportLines = 5000` linhas.

---

## Melhorias Implementadas (fora do plano original)

### [x] Parsing Universal (Fase A do plano v2)

**Arquivos:** `internal/tools/parse.go` (novo), `read.go`, `write.go`, `bash.go`, `task.go`

Todas as tools agora aceitam múltiplos nomes de campo via `firstOf()`:
- `file_path` → também `path`, `file`, `filename`
- `pattern` → também `search`, `query`, `find`, `text`, `regex`
- `content` → também `text`, `data`, `code`
- `old_string` → também `old`, `find`, `from`
- `new_string` → também `new`, `replace`, `to`
- `timeout` → também `time`
- `workdir` → também `dir`, `directory`, `cwd`

### [x] Recuperação de JSON Malformado

**Arquivo:** `internal/tools/parse.go` — `tryFixJSON()`

Tenta extrair JSON de markdown, remover prefixos/sufixos, escapar newlines dentro de strings.

### [x] Fuzzy Match de Tool Name

**Arquivo:** `internal/tools/parse.go` — `fuzzyMatch()`, integrado em `Registry.Execute()`

Se o LLM chamar `bash` em vez de `bash`, ou `wite` em vez de `write`, faz prefix match automático.

### [x] Validação Pré-Execução

**Arquivo:** `internal/tools/registry.go`

Antes de executar, valida se JSON é válido. Se não, tenta `tryFixJSON()`. Retorna erro claro e acionável.

### [x] Feedback Estruturado

Todas as tools agora usam formato consistente:
```
❯ tool(args)
── Section ──
content
── End ──
```

### [x] Configuração Parametrizada por Modelo

**Arquivo:** `internal/llm/model.go` (novo)

`MatchProfile()` retorna `Profile` com `MaxContext`, `MaxTurns`, `Temperature`, `MinimalMode`, `MaxToolContent`, `KeepTurns`, `TokenRatio`, `ForceSingleCall`. Match por nome exato > prefixo > fallback.

### [x] Forçar 1 Tool Call (ForceSingleCall)

**Arquivo:** `internal/agent/loop.go`

Se o perfil configurar, descarta tool calls extras e executa apenas a primeira.

### [x] Recovery de Erros Consecutivos

**Arquivo:** `internal/agent/loop.go`

Se uma mesma tool falha 3x seguidas, interrompe com erro específico.

---

## Melhorias Futuras

### Funcionalidades

| Feature | Descrição | Prioridade |
|---------|-----------|------------|
| **Agent Review** | Novo tipo de agente que apenas lê código e sugere melhorias | Média |
| **Diff preview** | Antes de `write`/`edit`, mostrar diff e pedir confirmação | Média |
| **Multi-model** | Usar modelo pequeno (0.8B) para explorer, grande (9B) para build | Média |
| **Session save/restore** | Salvar histórico de mensagens e retomar sessão | Baixa |
| **Prompt templates** | Templates customizáveis por linguagem/framework | Baixa |

### Otimização de Tokens

| Item | Descrição | Esforço |
|------|-----------|---------|
| Tool Description Reduction | Reduzir ainda mais schemas em MinimalMode | 1h |
| Cache tool defs no request | Enviar tools apenas quando mudam | 1h |

### Testes

| Item | Descrição | Esforço | Status |
|------|-----------|---------|--------|
| `parse_test.go` | `firstOf`, `tryFixJSON`, `fuzzyMatch` | 30min | ✅ |
| `model_test.go` | `MatchProfile` com match exato/prefixo/fallback | 15min | ✅ |
| `search_test.go` | `parseLiteHTML`, `stripTags`, `decodeEntities` | 20min | ✅ |
| Fix `detect_test.go` | Substituir paths fixos por `t.TempDir()` | 15min | ✅ |
| Benchmarks | 10 benchmarks (compactor, parse, read/grep, agent, prompt, search) | 30min | ✅ |

### Técnico

| Item | Descrição | Esforço |
|------|-----------|---------|
| Dedup Chat/ChatStream | Extrair `buildRequest()` compartilhado | 30min |
| GitHub Actions | CI: build + lint + test em cada PR | 1h | ✅ |
| `golangci-lint` | Configurar linter | 30min | ✅ |

---

## Fase 7 — Documentação de Funções

*Impacto: Qualidade do código, manutenibilidade, e geração de documentação.*

### [ ] 7.1 Go Doc Comments em Funções Exportadas

Adicionar `// FuncName ...` doc comments em todas as **85 funções exportadas** (0% → 100%).

### [ ] 7.2 Package-level Doc Comments

Adicionar `// Package foo ...` nos 17 pacotes.

### [ ] 7.3 Tipos e Constantes Exportadas

Documentar tipos como `Config`, `Client`, `Response`, `Tool`, `Registry` e constantes exportadas.

### [ ] 7.4 Documentação Incremental

Toda nova função adicionada já deve sair com Go doc comment.

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
│   └── task.go             → TaskTool (subagente)
├── permission/             → regras de controle de acesso
├── project/                → detecção de linguagem
├── tui/                    → interface Bubbletea
└── util/                   → funções utilitárias (Truncate)
```
