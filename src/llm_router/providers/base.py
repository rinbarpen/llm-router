from __future__ import annotations

from abc import ABC, abstractmethod
from typing import Any, AsyncIterator, Optional

import httpx

from ..config import RouterSettings
from ..db.models import Model, Provider
from ..schemas import ModelInvokeRequest, ModelInvokeResponse, ModelStreamChunk


class ProviderError(RuntimeError):
    """基类异常，表示Provider调用失败。"""


class BaseProviderClient(ABC):
    def __init__(self, provider: Provider, settings: RouterSettings) -> None:
        self.provider = provider
        self.settings = settings
        self._session: Optional[httpx.AsyncClient] = None
        self._session_key: Optional[tuple[Any, ...]] = None
    
    def update_provider(self, provider: Provider) -> None:
        """更新 provider 引用，用于确保 provider 对象在当前 session 中"""
        self.provider = provider

    @abstractmethod
    async def invoke(
        self, model: Model, request: ModelInvokeRequest
    ) -> ModelInvokeResponse:
        raise NotImplementedError

    async def stream_invoke(
        self, model: Model, request: ModelInvokeRequest
    ) -> AsyncIterator[ModelStreamChunk]:
        """默认不支持流式输出，由子类按需实现。"""
        raise ProviderError(f"{self.provider.type.value} 暂不支持流式输出")

    def merge_parameters(self, model: Model, request: ModelInvokeRequest) -> dict[str, Any]:
        params = dict(model.default_params or {})
        params.update(request.parameters)
        return params

    def client_options(self) -> dict[str, Any]:
        timeout = self.provider.settings.get("timeout", self.settings.default_timeout)
        options: dict[str, Any] = {
            "timeout": httpx.Timeout(timeout),
        }
        proxy = self.provider.settings.get("proxy")
        if proxy:
            options["proxies"] = {"http": proxy, "https": proxy}
        return options

    def _build_session_key(self) -> tuple[Any, ...]:
        proxy = self.provider.settings.get("proxy")
        return (proxy,)

    async def _get_session(self) -> httpx.AsyncClient:
        session_key = self._build_session_key()
        if self._session and self._session_key != session_key:
            await self._session.close()
            self._session = None

        if self._session is None:
            self._session = httpx.AsyncClient(**self.client_options())
            self._session_key = session_key

        return self._session

    async def aclose(self) -> None:
        """关闭底层 HTTP 会话，供应用关闭时调用。"""
        if self._session:
            await self._session.close()
            self._session = None



