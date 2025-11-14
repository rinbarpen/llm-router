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
            "trust_env": self.provider.settings.get("trust_env", False),
        }
        proxy = self.provider.settings.get("proxy")
        if proxy:
            options["proxy"] = proxy
        return options


