from __future__ import annotations

import os
from pathlib import Path
from typing import Any, Dict, List, Optional

import tomli
from pydantic import BaseModel, Field, ValidationError

from .db.models import ProviderType
from .schemas import ModelCreate, ProviderCreate, RateLimitConfig
from .services import ModelService


class ProviderConfig(BaseModel):
    name: str
    type: ProviderType
    base_url: Optional[str] = None
    api_key: Optional[str] = None
    api_key_env: Optional[str] = None
    is_active: bool = True
    settings: Dict[str, Any] = Field(default_factory=dict)

    def resolved_api_key(self) -> Optional[str]:
        if self.api_key:
            return self.api_key
        if self.api_key_env:
            return os.getenv(self.api_key_env)
        return None


class RateLimitEntry(BaseModel):
    max_requests: int
    per_seconds: int
    burst_size: Optional[int] = None
    notes: Optional[str] = None
    config: Dict[str, Any] = Field(default_factory=dict)

    def to_schema(self) -> RateLimitConfig:
        return RateLimitConfig(
            max_requests=self.max_requests,
            per_seconds=self.per_seconds,
            burst_size=self.burst_size,
            notes=self.notes,
            config=self.config,
        )


class ModelConfigEntry(BaseModel):
    name: str
    provider: str
    display_name: Optional[str] = None
    description: Optional[str] = None
    remote_identifier: Optional[str] = None
    is_active: bool = True
    tags: List[str] = Field(default_factory=list)
    default_params: Dict[str, Any] = Field(default_factory=dict)
    config: Dict[str, Any] = Field(default_factory=dict)
    download_uri: Optional[str] = None
    local_path: Optional[str] = None
    rate_limit: Optional[RateLimitEntry] = None

    def to_schema(self) -> ModelCreate:
        return ModelCreate(
            name=self.name,
            provider_name=self.provider,
            display_name=self.display_name,
            description=self.description,
            remote_identifier=self.remote_identifier,
            is_active=self.is_active,
            tags=self.tags,
            default_params=self.default_params,
            config=self.config,
            download_uri=self.download_uri,
            local_path=self.local_path,
            rate_limit=self.rate_limit.to_schema() if self.rate_limit else None,
        )


class RouterModelConfig(BaseModel):
    providers: List[ProviderConfig] = Field(default_factory=list)
    models: List[ModelConfigEntry] = Field(default_factory=list)


def load_model_config(path: Path) -> RouterModelConfig:
    with path.open("rb") as fh:
        data = tomli.load(fh)
    try:
        return RouterModelConfig.model_validate(data)
    except ValidationError as exc:  # pragma: no cover - configuration error
        raise ValueError(f"配置文件解析失败: {exc}") from exc


async def apply_model_config(
    config: RouterModelConfig,
    service: ModelService,
    session_factory,
) -> None:
    async with session_factory() as session:
        # Providers
        for provider_cfg in config.providers:
            await service.upsert_provider(
                session,
                ProviderCreate(
                    name=provider_cfg.name,
                    type=provider_cfg.type,
                    base_url=provider_cfg.base_url,
                    api_key=provider_cfg.resolved_api_key(),
                    is_active=provider_cfg.is_active,
                    settings=provider_cfg.settings,
                ),
            )

        # Models
        for model_cfg in config.models:
            payload = model_cfg.to_schema()
            await service.register_model(session, payload)


