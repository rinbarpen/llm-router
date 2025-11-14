from __future__ import annotations

from pathlib import Path

import pytest

from llm_router.config import RouterSettings
from llm_router.db import create_engine, create_session_factory, init_db
from llm_router.db.models import ProviderType
from llm_router.schemas import ModelCreate, ProviderCreate, RateLimitConfig
from llm_router.services import ModelDownloader, ModelService, RateLimiterManager


@pytest.mark.asyncio
async def test_register_model_with_tags_and_rate_limit(tmp_path: Path) -> None:
    db_url = f"sqlite+aiosqlite:///{tmp_path / 'service.db'}"
    settings = RouterSettings(
        database_url=db_url,
        model_store_dir=tmp_path / "models",
        download_cache_dir=tmp_path / "cache",
    )

    engine = create_engine(settings.database_url)
    await init_db(engine)
    session_factory = create_session_factory(engine)

    downloader = ModelDownloader(settings)
    rate_limiter = RateLimiterManager()
    service = ModelService(downloader, rate_limiter)

    async with session_factory() as session:
        provider = await service.upsert_provider(
            session,
            ProviderCreate(
                name="remote-provider",
                type=ProviderType.REMOTE_HTTP,
                base_url="https://example.com",
            ),
        )

        model = await service.register_model(
            session,
            ModelCreate(
                name="alpha",
                provider_id=provider.id,
                display_name="Alpha Model",
                tags=["chat", "general"],
                rate_limit=RateLimitConfig(
                    max_requests=5,
                    per_seconds=60,
                    burst_size=10,
                ),
            ),
        )

        assert model.provider_id == provider.id
        assert sorted(tag.name for tag in model.tags) == ["chat", "general"]
        assert model.rate_limit is not None
        assert rate_limiter.get_bucket(model.id) is not None

    await engine.dispose()


