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
    """创建StubProviderClient注册表"""
    class StubRegistry:
        def __init__(self, settings):
            self.settings = settings

        def get(self, provider):
            return StubProviderClient(provider, self.settings)

    return StubRegistry


@pytest.fixture
async def app_client(tmp_path: Path):
    """创建测试应用和客户端"""
    os.environ["LLM_ROUTER_DATABASE_URL"] = f"sqlite+aiosqlite:///{tmp_path / 'test.db'}"
    os.environ["LLM_ROUTER_MODEL_STORE"] = str(tmp_path / "models")
    load_settings.cache_clear()

    app = create_app()

    async with LifespanManager(app):
        settings = app.state.settings
        registry = stub_registry()(settings)
        app.state.provider_registry = registry
        app.state.router_engine.provider_registry = registry  # type: ignore[assignment]

        async with AsyncClient(
            transport=ASGITransport(app=app),
            base_url="http://testserver",
        ) as client:
            yield client

    load_settings.cache_clear()
    os.environ.pop("LLM_ROUTER_DATABASE_URL", None)
    os.environ.pop("LLM_ROUTER_MODEL_STORE", None)


# ==================== 健康检查 ====================

@pytest.mark.asyncio
async def test_health_check(app_client: AsyncClient) -> None:
    """测试健康检查端点"""
    response = await app_client.get("/health")
    assert response.status_code == 200
    data = response.json()
    assert data["status"] == "ok"


# ==================== Provider 管理 ====================

@pytest.mark.asyncio
async def test_create_provider(app_client: AsyncClient) -> None:
    """测试创建Provider"""
    response = await app_client.post(
        "/providers",
        json={
            "name": "test_provider",
            "type": "remote_http",
            "base_url": "https://example.com",
        },
    )
    assert response.status_code == 201
    data = response.json()
    assert data["name"] == "test_provider"
    assert data["type"] == "remote_http"
    assert data["is_active"] is True


@pytest.mark.asyncio
async def test_create_provider_invalid_type(app_client: AsyncClient) -> None:
    """测试创建Provider时使用无效类型"""
    response = await app_client.post(
        "/providers",
        json={
            "name": "test_provider",
            "type": "invalid_type",
            "base_url": "https://example.com",
        },
    )
    assert response.status_code == 400


@pytest.mark.asyncio
async def test_list_providers(app_client: AsyncClient) -> None:
    """测试列出所有Provider"""
    # 先创建一个Provider
    await app_client.post(
        "/providers",
        json={
            "name": "test_provider",
            "type": "remote_http",
            "base_url": "https://example.com",
        },
    )

    response = await app_client.get("/providers")
    assert response.status_code == 200
    data = response.json()
    assert isinstance(data, list)
    assert len(data) >= 1
    assert any(p["name"] == "test_provider" for p in data)


# ==================== Model 管理 ====================

@pytest.mark.asyncio
async def test_create_model(app_client: AsyncClient) -> None:
    """测试创建模型"""
    # 先创建Provider
    await app_client.post(
        "/providers",
        json={
            "name": "test_provider",
            "type": "remote_http",
            "base_url": "https://example.com",
        },
    )

    response = await app_client.post(
        "/models",
        json={
            "name": "test_model",
            "provider_name": "test_provider",
            "display_name": "Test Model",
            "tags": ["chat", "test"],
            "rate_limit": {"max_requests": 5, "per_seconds": 60},
        },
    )
    assert response.status_code == 201
    data = response.json()
    assert data["name"] == "test_model"
    assert data["provider_name"] == "test_provider"
    assert "chat" in data["tags"]
    assert "test" in data["tags"]


@pytest.mark.asyncio
async def test_create_model_invalid_provider(app_client: AsyncClient) -> None:
    """测试使用不存在的Provider创建模型"""
    response = await app_client.post(
        "/models",
        json={
            "name": "test_model",
            "provider_name": "nonexistent_provider",
            "display_name": "Test Model",
        },
    )
    assert response.status_code == 400


@pytest.mark.asyncio
async def test_list_models(app_client: AsyncClient) -> None:
    """测试列出所有模型"""
    # 先创建Provider和Model
    await app_client.post(
        "/providers",
        json={
            "name": "test_provider",
            "type": "remote_http",
            "base_url": "https://example.com",
        },
    )
    await app_client.post(
        "/models",
        json={
            "name": "test_model",
            "provider_name": "test_provider",
            "display_name": "Test Model",
            "tags": ["chat"],
        },
    )

    response = await app_client.get("/models")
    assert response.status_code == 200
    data = response.json()
    assert isinstance(data, list)
    assert len(data) >= 1


