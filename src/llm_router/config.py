from __future__ import annotations

import os
from functools import lru_cache
from pathlib import Path
from typing import Optional

from pydantic import BaseModel, Field, field_validator

from .db import DEFAULT_DB_FILENAME, build_sqlite_url


def _default_database_url() -> str:
    return build_sqlite_url(Path.cwd() / DEFAULT_DB_FILENAME)


def _default_model_store() -> Path:
    return Path.cwd() / "model_store"


class RouterSettings(BaseModel):
    """Runtime configuration loaded from environment variables."""

    database_url: str = Field(default_factory=_default_database_url)
    model_store_dir: Path = Field(default_factory=_default_model_store)
    download_cache_dir: Optional[Path] = None
    download_concurrency: int = Field(default=2, ge=1)
    default_timeout: float = Field(default=60.0, gt=0)
    log_level: str = Field(default="INFO")
    model_config_file: Optional[Path] = None

    @field_validator("model_store_dir", mode="before")
    @classmethod
    def _validate_model_store(cls, value: str | Path) -> Path:
        return Path(value).expanduser().resolve()

    @field_validator("download_cache_dir", mode="before")
    @classmethod
    def _validate_cache_dir(cls, value: str | Path | None) -> Optional[Path]:
        if value is None:
            return None
        return Path(value).expanduser().resolve()

    @field_validator("model_config_file", mode="before")
    @classmethod
    def _validate_model_config(cls, value: str | Path | None) -> Optional[Path]:
        if value is None:
            return None
        return Path(value).expanduser().resolve()

    def ensure_directories(self) -> None:
        self.model_store_dir.mkdir(parents=True, exist_ok=True)
        if self.download_cache_dir:
            self.download_cache_dir.mkdir(parents=True, exist_ok=True)


@lru_cache(1)
def load_settings() -> RouterSettings:
    """Load settings from environment variables, caching the result."""

    env_mapping = {
        "database_url": os.getenv("LLM_ROUTER_DATABASE_URL"),
        "model_store_dir": os.getenv("LLM_ROUTER_MODEL_STORE"),
        "download_cache_dir": os.getenv("LLM_ROUTER_DOWNLOAD_CACHE"),
        "download_concurrency": os.getenv("LLM_ROUTER_DOWNLOAD_CONCURRENCY"),
        "default_timeout": os.getenv("LLM_ROUTER_DEFAULT_TIMEOUT"),
        "log_level": os.getenv("LLM_ROUTER_LOG_LEVEL"),
        "model_config_file": os.getenv("LLM_ROUTER_MODEL_CONFIG"),
    }

    data = {key: value for key, value in env_mapping.items() if value is not None}
    settings = RouterSettings(**data)
    settings.ensure_directories()
    return settings


__all__ = ["RouterSettings", "load_settings"]


