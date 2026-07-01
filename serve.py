#!/usr/bin/env python3
"""TCE Python Backend - OpenAI-compatible local LLM server.

Serves an OpenAI-compatible API for small coding models (GGUF).
Auto-downloads model from HuggingFace if not present.

Usage:
    python serve.py                                    # default: Qwen2.5-Coder-1.5B
    python serve.py --model qwen2.5-coder-0.5b          # smaller model
    python serve.py --model path/to/model.gguf          # custom GGUF
    python serve.py --port 8001 --n-ctx 4096            # custom settings

Then in another terminal:
    ./tce --base-url http://127.0.0.1:8001/v1 --api-key not-needed
"""

import argparse
import json
import logging
import os
import socket
import sys
import time
import urllib.request
from http.server import ThreadingHTTPServer, BaseHTTPRequestHandler
from pathlib import Path
from typing import Optional

MODEL_REGISTRY = {
    "qwen2.5-coder-1.5b": {
        "url": "https://huggingface.co/Qwen/Qwen2.5-Coder-1.5B-Instruct-GGUF/resolve/main/qwen2.5-coder-1.5b-instruct-q4_k_m.gguf",
        "filename": "qwen2.5-coder-1.5b-instruct-q4_k_m.gguf",
        "chat_format": "chatml",
    },
    "deepseek-coder-1.3b": {
        "url": "https://huggingface.co/bartowski/DeepSeek-Coder-1.3B-Instruct-GGUF/resolve/main/DeepSeek-Coder-1.3B-Instruct-Q4_K_M.gguf",
        "filename": "deepseek-coder-1.3b-instruct-q4_k_m.gguf",
        "chat_format": "deepseek",
    },
    "qwen2.5-coder-0.5b": {
        "url": "https://huggingface.co/Qwen/Qwen2.5-Coder-0.5B-Instruct-GGUF/resolve/main/qwen2.5-coder-0.5b-instruct-q4_k_m.gguf",
        "filename": "qwen2.5-coder-0.5b-instruct-q4_k_m.gguf",
        "chat_format": "chatml",
    },
}

CACHE_DIR = Path.home() / ".cache" / "tce" / "models"

def port_available(host: str, port: int) -> bool:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.settimeout(1)
        return s.connect_ex((host, port)) != 0

def resolve_model(model_arg: str) -> tuple[str, str]:
    if model_arg in MODEL_REGISTRY:
        e = MODEL_REGISTRY[model_arg]
        return e["filename"], e["chat_format"]
    p = Path(model_arg)
    if p.exists() or model_arg.endswith(".gguf"):
        return model_arg, "chatml"
    logging.warning("Unknown model %r, using as-is", model_arg)
    return model_arg, "chatml"

def resolve_local_path(model_arg: str, model_path: str) -> str:
    CACHE_DIR.mkdir(parents=True, exist_ok=True)
    cached = str(CACHE_DIR / Path(model_path).name)
    if model_arg in MODEL_REGISTRY:
        return cached
    if Path(model_arg).exists():
        return model_arg
    if Path(cached).exists():
        return cached
    return model_arg

def download_model(url: str, dest: str):
    if Path(dest).exists():
        return
    logging.info("Downloading %s ...", url)
    logging.info("Destination: %s", dest)
    tmp = dest + ".tmp"
    try:
        urllib.request.urlretrieve(url, tmp, _progress_hook)
        os.rename(tmp, dest)
        print()
        logging.info("Download complete")
    except Exception as e:
        Path(tmp).unlink(missing_ok=True)
        raise RuntimeError(f"Download failed: {e}") from e

def _progress_hook(block: int, size: int, total: int):
    if total > 0:
        pct = block * size * 100 / total
        if pct % 5 == 0:
            print(f"\r  Download: {pct:.0f}%", end="", flush=True)

def convert_messages(messages: list) -> list:
    converted = []
    for msg in messages:
        m = {"role": msg.get("role", "user")}
        content = msg.get("content")
        if content is not None:
            m["content"] = content
        if msg.get("tool_calls"):
            m["tool_calls"] = msg["tool_calls"]
        if msg.get("tool_call_id"):
            m["tool_call_id"] = msg["tool_call_id"]
        converted.append(m)
    return converted

