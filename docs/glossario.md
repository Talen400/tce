# Glossário TCE

> Conceitos técnicos aprendidos durante o desenvolvimento do TCE. 2-3 linhas por termo.

---

## Go

### Goroutine
Thread leve gerenciada pelo runtime do Go. Usada no TCE para rodar o agente em background enquanto o TUI (Bubble Tea) mantém a interface responsiva. Comunicação via channels.

### Channel
Mecanismo de comunicação entre goroutines. No TCE, usado para enviar mensagens do agente (rodando em goroutine) para o TUI: `program.Send(tokenMsg(token))`.

### Interface
Tipo abstrato do Go que define um conjunto de métodos. No TCE, a interface `Tool` (5 métodos: `Name`, `Description`, `ShortDescription`, `Schema`, `Execute`) permite que qualquer implementação seja registrada no `Registry`.

### Struct Embedding
Go não tem herança, mas permite composição via embedding. Usado extensivamente no TCE para compor comportamentos sem hierarquia de classes.

### `json.RawMessage`
Tipo que mantém o JSON como bytes crus sem parsear. Usado nas tools do TCE para receber input da LLM sem definir structs fixas, permitindo field aliasing e recuperação de erros.

### `defer`
Garante que uma função execute ao final do escopo atual. Usado no TCE para fechar arquivos, liberar locks, e garantir cleanup mesmo em caso de panic.

---

## LLM / API

### Tool Calling (Function Calling)
Mecanismo onde o LLM retorna chamadas de função estruturadas (`{"name":"read","arguments":{"file_path":"x"}}`) em vez de texto. O agente executa e retorna o resultado. OpenAI native format vs JSON extraction fallback.

### SSE (Server-Sent Events)
Protocolo HTTP para streaming de eventos. Usado pelo TCE para receber tokens do LLM em tempo real (`data: {chunk}\n\n`). Parseado pelo `sseParser` que lida com linhas `data:` múltiplas, keep-alives, e `[DONE]`.

### Context Window
Número máximo de tokens que o modelo pode processar em uma única requisição. O TCE usa o `Compactor` para gerenciar: quando a janela está cheia, mensagens antigas são comprimidas ou descartadas (KeepTurns).

### Token
Unidade básica de texto para LLMs (~0.75 palavra em inglês). O TCE estima tokens por caracteres (`len(text) / 3.5`) — impreciso mas sem dependência de tokenizador real.

### Streaming
Resposta do LLM enviada token por token via SSE, em vez de esperar o texto completo. No TCE, `ChatStream` vs `Chat` — o primeiro mostra cada token conforme chega, o segundo espera a resposta inteira.

### Retry com Backoff
Estratégia de repetir chamadas de API falhas com espera crescente: 100ms → 300ms → 900ms. O TCE implementa no `doRequest` do LLM client, com jitter aleatório para evitar thundering herd.

---

## Arquitetura do Agente

### Tool Registry
Mapa central de tools registradas (`map[string]Tool`). O `Registry` gerencia: registro, listagem, execução, contagem de erros, desabilitação automática (após 3 falhas consecutivas), fuzzy matching de nomes.

### Sub-Agent
Agente filho com contexto fresco, lançado pela tool `task`. Limitado a 3 níveis de profundidade (`MaxSubAgentDepth`). Usa o mesmo LLM e tool registry do agente pai, mas com contexto independente.

### Compactor
Sistema de compressão de contexto que remove mensagens antigas do histórico quando o limite de tokens é atingido. Mantém as últimas N turns (KeepTurns) e descarta as mais antigas. Usa `Truncate` para encurtar resultados de tools longos.

### Permission Checker
Sistema de controle de acesso baseado em regras. Cada agente (build/plan/explore) tem um conjunto de regras que definem quais tools pode chamar. Suporta Allow, Deny, e Ask (confirmação do usuário).

### Stall Detection
O agente detecta quando tools consecutivas retornam resultados vazios ou erros. Após 3 "empty results" ou após erros consecutivos em qualquer tool, a sessão é interrompida com mensagem clara.

### JSON Extraction Fallback
Quando o LLM não suporta tool calling nativo, o TCE escaneia o texto de resposta procurando `{"name":"tool","arguments":{...}}`. Suporta markdown blocks, XML tags, single quotes, e brace matching.

### tryFixJSON
Função que tenta recuperar JSON malformado: extrai de dentro de markdown, converte aspas simples em duplas, fecha chaves faltando. Reduz falsos positivos em modelos que quase acertam o formato.

### Fuzzy Match
Quando o LLM chama uma tool com nome ligeiramente errado (e.g., `searc` em vez de `search`), o TCE tenta encontrar o match mais próximo por prefixo. Evita erros fatais por typos do modelo.

---

## MCP (Model Context Protocol)

### JSON-RPC 2.0
Protocolo de chamada remota leve, usado pelo MCP para comunicação entre cliente e servidor. Mensagens têm `jsonrpc`, `id`, `method`, `params` / `result`, `error`. O TCE implementa no `internal/mcp/`.

### Content-Length Framing
Formato de transporte do MCP sobre stdio: cada mensagem é precedida por `Content-Length: N\r\n\r\n` seguido de N bytes de JSON. O TCE usa `bufio.Reader` para ler as mensagens corretamente.

### Stdio Transport
Conexão MCP via subprocesso: o TCE spawna o servidor, conecta stdin/stdout, e troca mensagens JSON-RPC. Alternativa ao transporte SSE (HTTP), que não está implementado.

### ToolAdapter
Wrapper que implementa a interface `Tool` do TCE em cima de uma tool MCP. Converte chamadas `Execute()` em `tools/call` RPC e mapeia o schema MCP (`inputSchema`) para `Schema()`.

---

## TCE Específico

### IgnoreMatcher
Implementação de gitignore-style pattern matching para excluir diretórios das tools `glob` e `grep`. Lê `.tceignore` e compila padrões (suporta `*`, `?`, `[...]`, `**`).

### Blocklist (BashTool)
Lista de padrões regex de comandos proibidos na tool `bash`: `rm -rf /`, `dd`, `mkfs`, `chmod 777 /`, fork bombs, `curl|bash`. Impede execução de comandos destrutivos mesmo que o LLM tente.

### Unified Diff
Formato de diff usado na tool `edit` para mostrar preview antes de aplicar mudanças. Gerado pela função `unifiedDiff` que compara old e new string linha a linha.

### ExternalTool
Tool que executa um comando shell com template `{{param}}`. Definida em `.tce.yaml`, auto-descobre parâmetros do template, gera schema JSON automaticamente.

### Session Persistence
Salva histórico completo da sessão em `.tce/sessions/YYYY-MM-DD_HHMMSS.json`. Permite `--resume` para continuar exatamente de onde parou.

### ForceSingleCall
Configuração de perfil de modelo que limita o LLM a 1 tool call por turno. Necessário para modelos menores que não suportam chamadas paralelas corretamente.
