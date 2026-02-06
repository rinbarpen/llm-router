from __future__ import annotations

import contextlib
import logging
from pathlib import Path
from typing import Any, AsyncIterator

from sqlalchemy import event, text
from sqlalchemy.ext.asyncio import (
    AsyncEngine,
    AsyncSession,
    async_sessionmaker,
    create_async_engine,
)

from .base import Base

logger = logging.getLogger(__name__)

DEFAULT_DB_FILENAME = "llm_router.db"


def build_sqlite_url(db_path: Path) -> str:
    absolute = db_path.expanduser().resolve()
    return f"sqlite+aiosqlite:///{absolute}"


def create_engine(database_url: str, echo: bool = False) -> AsyncEngine:
    engine = create_async_engine(database_url, echo=echo, future=True)

    if database_url.startswith("sqlite"):

        @event.listens_for(engine.sync_engine, "connect")
        def _set_sqlite_pragma(dbapi_connection: Any, _: Any) -> None:
            cursor = dbapi_connection.cursor()
            cursor.execute("PRAGMA foreign_keys=ON")
            cursor.close()

    return engine


def create_session_factory(engine: AsyncEngine) -> async_sessionmaker[AsyncSession]:
    return async_sessionmaker(engine, expire_on_commit=False)


def _migrate_api_keys_key_nullable_sync(sync_conn: Any) -> None:
    """SQLite: 将 api_keys.key 改为可空（通过重建表），便于模型标签行 key=NULL。"""
    r = sync_conn.execute(text("SELECT name FROM sqlite_master WHERE type='table' AND name='api_keys'"))
    if r.fetchone() is None:
        return
    r = sync_conn.execute(text("PRAGMA table_info(api_keys)"))
    rows = r.fetchall()
    # sqlite table_info: (cid, name, type, notnull, dflt_value, pk)
    key_info = next((row for row in rows if row[1] == "key"), None)
    if key_info is None or key_info[3] == 0:
        return  # 已可空或无 key 列，无需迁移
    sync_conn.execute(text("PRAGMA foreign_keys=OFF"))
    sync_conn.execute(text("""
        CREATE TABLE api_keys_new (
            id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
            key VARCHAR(512),
            name VARCHAR(255),
            is_active BOOLEAN NOT NULL,
            allowed_models JSON,
            allowed_providers JSON,
            parameter_limits JSON,
            created_at DATETIME NOT NULL,
            updated_at DATETIME NOT NULL
        )
    """))
    sync_conn.execute(text("""
        INSERT INTO api_keys_new (id, key, name, is_active, allowed_models, allowed_providers, parameter_limits, created_at, updated_at)
        SELECT id, key, name, is_active, allowed_models, allowed_providers, parameter_limits, created_at, updated_at FROM api_keys
    """))
    sync_conn.execute(text("DROP TABLE api_keys"))
    sync_conn.execute(text("ALTER TABLE api_keys_new RENAME TO api_keys"))
    sync_conn.execute(text("CREATE UNIQUE INDEX ix_api_keys_key ON api_keys (key)"))
    sync_conn.execute(text("CREATE INDEX ix_api_keys_is_active ON api_keys (is_active)"))
    sync_conn.execute(text("PRAGMA foreign_keys=ON"))
    logger.info("Migrated api_keys.key to nullable for model tags support.")


async def init_db(engine: AsyncEngine) -> None:
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)
    # SQLite: 兼容旧库，将 api_keys.key 改为可空
    if str(engine.url).startswith("sqlite"):
        async with engine.begin() as conn:
            await conn.run_sync(_migrate_api_keys_key_nullable_sync)


@contextlib.asynccontextmanager
async def session_scope(
    session_factory: async_sessionmaker[AsyncSession],
) -> AsyncIterator[AsyncSession]:
    session = session_factory()
    try:
        yield session
        await session.commit()
    except Exception:
        await session.rollback()
        raise
    finally:
        await session.close()


