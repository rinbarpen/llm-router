from __future__ import annotations

from typing import List
from urllib.parse import urljoin

from ..db.models import Model
from ..schemas import ModelInvokeRequest, ModelInvokeResponse
from .base import BaseProviderClient, ProviderError


class AnthropicProviderClient(BaseProviderClient):
    DEFAULT_BASE_URL = "https://api.anthropic.com"
    DEFAULT_ENDPOINT = "/v1/messages"
    DEFAULT_VERSION = "2023-06-01"

    async def invoke(
        self, model: Model, request: ModelInvokeRequest
    ) -> ModelInvokeResponse:
        messages, system_prompt = self._build_messages(request)
        if not messages:
            raise ProviderError("Claude 调用至少需要一个用户消息")

        payload = self._build_payload(model, request, messages, system_prompt)
        url = self._build_url()
        timeout = self.provider.settings.get("timeout", self.settings.default_timeout)
        session = await self._get_session()

        async def _invoke_with_key_wrapper(api_key: str | None) -> ModelInvokeResponse:
            if not api_key:
                raise ProviderError("Claude Provider 需要 api_key")
            headers = self._build_headers(api_key)
            response = await session.post(
                url,
                json=payload,
                headers=headers,
                timeout=timeout,
            )

            if response.status_code >= 400:
                raise ProviderError(
                    f"Claude 请求失败: {response.status_code} {response.text}"
                )

            data = response.json()
            text = self._extract_output(data)
            return ModelInvokeResponse(output_text=text, raw=data)

        return await self._invoke_with_failover(
            _invoke_with_key_wrapper,
            require_api_key=True,
            error_message="Claude Provider 需要 api_key",
        )

    def _build_messages(
        self, request: ModelInvokeRequest
    ) -> tuple[List[dict], str | None]:
        system_messages: List[str] = []
        messages: List[dict] = []

        for message in request.messages or []:
            if not message.content:
                continue
            if message.role == "system":
                system_messages.append(message.content)
                continue
            role = "user" if message.role == "user" else "assistant"
            messages.append(
                {"role": role, "content": [{"type": "text", "text": message.content}]}
            )

        if request.prompt:
            messages.append(
                {"role": "user", "content": [{"type": "text", "text": request.prompt}]}
            )

        system_prompt = "\n".join(system_messages) if system_messages else None
        return messages, system_prompt

    def _build_payload(
        self,
        model: Model,
        request: ModelInvokeRequest,
        messages: List[dict],
        system_prompt: str | None,
    ) -> dict:
        params = self.merge_parameters(model, request)
        max_tokens = params.pop("max_tokens", params.pop("max_output_tokens", 1024))

        payload = {
            "model": self._resolve_model_identifier(model),
            "messages": messages,
            "max_tokens": max_tokens,
        }
        if system_prompt:
            payload["system"] = system_prompt
        payload.update(params)
        return payload

    def _build_headers(self, api_key: str) -> dict[str, str]:
        headers = {
            "Content-Type": "application/json",
            "x-api-key": api_key,
            "anthropic-version": self.provider.settings.get(
                "anthropic_version", self.DEFAULT_VERSION
            ),
        }
        headers.update(self.provider.settings.get("headers", {}))
        return headers

    def _build_url(self) -> str:
        base = (
            self.provider.base_url
            or self.provider.settings.get("base_url")
            or self.DEFAULT_BASE_URL
        )
        endpoint = self.provider.settings.get("endpoint") or self.DEFAULT_ENDPOINT
        return urljoin(base.rstrip("/") + "/", endpoint.lstrip("/"))

    def _resolve_model_identifier(self, model: Model) -> str:
        return (
            model.remote_identifier
            or model.config.get("model")
            or model.name
        )

    def _extract_output(self, data: dict) -> str:
        content = data.get("content") or []
        texts = [
            block.get("text", "")
            for block in content
            if isinstance(block, dict) and block.get("type") == "text"
        ]
        return "".join(texts)


__all__ = ["AnthropicProviderClient"]