def normalize_tools(tools: list) -> list:
    normalized = []
    for t in tools:
        if "function" in t and "type" in t:
            normalized.append(t)
        elif "function" not in t and ("name" in t or "description" in t):
            normalized.append({"type": "function", "function": t})
        else:
            normalized.append(t)
    return normalized

def build_chunk(content: str = "", tool_calls: Optional[list] = None,
                finish: Optional[str] = None) -> str:
    delta = {}
    if content:
        delta["content"] = content
    if tool_calls:
        delta["tool_calls"] = tool_calls
    chunk = {
        "id": f"chatcmpl-{int(time.time())}",
        "object": "chat.completion.chunk",
        "created": int(time.time()),
        "model": "",
        "choices": [{"index": 0, "delta": delta, "finish_reason": finish}],
    }
    return f"data: {json.dumps(chunk)}\n\n"

def build_nonstream(content: str = "", tool_calls: Optional[list] = None,
                    finish: str = "stop", model_name: str = "",
                    usage: Optional[dict] = None) -> dict:
    msg = {"role": "assistant"}
    if content:
        msg["content"] = content
    if tool_calls:
        msg["tool_calls"] = tool_calls
    return {
        "id": f"chatcmpl-{int(time.time())}",
        "object": "chat.completion",
        "created": int(time.time()),
        "model": model_name,
        "choices": [{"index": 0, "message": msg, "finish_reason": finish}],
        "usage": usage or {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0},
    }

class Handler(BaseHTTPRequestHandler):
    server_version = "TCE-Serve/0.1"

    def log_message(self, fmt, *args):
        if getattr(self.server, "verbose", False):
            super().log_message(fmt, *args)

    def _json(self, data: dict, status: int = 200):
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Access-Control-Allow-Origin", "*")
        self.end_headers()
        self.wfile.write(json.dumps(data).encode())

    def _error(self, status: int, msg: str):
        self._json({"error": {"message": msg, "type": "error"}}, status)

    def _sse_start(self):
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Cache-Control", "no-cache")
        self.send_header("Access-Control-Allow-Origin", "*")
        self.end_headers()

    def _sse_chunk(self, content: str = "", tool_calls=None, finish=None):
        self.wfile.write(build_chunk(content, tool_calls, finish).encode())
        self.wfile.flush()

    def _sse_done(self):
        self.wfile.write(b"data: [DONE]\n\n")
        self.wfile.flush()

    def do_OPTIONS(self):
        self.send_response(200)
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
        self.send_header("Access-Control-Allow-Headers", "Content-Type, Authorization")
        self.end_headers()

    def do_GET(self):
        if self.path == "/health":
            llm = getattr(self.server, "llm", None)
            status = "loaded" if llm is not None else "loading"
            self._json({"status": status, "model": self.server.model_name})
        elif self.path == "/v1/models":
            self._json({
                "object": "list",
                "data": [{"id": self.server.model_name, "object": "model",
                          "created": int(time.time()), "owned_by": "tce"}],
            })
        else:
            self._error(404, "Not Found")

    def do_POST(self):
        if self.path not in ("/v1/chat/completions", "/chat/completions"):
            return self._error(404, "Not Found")

        llm = getattr(self.server, "llm", None)
        if llm is None:
            return self._error(503, "Model not loaded yet")

        try:
            length = int(self.headers.get("Content-Length", 0))
            body = json.loads(self.rfile.read(length) if length else b"{}")
        except Exception as e:
            return self._error(400, f"Invalid JSON: {e}")

        stream = body.get("stream", False)
        messages = body.get("messages", [])
        tools = body.get("tools", None)
        temp = body.get("temperature", 0.0)
        max_tok = body.get("max_tokens", 4096)

        llm_msgs = convert_messages(messages)
        kwargs = dict(temperature=temp, max_tokens=max_tok)
        if tools:
            kwargs["tools"] = normalize_tools(tools)
            kwargs["tool_choice"] = "auto"

        try:
            if stream:
                self._handle_stream(llm, llm_msgs, kwargs)
            else:
                self._handle_nonstream(llm, llm_msgs, kwargs)
        except Exception as e:
            logging.exception("Chat completion failed")
            if stream:
                self._sse_chunk(finish="error")
                self._sse_done()
            else:
                self._error(500, str(e))

    def _handle_nonstream(self, llm, messages: list, kwargs: dict):
        result = llm.create_chat_completion(messages=messages, **kwargs)
        choice = result.get("choices", [{}])[0]
        msg = choice.get("message", {})
        resp = build_nonstream(
            content=msg.get("content", ""),
            tool_calls=msg.get("tool_calls", None),
            finish=choice.get("finish_reason", "stop"),
            model_name=self.server.model_name,
            usage=result.get("usage"),
        )
        self._json(resp)

    def _handle_stream(self, llm, messages: list, kwargs: dict):
        self._sse_start()
        for chunk in llm.create_chat_completion(messages=messages, stream=True, **kwargs):
            choice = chunk.get("choices", [{}])[0]
            delta = choice.get("delta", {})
            content = delta.get("content", "")
            tool_calls = delta.get("tool_calls", None)
            finish = choice.get("finish_reason", None)
            if not content and not tool_calls and finish is None:
                continue
            self._sse_chunk(content, tool_calls, finish)
        self._sse_done()

