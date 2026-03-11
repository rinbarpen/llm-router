from __future__ import annotations

import os
from pathlib import Path

import pytest

from llm_router.config import RouterSettings
from llm_router.db import create_engine, create_session_factory, init_db
from llm_router.model_config import apply_model_config, load_model_config
from llm_router.schemas import ModelQuery
from llm_router.services import ModelDownloader, ModelService, RateLimiterManager


@pytest.mark.asyncio
async def test_apply_model_config(tmp_path: Path) -> None:
    config_path = tmp_path / "router.toml"
    config_path.write_text(
        """
[[providers]]
name = "openai"
type = "openai"
api_key = "sk-test"

[[models]]
name = "gpt-4o-mini"
provider = "openai"
tags = ["chat"]
[models.rate_limit]
max_requests = 10
per_seconds = 60
[models.config]
context_window = "128k"
supports_vision = true
"""
    )

    settings = RouterSettings(
        database_url=f"sqlite+aiosqlite:///{tmp_path / 'cfg.db'}",
        model_store_dir=tmp_path / "store",
        model_config_file=config_path,
    )

    engine = create_engine(settings.database_url)
    await init_db(engine)
    session_factory = create_session_factory(engine)

    downloader = ModelDownloader(settings)
    rate_limiter = RateLimiterManager()
    service = ModelService(downloader, rate_limiter)

    config = load_model_config(config_path)
    await apply_model_config(config, service, session_factory)

    async with session_factory() as session:
        models = await service.list_models(session, ModelQuery(tags=["chat"]))
        assert len(models) == 1
        model = models[0]
        assert model.name == "gpt-4o-mini"
        assert model.rate_limit is not None
        assert model.config.get("context_window") == "128k"
        assert model.config.get("supports_vision") is True

    await engine.dispose()


@pytest.mark.asyncio
async def test_apply_model_config_with_audio_capabilities(tmp_path: Path) -> None:
    config_path = tmp_path / "router-audio.toml"
    config_path.write_text(
        """
[[providers]]
name = "qwen"
type = "qwen"
api_key = "dashscope-key"

[[models]]
name = "qwen3-tts-flash"
provider = "qwen"
tags = ["tts", "qwen"]
[models.config.capabilities]
tts = true
asr = false
"""
    )

    settings = RouterSettings(
        database_url=f"sqlite+aiosqlite:///{tmp_path / 'audio-cfg.db'}",
        model_store_dir=tmp_path / "store",
        model_config_file=config_path,
    )

    engine = create_engine(settings.database_url)
    await init_db(engine)
    session_factory = create_session_factory(engine)

    downloader = ModelDownloader(settings)
    rate_limiter = RateLimiterManager()
    service = ModelService(downloader, rate_limiter)

    config = load_model_config(config_path)
    await apply_model_config(config, service, session_factory)

    async with session_factory() as session:
        model = await service.get_model_by_name(session, "qwen", "qwen3-tts-flash")
        assert model is not None
        assert model.config["capabilities"]["tts"] is True
        assert model.config["capabilities"]["asr"] is False
        assert "tts" in [tag.name for tag in model.tags]

    await engine.dispose()


@pytest.mark.asyncio
async def test_get_model_by_remote_identifier(tmp_path: Path) -> None:
    """验证按 remote_identifier 查找模型（支持 openrouter/free 等 OpenRouter 模型 ID）"""
    config_path = tmp_path / "router-openrouter.toml"
    config_path.write_text(
        """
[[providers]]
name = "openrouter"
type = "openrouter"
api_key = "sk-test"

[[models]]
name = "openrouter-free"
provider = "openrouter"
remote_identifier = "openrouter/free"
display_name = "OpenRouter: Auto Free"
tags = ["chat"]
[models.config]
context_window = "128k"
"""
    )

    settings = RouterSettings(
        database_url=f"sqlite+aiosqlite:///{tmp_path / 'openrouter.db'}",
        model_store_dir=tmp_path / "store",
        model_config_file=config_path,
    )

    engine = create_engine(settings.database_url)
    await init_db(engine)
    session_factory = create_session_factory(engine)

    downloader = ModelDownloader(settings)
    rate_limiter = RateLimiterManager()
    service = ModelService(downloader, rate_limiter)

    config = load_model_config(config_path)
    await apply_model_config(config, service, session_factory)

    async with session_factory() as session:
        # 按 name 查找失败（用户传 openrouter/free 解析为 model_name=free）
        by_name = await service.get_model_by_name(session, "openrouter", "free")
        assert by_name is None

        # 按 remote_identifier 查找成功
        by_remote = await service.get_model_by_remote_identifier(
            session, "openrouter", "openrouter/free"
        )
        assert by_remote is not None
        assert by_remote.name == "openrouter-free"
        assert by_remote.remote_identifier == "openrouter/free"

    await engine.dispose()
