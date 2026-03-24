from __future__ import annotations

import os
from pathlib import Path

import pytest
from asgi_lifespan import LifespanManager
from sqlalchemy import select, func

from llm_router.api.app import create_app
from llm_router.config import load_settings
from llm_router.db.models import Provider


@pytest.mark.asyncio
async def test_pytest_env_does_not_auto_sync_default_router_toml(tmp_path: Path) -> None:
    """测试环境下，未显式指定 LLM_ROUTER_MODEL_CONFIG 时不应自动同步项目根 router.toml。"""
    os.environ["LLM_ROUTER_DATABASE_URL"] = f"sqlite+aiosqlite:///{tmp_path / 'test.db'}"
    os.environ["LLM_ROUTER_MONITOR_DATABASE_URL"] = (
        f"sqlite+aiosqlite:///{tmp_path / 'monitor.db'}"
    )
    os.environ["LLM_ROUTER_MODEL_STORE"] = str(tmp_path / "models")
    os.environ.pop("LLM_ROUTER_MODEL_CONFIG", None)
    load_settings.cache_clear()

    app = create_app()
    async with LifespanManager(app):
        session_factory = app.state.session_factory
        async with session_factory() as session:
            provider_count = await session.scalar(
                select(func.count()).select_from(Provider)
            )
            assert provider_count == 0

    load_settings.cache_clear()
    for key in (
        "LLM_ROUTER_DATABASE_URL",
        "LLM_ROUTER_MONITOR_DATABASE_URL",
        "LLM_ROUTER_MODEL_STORE",
    ):
        os.environ.pop(key, None)
