# TCE — Terminal Coding Assistant

Assistente de codificação via terminal que se conecta a LLMs (Ollama/OpenAI-compatível) com tool calling para ler, escrever, editar, buscar arquivos, executar comandos e lançar subagentes.

## Requisitos

- Go 1.24+
- Ollama (ou qualquer API OpenAI-compatível) rodando em `http://localhost:11434`
- Python 3.10+ (opcional, para o backend Python local)

## Instalação

```bash
go install github.com/talen/tce@latest
```

Ou a partir do repositório:

```bash
git clone <repo> && cd tce
go build -o tce .
```

## Uso Básico

```bash
# TUI (padrão)
./tce

# Modo CLI
./tce --cli

# Especificar modelo
./tce --model qwen3.5:4b

# Modo minimal (recomendado para modelos <4B)
./tce --minimal

# Tipo de agente
./tce --agent build     # Acesso total (padrão)
./tce --agent plan      # Leitura + bash com confirmação
./tce --agent explore   # Read-only

# Diretório do projeto
./tce --dir /meu/projeto

# API customizada
./tce --base-url http://localhost:11434/v1 --api-key ollama

# Backend Python local (auto-download + servidor)
./tce --serve
```

### Variáveis de Ambiente

| Variável | Descrição |
|----------|-----------|
| `TCE_API_URL` | Base URL da API |
| `TCE_API_KEY` | API Key |
| `TCE_MODEL` | Nome do modelo |

## Backend Python Local

Para ambientes sem Ollama (ex: clusters 42), o TCE inclui um servidor Python que roda modelos GGUF localmente:

```bash
# Instalar dependência
pip install llama-cpp-python requests

# Iniciar servidor manual
python serve.py

# Auto-spawn (TCE inicia o servidor sozinho)
./tce --serve

# Conectar a um servidor já rodando
./tce --base-url http://127.0.0.1:8001/v1 --api-key not-needed
```

Na primeira execução, o modelo é baixado automaticamente da HuggingFace (~1GB).

Detalhes em [`BACKEND.md`](BACKEND.md).

## Comandos (CLI)

```
/help     → Mostra ajuda
/exit     → Sai do programa
/clear    → Limpa o terminal
/project  → Mostra info do projeto detectado
```

## Tipos de Agente

| Tipo | Tools Permitidas | Uso |
|------|-----------------|-----|
| **build** | Todas | Desenvolvimento geral |
| **plan** | read, grep, glob, task + bash (pergunta) | Planejamento |
| **explore** | read, grep, glob (read-only) | Exploração |

## Tools Disponíveis

| Tool | Descrição |
|------|-----------|
| `read` | Lê arquivo com line numbers |
| `write` | Cria/sobrescreve arquivo |
| `edit` | Find/replace exato em arquivo |
| `grep` | Busca regex em arquivos |
| `glob` | Encontra arquivos por padrão |
| `bash` | Executa comandos shell |
| `ask` | Pergunta ao usuário |
| `task` | Lança subagente (explore/general) |

## Perfis de Modelo

O TCE ajusta configurações automaticamente conforme o modelo:

| Modelo | Contexto | Turns | Temperature | Minimal |
|--------|----------|-------|-------------|---------|
| qwen3.5:0.8b | 10K | 15 | 0.1 | sim |
| qwen3.5:2b | 16K | 20 | 0.15 | sim |
| qwen3.5:4b+ | 20K | 25 | 0.2 | não |
| outros | 24K | 25 | 0.2 | não |

Flags como `--minimal` e `--context-size` sobrescrevem o perfil.