def parse_args():
    p = argparse.ArgumentParser(description="TCE Python Backend - Local LLM Server")
    p.add_argument("--model", default="qwen2.5-coder-1.5b",
                   help="Model key or GGUF path")
    p.add_argument("--port", type=int, default=8001)
    p.add_argument("--host", default="127.0.0.1")
    p.add_argument("--n-ctx", type=int, default=8192)
    p.add_argument("--verbose", action="store_true")
    return p.parse_args()

def main():
    args = parse_args()
    logging.basicConfig(
        level=logging.DEBUG if args.verbose else logging.INFO,
        format="%(asctime)s [%(levelname)s] %(message)s",
    )

    if not port_available(args.host, args.port):
        print(f"Port {args.port} is already in use on {args.host}.")
        print(f"Check with: lsof -i :{args.port}")
        print("Use --port to change the port.")
        sys.exit(1)

    try:
        import llama_cpp
    except ImportError:
        print("llama-cpp-python is required.")
        print("Install: pip install llama-cpp-python")
        sys.exit(1)

    model_path, chat_format = resolve_model(args.model)
    local_path = resolve_local_path(args.model, model_path)

    if not Path(local_path).exists():
        if args.model in MODEL_REGISTRY:
            download_model(MODEL_REGISTRY[args.model]["url"], local_path)
        elif Path(args.model).exists():
            local_path = args.model
        else:
            print(f"Model not found: {args.model}")
            print(f"Available: {', '.join(MODEL_REGISTRY.keys())}")
            sys.exit(1)
    elif args.model not in MODEL_REGISTRY:
        local_path = args.model

    logging.info("Loading model: %s", local_path)
    logging.info("Chat format: %s  Context: %d tokens", chat_format, args.n_ctx)

    try:
        llm = llama_cpp.Llama(
            model_path=local_path,
            n_ctx=args.n_ctx,
            chat_format=chat_format,
            verbose=args.verbose,
            n_gpu_layers=0,
        )
    except Exception as e:
        print(f"Failed to load model: {e}")
        sys.exit(1)

    model_name = Path(local_path).stem
    server = ThreadingHTTPServer((args.host, args.port), Handler)
    server.llm = llm
    server.model_name = model_name
    server.verbose = args.verbose

    print(f"\n TCE Python Backend running on http://{args.host}:{args.port}")
    print(f"   Model: {model_name}")
    print(f"   Format: {chat_format}  Context: {args.n_ctx}")
    print(f"\n   Connect with TCE:")
    print(f"   ./tce --base-url http://{args.host}:{args.port}/v1 --api-key not-needed")
    print()

    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nShutting down...")
        server.shutdown()

if __name__ == "__main__":
    main()
