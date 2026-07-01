# Backend Python Local

O TCE inclui um backend Python (`serve.py`) que roda modelos GGUF localmente via `llama-cpp-python`, expondo uma API compatível com OpenAI.

Ideal para ambientes sem Ollama ou com versão desatualizada (ex: clusters 42).

## Requisitos

- Python 3.10+

## Setup rápido (Makefile)

```bash
make venv      # Cria .venv e instala dependências
make serve     # Sobe o servidor (já usa a venv)
make run       # Compila TCE + sobe com --serve
```

## Uso manual

### 1. Criar venv (opcional, recomendado)

```bash
python3 -m venv .venv
.venv/bin/pip install llama-cpp-python
```

No cluster 42 (CPU), instala com:

```bash
.venv/bin/pip install llama-cpp-python --extra-index-url https://abetlen.github.io/llama-cpp-python/whl/cpu
```

### 2. Iniciar o servidor

```bash
# Com venv
.venv/bin/python serve.py

# Sem venv
python3 serve.py

# Modelo mais leve (0.5B, ~400MB)
python3 serve.py --model qwen2.5-coder-0.5b
```

### 3. Conectar o TCE

```bash
./tce --base-url http://127.0.0.1:8001/v1 --api-key not-needed
```

### 4. Auto-spawn (usa venv se existir)

```bash
make run
# ou
./tce --serve
```

## Modelos disponíveis

### 1. Iniciar o servidor

```bash
# Modelo padrão (Qwen2.5-Coder-1.5B, ~1GB)
python serve.py

# Modelo mais leve (0.5B, ~400MB)
python serve.py --model qwen2.5-coder-0.5b

# Caminho customizado para GGUF
python serve.py --model /caminho/para/modelo.gguf

# Customizar porta e contexto
python serve.py --port 8001 --n-ctx 4096
```

Na primeira execução, o script baixa automaticamente o modelo da HuggingFace para `~/.cache/tce/models/`.

### 2. Conectar o TCE

```bash
./tce --base-url http://127.0.0.1:8001/v1 --api-key not-needed
```

### 3. Auto-spawn (recomendado)

```bash
./tce --serve
```

Isso inicia o servidor Python automaticamente, espera ficar pronto, e conecta.

## Modelos disponíveis

| Chave | Modelo | Tamanho | RAM |
|-------|--------|---------|-----|
| `qwen2.5-coder-1.5b` | Qwen2.5-Coder-1.5B (Q4_K_M) | ~1GB | ~2GB |
| `deepseek-coder-1.3b` | DeepSeek-Coder-1.3B (Q4_K_M) | ~900MB | ~1.8GB |
| `qwen2.5-coder-0.5b` | Qwen2.5-Coder-0.5B (Q4_K_M) | ~400MB | ~1GB |

## Argumentos do serve.py

| Flag | Default | Descrição |
|------|---------|-----------|
| `--model` | `qwen2.5-coder-1.5b` | Modelo (chave do registry ou caminho .gguf) |
| `--port` | `8001` | Porta do servidor |
| `--host` | `127.0.0.1` | Endereço para bind |
| `--n-ctx` | `8192` | Tamanho do contexto em tokens |
| `--verbose` | `false` | Log detalhado |

## API

### `POST /v1/chat/completions`

Endpoint OpenAI-compatível. Suporta:
- `stream: true/false`
- `tools` (function calling)
- `temperature`, `max_tokens`

### `GET /health`

Health check (usado pelo `--serve` do TCE).

### `GET /v1/models`

Lista o modelo carregado.

## Arquitetura

```
┌──────────┐     HTTP (OpenAI format)     ┌──────────────┐
│  TCE     │ ──────────────────────────▶  │  serve.py    │
│  (Go)    │ ◀──────────────────────────  │  (Python)    │
└──────────┘     SSE / JSON               └──────┬───────┘
                                                  │ llama.cpp
                                          ┌───────┴───────┐
                                          │  Modelo GGUF  │
                                          └───────────────┘
```

## Resolução de problemas

**"llama-cpp-python not installed"**: instala com `pip install llama-cpp-python`.

**Erro de compilação**: usa o wheel CPU: `pip install llama-cpp-python --extra-index-url https://abetlen.github.io/llama-cpp-python/whl/cpu`.

**Servidor não inicia**: verifica se a porta 8001 está livre: `lsof -i :8001`.

**Modelo não baixa**: verifica conexão com HuggingFace. Podes baixar manualmente e passar o caminho com `--model`.
