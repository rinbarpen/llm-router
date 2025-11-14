from __future__ import annotations

from typing import Dict
from urllib.parse import urljoin

import httpx

from ..db.models import Model, ProviderType
from ..schemas import ModelInvokeRequest, ModelInvokeResponse
from .base import BaseProviderClient, ProviderError


class OpenAICompatibleProviderClient(BaseProviderClient):
    DEFAULT_BASE_URLS: Dict[ProviderType, str] = {
        ProviderType.OPENAI: "https://api.openai.com",
        ProviderType.GROK: "https://api.x.ai",
        ProviderType.DEEPSEEK: "https://api.deepseek.com",
        ProviderType.QWEN: "https://dashscope.aliyuncs.com",
        ProviderType.KIMI: "https://api.moonshot.cn",
        ProviderType.GLM: "https://open.bigmodel.cn/api/paas",
        ProviderType.OPENROUTER: "https://openrouter.ai/api",
    }

    ENDPOINT_OVERRIDES: Dict[ProviderType, str] = {
        ProviderType.QWEN: "/compatible-mode/v1/chat/completions",
        ProviderType.GLM: "/v4/chat/completions",
    }

    DEFAULT_ENDPOINT = "/v1/chat/completions"

    async def invoke(
        self, model: Model, request: ModelInvokeRequest
    ) -> ModelInvokeResponse:
        if request.stream:
            raise ProviderError("OpenAI 兼容 Provider 暂不支持流式输出")

        messages = self._build_messages(request)
        if not messages:
            raise ProviderError("prompt 或 messages 至少需要提供一个")

        url = self._build_url()
        headers = self._build_headers()
        payload = self._build_payload(model, request, messages)

        timeout = self.provider.settings.get("timeout", self.settings.default_timeout)
        client_kwargs = self.client_options(timeout)
        async with httpx.AsyncClient(**client_kwargs) as client:
            response = await client.post(url, json=payload, headers=headers)

        if response.status_code >= 400:
            raise ProviderError(
                f"{self.provider.type.value} 请求失败: {response.status_code} {response.text}"
            )

        data = response.json()
        output_text = self._extract_output(data)
        return ModelInvokeResponse(output_text=output_text, raw=data)

    def _build_messages(self, request: ModelInvokeRequest) -> list[dict[str, str]]:
        messages = [
            {"role": message.role, "content": message.content}
            for message in request.messages or []
            if message.content
        ]

        if request.prompt:
            messages.append({"role": "user", "content": request.prompt})

        return messages

    def _build_payload(
        self,
        model: Model,
        request: ModelInvokeRequest,
        messages: list[dict[str, str]],
    ) -> dict:
        payload = {
            "model": self._resolve_model_identifier(model),
            "messages": messages,
        }
        payload.update(self.merge_parameters(model, request))
        return payload

    def _build_headers(self) -> dict[str, str]:
        headers = {"Content-Type": "application/json"}
        custom = self.provider.settings.get("headers", {})
        headers.update(custom)

        api_key = self.provider.api_key or self.provider.settings.get("api_key")
        if api_key:
            auth_header = self.provider.settings.get("auth_header", "Authorization")
            scheme = self.provider.settings.get("auth_scheme", "Bearer")
            headers[auth_header] = f"{scheme} {api_key}".strip()

        return headers

    def _build_url(self) -> str:
        base = (
            self.provider.base_url
            or self.provider.settings.get("base_url")
            or self.DEFAULT_BASE_URLS.get(self.provider.type)
            or self.DEFAULT_BASE_URLS[ProviderType.OPENAI]
        )
        endpoint = (
            self.provider.settings.get("endpoint")
            or self.ENDPOINT_OVERRIDES.get(self.provider.type)
            or self.DEFAULT_ENDPOINT
        )
        return urljoin(base.rstrip("/") + "/", endpoint.lstrip("/"))

    def _resolve_model_identifier(self, model: Model) -> str:
        return (
            model.remote_identifier
            or model.config.get("model")
            or model.name
        )

    def _extract_output(self, data: dict) -> str:
        choices = data.get("choices") or []
        if choices:
            choice = choices[0]
            message = choice.get("message") or {}
            text = message.get("content") or choice.get("text")
            if isinstance(text, list):
                return "".join(part for part in text if isinstance(part, str))
            if text is not None:
                return str(text)
        if "output" in data:
            return str(data["output"])
        return ""


__all__ = ["OpenAICompatibleProviderClient"]


