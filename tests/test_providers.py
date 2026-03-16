from __future__ import annotations

import asyncio
import json
from pathlib import Path
from typing import Any

import pytest
import respx
from httpx import Response

from llm_router.config import RouterSettings
from llm_router.db.models import Model, Provider, ProviderType
from llm_router.providers.anthropic import AnthropicProviderClient
from llm_router.providers.base import BaseProviderClient
from llm_router.providers.claude_code_cli import ClaudeCodeCLIProviderClient
from llm_router.providers.codex_cli import CodexCLIProviderClient
from llm_router.providers.kimi_code_cli import KimiCodeCLIProviderClient
from llm_router.providers.opencode_cli import OpenCodeCLIProviderClient
from llm_router.providers.qwen_code_cli import QwenCodeCLIProviderClient
from llm_router.providers.gemini import GeminiProviderClient
from llm_router.providers.openai_compatible import OpenAICompatibleProviderClient
from llm_router.schemas import ModelInvokeRequest


def _settings(tmp_path: Path) -> RouterSettings:
    return RouterSettings(
        database_url=f"sqlite+aiosqlite:///{tmp_path / 'provider.db'}",
        model_store_dir=tmp_path / "models",
    )


def _provider(provider_type: ProviderType, **kwargs) -> Provider:
    provider = Provider(
        name=provider_type.value,
        type=provider_type,
        is_active=True,
        base_url=kwargs.get("base_url"),
        api_key=kwargs.get("api_key"),
        settings=kwargs.get("settings") or {},
    )
    provider.id = 1
    return provider


def _model(provider: Provider, **kwargs) -> Model:
    model = Model(
        provider_id=provider.id or 1,
        name=kwargs.get("name", "test-model"),
        default_params=kwargs.get("default_params", {}),
        config=kwargs.get("config", {}),
        is_active=True,
    )
    model.provider = provider
    model.remote_identifier = kwargs.get("remote_identifier")
    return model


class _DummyFailoverClient(BaseProviderClient):
    async def invoke(self, model, request):  # type: ignore[override]
        async def _pick(api_key):
            return api_key
        return await self._invoke_with_failover(
            _pick,
            require_api_key=True,
        )


@pytest.mark.asyncio
@respx.mock
async def test_openai_compatible_invocation(tmp_path: Path) -> None:
    route = respx.post("https://api.openai.com/v1/chat/completions").mock(
        return_value=Response(
            200,
            json={
                "choices": [
                    {
                        "message": {"content": "hi there"},
                    }
                ]
            },
        )
    )

    provider = _provider(ProviderType.OPENAI, api_key="sk-test")
    model = _model(provider, remote_identifier="gpt-4o-mini")
    client = OpenAICompatibleProviderClient(provider, _settings(tmp_path))

    response = await client.invoke(model, ModelInvokeRequest(prompt="hello"))

    assert route.called
    assert response.output_text == "hi there"


@pytest.mark.asyncio
@respx.mock
async def test_qwen_native_tts_downloads_audio_url(tmp_path: Path) -> None:
    tts_route = respx.post(
        "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"
    ).mock(
        return_value=Response(
            200,
            json={
                "output": {
                    "audio": {
                        "url": "https://dashscope-result.example.com/audio/qwen3-tts.wav?token=abc"
                    }
                }
            },
        )
    )
    audio_route = respx.get(
        "https://dashscope-result.example.com/audio/qwen3-tts.wav?token=abc"
    ).mock(return_value=Response(200, content=b"RIFFdata", headers={"content-type": "audio/wav"}))

    provider = _provider(ProviderType.QWEN, api_key="dashscope-key")
    model = _model(provider, remote_identifier="qwen3-tts-flash")
    client = OpenAICompatibleProviderClient(provider, _settings(tmp_path))

    audio_bytes, media_type = await client.synthesize_speech(
        model,
        {"input": "你好", "voice": "Cherry"},
    )

    assert tts_route.called
    assert audio_route.called
    assert audio_bytes == b"RIFFdata"
    assert media_type == "audio/wav"


