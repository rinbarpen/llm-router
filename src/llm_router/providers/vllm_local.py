from __future__ import annotations

from typing import Any, Dict
from urllib.parse import urljoin

from ..db.models import Model
from ..schemas import ModelInvokeRequest, ModelInvokeResponse
from .base import BaseProviderClient, ProviderError


class VLLMProviderClient(BaseProviderClient):
    DEFAULT_ENDPOINT = "/v1/completions"

    def _build_url(self) -> str:
        base = self.provider.base_url or self.provider.settings.get("base_url")
        if not base:
            raise ProviderError("vLLM Provider 需要配置 base_url")
        endpoint = self.provider.settings.get("endpoint") or self.DEFAULT_ENDPOINT
        return urljoin(base.rstrip("/") + "/", endpoint.lstrip("/"))

    async def invoke(
        self, model: Model, request: ModelInvokeRequest
    ) -> ModelInvokeResponse:
        if request.stream:
            raise ProviderError("vLLM Provider 暂不支持流式输出")

        url = self._build_url()
        timeout = self.provider.settings.get("timeout", self.settings.default_timeout)

        body: Dict[str, Any] = {
            "model": model.remote_identifier or model.name,
            "prompt": request.prompt,
            "messages": [message.model_dump() for message in request.messages or []],
        }

        parameters = self.merge_parameters(model, request)
        body.update(parameters)

        session = await self._get_session()

        async def _invoke_with_key_wrapper(api_key: str | None) -> ModelInvokeResponse:
            headers = {"Content-Type": "application/json"}
            if api_key:
                headers["Authorization"] = f"Bearer {api_key}"

            response = await session.post(
                url,
                json=body,
                headers=headers,
                timeout=timeout,
            )

            if response.status_code >= 400:
                raise ProviderError(
                    f"vLLM 请求失败: {response.status_code} {response.text}"
                )

            data = response.json()
            choices = data.get("choices") or []
            text = ""
            if choices:
                choice = choices[0]
                text = (
                    choice.get("text")
                    or choice.get("message", {}).get("content")
                    or ""
                )

            return ModelInvokeResponse(output_text=text, raw=data)

        return await self._invoke_with_failover(
            _invoke_with_key_wrapper,
            require_api_key=False,
            error_message="vLLM Provider 需要至少一个 API key",
        )


__all__ = ["VLLMProviderClient"]