@pytest.mark.asyncio
async def test_list_models_with_tag_filter(app_client: AsyncClient) -> None:
    """测试按标签过滤模型"""
    # 先创建Provider和Model
    await app_client.post(
        "/providers",
        json={
            "name": "test_provider",
            "type": "remote_http",
            "base_url": "https://example.com",
        },
    )
    await app_client.post(
        "/models",
        json={
            "name": "test_model",
            "provider_name": "test_provider",
            "display_name": "Test Model",
            "tags": ["chat"],
        },
    )

    response = await app_client.get("/models", params={"tag": "chat"})
    assert response.status_code == 200
    data = response.json()
    assert isinstance(data, list)
    assert all("chat" in m["tags"] for m in data)


@pytest.mark.asyncio
async def test_list_models_with_provider_type_filter(app_client: AsyncClient) -> None:
    """测试按Provider类型过滤模型"""
    # 先创建Provider和Model
    await app_client.post(
        "/providers",
        json={
            "name": "test_provider",
            "type": "remote_http",
            "base_url": "https://example.com",
        },
    )
    await app_client.post(
        "/models",
        json={
            "name": "test_model",
            "provider_name": "test_provider",
            "display_name": "Test Model",
        },
    )

    response = await app_client.get("/models", params={"provider_types": "remote_http"})
    assert response.status_code == 200
    data = response.json()
    assert isinstance(data, list)


@pytest.mark.asyncio
async def test_get_provider_models(app_client: AsyncClient) -> None:
    """测试获取特定Provider的模型"""
    # 先创建Provider和Model
    await app_client.post(
        "/providers",
        json={
            "name": "test_provider",
            "type": "remote_http",
            "base_url": "https://example.com",
        },
    )
    await app_client.post(
        "/models",
        json={
            "name": "test_model",
            "provider_name": "test_provider",
            "display_name": "Test Model",
        },
    )

    response = await app_client.get("/models/test_provider")
    assert response.status_code == 200
    data = response.json()
    assert isinstance(data, list)
    assert all(m["provider_name"] == "test_provider" for m in data)


@pytest.mark.asyncio
async def test_get_model(app_client: AsyncClient) -> None:
    """测试获取单个模型"""
    # 先创建Provider和Model
    await app_client.post(
        "/providers",
        json={
            "name": "test_provider",
            "type": "remote_http",
            "base_url": "https://example.com",
        },
    )
    await app_client.post(
        "/models",
        json={
            "name": "test_model",
            "provider_name": "test_provider",
            "display_name": "Test Model",
        },
    )

    response = await app_client.get("/models/test_provider/test_model")
    assert response.status_code == 200
    data = response.json()
    assert data["name"] == "test_model"
    assert data["provider_name"] == "test_provider"


@pytest.mark.asyncio
async def test_get_model_not_found(app_client: AsyncClient) -> None:
    """测试获取不存在的模型"""
    response = await app_client.get("/models/nonexistent_provider/nonexistent_model")
    assert response.status_code == 404


@pytest.mark.asyncio
async def test_update_model(app_client: AsyncClient) -> None:
    """测试更新模型"""
    # 先创建Provider和Model
    await app_client.post(
        "/providers",
        json={
            "name": "test_provider",
            "type": "remote_http",
            "base_url": "https://example.com",
        },
    )
    await app_client.post(
        "/models",
        json={
            "name": "test_model",
            "provider_name": "test_provider",
            "display_name": "Test Model",
        },
    )

    response = await app_client.patch(
        "/models/test_provider/test_model",
        json={
            "display_name": "Updated Model",
            "tags": ["updated"],
        },
    )
    assert response.status_code == 200
    data = response.json()
    assert data["display_name"] == "Updated Model"
    assert "updated" in data["tags"]


@pytest.mark.asyncio
async def test_update_model_not_found(app_client: AsyncClient) -> None:
    """测试更新不存在的模型"""
    response = await app_client.patch(
        "/models/nonexistent_provider/nonexistent_model",
        json={"display_name": "Updated"},
    )
    assert response.status_code == 404


# ==================== 模型调用 ====================

