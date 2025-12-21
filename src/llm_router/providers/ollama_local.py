from __future__ import annotations

from typing import Any, Dict
from urllib.parse import urljoin

from ..db.models import Model
from ..schemas import ModelInvokeRequest, ModelInvokeResponse
from .base import BaseProviderClient, ProviderError


class OllamaProviderClient(BaseProviderClient):
    DEFAULT_BASE = "http://127.0.0.1:11434"

    def _base_url(self) -> str:
        return (
            self.provider.base_url
            or self.provider.settings.get("base_url")
            or self.DEFAULT_BASE
        )

    async def invoke(
        self, model: Model, request: ModelInvokeRequest
    ) -> ModelInvokeResponse:
        if request.stream:
            raise ProviderError("Ollama Provider 暂不支持流式输出")

        url = urljoin(self._base_url().rstrip("/") + "/", "api/generate")
        timeout = self.provider.settings.get(
            "timeout", self.settings.default_timeout
        )
        payload: Dict[str, Any] = {
            "model": model.remote_identifier or model.name,
            "prompt": request.prompt or "",
            "options": self.merge_parameters(model, request),
        }
        if request.messages:
            payload["messages"] = [message.model_dump() for message in request.messages]

        session = await self._get_session()
        response = await session.post(url, json=payload, timeout=timeout)

        if response.status_code >= 400:
            raise ProviderError(
                f"Ollama 请求失败: {response.status_code} {response.text}"
            )

        data = response.json()
        text = data.get("response") or data.get("output") or ""
        return ModelInvokeResponse(output_text=text, raw=data)


