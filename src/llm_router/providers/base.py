from __future__ import annotations

from abc import ABC, abstractmethod
from typing import Any, AsyncIterator, List, Optional

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

    def _get_api_keys(self) -> List[str]:
        """获取可用的 API keys 列表（支持逗号分隔的多个 key）"""
        api_key = self.provider.api_key or self.provider.settings.get("api_key")
        if not api_key:
            return []
        # 支持逗号分隔的多个 key
        keys = [k.strip() for k in api_key.split(",") if k.strip()]
        return keys

    def _is_retryable_error(self, status_code: int) -> bool:
        """判断是否为可重试的错误（需要切换 key）
        
        可重试的错误包括：
        - 401 (Unauthorized) - 认证失败
        - 403 (Forbidden) - 权限不足
        - 429 (Too Many Requests) - 速率限制
        """
        return status_code in (401, 403, 429)

    def _extract_status_code_from_error(self, error: ProviderError) -> Optional[int]:
        """从 ProviderError 中提取 HTTP 状态码"""
        error_msg = str(error)
        # 尝试从错误消息中提取状态码
        # 格式通常是: "Provider 请求失败: 401 ..." 或 "HTTP Provider 调用失败: 401 ..."
        import re
        match = re.search(r'\b(\d{3})\b', error_msg)
        if match:
            try:
                return int(match.group(1))
            except ValueError:
                pass
        return None

    async def _invoke_with_failover(
        self,
        invoke_func: callable,
        require_api_key: bool = True,
        error_message: str = "需要至少一个 API key",
    ) -> Any:
        """通用的故障转移包装器，用于处理多个 API key 的故障转移
        
        Args:
            invoke_func: 接受 api_key (str | None) 作为参数的异步函数，返回 ModelInvokeResponse
            require_api_key: 是否要求必须有 API key
            error_message: 当没有 API key 时的错误消息
        
        Returns:
            invoke_func 的返回值（通常是 ModelInvokeResponse）
        
        Raises:
            ProviderError: 当所有 key 都失败时抛出最后一个错误
        """
        api_keys = self._get_api_keys()
        
        if not api_keys:
            if require_api_key:
                raise ProviderError(error_message)
            # 如果没有配置 API key，尝试不使用 key（某些服务可能不需要）
            return await invoke_func(None)
        
        last_error = None
        for api_key in api_keys:
            try:
                return await invoke_func(api_key)
            except ProviderError as e:
                # 如果是最后一个 key，直接抛出错误
                if api_key == api_keys[-1]:
                    raise
                # 否则检查是否为可重试错误
                status_code = self._extract_status_code_from_error(e)
                if status_code and self._is_retryable_error(status_code):
                    last_error = e
                    continue
                else:
                    # 非可重试错误直接抛出
                    raise
        
        # 所有 key 都失败
        if last_error:
            raise last_error
        raise ProviderError("所有 API key 都不可用")



