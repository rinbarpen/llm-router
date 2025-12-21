from __future__ import annotations

import os
from datetime import datetime, timedelta
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
async def test_monitor_record_invocation(tmp_path: Path) -> None:
    """测试monitor记录模型调用"""
    os.environ["LLM_ROUTER_DATABASE_URL"] = f"sqlite+aiosqlite:///{tmp_path / 'monitor.db'}"
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

            # 创建provider
            response = await client.post(
                "/providers",
                json={
                    "name": "test_provider",
                    "type": "remote_http",
                    "base_url": "https://example.com",
                },
            )
            assert response.status_code == 201

            # 创建model
            response = await client.post(
                "/models",
                json={
                    "name": "test_model",
                    "provider_name": "test_provider",
                    "display_name": "Test Model",
                    "tags": ["test"],
                },
            )
            assert response.status_code == 201

            # 调用模型（这会自动记录到monitor）
            response = await client.post(
                "/models/test_provider/test_model/invoke",
                json={"prompt": "Hello, world!"},
            )
            assert response.status_code == 200
            assert response.json()["output_text"] == "stub:test_model"

            # 等待一下确保数据已提交
            import asyncio
            await asyncio.sleep(0.1)

            # 获取调用历史
            response = await client.get("/monitor/invocations")
            assert response.status_code == 200
            data = response.json()
            assert "items" in data
            assert "total" in data
            assert data["total"] >= 1
            assert len(data["items"]) >= 1

            # 验证调用记录
            invocation = data["items"][0]
            assert invocation["model_name"] == "test_model"
            assert invocation["provider_name"] == "test_provider"
            assert invocation["status"] == "success"
            assert invocation["request_prompt"] == "Hello, world!"
            assert invocation["response_text"] == "stub:test_model"

    load_settings.cache_clear()
    os.environ.pop("LLM_ROUTER_DATABASE_URL", None)
    os.environ.pop("LLM_ROUTER_MODEL_STORE", None)


@pytest.mark.asyncio
async def test_monitor_get_invocations_with_filters(tmp_path: Path) -> None:
    """测试获取调用历史（带筛选条件）"""
    os.environ["LLM_ROUTER_DATABASE_URL"] = f"sqlite+aiosqlite:///{tmp_path / 'monitor_filter.db'}"
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

            # 创建两个providers和models
            await client.post(
                "/providers",
                json={
                    "name": "provider1",
                    "type": "remote_http",
                    "base_url": "https://example.com",
                },
            )
            await client.post(
                "/providers",
                json={
                    "name": "provider2",
                    "type": "remote_http",
                    "base_url": "https://example.com",
                },
            )

            await client.post(
                "/models",
                json={
                    "name": "model1",
                    "provider_name": "provider1",
                    "display_name": "Model 1",
                },
            )
            await client.post(
                "/models",
                json={
                    "name": "model2",
                    "provider_name": "provider2",
                    "display_name": "Model 2",
                },
            )

            # 调用两个模型
            await client.post(
                "/models/provider1/model1/invoke",
                json={"prompt": "Test 1"},
            )
            await client.post(
                "/models/provider2/model2/invoke",
                json={"prompt": "Test 2"},
            )

            import asyncio
            await asyncio.sleep(0.1)

            # 测试按provider筛选
            response = await client.get("/monitor/invocations?provider_name=provider1")
            assert response.status_code == 200
            data = response.json()
            assert data["total"] >= 1
            assert all(inv["provider_name"] == "provider1" for inv in data["items"])

            # 测试按model筛选
            response = await client.get("/monitor/invocations?model_name=model2")
            assert response.status_code == 200
            data = response.json()
            assert data["total"] >= 1
            assert all(inv["model_name"] == "model2" for inv in data["items"])

            # 测试按status筛选
            response = await client.get("/monitor/invocations?status=success")
            assert response.status_code == 200
            data = response.json()
            assert data["total"] >= 2
            assert all(inv["status"] == "success" for inv in data["items"])

            # 测试分页
            response = await client.get("/monitor/invocations?limit=1&offset=0")
            assert response.status_code == 200
            data = response.json()
            assert len(data["items"]) == 1
            assert data["limit"] == 1
            assert data["offset"] == 0

    load_settings.cache_clear()
    os.environ.pop("LLM_ROUTER_DATABASE_URL", None)
    os.environ.pop("LLM_ROUTER_MODEL_STORE", None)


@pytest.mark.asyncio
async def test_monitor_get_invocation_by_id(tmp_path: Path) -> None:
    """测试获取单次调用详情"""
    os.environ["LLM_ROUTER_DATABASE_URL"] = f"sqlite+aiosqlite:///{tmp_path / 'monitor_detail.db'}"
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

            # 创建provider和model
            await client.post(
                "/providers",
                json={
                    "name": "test_provider",
                    "type": "remote_http",
                    "base_url": "https://example.com",
                },
            )
            await client.post(
                "/models",
                json={
                    "name": "test_model",
                    "provider_name": "test_provider",
                    "display_name": "Test Model",
                },
            )

            # 调用模型
            await client.post(
                "/models/test_provider/test_model/invoke",
                json={"prompt": "Detail test", "parameters": {"temperature": 0.7}},
            )

            import asyncio
            await asyncio.sleep(0.1)

            # 获取调用列表以获取ID
            response = await client.get("/monitor/invocations?limit=1")
            assert response.status_code == 200
            data = response.json()
            assert len(data["items"]) > 0
            invocation_id = data["items"][0]["id"]

            # 获取单次调用详情
            response = await client.get(f"/monitor/invocations/{invocation_id}")
            assert response.status_code == 200
            invocation = response.json()
            assert invocation["id"] == invocation_id
            assert invocation["model_name"] == "test_model"
            assert invocation["provider_name"] == "test_provider"
            assert invocation["request_prompt"] == "Detail test"
            assert "request_parameters" in invocation
            assert invocation["request_parameters"]["temperature"] == 0.7

            # 测试不存在的ID
            response = await client.get("/monitor/invocations/99999")
            assert response.status_code == 404

    load_settings.cache_clear()
    os.environ.pop("LLM_ROUTER_DATABASE_URL", None)
    os.environ.pop("LLM_ROUTER_MODEL_STORE", None)


