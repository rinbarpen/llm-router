from __future__ import annotations

import os
from pathlib import Path

import pytest
from asgi_lifespan import LifespanManager
from httpx import ASGITransport, AsyncClient

from llm_router.api.app import create_app
from llm_router.config import load_settings
from llm_router.providers.base import BaseProviderClient
from llm_router.schemas import ModelInvokeRequest, ModelInvokeResponse


class StubProviderClient(BaseProviderClient):
    async def invoke(self, model, request: ModelInvokeRequest) -> ModelInvokeResponse:  # type: ignore[override]
        return ModelInvokeResponse(
            output_text=f"stub:{model.name}",
            raw={"prompt": request.prompt},
        )


@pytest.mark.asyncio
async def test_api_full_flow(tmp_path: Path) -> None:
    os.environ["LLM_ROUTER_DATABASE_URL"] = f"sqlite+aiosqlite:///{tmp_path / 'api.db'}"
    os.environ["LLM_ROUTER_MODEL_STORE"] = str(tmp_path / "models")
    load_settings.cache_clear()

    app = create_app()

    async with LifespanManager(app):
        async with AsyncClient(
            transport=ASGITransport(app=app),
            base_url="http://testserver",
        ) as client:
            settings = app.state.settings

            class StubRegistry:
                def __init__(self, settings):
                    self.settings = settings

                def get(self, provider):
                    return StubProviderClient(provider, self.settings)

            stub_registry = StubRegistry(settings)
            app.state.provider_registry = stub_registry
            app.state.router_engine.provider_registry = stub_registry  # type: ignore[assignment]

            response = await client.post(
                "/providers",
                json={
                    "name": "openai",
                    "type": "remote_http",
                    "base_url": "https://example.com",
                },
            )
            assert response.status_code == 201

            response = await client.post(
                "/models",
                json={
                    "name": "gpt-4",
                    "provider_name": "openai",
                    "display_name": "GPT-4",
                    "tags": ["chat", "english"],
                    "rate_limit": {"max_requests": 5, "per_seconds": 60},
                },
            )
            assert response.status_code == 201
            model_data = response.json()
            assert model_data["name"] == "gpt-4"
            assert "chat" in model_data["tags"]

            response = await client.get("/models", params={"tag": "chat"})
            assert response.status_code == 200
            models = response.json()
            assert len(models) == 1

            response = await client.post(
                "/models/openai/gpt-4/invoke",
                json={"prompt": "Hello"},
            )
            assert response.status_code == 200
            assert response.json()["output_text"] == "stub:gpt-4"

            response = await client.post(
                "/route/invoke",
                json={
                    "query": {"tags": ["chat"]},
                    "request": {"prompt": "Hi"},
                },
            )
            assert response.status_code == 200
            assert response.json()["output_text"] == "stub:gpt-4"

    load_settings.cache_clear()
    os.environ.pop("LLM_ROUTER_DATABASE_URL", None)
    os.environ.pop("LLM_ROUTER_MODEL_STORE", None)

