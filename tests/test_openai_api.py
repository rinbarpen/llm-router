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

class StubProviderClient(BaseProviderClient):
    async def invoke(self, model, request: ModelInvokeRequest) -> ModelInvokeResponse:
        return ModelInvokeResponse(
            output_text=f"openai-resp:{model.name}",
            raw={"id": "cmpl-123", "usage": {"total_tokens": 10}, "created": 1600000000}
        )

@pytest.mark.asyncio
async def test_openai_api_compatibility(tmp_path: Path) -> None:
    os.environ["LLM_ROUTER_DATABASE_URL"] = f"sqlite+aiosqlite:///{tmp_path / 'openai_api.db'}"
    os.environ["LLM_ROUTER_MODEL_STORE"] = str(tmp_path / "models")
    load_settings.cache_clear()

    app = create_app()

    async with LifespanManager(app):
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

    load_settings.cache_clear()
    os.environ.pop("LLM_ROUTER_DATABASE_URL", None)
    os.environ.pop("LLM_ROUTER_MODEL_STORE", None)


def _stub_registry_class():
    class StubRegistry:
        def __init__(self, settings):
            self.settings = settings

        def get(self, provider):
            return StubProviderClient(provider, self.settings)

    return StubRegistry


@pytest.mark.asyncio
async def test_claude_api_compatibility(tmp_path: Path) -> None:
    os.environ["LLM_ROUTER_DATABASE_URL"] = f"sqlite+aiosqlite:///{tmp_path / 'claude_api.db'}"
    os.environ["LLM_ROUTER_MODEL_STORE"] = str(tmp_path / "models")
    load_settings.cache_clear()

    app = create_app()

    async with LifespanManager(app):
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

    load_settings.cache_clear()
    os.environ.pop("LLM_ROUTER_DATABASE_URL", None)
    os.environ.pop("LLM_ROUTER_MODEL_STORE", None)


@pytest.mark.asyncio
async def test_gemini_api_compatibility(tmp_path: Path) -> None:
    os.environ["LLM_ROUTER_DATABASE_URL"] = f"sqlite+aiosqlite:///{tmp_path / 'gemini_api.db'}"
    os.environ["LLM_ROUTER_MODEL_STORE"] = str(tmp_path / "models")
    load_settings.cache_clear()

    app = create_app()

    async with LifespanManager(app):
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

    load_settings.cache_clear()
    os.environ.pop("LLM_ROUTER_DATABASE_URL", None)
    os.environ.pop("LLM_ROUTER_MODEL_STORE", None)