@pytest.mark.asyncio
@respx.mock
async def test_qwen_native_tts_strips_compatible_mode_base_url(tmp_path: Path) -> None:
    route = respx.post(
        "https://dashscope-intl.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"
    ).mock(
        return_value=Response(
            200,
            json={
                "output": {
                    "audio": {
                        "data": "RkFLRUFVRElP"
                    }
                }
            },
        )
    )

    provider = _provider(
        ProviderType.QWEN,
        api_key="dashscope-key",
        base_url="https://dashscope-intl.aliyuncs.com/compatible-mode/v1",
    )
    model = _model(provider, remote_identifier="qwen3-tts-instruct-flash")
    client = OpenAICompatibleProviderClient(provider, _settings(tmp_path))

    audio_bytes, media_type = await client.synthesize_speech(
        model,
        {"input": "hello", "voice": "Chelsie", "response_format": "wav"},
    )

    assert route.called
    assert audio_bytes == b"FAKEAUDIO"
    assert media_type == "audio/x-wav"


@pytest.mark.asyncio
@respx.mock
async def test_qwen_instruct_tts_with_instructions_no_voice(tmp_path: Path) -> None:
    """instruct 模型通过 instructions 调用，不需要传 voice。"""
    tts_route = respx.post(
        "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"
    ).mock(
        return_value=Response(
            200,
            json={
                "output": {
                    "audio": {
                        "data": "RkFLRUFVRElP"
                    }
                }
            },
        )
    )

    provider = _provider(ProviderType.QWEN, api_key="dashscope-key")
    model = _model(provider, remote_identifier="qwen3-tts-instruct-flash")
    client = OpenAICompatibleProviderClient(provider, _settings(tmp_path))

    audio_bytes, _media_type = await client.synthesize_speech(
        model,
        {"input": "你好", "instructions": "用温柔的语气朗读"},
    )

    assert tts_route.called
    assert audio_bytes == b"FAKEAUDIO"
    # 请求体中不应包含 voice 字段
    sent_body = tts_route.calls[0].request
    import json as _json
    body_json = _json.loads(sent_body.content)
    assert "voice" not in body_json["input"]
    assert body_json["parameters"]["instructions"] == "用温柔的语气朗读"


@pytest.mark.asyncio
@respx.mock
async def test_qwen_compatible_mode_audio_speech_endpoint(tmp_path: Path) -> None:
    """当 audio_mode != qwen_native_tts 时，QWEN provider 自动使用 compatible-mode 端点。"""
    speech_route = respx.post(
        "https://dashscope.aliyuncs.com/compatible-mode/v1/audio/speech"
    ).mock(
        return_value=Response(200, content=b"audiodata", headers={"content-type": "audio/mpeg"})
    )

    provider = _provider(
        ProviderType.QWEN,
        api_key="dashscope-key",
        settings={"audio_mode": "compatible"},
    )
    model = _model(provider, remote_identifier="qwen3-tts-flash")
    client = OpenAICompatibleProviderClient(provider, _settings(tmp_path))

    audio_bytes, media_type = await client.synthesize_speech(
        model,
        {"input": "你好", "voice": "Cherry", "model": "qwen3-tts-flash"},
    )

    assert speech_route.called
    assert audio_bytes == b"audiodata"
    assert media_type == "audio/mpeg"


@pytest.mark.asyncio
@respx.mock
async def test_gemini_invocation(tmp_path: Path) -> None:
    route = respx.post(
        "https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:generateContent?key=gm-key"
    ).mock(
        return_value=Response(
            200,
            json={
                "candidates": [
                    {"content": {"parts": [{"text": "gemini-response"}]}}
                ]
            },
        )
    )

    provider = _provider(ProviderType.GEMINI, api_key="gm-key")
    model = _model(provider, remote_identifier="gemini-pro")
    client = GeminiProviderClient(provider, _settings(tmp_path))

    await client.invoke(model, ModelInvokeRequest(prompt="hi"))
    assert route.called


