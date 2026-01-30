"""认证中间件与公开端点测试：在启用认证且存在 api_keys 时验证 401/403/200 及公开端点可访问。"""

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
            raw={"prompt": request.prompt, "usage": {"prompt_tokens": 10, "completion_tokens": 20, "total_tokens": 30}},
        )

    async def stream(self, model, request: ModelInvokeRequest):  # type: ignore[override]
        from llm_router.schemas import ModelStreamChunk
        yield ModelStreamChunk(text="stub:", is_final=False)
        yield ModelStreamChunk(text=f"{model.name}", is_final=True, finish_reason="stop")


@pytest.fixture
def stub_registry():
    class StubRegistry:
        def __init__(self, settings):
            self.settings = settings

        def get(self, provider):
            return StubProviderClient(provider, self.settings)

    return StubRegistry


@pytest.fixture
async def app_client_with_auth(tmp_path: Path, stub_registry):
    """启用认证的测试应用：require_auth=True，预置 API Key，本机请求也需认证。"""
    os.environ["LLM_ROUTER_DATABASE_URL"] = f"sqlite+aiosqlite:///{tmp_path / 'test.db'}"
    os.environ["LLM_ROUTER_MODEL_STORE"] = str(tmp_path / "models")
    os.environ["LLM_ROUTER_REQUIRE_AUTH"] = "true"
    os.environ["LLM_ROUTER_API_KEYS"] = "auth-fixture-key"
    os.environ["LLM_ROUTER_ALLOW_LOCAL_WITHOUT_AUTH"] = "false"
    load_settings.cache_clear()

    app = create_app()

    async with LifespanManager(app):
        settings = app.state.settings
        registry = stub_registry(settings)
        app.state.provider_registry = registry
        app.state.router_engine.provider_registry = registry  # type: ignore[assignment]

        async with AsyncClient(
            transport=ASGITransport(app=app),
            base_url="http://testserver",
        ) as client:
            yield client

    load_settings.cache_clear()
    for key in (
        "LLM_ROUTER_DATABASE_URL",
        "LLM_ROUTER_MODEL_STORE",
        "LLM_ROUTER_REQUIRE_AUTH",
        "LLM_ROUTER_API_KEYS",
        "LLM_ROUTER_ALLOW_LOCAL_WITHOUT_AUTH",
    ):
        os.environ.pop(key, None)


# ==================== 中间件：受保护端点 ====================


@pytest.mark.asyncio
async def test_protected_endpoint_returns_401_without_auth(app_client_with_auth: AsyncClient) -> None:
    """未带认证访问受保护端点应返回 401。"""
    response = await app_client_with_auth.get("/models")
    assert response.status_code == 401


@pytest.mark.asyncio
async def test_protected_endpoint_returns_403_with_invalid_key(app_client_with_auth: AsyncClient) -> None:
    """无效 API Key 访问受保护端点应返回 403。"""
    response = await app_client_with_auth.get(
        "/models",
        headers={"Authorization": "Bearer invalid-key"},
    )
    assert response.status_code == 403


@pytest.mark.asyncio
async def test_protected_endpoint_returns_200_with_valid_api_key(app_client_with_auth: AsyncClient) -> None:
    """有效 API Key（Bearer）访问受保护端点应返回 200。"""
    response = await app_client_with_auth.get(
        "/models",
        headers={"Authorization": "Bearer auth-fixture-key"},
    )
    assert response.status_code == 200


@pytest.mark.asyncio
async def test_protected_endpoint_returns_200_with_x_api_key(app_client_with_auth: AsyncClient) -> None:
    """有效 X-API-Key 头访问受保护端点应返回 200。"""
    response = await app_client_with_auth.get(
        "/models",
        headers={"X-API-Key": "auth-fixture-key"},
    )
    assert response.status_code == 200


@pytest.mark.asyncio
async def test_protected_endpoint_returns_200_with_session_token(app_client_with_auth: AsyncClient) -> None:
    """先登录获取 Session Token，再带 token 访问受保护端点应返回 200。"""
    login_resp = await app_client_with_auth.post(
        "/auth/login",
        json={"api_key": "auth-fixture-key"},
    )
    assert login_resp.status_code == 200
    token = login_resp.json()["token"]

    response = await app_client_with_auth.get(
        "/models",
        headers={"Authorization": f"Bearer {token}"},
    )
    assert response.status_code == 200


# ==================== 公开端点 ====================


@pytest.mark.asyncio
async def test_public_endpoint_health_no_auth_required(app_client_with_auth: AsyncClient) -> None:
    """GET /health 无需认证即可访问。"""
    response = await app_client_with_auth.get("/health")
    assert response.status_code == 200
    assert response.json().get("status") == "ok"


@pytest.mark.asyncio
async def test_public_endpoint_login_accessible_without_auth(app_client_with_auth: AsyncClient) -> None:
    """POST /auth/login 为公开端点，未带 key 时返回 401（由 login 逻辑返回），非中间件拦截。"""
    response = await app_client_with_auth.post("/auth/login", json={})
    assert response.status_code == 401
