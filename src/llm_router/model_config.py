from __future__ import annotations

import os
from pathlib import Path
from typing import Any, Dict, List, Optional

import tomli
from pydantic import BaseModel, Field, ValidationError

from .api_key_config import APIKeyConfig, ParameterLimits
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
        """获取第一个 API key（向后兼容）"""
        keys = self.resolved_api_keys()
        return keys[0] if keys else None

    def resolved_api_keys(self) -> List[str]:
        """从环境变量或直接值解析 API Key（支持多个，逗号分隔）"""
        if self.api_key:
            # 支持逗号分隔的多个 key
            keys = [k.strip() for k in self.api_key.split(",") if k.strip()]
            return keys
        if self.api_key_env:
            env_value = os.getenv(self.api_key_env)
            if env_value:
                # 支持逗号分隔的多个 key
                keys = [k.strip() for k in env_value.split(",") if k.strip()]
                return keys
        return []


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


class ServerConfig(BaseModel):
    """服务器配置"""
    host: Optional[str] = Field(default=None, description="服务绑定的主机地址")
    port: Optional[int] = Field(default=None, ge=1, le=65535, description="服务绑定的端口")


class FrontendConfig(BaseModel):
    """前端配置"""
    port: Optional[int] = Field(default=None, ge=1, le=65535, description="前端开发服务器端口")
    api_url: Optional[str] = Field(default=None, description="后端API服务器地址（开发环境代理用）")
    api_base_url: Optional[str] = Field(default=None, description="生产环境API基础路径")


class RouterModelConfig(BaseModel):
    providers: List[ProviderConfig] = Field(default_factory=list)
    models: List[ModelConfigEntry] = Field(default_factory=list)
    api_keys: List[APIKeyConfig] = Field(default_factory=list)
    server: Optional[ServerConfig] = Field(default=None, description="服务器配置")
    frontend: Optional[FrontendConfig] = Field(default=None, description="前端配置")


def load_model_config(path: Path) -> RouterModelConfig:
    with path.open("rb") as fh:
        data = tomli.load(fh)
    
    # 支持嵌套格式：从 provider_name.models 中提取模型配置
    # 例如：[[glm.models]] 会被解析为 data['glm']['models']
    all_models = []
    
    # 首先收集标准的 [[models]] 配置
    if "models" in data:
        all_models.extend(data["models"])
    
    # 然后收集嵌套在 provider 下的模型配置
    # 遍历所有顶级键，查找可能的 provider.models 结构
    for key, value in data.items():
        if key != "models" and key != "providers" and key != "api_keys" and key != "server" and key != "frontend":
            # 检查是否是 provider.models 结构
            if isinstance(value, dict) and "models" in value:
                provider_models = value["models"]
                if isinstance(provider_models, list):
                    all_models.extend(provider_models)
    
    # 将收集到的模型配置合并回 data
    if all_models:
        data["models"] = all_models
    
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
            # 获取所有 API keys（支持多个，逗号分隔）
            api_keys = provider_cfg.resolved_api_keys()
            # 如果有多个 key，用逗号连接存储；如果只有一个，直接使用；如果没有，为 None
            api_key_value = ",".join(api_keys) if api_keys else None
            
            await service.upsert_provider(
                session,
                ProviderCreate(
                    name=provider_cfg.name,
                    type=provider_cfg.type,
                    base_url=provider_cfg.base_url,
                    api_key=api_key_value,
                    is_active=provider_cfg.is_active,
                    settings=provider_cfg.settings,
                ),
            )

        await session.commit()  # 确保 Provider 已提交

        # Models
        for model_cfg in config.models:
            payload = model_cfg.to_schema()
            await service.register_model(session, payload)
        
        await session.commit()  # 确保 Models 已提交
