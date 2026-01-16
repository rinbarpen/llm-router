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
            # Mock registry
            class StubRegistry:
                def __init__(self, settings):
                    self.settings = settings
                def get(self, provider):
                    return StubProviderClient(provider, self.settings)

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

            # 3. Test /v1/chat/completions
            resp = await client.post(
                "/v1/chat/completions",
                json={
                    "model": "gpt-3.5-turbo",
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
            assert data["id"] == "cmpl-123"
            assert data["created"] == 1600000000

    load_settings.cache_clear()
    os.environ.pop("LLM_ROUTER_DATABASE_URL", None)
    os.environ.pop("LLM_ROUTER_MODEL_STORE", None)
