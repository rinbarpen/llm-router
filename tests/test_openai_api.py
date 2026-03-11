from __future__ import annotations

import os
import pytest
from pathlib import Path
from asgi_lifespan import LifespanManager
from httpx import ASGITransport, AsyncClient

from llm_router.api.app import create_app
from llm_router.config import load_settings
from llm_router.providers.base import BaseProviderClient
from llm_router.schemas import ModelInvokeRequest, ModelInvokeResponse


def _setup_test_env(tmp_path: Path, db_name: str) -> None:
    empty_config = tmp_path / "empty-router.toml"
    empty_config.write_text("")
    os.environ["LLM_ROUTER_DATABASE_URL"] = f"sqlite+aiosqlite:///{tmp_path / db_name}"
    os.environ["LLM_ROUTER_MONITOR_DATABASE_URL"] = (
        f"sqlite+aiosqlite:///{tmp_path / f'monitor-{db_name}'}"
    )
    os.environ["LLM_ROUTER_MODEL_STORE"] = str(tmp_path / "models")
    os.environ["LLM_ROUTER_MODEL_CONFIG"] = str(empty_config)
    os.environ["LLM_ROUTER_REDIS_URL"] = "redis://127.0.0.1:1/0"
    load_settings.cache_clear()


def _clear_test_env() -> None:
    load_settings.cache_clear()
    os.environ.pop("LLM_ROUTER_DATABASE_URL", None)
    os.environ.pop("LLM_ROUTER_MONITOR_DATABASE_URL", None)
    os.environ.pop("LLM_ROUTER_MODEL_STORE", None)
    os.environ.pop("LLM_ROUTER_MODEL_CONFIG", None)
    os.environ.pop("LLM_ROUTER_REDIS_URL", None)


class StubProviderClient(BaseProviderClient):
    async def invoke(self, model, request: ModelInvokeRequest) -> ModelInvokeResponse:
        return ModelInvokeResponse(
            output_text=f"openai-resp:{model.name}",
            raw={"id": "cmpl-123", "usage": {"total_tokens": 10}, "created": 1600000000}
        )

    async def stream_invoke(self, model, request: ModelInvokeRequest):  # type: ignore[override]
        from llm_router.schemas import ModelStreamChunk

        yield ModelStreamChunk(text=f"openai-resp:", is_final=False)
        yield ModelStreamChunk(text=f"{model.name}", is_final=True, finish_reason="stop")

    async def embed(self, model, payload):  # type: ignore[override]
        return {
            "object": "list",
            "data": [{"object": "embedding", "embedding": [0.1, 0.2], "index": 0}],
            "model": model.name,
        }

    async def synthesize_speech(self, model, payload):  # type: ignore[override]
        return b"FAKEAUDIO", "audio/mpeg"

    async def transcribe_audio(self, model, data, filename, mime_type, extra_payload):  # type: ignore[override]
        return {"text": f"transcribed:{model.name}"}

    async def translate_audio(self, model, data, filename, mime_type, extra_payload):  # type: ignore[override]
        return {"text": f"translated:{model.name}"}

    async def generate_image(self, model, payload):  # type: ignore[override]
        return {"created": 1600000000, "data": [{"url": "https://example.com/image.png"}]}

    async def generate_video(self, model, payload):  # type: ignore[override]
        return {"created": 1600000000, "data": [{"url": "https://example.com/video.mp4"}]}

