from __future__ import annotations

from typing import Any, Dict
from urllib.parse import urljoin

from ..db.models import Model
from ..schemas import ModelInvokeRequest, ModelInvokeResponse
from .base import BaseProviderClient, ProviderError


class RemoteHTTPProviderClient(BaseProviderClient):
    DEFAULT_ENDPOINT = "/invoke"

    def _build_url(self, model: Model) -> str:
        base = self.provider.base_url or ""
        endpoint = (
            model.config.get("endpoint")
            or self.provider.settings.get("endpoint")
            or self.DEFAULT_ENDPOINT
        )
        return urljoin(base.rstrip("/") + "/", endpoint.lstrip("/"))

    async def invoke(
        self, model: Model, request: ModelInvokeRequest
    ) -> ModelInvokeResponse:
        if request.stream:
            raise ProviderError("当前HTTP Provider暂不支持流式输出")

        url = self._build_url(model)
        timeout = self.provider.settings.get(
            "timeout", self.settings.default_timeout
        )

        payload: Dict[str, Any] = {
            "model": model.remote_identifier or model.name,
            "prompt": request.prompt,
            "messages": [message.model_dump() for message in request.messages or []],
            "parameters": self.merge_parameters(model, request),
        }

        session = await self._get_session()

        async def _invoke_with_key_wrapper(api_key: str | None) -> ModelInvokeResponse:
            headers = self.provider.settings.get("headers", {}).copy()
            if api_key:
                auth_header = self.provider.settings.get("auth_header", "Authorization")
                headers[auth_header] = f"Bearer {api_key}"

            response = await session.post(
                url,
                json=payload,
                headers=headers,
                timeout=timeout,
            )

            if response.status_code >= 400:
                raise ProviderError(
                    f"HTTP Provider 调用失败: {response.status_code} {response.text}"
                )

            data = response.json()
            output_text = data.get("output") or data.get("text") or data.get("data")
            if isinstance(output_text, list):
                output_text = "\n".join(str(item) for item in output_text)
            if output_text is None:
                output_text = ""

            return ModelInvokeResponse(output_text=output_text, raw=data)

        return await self._invoke_with_failover(
            _invoke_with_key_wrapper,
            require_api_key=False,
            error_message="HTTP Provider 需要至少一个 API key",
        )


