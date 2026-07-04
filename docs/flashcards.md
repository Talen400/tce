# Flashcards — O que eu não sabia antes e sei agora

> Resumo de aprendizado por fase do roadmap. Formato pergunta-resposta para revisão rápida.

---

## Fase 0 — Fundação

**P:** Como estruturar um projeto Go com múltiplos pacotes internos?  
**R:** `internal/` protege pacotes de importação externa. Cada pacote tem uma responsabilidade clara: `agent/` (orquestração), `tools/` (ferramentas), `llm/` (cliente API), `tui/` (interface).

**P:** Como configurar GitHub Actions para Go?  
**R:** `actions/setup-go@v5` + `go build ./...` + `go vet ./...` + `golangci-lint` + `go test ./...`. Lint precisa de ação separada `golangci/golangci-lint-action@v6`.

**P:** Como implementar gitignore-style pattern matching em Go?  
**R:** `filepath.Match` para padrões simples, `strings.HasPrefix` para `**/` patterns. Compilar padrões uma vez e reusar contra cada path.

---

## Fase 1 — Segurança

**P:** Como evitar que o LLM execute comandos destrutivos?  
**R:** Blocklist de regex na BashTool: `rm -rf /`, `dd`, `mkfs`, fork bombs, `curl|bash`. Qualquer match rejeita o comando antes de executar.

**P:** Como dar feedback visual antes de uma edição?  
**R:** `unifiedDiff` gera diff colorido (linhas `+` verde, `-` vermelho). A EditTool mostra o diff e pede confirmação antes de aplicar.

**P:** Como implementar undo de edições?  
**R:** Buffer circular com stack de `[]change` (old/new content por path). PushAntes de editar, Pop para reverter. Pacote-level `sync.Mutex` para acesso concorrente.

---

## Fase 2 — UX

**P:** Como fazer streaming de tokens do LLM no terminal?  
**R:** SSE parser que lê `data: {chunk}\n\n` eventos do HTTP response. Cada chunk é enviado como mensagem Bubble Tea: `program.Send(tokenMsg(token))`.

**P:** Como salvar e restaurar uma sessão?  
**R:** JSON com array de mensagens + metadados (model, turns, tokens). `session.Save()` serializa, `session.Load()` desserializa e injeta no agente via `SetMessages()`.

**P:** Como implementar um parser YAML mínimo sem dependência?  
**R:** Scanner linha a linha com indentação. `key: value` para config simples. Blocos indentados para seções complexas (`tools:`, `mcp_servers:`). Aceita arrays JSON inline (`args: ["a", "b"]`).

---

## Fase 3 — Git

**P:** Como gerar mensagem de commit automaticamente?  
**R:** Se CommitTool é chamada sem `message`, ela retorna `git diff --stat` + diff preview. O LLM vê as mudanças e escreve a mensagem na próxima chamada.

**P:** Como criar um branch antes da sessão?  
**R:** Flag `--branch` executa `git checkout -b <name>` no diretório do projeto antes de iniciar o loop do agente. Erro se não for um repositório git.

---

## Fase 4 — Extensibilidade

**P:** Como permitir tools customizadas sem recompilar?  
**R:** ExternalTool com template `{{param}}`. Definida em `.tce.yaml`, parseia `{{var}}` do comando, gera schema JSON automaticamente, executa via `sh -c`.

**P:** Como conectar servidores MCP como tools?  
**R:** Cliente JSON-RPC 2.0 sobre stdio com Content-Length framing. `tools/list` descobre tools do servidor. `ToolAdapter` wrappeia cada uma como `Tool` do TCE com prefixo `mcp_`.

**P:** Como parsear YAML com blocos aninhados sem biblioteca?  
**R:** Função `parseBlock` que conta indentação (`countIndent`), coleta linhas de bloco (`collectBlock`), e chama handlers recursivamente. Suporta mapas aninhados para config de tools/MCP.

---

## Fase 5 — Distribuição

**P:** Como fazer cross-compile para múltiplas plataformas no CI?  
**R:** Loop sobre `GOOS`/`GOARCH` (linux/darwin/windows × amd64/arm64). `go build -ldflags="-s -w -X main.version=$TAG"`. Checksums com `sha256sum`. Release com `softprops/action-gh-release`.

**P:** Como distribuir um binário Go via curl|bash?  
**R:** Script que detecta OS/arch (`uname -s`/`uname -m`), normaliza nomes (x86_64 → amd64), baixa da GitHub release, instala em `~/.local/bin` ou `$BINDIR`.

**P:** Como fazer uma fórmula Homebrew?  
**R:** Arquivo `.rb` com `url` apontando para a release do GitHub, `sha256` do checksum, `def install` copiando para `bin/`, `test do` verificando `--version`. Colocar em `github.com/user/homebrew-tce/Formula/tce.rb`.

---

## Fase 6 — Qualidade

**P:** Como testar o loop do agente sem um LLM real?  
**R:** `httptest.NewServer` com handler que implementa o formato OpenAI (choices[].message.tool_calls para não-streaming, choices[].delta.tool_calls para streaming). Cliente LLM aponta para o mock.

**P:** Como medir latência de cada tool isoladamente?  
**R:** Benchmark Go com `mockLLM` que retorna tool calls pré-definidas. `b.ResetTimer()` antes do loop, `b.N` iterações do `agent.Run()`. Mede o tempo total incluindo tool execution + overhead do loop.

**P:** Como implementar telemetria opt-in?  
**R:** `Reporter` com flag `enabled`. Quando ativo, escreve JSON Lines em `.tce/errors.jsonl`. Cada linha é um `ErrorReport` com timestamp, tool, erro, turno, modelo. Thread-safe com mutex.

---

## Fase 7 — Documentação

**P:** Como documentar decisões técnicas de forma rastreável?  
**R:** ADRs (Architecture Decision Records) em `docs/decisions/ADR-NNN-title.md`. Cada ADR tem: Context, Decision, Consequences, Alternatives Considered. Numeração sequencial, versionado no git.

**P:** Como criar diagramas versionados que renderizam no GitHub?  
**R:** Mermaid syntax dentro de blocos ```mermaid no Markdown. GitHub renderiza nativamente. Diagramas de fluxo, sequência, e arquitetura.

**P:** Como garantir que commits contem uma história coerente?  
**R:** Conventional Commits (`feat:`, `fix:`, `docs:`, etc.) + corpo do commit explicando o que foi feito e por quê. Mensagens descritivas o suficiente para servirem como changelog.
