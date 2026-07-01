import json
import os
import sys
import tempfile
from pathlib import Path
from unittest.mock import MagicMock, patch, PropertyMock

sys.path.insert(0, str(Path(__file__).parent.parent))

from serve import (
    MODEL_REGISTRY,
    resolve_model,
    resolve_local_path,
    port_available,
    convert_messages,
    build_chunk,
    build_nonstream,
    normalize_tools,
)


class TestResolveModel:
    def test_registry_key(self):
        name, fmt = resolve_model("qwen2.5-coder-1.5b")
        assert name == "qwen2.5-coder-1.5b-instruct-q4_k_m.gguf"
        assert fmt == "chatml"

    def test_registry_key_deepseek(self):
        name, fmt = resolve_model("deepseek-coder-1.3b")
        assert name == "deepseek-coder-1.3b-instruct-q4_k_m.gguf"
        assert fmt == "deepseek"

    def test_gguf_path(self):
        name, fmt = resolve_model("some/file.gguf")
        assert name == "some/file.gguf"
        assert fmt == "chatml"

    def test_unknown_fallback(self):
        name, fmt = resolve_model("unknown-model")
        assert name == "unknown-model"
        assert fmt == "chatml"

    def test_empty_string(self):
        name, fmt = resolve_model("")
        assert name == ""
        assert fmt == "chatml"

    def test_case_sensitive_registry(self):
        name, fmt = resolve_model("Qwen2.5-Coder-1.5B")
        assert name == "Qwen2.5-Coder-1.5B"
        assert fmt == "chatml"


class TestResolveLocalPath:
    def test_registry_key_returns_cached(self):
        p = resolve_local_path("qwen2.5-coder-1.5b", "qwen2.5-coder-1.5b-instruct-q4_k_m.gguf")
        assert p.endswith("qwen2.5-coder-1.5b-instruct-q4_k_m.gguf")
        assert ".cache/tce/models" in p

    def test_existing_file(self):
        with tempfile.NamedTemporaryFile(suffix=".gguf", delete=False) as f:
            path = f.name
        try:
            p = resolve_local_path(path, path)
            assert p == path
        finally:
            os.unlink(path)

    def test_nonexistent_fallback(self):
        p = resolve_local_path("nonexistent.gguf", "nonexistent.gguf")
        assert p == "nonexistent.gguf"


class TestPortAvailable:
    def test_available_port(self):
        assert port_available("127.0.0.1", 19999) is True

    def test_occupied_port(self):
        import socket
        s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        s.bind(("127.0.0.1", 19998))
        s.listen(1)
        try:
            assert port_available("127.0.0.1", 19998) is False
        finally:
            s.close()


class TestConvertMessages:
    def test_simple_user_message(self):
        msgs = [{"role": "user", "content": "hello"}]
        result = convert_messages(msgs)
        assert result == [{"role": "user", "content": "hello"}]

    def test_system_message(self):
        msgs = [{"role": "system", "content": "be helpful"}]
        result = convert_messages(msgs)
        assert result == [{"role": "system", "content": "be helpful"}]

    def test_message_with_tool_calls(self):
        msgs = [{
            "role": "assistant",
            "content": "",
            "tool_calls": [{"id": "call_1", "type": "function",
                            "function": {"name": "bash", "arguments": '{"cmd":"ls"}'}}],
        }]
        result = convert_messages(msgs)
        assert result[0]["role"] == "assistant"
        assert len(result[0]["tool_calls"]) == 1

    def test_tool_result(self):
        msgs = [{"role": "tool", "tool_call_id": "call_1", "content": "file list"}]
        result = convert_messages(msgs)
        assert result[0]["role"] == "tool"
        assert result[0]["tool_call_id"] == "call_1"
        assert result[0]["content"] == "file list"

    def test_empty_messages(self):
        assert convert_messages([]) == []

    def test_missing_content(self):
        msgs = [{"role": "user"}]
        result = convert_messages(msgs)
        assert "content" not in result[0]

    def test_none_content(self):
        msgs = [{"role": "user", "content": None}]
        result = convert_messages(msgs)
        assert "content" not in result[0]