@pytest.mark.asyncio
async def test_openai_api_compatibility(tmp_path: Path) -> None:
    _setup_test_env(tmp_path, "openai_api.db")

    app = create_app()

    async with LifespanManager(app, startup_timeout=20):
        async with AsyncClient(
            transport=ASGITransport(app=app),
            base_url="http://testserver",
        ) as client:
            StubRegistry = _stub_registry_class()
            stub_registry = StubRegistry(app.state.settings)
            app.state.provider_registry = stub_registry
            app.state.router_engine.provider_registry = stub_registry

            # 1. Setup provider and model
            await client.post("/providers", json={"name": "p1", "type": "openai"})
            await client.post("/models", json={"name": "gpt-3.5-turbo", "provider_name": "p1"})

            # 2. Test /v1/models
            resp = await client.get("/v1/models")
            assert resp.status_code == 200
            data = resp.json()
            assert data["object"] == "list"
            assert any(m["id"] == "gpt-3.5-turbo" for m in data["data"])

            # 3. Test /v1/chat/completions (model 格式为 provider/model)
            resp = await client.post(
                "/v1/chat/completions",
                json={
                    "model": "p1/gpt-3.5-turbo",
                    "messages": [{"role": "user", "content": "hello"}],
                    "temperature": 0.5
                }
            )
            assert resp.status_code == 200
            data = resp.json()
            assert data["object"] == "chat.completion"
            assert data["model"] == "gpt-3.5-turbo"
            assert data["choices"][0]["message"]["content"] == "openai-resp:gpt-3.5-turbo"
            assert data["usage"]["total_tokens"] == 10
            assert data["id"].startswith("chatcmpl-")
            assert isinstance(data["created"], int)

            # 4. Test /{provider}/v1/chat/completions (provider 在路径中，model 只需模型名)
            resp = await client.post(
                "/p1/v1/chat/completions",
                json={
                    "model": "gpt-3.5-turbo",
                    "messages": [{"role": "user", "content": "hello"}],
                    "temperature": 0.5
                }
            )
            assert resp.status_code == 200, resp.text
            data = resp.json()
            assert data["object"] == "chat.completion"
            assert data["model"] == "gpt-3.5-turbo"
            assert data["choices"][0]["message"]["content"] == "openai-resp:gpt-3.5-turbo"

            # 4b. Test model 含 provider/model 时 strip 前缀（避免 openrouter/openrouter/xxx）
            resp = await client.post(
                "/p1/v1/chat/completions",
                json={
                    "model": "p1/gpt-3.5-turbo",
                    "messages": [{"role": "user", "content": "hi"}],
                }
            )
            assert resp.status_code == 200, resp.text
            data = resp.json()
            assert data["choices"][0]["message"]["content"] == "openai-resp:gpt-3.5-turbo"

            # 5. Test remote_identifier fallback (openrouter/free 等 OpenRouter 模型 ID)
            await client.post("/providers", json={"name": "openrouter", "type": "openai"})
            await client.post(
                "/models",
                json={
                    "name": "openrouter-free",
                    "provider_name": "openrouter",
                    "remote_identifier": "openrouter/free",
                },
            )
            resp = await client.post(
                "/v1/chat/completions",
                json={
                    "model": "openrouter/free",
                    "messages": [{"role": "user", "content": "hello"}],
                },
            )
            assert resp.status_code == 200, resp.text
            data = resp.json()
            assert data["choices"][0]["message"]["content"] == "openai-resp:openrouter-free"

    _clear_test_env()


def _stub_registry_class():
    class StubRegistry:
        def __init__(self, settings):
            self.settings = settings

        def get(self, provider):
            return StubProviderClient(provider, self.settings)

    return StubRegistry


@pytest.mark.asyncio
async def test_claude_api_compatibility(tmp_path: Path) -> None:
    _setup_test_env(tmp_path, "claude_api.db")

    app = create_app()

    async with LifespanManager(app, startup_timeout=20):
        async with AsyncClient(
            transport=ASGITransport(app=app),
            base_url="http://testserver",
        ) as client:
            StubRegistry = _stub_registry_class()
            stub_registry = StubRegistry(app.state.settings)
            app.state.provider_registry = stub_registry
            app.state.router_engine.provider_registry = stub_registry

            await client.post("/providers", json={"name": "claude", "type": "openai"})
            await client.post("/models", json={"name": "claude-4.5-sonnet", "provider_name": "claude"})

            resp = await client.post(
                "/v1/messages",
                json={
                    "model": "claude-4.5-sonnet",
                    "messages": [{"role": "user", "content": "hello"}],
                    "max_tokens": 1024,
                },
            )
            assert resp.status_code == 200
            data = resp.json()
            assert "id" in data
            assert data.get("type") == "message"
            assert data.get("role") == "assistant"
            assert "content" in data
            assert isinstance(data["content"], list)
            assert any(
                block.get("type") == "text" and block.get("text") == "openai-resp:claude-4.5-sonnet"
                for block in data["content"]
            )
            assert data.get("model") == "claude-4.5-sonnet"
            assert data.get("stop_reason") == "end_turn"
            assert "usage" in data
            assert isinstance(data["usage"], dict)

    _clear_test_env()