@pytest.mark.asyncio
async def test_invoke_model(app_client: AsyncClient) -> None:
    """测试调用模型"""
    # 先创建Provider和Model
    await app_client.post(
        "/providers",
        json={
            "name": "test_provider",
            "type": "remote_http",
            "base_url": "https://example.com",
        },
    )
    await app_client.post(
        "/models",
        json={
            "name": "test_model",
            "provider_name": "test_provider",
            "display_name": "Test Model",
        },
    )

    response = await app_client.post(
        "/models/test_provider/test_model/invoke",
        json={"prompt": "Hello"},
    )
    assert response.status_code == 200
    data = response.json()
    assert "output_text" in data
    assert data["output_text"] == "stub:test_model"


@pytest.mark.asyncio
async def test_invoke_model_with_messages(app_client: AsyncClient) -> None:
    """测试使用messages格式调用模型"""
    # 先创建Provider和Model
    await app_client.post(
        "/providers",
        json={
            "name": "test_provider",
            "type": "remote_http",
            "base_url": "https://example.com",
        },
    )
    await app_client.post(
        "/models",
        json={
            "name": "test_model",
            "provider_name": "test_provider",
            "display_name": "Test Model",
        },
    )

    response = await app_client.post(
        "/models/test_provider/test_model/invoke",
        json={
            "messages": [
                {"role": "user", "content": "Hello"}
            ],
        },
    )
    assert response.status_code == 200
    data = response.json()
    assert "output_text" in data


@pytest.mark.asyncio
async def test_invoke_model_not_found(app_client: AsyncClient) -> None:
    """测试调用不存在的模型"""
    response = await app_client.post(
        "/models/nonexistent_provider/nonexistent_model/invoke",
        json={"prompt": "Hello"},
    )
    assert response.status_code == 400


@pytest.mark.asyncio
async def test_invoke_model_streaming(app_client: AsyncClient) -> None:
    """测试流式调用模型"""
    # 先创建Provider和Model
    await app_client.post(
        "/providers",
        json={
            "name": "test_provider",
            "type": "remote_http",
            "base_url": "https://example.com",
        },
    )
    await app_client.post(
        "/models",
        json={
            "name": "test_model",
            "provider_name": "test_provider",
            "display_name": "Test Model",
        },
    )

    response = await app_client.post(
        "/models/test_provider/test_model/invoke",
        json={"prompt": "Hello", "stream": True},
    )
    assert response.status_code == 200
    # 流式响应应该是JSONL格式
    assert response.headers.get("content-type") == "application/jsonl"
    text = response.text
    assert "stub:" in text
    assert "test_model" in text


@pytest.mark.asyncio
async def test_route_model(app_client: AsyncClient) -> None:
    """测试智能路由调用"""
    # 先创建Provider和Model
    await app_client.post(
        "/providers",
        json={
            "name": "test_provider",
            "type": "remote_http",
            "base_url": "https://example.com",
        },
    )
    await app_client.post(
        "/models",
        json={
            "name": "test_model",
            "provider_name": "test_provider",
            "display_name": "Test Model",
            "tags": ["chat"],
        },
    )

    response = await app_client.post(
        "/route/invoke",
        json={
            "query": {"tags": ["chat"]},
            "request": {"prompt": "Hello"},
        },
    )
    assert response.status_code == 200
    data = response.json()
    assert "output_text" in data
    assert data["output_text"] == "stub:test_model"


@pytest.mark.asyncio
async def test_route_model_no_match(app_client: AsyncClient) -> None:
    """测试路由时没有匹配的模型"""
    response = await app_client.post(
        "/route/invoke",
        json={
            "query": {"tags": ["nonexistent_tag"]},
            "request": {"prompt": "Hello"},
        },
    )
    assert response.status_code == 400


# ==================== OpenAI 兼容 API ====================

@pytest.mark.asyncio
async def test_openai_chat_completions(app_client: AsyncClient) -> None:
    """测试OpenAI兼容的聊天完成端点"""
    # 先创建Provider和Model
    await app_client.post(
        "/providers",
        json={
            "name": "test_provider",
            "type": "remote_http",
            "base_url": "https://example.com",
        },
    )
    await app_client.post(
        "/models",
        json={
            "name": "test_model",
            "provider_name": "test_provider",
            "display_name": "Test Model",
        },
    )

    response = await app_client.post(
        "/v1/chat/completions",
        json={
            "model": "test_provider/test_model",
            "messages": [
                {"role": "user", "content": "Hello"}
            ],
        },
    )
    assert response.status_code == 200
    data = response.json()
    assert "choices" in data
    assert len(data["choices"]) > 0
    assert "message" in data["choices"][0]
    assert "content" in data["choices"][0]["message"]