class TestBuildChunk:
    def test_content_chunk(self):
        chunk = build_chunk(content="Hello")
        data = json.loads(chunk[len("data: "):-len("\n\n")])
        assert data["choices"][0]["delta"]["content"] == "Hello"
        assert data["choices"][0]["finish_reason"] is None

    def test_finish_chunk(self):
        chunk = build_chunk(finish="stop")
        data = json.loads(chunk[len("data: "):-len("\n\n")])
        assert data["choices"][0]["delta"] == {}
        assert data["choices"][0]["finish_reason"] == "stop"

    def test_tool_call_chunk(self):
        tc = [{"index": 0, "id": "call_1", "type": "function",
               "function": {"name": "bash", "arguments": ""}}]
        chunk = build_chunk(tool_calls=tc)
        data = json.loads(chunk[len("data: "):-len("\n\n")])
        assert data["choices"][0]["delta"]["tool_calls"] == tc

    def test_content_and_finish(self):
        chunk = build_chunk(content="done", finish="stop")
        data = json.loads(chunk[len("data: "):-len("\n\n")])
        assert data["choices"][0]["delta"]["content"] == "done"
        assert data["choices"][0]["finish_reason"] == "stop"

    def test_done_format(self):
        # The [DONE] signal is sent separately, not via build_chunk
        pass

    def test_empty_chunk(self):
        chunk = build_chunk()
        data = json.loads(chunk[len("data: "):-len("\n\n")])
        assert data["choices"][0]["delta"] == {}
        assert data["choices"][0]["finish_reason"] is None


class TestNormalizeTools:
    def test_flat_format(self):
        tools = [{"name": "bash", "description": "run command", "parameters": {}}]
        result = normalize_tools(tools)
        assert result == [{"type": "function", "function": tools[0]}]

    def test_openai_format(self):
        tools = [{"type": "function", "function": {"name": "bash", "description": "run"}}]
        result = normalize_tools(tools)
        assert result == tools

    def test_mixed(self):
        tools = [
            {"name": "read", "description": "read file"},
            {"type": "function", "function": {"name": "bash"}},
        ]
        result = normalize_tools(tools)
        assert result[0] == {"type": "function", "function": tools[0]}
        assert result[1] == tools[1]

    def test_empty(self):
        assert normalize_tools([]) == []

    def test_single_field(self):
        tools = [{"name": "bash"}]
        result = normalize_tools(tools)
        assert result == [{"type": "function", "function": tools[0]}]


class TestBuildNonstream:
    def test_basic_response(self):
        resp = build_nonstream(content="Hello", model_name="test-model")
        assert resp["object"] == "chat.completion"
        assert resp["model"] == "test-model"
        assert resp["choices"][0]["message"]["content"] == "Hello"
        assert resp["choices"][0]["finish_reason"] == "stop"

    def test_with_tool_calls(self):
        tc = [{"id": "call_1", "type": "function",
               "function": {"name": "bash", "arguments": '{"cmd":"ls"}'}}]
        resp = build_nonstream(tool_calls=tc, model_name="m")
        assert resp["choices"][0]["message"]["tool_calls"] == tc

    def test_finish_reason(self):
        resp = build_nonstream(finish="tool_calls", model_name="m")
        assert resp["choices"][0]["finish_reason"] == "tool_calls"

    def test_usage_default(self):
        resp = build_nonstream(model_name="m")
        assert resp["usage"] == {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0}

    def test_custom_usage(self):
        usage = {"prompt_tokens": 10, "completion_tokens": 20, "total_tokens": 30}
        resp = build_nonstream(model_name="m", usage=usage)
        assert resp["usage"] == usage

    def test_model_name_in_response(self):
        resp = build_nonstream(model_name="my-model-v1")
        assert resp["model"] == "my-model-v1"
