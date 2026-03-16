from __future__ import annotations

import os
from abc import ABC, abstractmethod
from typing import Any, AsyncIterator, Dict, List, Optional

from curl_cffi import requests
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
        self._session: Optional[Any] = None
        self._session_key: Optional[tuple[Any, ...]] = None
        self._api_key_cursor: int = 0

    @staticmethod
    def _use_httpx_backend() -> bool:
        return os.getenv("LLM_ROUTER_HTTP_BACKEND", "").lower() == "httpx"
    
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
        """默认不支持流式输出，由子类按需实现。改为 async generator 以便调用方统一 async for。"""
        if False:
            yield  # type: ignore[misc]  # 使本函数成为 async generator，调用方得到 __aiter__
        raise ProviderError(f"{self.provider.type.value} 暂不支持流式输出")

    async def embed(self, model: Model, payload: Dict[str, Any]) -> Dict[str, Any]:
        raise ProviderError(f"{self.provider.type.value} 暂不支持 embedding")

    async def synthesize_speech(self, model: Model, payload: Dict[str, Any]) -> tuple[bytes, str]:
        raise ProviderError(f"{self.provider.type.value} 暂不支持 tts")

    async def transcribe_audio(
        self,
        model: Model,
        data: bytes,
        filename: str,
        mime_type: str,
        extra_payload: Dict[str, Any],
    ) -> Dict[str, Any]:
        raise ProviderError(f"{self.provider.type.value} 暂不支持 asr")

    async def translate_audio(
        self,
        model: Model,
        data: bytes,
        filename: str,
        mime_type: str,
        extra_payload: Dict[str, Any],
    ) -> Dict[str, Any]:
        raise ProviderError(f"{self.provider.type.value} 暂不支持 asr translation")

    async def generate_image(self, model: Model, payload: Dict[str, Any]) -> Dict[str, Any]:
        raise ProviderError(f"{self.provider.type.value} 暂不支持 image generation")

    async def generate_video(self, model: Model, payload: Dict[str, Any]) -> Dict[str, Any]:
        raise ProviderError(f"{self.provider.type.value} 暂不支持 video generation")

    async def list_supported_models(self) -> List[str]:
        """可选能力：返回 provider 侧支持的模型列表。"""
        raise ProviderError(f"{self.provider.type.value} 暂不支持列出模型")

    def merge_parameters(self, model: Model, request: ModelInvokeRequest) -> dict[str, Any]:
        params = dict(model.default_params or {})
        params.update(request.parameters)
        return params

    def client_options(self) -> dict[str, Any]:
        timeout = self.provider.settings.get("timeout", self.settings.default_timeout)
        if self._use_httpx_backend():
            options: dict[str, Any] = {
                "timeout": httpx.Timeout(float(timeout)),
                "follow_redirects": True,
                "trust_env": False,
            }
        else:
            options = {
                "timeout": float(timeout),
                "allow_redirects": True,
                "trust_env": False,
            }

        # 仅使用 provider 显式配置的代理，避免被进程环境变量隐式污染。
        proxy = self.provider.settings.get("proxy")

        if proxy:
            options["proxy"] = proxy
            import logging
            logging.getLogger(__name__).debug(f"Provider {self.provider.name} 使用代理: {proxy}")
            
        return options

    def _build_session_key(self) -> tuple[Any, ...]:
        proxy = self.provider.settings.get("proxy")
        return (proxy,)

    async def _get_session(self) -> Any:
        session_key = self._build_session_key()
        if self._session and self._session_key != session_key:
            if hasattr(self._session, "aclose"):
                await self._session.aclose()
            else:
                await self._session.close()
            self._session = None

        if self._session is None:
            if self._use_httpx_backend():
                self._session = httpx.AsyncClient(**self.client_options())
            else:
                self._session = requests.AsyncSession(**self.client_options())
            self._session_key = session_key

        return self._session

    async def aclose(self) -> None:
        """关闭底层 HTTP 会话，供应用关闭时调用。"""
        if self._session:
            if hasattr(self._session, "aclose"):
                await self._session.aclose()
            else:
                await self._session.close()
            self._session = None

    async def _read_response_text(self, response: Any) -> str:
        """兼容 httpx / curl_cffi 的异步响应文本读取。"""
        if hasattr(response, "atext"):
            return await response.atext()
        if hasattr(response, "aread"):
            return (await response.aread()).decode(errors="replace")
        return getattr(response, "text", "")

    def _get_api_keys(self) -> List[str]:
        """获取可用的 API keys 列表（支持逗号分隔的多个 key）"""
        api_key = self.provider.api_key or self.provider.settings.get("api_key")
        if not api_key:
            return []
        # 支持逗号分隔的多个 key
        keys = [k.strip() for k in api_key.split(",") if k.strip()]
        return keys

    def _iter_api_keys_round_robin(self, api_keys: List[str]) -> List[str]:
        """按轮询顺序返回 key 列表，避免每次都优先打第一个 key。"""
        if len(api_keys) <= 1:
            return api_keys
        start = self._api_key_cursor % len(api_keys)
        ordered = api_keys[start:] + api_keys[:start]
        self._api_key_cursor = (self._api_key_cursor + 1) % len(api_keys)
        return ordered

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
        api_keys = self._iter_api_keys_round_robin(api_keys)
        
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