@pytest.mark.asyncio
async def test_monitor_get_statistics(tmp_path: Path) -> None:
    """测试获取统计信息"""
    os.environ["LLM_ROUTER_DATABASE_URL"] = f"sqlite+aiosqlite:///{tmp_path / 'monitor_stats.db'}"
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

            # 创建多个providers和models
            await client.post(
                "/providers",
                json={
                    "name": "provider1",
                    "type": "remote_http",
                    "base_url": "https://example.com",
                },
            )
            await client.post(
                "/providers",
                json={
                    "name": "provider2",
                    "type": "remote_http",
                    "base_url": "https://example.com",
                },
            )

            await client.post(
                "/models",
                json={
                    "name": "model1",
                    "provider_name": "provider1",
                    "display_name": "Model 1",
                },
            )
            await client.post(
                "/models",
                json={
                    "name": "model2",
                    "provider_name": "provider2",
                    "display_name": "Model 2",
                },
            )

            # 调用模型多次
            for i in range(3):
                await client.post(
                    "/models/provider1/model1/invoke",
                    json={"prompt": f"Test {i}"},
                )
            for i in range(2):
                await client.post(
                    "/models/provider2/model2/invoke",
                    json={"prompt": f"Test {i}"},
                )

            import asyncio
            await asyncio.sleep(0.1)

            # 获取统计信息
            response = await client.get("/monitor/statistics?time_range_hours=24&limit=10")
            assert response.status_code == 200
            stats = response.json()

            # 验证总体统计
            assert "overall" in stats
            overall = stats["overall"]
            assert overall["total_calls"] >= 5
            assert overall["success_calls"] >= 5
            assert overall["error_calls"] >= 0
            assert overall["success_rate"] >= 0
            assert "time_range" in overall

            # 验证按模型统计
            assert "by_model" in stats
            by_model = stats["by_model"]
            assert len(by_model) >= 2

            # 验证每个模型的统计
            model1_stats = next((m for m in by_model if m["model_name"] == "model1"), None)
            assert model1_stats is not None
            assert model1_stats["total_calls"] >= 3
            assert model1_stats["success_calls"] >= 3

            model2_stats = next((m for m in by_model if m["model_name"] == "model2"), None)
            assert model2_stats is not None
            assert model2_stats["total_calls"] >= 2
            assert model2_stats["success_calls"] >= 2

            # 验证最近错误
            assert "recent_errors" in stats
            assert isinstance(stats["recent_errors"], list)

            # 测试不同的时间范围
            response = await client.get("/monitor/statistics?time_range_hours=1&limit=5")
            assert response.status_code == 200

            # 测试无效参数
            response = await client.get("/monitor/statistics?time_range_hours=0")
            assert response.status_code == 400

            response = await client.get("/monitor/statistics?time_range_hours=200")
            assert response.status_code == 400

            response = await client.get("/monitor/statistics?limit=0")
            assert response.status_code == 400

            response = await client.get("/monitor/statistics?limit=200")
            assert response.status_code == 400

    load_settings.cache_clear()
    os.environ.pop("LLM_ROUTER_DATABASE_URL", None)
    os.environ.pop("LLM_ROUTER_MODEL_STORE", None)


@pytest.mark.asyncio
async def test_monitor_time_range_filter(tmp_path: Path) -> None:
    """测试时间范围筛选"""
    os.environ["LLM_ROUTER_DATABASE_URL"] = f"sqlite+aiosqlite:///{tmp_path / 'monitor_time.db'}"
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

            # 创建provider和model
            await client.post(
                "/providers",
                json={
                    "name": "test_provider",
                    "type": "remote_http",
                    "base_url": "https://example.com",
                },
            )
            await client.post(
                "/models",
                json={
                    "name": "test_model",
                    "provider_name": "test_provider",
                    "display_name": "Test Model",
                },
            )

            # 调用模型
            await client.post(
                "/models/test_provider/test_model/invoke",
                json={"prompt": "Time test"},
            )

            import asyncio
            await asyncio.sleep(0.1)

            # 测试时间范围筛选
            now = datetime.utcnow()
            start_time = (now - timedelta(hours=1)).isoformat()
            end_time = (now + timedelta(hours=1)).isoformat()

            response = await client.get(
                f"/monitor/invocations?start_time={start_time}&end_time={end_time}"
            )
            assert response.status_code == 200
            data = response.json()
            assert data["total"] >= 1

            # 测试无效时间格式
            response = await client.get("/monitor/invocations?start_time=invalid")
            assert response.status_code == 400

    load_settings.cache_clear()
    os.environ.pop("LLM_ROUTER_DATABASE_URL", None)
    os.environ.pop("LLM_ROUTER_MODEL_STORE", None)

