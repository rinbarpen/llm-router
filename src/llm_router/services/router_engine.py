from __future__ import annotations

from typing import Any, List

from sqlalchemy.ext.asyncio import AsyncSession

from ..db.models import Model
from ..providers import ProviderError, ProviderRegistry
from ..schemas import ModelInvokeRequest, ModelInvokeResponse, ModelQuery, ModelRead
from .model_service import ModelService
from .rate_limit import RateLimiterManager


class RoutingError(RuntimeError):
    pass


class RouterEngine:
    def __init__(
        self,
        model_service: ModelService,
        provider_registry: ProviderRegistry,
        rate_limiter: RateLimiterManager,
    ) -> None:
        self.model_service = model_service
        self.provider_registry = provider_registry
        self.rate_limiter = rate_limiter

    async def route_by_tags(
        self, session: AsyncSession, query: ModelQuery, request: ModelInvokeRequest
    ) -> ModelInvokeResponse:
        candidates = await self.model_service.list_models(session, query)
        if not candidates:
            raise RoutingError("未找到符合条件的模型")

        selected_info = self._select_candidate(candidates)
        model = await self.model_service.get_model_by_id(session, selected_info.id)
        if not model or not model.provider or not model.is_active:
            raise RoutingError("选定的模型不可用")

        return await self._invoke_model(session, model, request)

    async def invoke_by_identifier(
        self,
        session: AsyncSession,
        provider_name: str,
        model_name: str,
        request: ModelInvokeRequest,
    ) -> ModelInvokeResponse:
        model = await self.model_service.get_model_by_name(
            session, provider_name, model_name
        )
        if not model or not model.provider or not model.is_active:
            raise RoutingError("指定的模型不可用")

        return await self._invoke_model(session, model, request)

    async def _invoke_model(
        self, session: AsyncSession, model: Model, request: ModelInvokeRequest
    ) -> ModelInvokeResponse:
        if model.rate_limit:
            await self.rate_limiter.acquire(model.id)

        provider = model.provider
        if provider is None or not provider.is_active:
            raise RoutingError("模型的Provider已禁用")

        client = self.provider_registry.get(provider)
        try:
            response = await client.invoke(model, request)
        except ProviderError as exc:
            raise RoutingError(str(exc)) from exc

        return response

    def _select_candidate(self, candidates: List[ModelRead]) -> ModelRead:
        # Candidates are ModelRead objects. Use priority if provided.
        def sort_key(item: ModelRead) -> tuple[int, str]:
            priority = item.config.get("priority", 0)
            return (priority, item.name)

        return sorted(candidates, key=sort_key, reverse=True)[0]


__all__ = ["RouterEngine", "RoutingError"]