@pytest.mark.asyncio
async def test_claude_models_export_compatibility(tmp_path: Path) -> None:
    _setup_test_env(tmp_path, "claude_models_export.db")

    app = create_app()

    async with LifespanManager(app, startup_timeout=20):
        async with AsyncClient(
            transport=ASGITransport(app=app),
            base_url="http://testserver",
        ) as client:
            StubRegistry = _stub_registry_class()
            stub_registry = StubRegistry(app.state.settings)
            app.state.provider_registry = stub_registry
            app.state.router_engine.provider_registry = stub_registry

            await client.post("/providers", json={"name": "claude_code", "type": "claude_code"})
            await client.post(
                "/models",
                json={
                    "name": "claude-sonnet-4-5",
                    "provider_name": "claude_code",
                    "display_name": "Claude Sonnet 4.5",
                },
            )
            # 非 Claude 模型不应出现在 Claude 原生 /v1/models 响应中
            await client.post("/providers", json={"name": "p1", "type": "openai"})
            await client.post("/models", json={"name": "gpt-5.1", "provider_name": "p1"})

            resp = await client.get(
                "/v1/models",
                headers={"anthropic-version": "2023-06-01"},
            )
            assert resp.status_code == 200
            data = resp.json()
            assert isinstance(data.get("data"), list)
            assert data.get("has_more") is False
            assert any(
                m.get("type") == "model" and m.get("id") == "claude-sonnet-4-5"
                for m in data["data"]
            )
            assert all(m.get("id") != "gpt-5.1" for m in data["data"])
            assert data.get("first_id") == "claude-sonnet-4-5"
            assert data.get("last_id") == "claude-sonnet-4-5"

    _clear_test_env()


@pytest.mark.asyncio
async def test_gemini_api_compatibility(tmp_path: Path) -> None:
    _setup_test_env(tmp_path, "gemini_api.db")

    app = create_app()

    async with LifespanManager(app, startup_timeout=20):
        async with AsyncClient(
            transport=ASGITransport(app=app),
            base_url="http://testserver",
        ) as client:
            StubRegistry = _stub_registry_class()
            stub_registry = StubRegistry(app.state.settings)
            app.state.provider_registry = stub_registry
            app.state.router_engine.provider_registry = stub_registry

            await client.post("/providers", json={"name": "gemini", "type": "openai"})
            await client.post("/models", json={"name": "gemini-2.5-pro", "provider_name": "gemini"})

            resp = await client.post(
                "/v1beta/models/gemini-2.5-pro:generateContent",
                json={
                    "contents": [{"role": "user", "parts": [{"text": "hello"}]}],
                },
            )
            assert resp.status_code == 200
            data = resp.json()
            assert "candidates" in data
            assert len(data["candidates"]) >= 1
            cand = data["candidates"][0]
            assert cand.get("content", {}).get("role") == "model"
            parts = cand.get("content", {}).get("parts", [])
            assert parts
            assert parts[0].get("text") == "openai-resp:gemini-2.5-pro"
            assert cand.get("finishReason") == "STOP"
            assert "usageMetadata" in data
            meta = data["usageMetadata"]
            assert isinstance(meta, dict)
            assert "promptTokenCount" in meta or "totalTokenCount" in meta or "candidatesTokenCount" in meta

    _clear_test_env()


@pytest.mark.asyncio
async def test_new_openai_modal_endpoints(tmp_path: Path) -> None:
    _setup_test_env(tmp_path, "new_modal.db")

    app = create_app()

    async with LifespanManager(app, startup_timeout=20):
        async with AsyncClient(
            transport=ASGITransport(app=app),
            base_url="http://testserver",
        ) as client:
            StubRegistry = _stub_registry_class()
            stub_registry = StubRegistry(app.state.settings)
            app.state.provider_registry = stub_registry
            app.state.router_engine.provider_registry = stub_registry

            await client.post("/providers", json={"name": "p1", "type": "openai"})
            await client.post(
                "/models",
                json={
                    "name": "multi-model",
                    "provider_name": "p1",
                    "tags": ["embedding", "tts", "asr", "image-generation", "video-generation"],
                    "config": {"capabilities": {"realtime": True}},
                },
            )

            emb = await client.post(
                "/v1/embeddings",
                json={"model": "p1/multi-model", "input": "hello"},
            )
            assert emb.status_code == 200
            assert emb.json()["data"][0]["object"] == "embedding"

            tts = await client.post(
                "/v1/audio/speech",
                json={"model": "p1/multi-model", "input": "hello", "voice": "alloy"},
            )
            assert tts.status_code == 200
            assert tts.headers["content-type"].startswith("audio/")

            asr = await client.post(
                "/v1/audio/transcriptions",
                json={"model": "p1/multi-model", "file": "data:audio/wav;base64,YWJj"},
            )
            assert asr.status_code == 200
            assert "transcribed" in asr.json()["text"]

            img = await client.post(
                "/v1/images/generations",
                json={"model": "p1/multi-model", "prompt": "cat"},
            )
            assert img.status_code == 200
            assert img.json()["data"][0]["url"]

            video = await client.post(
                "/v1/videos/generations",
                json={"model": "p1/multi-model", "prompt": "running cat"},
            )
            assert video.status_code == 202
            job_id = video.json()["id"]
            job = await client.get(f"/v1/videos/generations/{job_id}")
            assert job.status_code == 200
            assert job.json()["status"] in {"queued", "running", "completed"}

    _clear_test_env()