@pytest.mark.asyncio
async def test_openai_chat_completions_streaming(app_client: AsyncClient) -> None:
    """测试OpenAI兼容的流式聊天完成"""
    # 先创建Provider和Model
    await app_client.post(
        "/providers",
        json={
            "name": "test_provider",
            "type": "remote_http",
            "base_url": "https://example.com",
        },
    )
    await app_client.post(
        "/models",
        json={
            "name": "test_model",
            "provider_name": "test_provider",
            "display_name": "Test Model",
        },
    )

    response = await app_client.post(
        "/v1/chat/completions",
        json={
            "model": "test_provider/test_model",
            "messages": [
                {"role": "user", "content": "Hello"}
            ],
            "stream": True,
        },
    )
    assert response.status_code == 200
    assert response.headers.get("content-type") == "text/event-stream"
    text = response.text
    assert "data:" in text


@pytest.mark.asyncio
async def test_openai_chat_completions_invalid_model(app_client: AsyncClient) -> None:
    """测试OpenAI兼容API使用无效模型"""
    response = await app_client.post(
        "/v1/chat/completions",
        json={
            "model": "nonexistent_provider/nonexistent_model",
            "messages": [
                {"role": "user", "content": "Hello"}
            ],
        },
    )
    assert response.status_code == 404


@pytest.mark.asyncio
async def test_openai_chat_completions_no_messages(app_client: AsyncClient) -> None:
    """测试OpenAI兼容API没有messages"""
    response = await app_client.post(
        "/v1/chat/completions",
        json={
            "model": "test_provider/test_model",
        },
    )
    assert response.status_code == 400


# ==================== API Key 管理 ====================

@pytest.mark.asyncio
async def test_create_api_key(app_client: AsyncClient) -> None:
    """测试创建API Key"""
    response = await app_client.post(
        "/api-keys",
        json={
            "key": "test-api-key-123",
            "name": "Test API Key",
            "is_active": True,
        },
    )
    assert response.status_code == 201
    data = response.json()
    assert data["key"] == "test-api-key-123"
    assert data["name"] == "Test API Key"
    assert data["is_active"] is True
    assert "id" in data


@pytest.mark.asyncio
async def test_create_api_key_with_restrictions(app_client: AsyncClient) -> None:
    """测试创建带限制的API Key"""
    response = await app_client.post(
        "/api-keys",
        json={
            "key": "restricted-key",
            "name": "Restricted Key",
            "is_active": True,
            "allowed_models": ["test_provider/test_model"],
            "allowed_providers": ["test_provider"],
            "parameter_limits": {
                "max_tokens": 1000,
                "temperature": 0.7,
            },
        },
    )
    assert response.status_code == 201
    data = response.json()
    assert data["allowed_models"] == ["test_provider/test_model"]
    assert data["allowed_providers"] == ["test_provider"]
    assert data["parameter_limits"] is not None


@pytest.mark.asyncio
async def test_create_api_key_duplicate(app_client: AsyncClient) -> None:
    """测试创建重复的API Key"""
    # 先创建一个
    await app_client.post(
        "/api-keys",
        json={
            "key": "duplicate-key",
            "name": "First Key",
        },
    )

    # 尝试创建重复的
    response = await app_client.post(
        "/api-keys",
        json={
            "key": "duplicate-key",
            "name": "Second Key",
        },
    )
    assert response.status_code == 400


@pytest.mark.asyncio
async def test_list_api_keys(app_client: AsyncClient) -> None:
    """测试列出所有API Key"""
    # 先创建几个API Key
    await app_client.post(
        "/api-keys",
        json={
            "key": "key1",
            "name": "Key 1",
        },
    )
    await app_client.post(
        "/api-keys",
        json={
            "key": "key2",
            "name": "Key 2",
        },
    )

    response = await app_client.get("/api-keys")
    assert response.status_code == 200
    data = response.json()
    assert isinstance(data, list)
    assert len(data) >= 2


@pytest.mark.asyncio
async def test_list_api_keys_include_inactive(app_client: AsyncClient) -> None:
    """测试列出API Key（包含非激活的）"""
    # 创建一个激活的
    await app_client.post(
        "/api-keys",
        json={
            "key": "active-key",
            "name": "Active Key",
            "is_active": True,
        },
    )
    # 创建一个非激活的
    await app_client.post(
        "/api-keys",
        json={
            "key": "inactive-key",
            "name": "Inactive Key",
            "is_active": False,
        },
    )

    # 默认只返回激活的
    response = await app_client.get("/api-keys")
    assert response.status_code == 200
    data = response.json()
    active_keys = [k for k in data if k["is_active"]]
    assert len(active_keys) >= 1

    # 包含非激活的
    response = await app_client.get("/api-keys", params={"include_inactive": "true"})
    assert response.status_code == 200
    data = response.json()
    assert len(data) >= 2


