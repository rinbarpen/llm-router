from __future__ import annotations

from typing import Dict, Type

from ..config import RouterSettings
from ..db.models import Provider, ProviderType
from .anthropic import AnthropicProviderClient
from .base import BaseProviderClient, ProviderError
from .gemini import GeminiProviderClient
from .ollama_local import OllamaProviderClient
from .openai_compatible import OpenAICompatibleProviderClient
from .remote_http import RemoteHTTPProviderClient
from .transformers_local import TransformersProviderClient
from .vllm_local import VLLMProviderClient


CLIENT_MAPPING: Dict[ProviderType, Type[BaseProviderClient]] = {
    ProviderType.REMOTE_HTTP: RemoteHTTPProviderClient,
    ProviderType.CUSTOM_HTTP: RemoteHTTPProviderClient,
    ProviderType.TRANSFORMERS: TransformersProviderClient,
    ProviderType.OLLAMA: OllamaProviderClient,
    ProviderType.VLLM: VLLMProviderClient,
    ProviderType.OPENAI: OpenAICompatibleProviderClient,
    ProviderType.GROK: OpenAICompatibleProviderClient,
    ProviderType.DEEPSEEK: OpenAICompatibleProviderClient,
    ProviderType.QWEN: OpenAICompatibleProviderClient,
    ProviderType.KIMI: OpenAICompatibleProviderClient,
    ProviderType.GLM: OpenAICompatibleProviderClient,
    ProviderType.OPENROUTER: OpenAICompatibleProviderClient,
    ProviderType.GEMINI: GeminiProviderClient,
    ProviderType.CLAUDE: AnthropicProviderClient,
}


class ProviderRegistry:
    def __init__(self, settings: RouterSettings) -> None:
        self.settings = settings
        self._clients: Dict[int, BaseProviderClient] = {}

    def get(self, provider: Provider) -> BaseProviderClient:
        client = self._clients.get(provider.id)
        if client is not None:
            return client

        factory = CLIENT_MAPPING.get(provider.type)
        if factory is None:
            raise ProviderError(f"暂不支持的Provider类型: {provider.type}")

        client = factory(provider, self.settings)
        self._clients[provider.id] = client
        return client

    def remove(self, provider_id: int) -> None:
        self._clients.pop(provider_id, None)

    def clear(self) -> None:
        self._clients.clear()

    async def aclose(self) -> None:
        """关闭所有 Provider client 的连接。"""
        for client in self._clients.values():
            try:
                await client.aclose()
            except Exception:
                # 关闭失败不影响整体退出
                pass
        self._clients.clear()


__all__ = ["ProviderRegistry"]


