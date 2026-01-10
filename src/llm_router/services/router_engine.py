from __future__ import annotations

from datetime import datetime
from typing import Any, AsyncIterator, List, Optional

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from ..api_key_config import APIKeyConfig
from ..db.models import InvocationStatus, Model, Provider
from ..providers import ProviderError, ProviderRegistry
from ..schemas import (
    ModelInvokeRequest,
    ModelInvokeResponse,
    ModelQuery,
    ModelRead,
    ModelStreamChunk,
)
from .model_service import ModelService
from .monitor_service import MonitorService
from .rate_limit import RateLimiterManager


class RoutingError(RuntimeError):
    pass


class RouterEngine:
    def __init__(
        self,
        model_service: ModelService,
        provider_registry: ProviderRegistry,
        rate_limiter: RateLimiterManager,
        monitor_service: Optional[MonitorService] = None,
    ) -> None:
        self.model_service = model_service
        self.provider_registry = provider_registry
        self.rate_limiter = rate_limiter
        self.monitor_service = monitor_service

    async def route_by_tags(
        self,
        session: AsyncSession,
        query: ModelQuery,
        request: ModelInvokeRequest,
        api_key_config: Optional[APIKeyConfig] = None,
    ) -> ModelInvokeResponse:
        candidates = await self.model_service.list_models(session, query)
        if not candidates:
            raise RoutingError("未找到符合条件的模型")

        # 如果提供了 API Key 配置，过滤掉不允许的模型
        if api_key_config:
            filtered_candidates = []
            for candidate in candidates:
                # ModelRead 包含 provider_name 字段
                provider_name = candidate.provider_name
                
                if api_key_config.is_model_allowed(provider_name, candidate.name):
                    filtered_candidates.append(candidate)
            
            if not filtered_candidates:
                raise RoutingError("API Key 不允许调用任何符合条件的模型")
            candidates = filtered_candidates

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

    async def stream_by_identifier(
        self,
        session: AsyncSession,
        provider_name: str,
        model_name: str,
        request: ModelInvokeRequest,
    ) -> AsyncIterator[ModelStreamChunk]:
        model = await self.model_service.get_model_by_name(
            session, provider_name, model_name
        )
        if not model or not model.provider or not model.is_active:
            raise RoutingError("指定的模型不可用")

        return self._stream_model(session, model, request)

    async def stream_by_tags(
        self,
        session: AsyncSession,
        query: ModelQuery,
        request: ModelInvokeRequest,
        api_key_config: Optional[APIKeyConfig] = None,
    ) -> AsyncIterator[ModelStreamChunk]:
        candidates = await self.model_service.list_models(session, query)
        if not candidates:
            raise RoutingError("未找到符合条件的模型")

        if api_key_config:
            filtered_candidates = [
                candidate
                for candidate in candidates
                if api_key_config.is_model_allowed(
                    candidate.provider_name, candidate.name
                )
            ]
            if not filtered_candidates:
                raise RoutingError("API Key 不允许调用任何符合条件的模型")
            candidates = filtered_candidates

        selected_info = self._select_candidate(candidates)
        model = await self.model_service.get_model_by_id(session, selected_info.id)
        if not model or not model.provider or not model.is_active:
            raise RoutingError("选定的模型不可用")
        return self._stream_model(session, model, request)

    async def _invoke_model(
        self, session: AsyncSession, model: Model, request: ModelInvokeRequest
    ) -> ModelInvokeResponse:
        if model.rate_limit:
            await self.rate_limiter.acquire(model.id)

        provider = model.provider
        if provider is None or not provider.is_active:
            raise RoutingError("模型的Provider已禁用")

        # 确保 provider 对象在当前 session 中，避免 DetachedInstanceError
        # 使用 merge 将 provider 对象合并到当前 session，如果对象已分离
        provider = await session.merge(provider)
        
        # 在 session 仍然活跃时，预先访问 provider 的属性
        # 这样可以确保数据在 session 中可用，避免在异步调用时出现 DetachedInstanceError
        _ = provider.api_key
        _ = provider.settings
        _ = provider.base_url

        # 记录调用开始时间
        started_at = datetime.utcnow()
        status = InvocationStatus.SUCCESS
        error_message: Optional[str] = None
        response: Optional[ModelInvokeResponse] = None

        client = self.provider_registry.get(provider)
        # 更新 client 中的 provider 引用，确保使用当前 session 中的 provider 对象
        client.update_provider(provider)
        try:
            response = await client.invoke(model, request)
            
            # 如果成功且有 monitor_service，计算并设置费用
            if response and self.monitor_service:
                usage = response.raw.get("usage") if response.raw else None
                prompt_tokens = None
                completion_tokens = None
                if usage:
                    prompt_tokens = usage.get("prompt_tokens")
                    completion_tokens = usage.get("completion_tokens")
                elif response.raw:
                    prompt_tokens = response.raw.get("prompt_tokens")
                    completion_tokens = response.raw.get("completion_tokens")
                
                response.cost = self.monitor_service.calculate_cost(
                    model, prompt_tokens, completion_tokens
                )
        except ProviderError as exc:
            status = InvocationStatus.ERROR
            error_message = str(exc)
            raise RoutingError(str(exc)) from exc
        finally:
            # 记录监控信息（如果monitor_service可用）
            if self.monitor_service:
                completed_at = datetime.utcnow()
                
                # 提取token信息
                prompt_tokens: Optional[int] = None
                completion_tokens: Optional[int] = None
                total_tokens: Optional[int] = None
                raw_response: Optional[dict] = None
                response_text: Optional[str] = None
                
                if response:
                    response_text = response.output_text
                    raw_response = response.raw
                    # 从raw字段提取token使用信息（支持多种格式）
                    usage = raw_response.get("usage") if raw_response else None
                    if usage:
                        prompt_tokens = usage.get("prompt_tokens")
                        completion_tokens = usage.get("completion_tokens")
                        total_tokens = usage.get("total_tokens")
                    # 如果没有usage字段，尝试其他可能的字段名
                    elif raw_response:
                        prompt_tokens = raw_response.get("prompt_tokens")
                        completion_tokens = raw_response.get("completion_tokens")
                        total_tokens = raw_response.get("total_tokens")
                
                # 准备请求信息
                request_prompt = request.prompt
                request_messages = self._build_request_messages_snapshot(request)
                if request_prompt and len(request_prompt) > 1000:
                    request_prompt = request_prompt[:1000] + "..."
                
                # 限制响应文本长度
                if response_text and len(response_text) > 2000:
                    response_text = response_text[:2000] + "..."
                
                # 异步记录（不等待完成，避免阻塞）
                try:
                    await self.monitor_service.record_invocation(
                        session=session,
                        model=model,
                        provider=provider,
                        started_at=started_at,
                        completed_at=completed_at,
                        status=status,
                        request_prompt=request_prompt,
                        request_messages=request_messages,
                        request_parameters=dict(request.parameters),
                        response_text=response_text,
                        error_message=error_message,
                        prompt_tokens=prompt_tokens,
                        completion_tokens=completion_tokens,
                        total_tokens=total_tokens,
                        raw_response=raw_response,
                    )
                except Exception:
                    # 监控记录失败不应该影响主流程
                    pass

        if response is None:
            raise RoutingError("调用失败")
        
        return response

    def _build_request_messages_snapshot(
        self, request: ModelInvokeRequest
    ) -> Optional[List[dict[str, str]]]:
        if not request.messages:
            return None
        messages = [
            {"role": msg.role, "content": msg.content[:500]}
            for msg in request.messages
            if msg.content
        ]
        return messages or None

    async def _stream_model(
        self, session: AsyncSession, model: Model, request: ModelInvokeRequest
    ) -> AsyncIterator[ModelStreamChunk]:
        if model.rate_limit:
            await self.rate_limiter.acquire(model.id)

        provider = model.provider
        if provider is None or not provider.is_active:
            raise RoutingError("模型的Provider已禁用")

        provider = await session.merge(provider)
        _ = provider.api_key
        _ = provider.settings
        _ = provider.base_url

        started_at = datetime.utcnow()
        status = InvocationStatus.SUCCESS
        error_message: Optional[str] = None
        request_prompt = request.prompt
        request_parameters = dict(request.parameters)
        request_messages = self._build_request_messages_snapshot(request)
        text_parts: List[str] = []
        raw_chunks: List[dict[str, Any]] = []
        usage_info: Optional[dict[str, Any]] = None
        completed_at: Optional[datetime] = None

        client = self.provider_registry.get(provider)
        client.update_provider(provider)

        async def generator() -> AsyncIterator[ModelStreamChunk]:
            nonlocal status, error_message, usage_info, completed_at
            try:
                async for chunk in client.stream_invoke(model, request):
                    if chunk.text:
                        text_parts.append(chunk.text)
                    if chunk.raw is not None:
                        raw_chunks.append(chunk.raw)
                    if chunk.usage:
                        usage_info = chunk.usage
                        # 如果有 usage 信息，计算费用并设置到 chunk 中
                        if self.monitor_service:
                            chunk.cost = self.monitor_service.calculate_cost(
                                model,
                                usage_info.get("prompt_tokens"),
                                usage_info.get("completion_tokens")
                            )
                    yield chunk
                completed_at = datetime.utcnow()
            except ProviderError as exc:
                status = InvocationStatus.ERROR
                error_message = str(exc)
                completed_at = datetime.utcnow()
                raise RoutingError(str(exc)) from exc
            finally:
                if self.monitor_service:
                    await self._record_stream_invocation(
                        session=session,
                        model=model,
                        provider=provider,
                        started_at=started_at,
                        completed_at=completed_at or datetime.utcnow(),
                        status=status,
                        error_message=error_message,
                        request_prompt=request_prompt,
                        request_messages=request_messages,
                        request_parameters=request_parameters,
                        text_parts=text_parts,
                        raw_chunks=raw_chunks,
                        usage_info=usage_info,
                    )

        return generator()

    async def _record_stream_invocation(
        self,
        session: AsyncSession,
        model: Model,
        provider: Provider,
        started_at: datetime,
        completed_at: datetime,
        status: InvocationStatus,
        error_message: Optional[str],
        request_prompt: Optional[str],
        request_messages: Optional[List[dict[str, str]]],
        request_parameters: Optional[dict[str, Any]],
        text_parts: List[str],
        raw_chunks: List[dict[str, Any]],
        usage_info: Optional[dict[str, Any]],
    ) -> None:
        if not self.monitor_service:
            return

        response_text = "".join(text_parts)
        if response_text and len(response_text) > 2000:
            response_text = response_text[:2000] + "..."

        prompt_value = request_prompt
        if prompt_value and len(prompt_value) > 1000:
            prompt_value = prompt_value[:1000] + "..."

        raw_response: dict[str, Any] | None = None
        if raw_chunks:
            raw_response = {"stream": raw_chunks}

        try:
            await self.monitor_service.record_invocation(
                session=session,
                model=model,
                provider=provider,
                started_at=started_at,
                completed_at=completed_at,
                status=status,
                request_prompt=prompt_value,
                request_messages=request_messages,
                request_parameters=request_parameters or {},
                response_text=response_text,
                error_message=error_message,
                prompt_tokens=(usage_info or {}).get("prompt_tokens"),
                completion_tokens=(usage_info or {}).get("completion_tokens"),
                total_tokens=(usage_info or {}).get("total_tokens"),
                raw_response=raw_response,
            )
        except Exception:
            pass

    def _select_candidate(self, candidates: List[ModelRead]) -> ModelRead:
        # Candidates are ModelRead objects. Use priority if provided.
        def sort_key(item: ModelRead) -> tuple[int, str]:
            priority = item.config.get("priority", 0)
            return (priority, item.name)

        return sorted(candidates, key=sort_key, reverse=True)[0]


__all__ = ["RouterEngine", "RoutingError"]