@pytest.mark.asyncio
async def test_get_api_key(app_client: AsyncClient) -> None:
    """测试获取单个API Key"""
    # 先创建一个
    create_response = await app_client.post(
        "/api-keys",
        json={
            "key": "get-test-key",
            "name": "Get Test Key",
        },
    )
    api_key_id = create_response.json()["id"]

    response = await app_client.get(f"/api-keys/{api_key_id}")
    assert response.status_code == 200
    data = response.json()
    assert data["id"] == api_key_id
    assert data["key"] == "get-test-key"


@pytest.mark.asyncio
async def test_get_api_key_not_found(app_client: AsyncClient) -> None:
    """测试获取不存在的API Key"""
    response = await app_client.get("/api-keys/99999")
    assert response.status_code == 404


@pytest.mark.asyncio
async def test_update_api_key(app_client: AsyncClient) -> None:
    """测试更新API Key"""
    # 先创建一个
    create_response = await app_client.post(
        "/api-keys",
        json={
            "key": "update-test-key",
            "name": "Original Name",
            "is_active": True,
        },
    )
    api_key_id = create_response.json()["id"]

    # 更新
    response = await app_client.patch(
        f"/api-keys/{api_key_id}",
        json={
            "name": "Updated Name",
            "is_active": False,
        },
    )
    assert response.status_code == 200
    data = response.json()
    assert data["name"] == "Updated Name"
    assert data["is_active"] is False


@pytest.mark.asyncio
async def test_update_api_key_not_found(app_client: AsyncClient) -> None:
    """测试更新不存在的API Key"""
    response = await app_client.patch(
        "/api-keys/99999",
        json={"name": "Updated"},
    )
    assert response.status_code == 404


@pytest.mark.asyncio
async def test_delete_api_key(app_client: AsyncClient) -> None:
    """测试删除API Key"""
    # 先创建一个
    create_response = await app_client.post(
        "/api-keys",
        json={
            "key": "delete-test-key",
            "name": "Delete Test Key",
        },
    )
    api_key_id = create_response.json()["id"]

    # 删除
    response = await app_client.delete(f"/api-keys/{api_key_id}")
    assert response.status_code == 204

    # 验证已删除
    response = await app_client.get(f"/api-keys/{api_key_id}")
    assert response.status_code == 404


@pytest.mark.asyncio
async def test_delete_api_key_not_found(app_client: AsyncClient) -> None:
    """测试删除不存在的API Key"""
    response = await app_client.delete("/api-keys/99999")
    assert response.status_code == 404


# ==================== 认证 ====================

@pytest.mark.asyncio
async def test_login_with_api_key(app_client: AsyncClient) -> None:
    """测试使用API Key登录"""
    # 先创建一个API Key
    await app_client.post(
        "/api-keys",
        json={
            "key": "login-test-key",
            "name": "Login Test Key",
            "is_active": True,
        },
    )

    response = await app_client.post(
        "/auth/login",
        json={"api_key": "login-test-key"},
    )
    assert response.status_code == 200
    data = response.json()
    assert "token" in data
    assert "expires_in" in data
    assert data["expires_in"] > 0


@pytest.mark.asyncio
async def test_login_with_invalid_api_key(app_client: AsyncClient) -> None:
    """测试使用无效API Key登录"""
    response = await app_client.post(
        "/auth/login",
        json={"api_key": "invalid-key"},
    )
    assert response.status_code == 403


@pytest.mark.asyncio
async def test_login_with_header(app_client: AsyncClient) -> None:
    """测试使用Header中的API Key登录"""
    # 先创建一个API Key
    await app_client.post(
        "/api-keys",
        json={
            "key": "header-test-key",
            "name": "Header Test Key",
            "is_active": True,
        },
    )

    response = await app_client.post(
        "/auth/login",
        headers={"Authorization": "Bearer header-test-key"},
    )
    assert response.status_code == 200
    data = response.json()
    assert "token" in data