@pytest.mark.asyncio
async def test_audio_speech_requires_tts_capability(tmp_path: Path) -> None:
    _setup_test_env(tmp_path, "tts_capability.db")

    app = create_app()

    async with LifespanManager(app, startup_timeout=20):
        async with AsyncClient(
            transport=ASGITransport(app=app),
            base_url="http://testserver",
        ) as client:
            StubRegistry = _stub_registry_class()
            stub_registry = StubRegistry(app.state.settings)
            app.state.provider_registry = stub_registry
            app.state.router_engine.provider_registry = stub_registry

            await client.post("/providers", json={"name": "p1", "type": "openai"})
            await client.post(
                "/models",
                json={
                    "name": "chat-only-model",
                    "provider_name": "p1",
                    "tags": ["tts"],
                    "config": {"capabilities": {"tts": False}},
                },
            )

            response = await client.post(
                "/v1/audio/speech",
                json={"model": "p1/chat-only-model", "input": "hello", "voice": "alloy"},
            )
            assert response.status_code == 400
            assert "未声明能力 tts" in response.text

    _clear_test_env()


@pytest.mark.asyncio
async def test_openai_responses_api_compatibility(tmp_path: Path) -> None:
    _setup_test_env(tmp_path, "responses_api.db")

    app = create_app()

    async with LifespanManager(app, startup_timeout=20):
        async with AsyncClient(
            transport=ASGITransport(app=app),
            base_url="http://testserver",
        ) as client:
            StubRegistry = _stub_registry_class()
            stub_registry = StubRegistry(app.state.settings)
            app.state.provider_registry = stub_registry
            app.state.router_engine.provider_registry = stub_registry

            await client.post("/providers", json={"name": "codex_cli", "type": "codex_cli"})
            await client.post("/models", json={"name": "gpt-5.1", "provider_name": "codex_cli"})

            resp = await client.post(
                "/v1/responses",
                json={
                    "model": "codex_cli/gpt-5.1",
                    "input": [{"role": "user", "content": "hello"}],
                    "max_output_tokens": 256,
                },
            )
            assert resp.status_code == 200
            data = resp.json()
            assert data["object"] == "response"
            assert data["status"] == "completed"
            assert data["model"] == "gpt-5.1"
            assert data["output_text"] == "openai-resp:gpt-5.1"

    _clear_test_env()


@pytest.mark.asyncio
async def test_openai_responses_streaming(tmp_path: Path) -> None:
    _setup_test_env(tmp_path, "responses_stream.db")

    app = create_app()

    async with LifespanManager(app, startup_timeout=20):
        async with AsyncClient(
            transport=ASGITransport(app=app),
            base_url="http://testserver",
        ) as client:
            StubRegistry = _stub_registry_class()
            stub_registry = StubRegistry(app.state.settings)
            app.state.provider_registry = stub_registry
            app.state.router_engine.provider_registry = stub_registry

            await client.post("/providers", json={"name": "codex_cli", "type": "codex_cli"})
            await client.post("/models", json={"name": "gpt-5.1", "provider_name": "codex_cli"})

            async with client.stream(
                "POST",
                "/v1/responses",
                json={
                    "model": "codex_cli/gpt-5.1",
                    "input": "hello",
                    "stream": True,
                },
            ) as response:
                assert response.status_code == 200
                text = await response.aread()
                body = text.decode("utf-8")
                assert "response.created" in body
                assert "response.output_text.delta" in body
                assert "response.completed" in body
                assert "[DONE]" in body

    _clear_test_env()


@pytest.mark.asyncio
async def test_claude_count_tokens_api_compatibility(tmp_path: Path) -> None:
    _setup_test_env(tmp_path, "claude_count_tokens.db")

    app = create_app()

    async with LifespanManager(app, startup_timeout=20):
        async with AsyncClient(
            transport=ASGITransport(app=app),
            base_url="http://testserver",
        ) as client:
            StubRegistry = _stub_registry_class()
            stub_registry = StubRegistry(app.state.settings)
            app.state.provider_registry = stub_registry
            app.state.router_engine.provider_registry = stub_registry

            await client.post("/providers", json={"name": "claude_code", "type": "claude_code"})
            await client.post("/models", json={"name": "claude-sonnet-4-5", "provider_name": "claude_code"})

            resp = await client.post(
                "/v1/messages/count_tokens",
                json={
                    "model": "claude-sonnet-4-5",
                    "messages": [{"role": "user", "content": "hello world"}],
                },
            )
            assert resp.status_code == 200
            data = resp.json()
            assert "input_tokens" in data
            assert isinstance(data["input_tokens"], int)
            assert data["input_tokens"] > 0

    _clear_test_env()
