from __future__ import annotations

from typing import Iterable, List, Optional

from sqlalchemy import and_, delete, func, select
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy.orm import selectinload

from ..db.models import Model, Provider, ProviderType, RateLimit, Tag
from ..schemas import (
    ModelCreate,
    ModelQuery,
    ModelRead,
    ModelUpdate,
    ProviderCreate,
    RateLimitConfig,
)
from .download import ModelDownloader
from .rate_limit import RateLimiterManager


class ModelService:
    def __init__(
        self,
        downloader: ModelDownloader,
        rate_limiter: RateLimiterManager,
    ) -> None:
        self.downloader = downloader
        self.rate_limiter = rate_limiter

    async def upsert_provider(
        self, session: AsyncSession, payload: ProviderCreate
    ) -> Provider:
        stmt = select(Provider).where(Provider.name == payload.name)
        provider = await session.scalar(stmt)

        if provider is None:
            provider = Provider(
                name=payload.name,
                type=payload.type,
                is_active=payload.is_active,
                base_url=payload.base_url,
                api_key=payload.api_key,
                settings=payload.settings,
            )
            session.add(provider)
        else:
            provider.type = payload.type
            provider.is_active = payload.is_active
            provider.base_url = payload.base_url
            provider.api_key = payload.api_key
            provider.settings = payload.settings

        await session.flush()
        return provider

    async def register_model(
        self, session: AsyncSession, payload: ModelCreate
    ) -> Model:
        provider = await self._resolve_provider(session, payload)

        stmt = select(Model).where(
            and_(Model.provider_id == provider.id, Model.name == payload.name)
        )
        model = await session.scalar(stmt)

        created = False
        if model is None:
            model = Model(provider_id=provider.id, name=payload.name)
            model.provider = provider
            session.add(model)
            created = True
        else:
            model.provider = provider

        model.display_name = payload.display_name or payload.name
        model.description = payload.description
        model.remote_identifier = payload.remote_identifier
        model.is_active = payload.is_active
        model.default_params = payload.default_params
        model.config = payload.config
        model.download_uri = payload.download_uri
        if payload.local_path:
            model.local_path = payload.local_path

        await session.flush()

        await session.refresh(model, attribute_names=["tags"])
        tags = await self._ensure_tags(session, payload.tags)
        model.tags = list(tags)

        await session.flush()

        await self._synchronize_rate_limit(session, model, payload.rate_limit)

        download_path = await self.downloader.ensure_available(provider, model)
        if download_path and model.local_path != str(download_path):
            model.local_path = str(download_path)
            await session.flush()

        await session.refresh(model, attribute_names=["provider", "tags", "rate_limit"])

        return model

    async def update_model(
        self, session: AsyncSession, model: Model, payload: ModelUpdate
    ) -> Model:
        if payload.display_name is not None:
            model.display_name = payload.display_name
        if payload.description is not None:
            model.description = payload.description
        if payload.is_active is not None:
            model.is_active = payload.is_active
        if payload.default_params is not None:
            model.default_params = payload.default_params
        if payload.config is not None:
            model.config = payload.config
        if payload.download_uri is not None:
            model.download_uri = payload.download_uri
        if payload.local_path is not None:
            model.local_path = payload.local_path

        if "tags" in payload.model_fields_set:
            await session.refresh(model, attribute_names=["tags"])
            tags = await self._ensure_tags(session, payload.tags or [])
            model.tags = list(tags)

        await session.flush()

        if "rate_limit" in payload.model_fields_set:
            await self._synchronize_rate_limit(session, model, payload.rate_limit)

        provider = await session.get(Provider, model.provider_id)
        if provider:
            download_path = await self.downloader.ensure_available(provider, model)
            if download_path and model.local_path != str(download_path):
                model.local_path = str(download_path)
                await session.flush()

        await session.refresh(model, attribute_names=["provider", "tags", "rate_limit"])

        return model

    async def remove_model(
        self, session: AsyncSession, model: Model, delete_rate_limit: bool = True
    ) -> None:
        if delete_rate_limit:
            await session.execute(
                delete(RateLimit).where(RateLimit.model_id == model.id)
            )
            self.rate_limiter.remove(model.id)
        await session.delete(model)
        await session.flush()

    async def list_models(
        self, session: AsyncSession, query: ModelQuery
    ) -> List[ModelRead]:
        stmt = select(Model).options(
            selectinload(Model.provider),
            selectinload(Model.tags),
            selectinload(Model.rate_limit),
        ).join(Model.provider)

        if not query.include_inactive:
            stmt = stmt.where(Model.is_active.is_(True), Provider.is_active.is_(True))

        if query.name:
            stmt = stmt.where(Model.name == query.name)

        if query.provider_types:
            stmt = stmt.where(Provider.type.in_(query.provider_types))

        if query.tags:
            stmt = (
                stmt.join(Model.tags)
                .where(Tag.name.in_(query.tags))
                .group_by(Model.id, Provider.id)
                .having(func.count(func.distinct(Tag.name)) >= len(set(query.tags)))
            )
        else:
            stmt = stmt.group_by(Model.id, Provider.id)

        result = await session.scalars(stmt)
        models = result.unique().all()

        return [self.to_model_read(model) for model in models]

    async def get_model_by_id(self, session: AsyncSession, model_id: int) -> Optional[Model]:
        stmt = (
            select(Model)
            .where(Model.id == model_id)
            .options(
                selectinload(Model.provider),
                selectinload(Model.tags),
                selectinload(Model.rate_limit),
            )
        )
        return await session.scalar(stmt)

    async def get_model_by_name(
        self, session: AsyncSession, provider_name: str, model_name: str
    ) -> Optional[Model]:
        stmt = (
            select(Model)
            .join(Model.provider)
            .where(Provider.name == provider_name, Model.name == model_name)
            .options(
                selectinload(Model.provider),
                selectinload(Model.tags),
                selectinload(Model.rate_limit),
            )
        )
        return await session.scalar(stmt)

    async def _ensure_tags(
        self, session: AsyncSession, tag_names: Iterable[str]
    ) -> List[Tag]:
        unique_names = {name.strip() for name in tag_names if name.strip()}
        if not unique_names:
            return []

        stmt = select(Tag).where(Tag.name.in_(unique_names))
        existing_result = await session.scalars(stmt)
        existing = {tag.name: tag for tag in existing_result}

        tags: List[Tag] = []
        for name in unique_names:
            tag = existing.get(name)
            if tag is None:
                tag = Tag(name=name)
                session.add(tag)
                await session.flush([tag])
            tags.append(tag)

        return tags

    async def _synchronize_rate_limit(
        self,
        session: AsyncSession,
        model: Model,
        config: Optional[RateLimitConfig],
    ) -> None:
        existing = await session.scalar(
            select(RateLimit).where(RateLimit.model_id == model.id)
        )

        if config is None:
            if existing:
                await session.delete(existing)
                self.rate_limiter.remove(model.id)
            return

        if existing is None:
            existing = RateLimit(
                model_id=model.id,
                max_requests=config.max_requests,
                per_seconds=config.per_seconds,
            )
            session.add(existing)

        existing.max_requests = config.max_requests
        existing.per_seconds = config.per_seconds
        existing.burst_size = config.burst_size
        existing.notes = config.notes
        existing.config = config.config

        await session.flush()
        self.rate_limiter.upsert(model.id, config)

    async def _resolve_provider(self, session: AsyncSession, payload: ModelCreate) -> Provider:
        provider: Optional[Provider] = None
        if payload.provider_id is not None:
            provider = await session.get(Provider, payload.provider_id)
        if provider is None and payload.provider_name is not None:
            provider = await session.scalar(
                select(Provider).where(Provider.name == payload.provider_name)
            )

        if provider is None:
            raise ValueError("未找到指定的Provider")

        return provider

    def to_model_read(self, model: Model) -> ModelRead:
        rate_limit = None
        if model.rate_limit:
            rate_limit = RateLimitConfig(
                max_requests=model.rate_limit.max_requests,
                per_seconds=model.rate_limit.per_seconds,
                burst_size=model.rate_limit.burst_size,
                notes=model.rate_limit.notes,
                config=model.rate_limit.config,
            )

        provider = model.provider

        return ModelRead(
            id=model.id,
            name=model.name,
            display_name=model.display_name,
            description=model.description,
            provider_id=model.provider_id,
            provider_name=provider.name if provider else "",
            provider_type=provider.type if provider else ProviderType.REMOTE_HTTP,
            tags=[tag.name for tag in model.tags],
            default_params=model.default_params,
            config=model.config,
            rate_limit=rate_limit,
            local_path=model.local_path,
        )


__all__ = ["ModelService"]