@pytest.mark.asyncio
@respx.mock
async def test_anthropic_invocation(tmp_path: Path) -> None:
    route = respx.post("https://api.anthropic.com/v1/messages").mock(
        return_value=Response(
            200,
            json={
                "content": [
                    {"type": "text", "text": "claude says hi"},
                ]
            },
        )
    )

    provider = _provider(ProviderType.CLAUDE, api_key="anthropic-key")
    model = _model(provider, remote_identifier="claude-3")
    client = AnthropicProviderClient(provider, _settings(tmp_path))

    response = await client.invoke(model, ModelInvokeRequest(prompt="ping"))

    assert route.called
    assert response.output_text == "claude says hi"


@pytest.mark.asyncio
async def test_codex_cli_invocation(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    class _FakeProcess:
        def __init__(self) -> None:
            self.returncode = 0

        async def communicate(self) -> tuple[bytes, bytes]:
            stdout = (
                '{"type":"item.completed","item":{"text":"codex says hi"}}\n'
                '{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":2}}\n'
            )
            return stdout.encode("utf-8"), b""

        def kill(self) -> None:
            self.returncode = -9

    async def _fake_create_subprocess_exec(*args, **kwargs):  # type: ignore[no-untyped-def]
        return _FakeProcess()

    monkeypatch.setattr(asyncio, "create_subprocess_exec", _fake_create_subprocess_exec)

    provider = _provider(ProviderType.CODEX_CLI, api_key="sk-test")
    model = _model(provider, remote_identifier="gpt-5.1")
    client = CodexCLIProviderClient(provider, _settings(tmp_path))

    response = await client.invoke(model, ModelInvokeRequest(prompt="ping"))

    assert response.output_text == "codex says hi"


@pytest.mark.asyncio
async def test_claude_code_cli_invocation(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    class _FakeProcess:
        def __init__(self) -> None:
            self.returncode = 0

        async def communicate(self) -> tuple[bytes, bytes]:
            stdout = (
                '{"result": "claude says hi", "usage": {"input_tokens": 1, "output_tokens": 2}}'
            )
            return stdout.encode("utf-8"), b""

        def kill(self) -> None:
            self.returncode = -9

    async def _fake_create_subprocess_exec(*args, **kwargs):  # type: ignore[no-untyped-def]
        return _FakeProcess()

    monkeypatch.setattr(asyncio, "create_subprocess_exec", _fake_create_subprocess_exec)

    provider = _provider(ProviderType.CLAUDE_CODE_CLI)
    model = _model(provider, remote_identifier="claude-sonnet-4-5")
    client = ClaudeCodeCLIProviderClient(provider, _settings(tmp_path))

    response = await client.invoke(model, ModelInvokeRequest(prompt="ping"))

    assert response.output_text == "claude says hi"


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("provider_type", "client_cls"),
    [
        (ProviderType.OPENCODE_CLI, OpenCodeCLIProviderClient),
        (ProviderType.KIMI_CODE_CLI, KimiCodeCLIProviderClient),
        (ProviderType.QWEN_CODE_CLI, QwenCodeCLIProviderClient),
    ],
)
async def test_code_cli_providers_invocation(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
    provider_type: ProviderType,
    client_cls,
) -> None:
    class _FakeProcess:
        def __init__(self) -> None:
            self.returncode = 0

        async def communicate(self) -> tuple[bytes, bytes]:
            stdout = (
                '{"type":"item.completed","item":{"text":"code cli says hi"}}\n'
                '{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":2}}\n'
            )
            return stdout.encode("utf-8"), b""

        def kill(self) -> None:
            self.returncode = -9

    async def _fake_create_subprocess_exec(*args, **kwargs):  # type: ignore[no-untyped-def]
        return _FakeProcess()

    monkeypatch.setattr(asyncio, "create_subprocess_exec", _fake_create_subprocess_exec)

    provider = _provider(provider_type)
    model = _model(provider, remote_identifier="default")
    client = client_cls(provider, _settings(tmp_path))

    response = await client.invoke(model, ModelInvokeRequest(prompt="ping"))

    assert response.output_text == "code cli says hi"


@pytest.mark.asyncio
async def test_code_cli_resume_args_template(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    captured_calls: list[tuple[Any, ...]] = []

    class _FakeProcess:
        def __init__(self, session_id: str) -> None:
            self.returncode = 0
            self._session_id = session_id

        async def communicate(self) -> tuple[bytes, bytes]:
            payload = {
                "result": "ok",
                "usage": {"input_tokens": 1, "output_tokens": 1},
                "session_id": self._session_id,
            }
            return json.dumps(payload).encode("utf-8"), b""

        def kill(self) -> None:
            self.returncode = -9

    async def _fake_create_subprocess_exec(*args, **kwargs):  # type: ignore[no-untyped-def]
        captured_calls.append(tuple(args))
        return _FakeProcess(session_id=f"sess-{len(captured_calls)}")

    monkeypatch.setattr(asyncio, "create_subprocess_exec", _fake_create_subprocess_exec)

    provider = _provider(
        ProviderType.OPENCODE_CLI,
        settings={
            "parser": "single_json",
            "args_template": ["run", "--model", "{model}", "--prompt", "{prompt}"],
            "resume_args_template": [
                "run",
                "--resume",
                "{session_id}",
                "--model",
                "{model}",
                "--prompt",
                "{prompt}",
            ],
        },
    )
    model = _model(provider, remote_identifier="default")
    client = OpenCodeCLIProviderClient(provider, _settings(tmp_path))

    req = ModelInvokeRequest(prompt="ping", conversation_id="conv-1")
    first = await client.invoke(model, req)
    second = await client.invoke(model, req)

    assert first.output_text == "ok"
    assert second.output_text == "ok"
    assert "--resume" not in captured_calls[0]
    assert "--resume" in captured_calls[1]
    assert "sess-1" in captured_calls[1]


@pytest.mark.asyncio
@respx.mock
async def test_claude_count_tokens(tmp_path: Path) -> None:
    route = respx.post("https://api.anthropic.com/v1/messages/count_tokens").mock(
        return_value=Response(
            200,
            json={"input_tokens": 42},
        )
    )

    provider = _provider(ProviderType.CLAUDE, api_key="anthropic-key")
    model = _model(provider, remote_identifier="claude-sonnet-4-5")
    client = AnthropicProviderClient(provider, _settings(tmp_path))

    result = await client.count_tokens(
        model,
        {"messages": [{"role": "user", "content": "hello"}]},
    )

    assert route.called
    assert result["input_tokens"] == 42


@pytest.mark.asyncio
@respx.mock
async def test_claude_batches(tmp_path: Path) -> None:
    create_route = respx.post("https://api.anthropic.com/v1/messages/batches").mock(
        return_value=Response(200, json={"id": "msgbatch_123", "processing_status": "in_progress"})
    )
    get_route = respx.get("https://api.anthropic.com/v1/messages/batches/msgbatch_123").mock(
        return_value=Response(200, json={"id": "msgbatch_123", "processing_status": "ended"})
    )
    cancel_route = respx.post("https://api.anthropic.com/v1/messages/batches/msgbatch_123/cancel").mock(
        return_value=Response(200, json={"id": "msgbatch_123", "processing_status": "canceling"})
    )

    provider = _provider(ProviderType.CLAUDE, api_key="anthropic-key")
    client = AnthropicProviderClient(provider, _settings(tmp_path))

    created = await client.create_message_batch({"requests": []})
    fetched = await client.get_message_batch("msgbatch_123")
    canceled = await client.cancel_message_batch("msgbatch_123")

    assert create_route.called
    assert get_route.called
    assert cancel_route.called
    assert created["id"] == "msgbatch_123"
    assert fetched["processing_status"] == "ended"
    assert canceled["processing_status"] == "canceling"


@pytest.mark.asyncio
async def test_api_key_round_robin_order(tmp_path: Path) -> None:
    provider = _provider(ProviderType.OPENAI, api_key="k1,k2,k3")
    client = _DummyFailoverClient(provider, _settings(tmp_path))

    first = await client.invoke(None, None)  # type: ignore[arg-type]
    second = await client.invoke(None, None)  # type: ignore[arg-type]
    third = await client.invoke(None, None)  # type: ignore[arg-type]

    assert first == "k1"
    assert second == "k2"
    assert third == "k3"
