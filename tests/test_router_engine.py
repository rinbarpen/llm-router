from __future__ import annotations

import pytest
from pathlib import Path

from llm_router.config import RouterSettings
from llm_router.db import create_engine, create_session_factory, init_db
from llm_router.db.models import ProviderType
from llm_router.schemas import ModelCreate, ProviderCreate, ModelInvokeRequest, ModelInvokeResponse
from llm_router.services import ModelDownloader, ModelService, RateLimiterManager, RouterEngine
from llm_router.providers import ProviderRegistry

class StubProviderClient:
    def __init__(self, provider, settings):
        self.provider = provider
        self.settings = settings
    
    async def invoke(self, model, request: ModelInvokeRequest) -> ModelInvokeResponse:
        return ModelInvokeResponse(
            output_text=f"resp:{model.name}:{self.provider.name}",
            raw={"id": "test-id"}
        )

@pytest.mark.asyncio
async def test_route_by_name(tmp_path: Path) -> None:
    db_url = f"sqlite+aiosqlite:///{tmp_path / 'engine.db'}"
    settings = RouterSettings(
        database_url=db_url,
        model_store_dir=tmp_path / "models",
    )

    engine_db = create_engine(settings.database_url)
    await init_db(engine_db)
    session_factory = create_session_factory(engine_db)

    downloader = ModelDownloader(settings)
    rate_limiter = RateLimiterManager()
    model_service = ModelService(downloader, rate_limiter)
    
    class StubRegistry(ProviderRegistry):
        def get(self, provider):
            return StubProviderClient(provider, self.settings)

    registry = StubRegistry(settings)
    router_engine = RouterEngine(model_service, registry, rate_limiter)

    async with session_factory() as session:
        p1 = await model_service.upsert_provider(
            session,
            ProviderCreate(name="p1", type=ProviderType.OPENAI)
        )
        p2 = await model_service.upsert_provider(
            session,
            ProviderCreate(name="p2", type=ProviderType.OPENAI)
        )

        # p1 has gpt-4 with priority 10
        await model_service.register_model(
            session,
            ModelCreate(name="gpt-4", provider_id=p1.id, config={"priority": 10})
        )
        # p2 has gpt-4 with priority 20
        await model_service.register_model(
            session,
            ModelCreate(name="gpt-4", provider_id=p2.id, config={"priority": 20})
        )

        # Should pick p2 because of higher priority
        resp = await router_engine.route_by_name(
            session, "gpt-4", ModelInvokeRequest(prompt="hi")
        )
        assert resp.output_text == "resp:gpt-4:p2"

    await engine_db.dispose()
