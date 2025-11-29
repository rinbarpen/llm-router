from __future__ import annotations

import os
from functools import lru_cache
from pathlib import Path
from typing import Dict, List, Optional, Set

from pydantic import BaseModel, Field, field_validator

from .api_key_config import APIKeyConfig
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
    api_keys: List[APIKeyConfig] = Field(default_factory=list)
    require_auth: bool = Field(default=True)  # 默认开启认证
    host: str = Field(default="0.0.0.0", description="服务绑定的主机地址")
    port: int = Field(default=8000, ge=1, le=65535, description="服务绑定的端口")
    host: str = Field(default="0.0.0.0")
    port: int = Field(default=8000, ge=1, le=65535)

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

    @field_validator("api_keys", mode="before")
    @classmethod
    def _validate_api_keys(cls, value: str | List[str] | List[APIKeyConfig] | None) -> List[APIKeyConfig]:
        if value is None:
            return []
        if isinstance(value, str):
            # 支持逗号分隔的简单 API Key（向后兼容）
            keys = [k.strip() for k in value.split(",") if k.strip()]
            return [APIKeyConfig(key=k) for k in keys]
        if isinstance(value, list):
            result = []
            for item in value:
                if isinstance(item, APIKeyConfig):
                    # 如果已经是 APIKeyConfig，解析环境变量（支持多个 key）
                    resolved_keys = item.resolved_keys()
                    for idx, key in enumerate(resolved_keys):
                        name = item.name
                        if len(resolved_keys) > 1:
                            name = f"{item.name or 'API Key'} #{idx + 1}"
                        result.append(APIKeyConfig(
                            key=key,
                            name=name,
                            allowed_models=item.allowed_models,
                            allowed_providers=item.allowed_providers,
                            parameter_limits=item.parameter_limits,
                            is_active=item.is_active,
                        ))
                elif isinstance(item, str):
                    result.append(APIKeyConfig(key=item.strip()))
                elif isinstance(item, dict):
                    # 从字典创建，支持 key_env（支持多个 key）
                    api_key_cfg = APIKeyConfig(**item)
                    resolved_keys = api_key_cfg.resolved_keys()
                    for idx, key in enumerate(resolved_keys):
                        name = api_key_cfg.name
                        if len(resolved_keys) > 1:
                            name = f"{api_key_cfg.name or 'API Key'} #{idx + 1}"
                        result.append(APIKeyConfig(
                            key=key,
                            name=name,
                            allowed_models=api_key_cfg.allowed_models,
                            allowed_providers=api_key_cfg.allowed_providers,
                            parameter_limits=api_key_cfg.parameter_limits,
                            is_active=api_key_cfg.is_active,
                        ))
            return result
        return []

    def get_api_key_config(self, key: str) -> Optional[APIKeyConfig]:
        """根据 key 值获取 API Key 配置"""
        for api_key_config in self.api_keys:
            if api_key_config.key == key:
                return api_key_config
        return None

    def has_api_keys(self) -> bool:
        """检查是否配置了 API Key"""
        return len(self.api_keys) > 0

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
        "api_keys": os.getenv("LLM_ROUTER_API_KEYS"),  # 向后兼容：支持简单字符串
        "require_auth": os.getenv("LLM_ROUTER_REQUIRE_AUTH", "true").lower() in ("true", "1", "yes"),
        "host": os.getenv("LLM_ROUTER_HOST", "0.0.0.0"),
        "port": int(os.getenv("LLM_ROUTER_PORT", "8000")),
    }

    data = {key: value for key, value in env_mapping.items() if value is not None}
    settings = RouterSettings(**data)
    settings.ensure_directories()
    return settings


__all__ = ["RouterSettings", "load_settings"]


