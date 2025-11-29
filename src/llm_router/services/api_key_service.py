from __future__ import annotations

from typing import List, Optional

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from ..api_key_config import APIKeyConfig, ParameterLimits
from ..db.models import APIKey


class APIKeyService:
    """API Key 服务，用于管理数据库中的 API Key"""

    async def create_api_key(
        self,
        session: AsyncSession,
        key: str,
        name: Optional[str] = None,
        is_active: bool = True,
        allowed_models: Optional[List[str]] = None,
        allowed_providers: Optional[List[str]] = None,
        parameter_limits: Optional[ParameterLimits] = None,
    ) -> APIKey:
        """创建新的 API Key"""
        # 检查 key 是否已存在
        existing = await self.get_api_key_by_key(session, key)
        if existing:
            raise ValueError(f"API Key '{key}' 已存在")

        api_key = APIKey(
            key=key,
            name=name,
            is_active=is_active,
            allowed_models=allowed_models,
            allowed_providers=allowed_providers,
            parameter_limits=parameter_limits.model_dump() if parameter_limits else None,
        )
        session.add(api_key)
        await session.flush()
        return api_key

    async def get_api_key_by_id(self, session: AsyncSession, api_key_id: int) -> Optional[APIKey]:
        """根据 ID 获取 API Key"""
        stmt = select(APIKey).where(APIKey.id == api_key_id)
        return await session.scalar(stmt)

    async def get_api_key_by_key(self, session: AsyncSession, key: str) -> Optional[APIKey]:
        """根据 key 值获取 API Key"""
        stmt = select(APIKey).where(APIKey.key == key)
        return await session.scalar(stmt)

    async def list_api_keys(
        self, session: AsyncSession, include_inactive: bool = False
    ) -> List[APIKey]:
        """列出所有 API Key"""
        stmt = select(APIKey)
        if not include_inactive:
            stmt = stmt.where(APIKey.is_active == True)
        stmt = stmt.order_by(APIKey.created_at.desc())
        result = await session.scalars(stmt)
        return list(result.all())

    async def update_api_key(
        self,
        session: AsyncSession,
        api_key: APIKey,
        name: Optional[str] = None,
        is_active: Optional[bool] = None,
        allowed_models: Optional[List[str]] = None,
        allowed_providers: Optional[List[str]] = None,
        parameter_limits: Optional[ParameterLimits] = None,
    ) -> APIKey:
        """更新 API Key"""
        if name is not None:
            api_key.name = name
        if is_active is not None:
            api_key.is_active = is_active
        if allowed_models is not None:
            api_key.allowed_models = allowed_models
        if allowed_providers is not None:
            api_key.allowed_providers = allowed_providers
        if parameter_limits is not None:
            api_key.parameter_limits = parameter_limits.model_dump()
        await session.flush()
        return api_key

    async def delete_api_key(self, session: AsyncSession, api_key: APIKey) -> None:
        """删除 API Key"""
        await session.delete(api_key)
        await session.flush()

    def to_api_key_config(self, api_key: APIKey) -> APIKeyConfig:
        """将数据库模型转换为 APIKeyConfig"""
        parameter_limits = None
        if api_key.parameter_limits:
            parameter_limits = ParameterLimits(**api_key.parameter_limits)

        return APIKeyConfig(
            key=api_key.key,
            name=api_key.name,
            allowed_models=api_key.allowed_models,
            allowed_providers=api_key.allowed_providers,
            parameter_limits=parameter_limits,
            is_active=api_key.is_active,
        )

    async def load_all_active_keys(self, session: AsyncSession) -> List[APIKeyConfig]:
        """加载所有激活的 API Key 配置"""
        api_keys = await self.list_api_keys(session, include_inactive=False)
        return [self.to_api_key_config(api_key) for api_key in api_keys]


__all__ = ["APIKeyService"]