@pytest.mark.asyncio
async def test_logout(app_client: AsyncClient) -> None:
    """测试登出"""
    # 先创建一个API Key并登录
    await app_client.post(
        "/api-keys",
        json={
            "key": "logout-test-key",
            "name": "Logout Test Key",
            "is_active": True,
        },
    )
    login_response = await app_client.post(
        "/auth/login",
        json={"api_key": "logout-test-key"},
    )
    token = login_response.json()["token"]

    # 登出
    response = await app_client.post(
        "/auth/logout",
        headers={"Authorization": f"Bearer {token}"},
    )
    assert response.status_code == 200
    data = response.json()
    assert "message" in data


@pytest.mark.asyncio
async def test_logout_invalid_token(app_client: AsyncClient) -> None:
    """测试使用无效Token登出"""
    response = await app_client.post(
        "/auth/logout",
        headers={"Authorization": "Bearer invalid-token"},
    )
    assert response.status_code == 404


# ==================== 监控导出 ====================

@pytest.mark.asyncio
async def test_export_data_json(app_client: AsyncClient) -> None:
    """测试导出JSON数据"""
    # 先创建Provider和Model并调用一次
    await app_client.post(
        "/providers",
        json={
            "name": "test_provider",
            "type": "remote_http",
            "base_url": "https://example.com",
        },
    )
    await app_client.post(
        "/models",
        json={
            "name": "test_model",
            "provider_name": "test_provider",
            "display_name": "Test Model",
        },
    )
    await app_client.post(
        "/models/test_provider/test_model/invoke",
        json={"prompt": "Test"},
    )

    # 等待数据写入
    import asyncio
    await asyncio.sleep(0.1)

    response = await app_client.get("/monitor/export/json")
    assert response.status_code == 200
    data = response.json()
    assert "export_time" in data
    assert "statistics" in data
    assert "invocations" in data


@pytest.mark.asyncio
async def test_export_data_excel(app_client: AsyncClient) -> None:
    """测试导出Excel数据"""
    # 先创建Provider和Model并调用一次
    await app_client.post(
        "/providers",
        json={
            "name": "test_provider",
            "type": "remote_http",
            "base_url": "https://example.com",
        },
    )
    await app_client.post(
        "/models",
        json={
            "name": "test_model",
            "provider_name": "test_provider",
            "display_name": "Test Model",
        },
    )
    await app_client.post(
        "/models/test_provider/test_model/invoke",
        json={"prompt": "Test"},
    )

    # 等待数据写入
    import asyncio
    await asyncio.sleep(0.1)

    response = await app_client.get("/monitor/export/excel")
    assert response.status_code == 200
    assert response.headers.get("content-type") == "text/csv"
    assert "ID" in response.text
    assert "Model" in response.text


@pytest.mark.asyncio
async def test_download_database(app_client: AsyncClient) -> None:
    """测试下载数据库"""
    response = await app_client.get("/monitor/database")
    assert response.status_code == 200
    assert response.headers.get("content-type") == "application/x-sqlite3"
    assert "Content-Disposition" in response.headers


# ==================== 完整流程测试 ====================

@pytest.mark.asyncio
async def test_full_workflow(app_client: AsyncClient) -> None:
    """测试完整工作流程"""
    # 1. 创建Provider
    provider_response = await app_client.post(
        "/providers",
        json={
            "name": "workflow_provider",
            "type": "remote_http",
            "base_url": "https://example.com",
        },
    )
    assert provider_response.status_code == 201

    # 2. 创建Model
    model_response = await app_client.post(
        "/models",
        json={
            "name": "workflow_model",
            "provider_name": "workflow_provider",
            "display_name": "Workflow Model",
            "tags": ["workflow"],
        },
    )
    assert model_response.status_code == 201

    # 3. 列出模型
    list_response = await app_client.get("/models", params={"tag": "workflow"})
    assert list_response.status_code == 200
    models = list_response.json()
    assert len(models) >= 1

    # 4. 调用模型
    invoke_response = await app_client.post(
        "/models/workflow_provider/workflow_model/invoke",
        json={"prompt": "Hello from workflow"},
    )
    assert invoke_response.status_code == 200
    assert "output_text" in invoke_response.json()

    # 5. 智能路由
    route_response = await app_client.post(
        "/route/invoke",
        json={
            "query": {"tags": ["workflow"]},
            "request": {"prompt": "Route test"},
        },
    )
    assert route_response.status_code == 200

    # 6. OpenAI兼容API
    openai_response = await app_client.post(
        "/v1/chat/completions",
        json={
            "model": "workflow_provider/workflow_model",
            "messages": [{"role": "user", "content": "OpenAI test"}],
        },
    )
    assert openai_response.status_code == 200
    assert "choices" in openai_response.json()
