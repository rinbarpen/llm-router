from __future__ import annotations

from abc import ABC, abstractmethod
from typing import Any

from ..config import RouterSettings
from ..db.models import Model, Provider
from ..schemas import ModelInvokeRequest, ModelInvokeResponse


class ProviderError(RuntimeError):
    """基类异常，表示Provider调用失败。"""


class BaseProviderClient(ABC):
    def __init__(self, provider: Provider, settings: RouterSettings) -> None:
        self.provider = provider
        self.settings = settings
    
    def update_provider(self, provider: Provider) -> None:
        """更新 provider 引用，用于确保 provider 对象在当前 session 中"""
        self.provider = provider

    @abstractmethod
    async def invoke(
        self, model: Model, request: ModelInvokeRequest
    ) -> ModelInvokeResponse:
        raise NotImplementedError

    def merge_parameters(self, model: Model, request: ModelInvokeRequest) -> dict[str, Any]:
        params = dict(model.default_params or {})
        params.update(request.parameters)
        return params

    def client_options(self, timeout: float) -> dict[str, Any]:
        options: dict[str, Any] = {
            "timeout": timeout,
        }
        proxy = self.provider.settings.get("proxy")
        if proxy:
            options["proxies"] = {"http": proxy, "https": proxy}
        # curl_cffi 不支持 trust_env 参数，如果需要可以手动处理环境变量
        return options


