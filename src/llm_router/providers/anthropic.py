from __future__ import annotations

import json
from typing import Any, AsyncIterator, List
from urllib.parse import urljoin

from ..db.models import Model
from ..schemas import (
    ModelInvokeRequest,
    ModelInvokeResponse,
    ModelStreamChunk,
)
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

    async def stream_invoke(
        self, model: Model, request: ModelInvokeRequest
    ) -> AsyncIterator[ModelStreamChunk]:
        messages, system_prompt = self._build_messages(request)
        if not messages:
            raise ProviderError("Claude 调用至少需要一个用户消息")

        payload = self._build_payload(model, request, messages, system_prompt)
        payload["stream"] = True
        url = self._build_url()
        timeout = self.provider.settings.get("timeout", self.settings.default_timeout)
        session = await self._get_session()

        api_keys = self._get_api_keys()
        if not api_keys:
            raise ProviderError("Claude Provider 需要 api_key")

        last_error = None
        for api_key in api_keys:
            try:
                headers = self._build_headers(api_key)
                async with session.stream(
                    "POST",
                    url,
                    json=payload,
                    headers=headers,
                    timeout=timeout,
                ) as response:
                    if response.status_code >= 400:
                        error_text = await self._read_response_text(response)
                        err = ProviderError(
                            f"Claude 请求失败: {response.status_code} {error_text.decode()}"
                        )
                        if api_key != api_keys[-1] and self._is_retryable_error(
                            response.status_code
                        ):
                            last_error = err
                            continue
                        raise err
                    usage = None
                    async for line in response.aiter_lines():
                        if not line or line.startswith(":"):
                            continue
                        if not line.startswith("data:"):
                            continue
                        raw_data = line[5:].strip()
                        if not raw_data:
                            continue
                        try:
                            event = json.loads(raw_data)
                        except json.JSONDecodeError:
                            continue
                        event_type = event.get("type")
                        if event_type == "content_block_delta":
                            delta = event.get("delta") or {}
                            if delta.get("type") == "text_delta":
                                text_piece = delta.get("text") or ""
                                if text_piece:
                                    yield ModelStreamChunk(
                                        text=text_piece,
                                        raw=event,
                                        usage=usage,
                                    )
                        elif event_type == "message_delta":
                            usage_info = event.get("usage") or {}
                            usage = {
                                "prompt_tokens": usage_info.get("input_tokens"),
                                "completion_tokens": usage_info.get(
                                    "output_tokens"
                                ),
                                "total_tokens": (
                                    (usage_info.get("input_tokens") or 0)
                                    + (usage_info.get("output_tokens") or 0)
                                ),
                            }
                        elif event_type == "message_stop":
                            yield ModelStreamChunk(is_final=True)
                            return
                    yield ModelStreamChunk(is_final=True)
                    return
            except ProviderError as e:
                if api_key == api_keys[-1]:
                    raise
                status_code = self._extract_status_code_from_error(e)
                if status_code and self._is_retryable_error(status_code):
                    last_error = e
                    continue
                raise
        if last_error:
            raise last_error
        raise ProviderError("Claude Provider 需要 api_key")

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
        texts = []
        for block in content:
            if isinstance(block, dict) and block.get("type") == "text":
                texts.append(block.get("text", ""))
        return "".join(texts)

    def _build_url_with_endpoint(self, endpoint: str) -> str:
        base = (
            self.provider.base_url
            or self.provider.settings.get("base_url")
            or self.DEFAULT_BASE_URL
        ).rstrip("/")
        return urljoin(base + "/", endpoint.lstrip("/"))

    async def count_tokens(self, model: Model, payload: dict[str, Any]) -> dict[str, Any]:
        request_payload = dict(payload)
        request_payload.setdefault("model", self._resolve_model_identifier(model))
        endpoint = self.provider.settings.get("count_tokens_endpoint", "/v1/messages/count_tokens")
        return await self._post_json_with_failover(endpoint, request_payload)

    async def create_message_batch(self, payload: dict[str, Any]) -> dict[str, Any]:
        endpoint = self.provider.settings.get("messages_batches_endpoint", "/v1/messages/batches")
        return await self._post_json_with_failover(endpoint, payload)

    async def get_message_batch(self, batch_id: str) -> dict[str, Any]:
        endpoint_template = self.provider.settings.get(
            "messages_batches_get_endpoint",
            "/v1/messages/batches/{batch_id}",
        )
        endpoint = endpoint_template.format(batch_id=batch_id)
        return await self._get_json_with_failover(endpoint)

    async def cancel_message_batch(self, batch_id: str) -> dict[str, Any]:
        endpoint_template = self.provider.settings.get(
            "messages_batches_cancel_endpoint",
            "/v1/messages/batches/{batch_id}/cancel",
        )
        endpoint = endpoint_template.format(batch_id=batch_id)
        return await self._post_json_with_failover(endpoint, {})

    async def _post_json_with_failover(
        self,
        endpoint: str,
        payload: dict[str, Any],
    ) -> dict[str, Any]:
        url = self._build_url_with_endpoint(endpoint)
        timeout = self.provider.settings.get("timeout", self.settings.default_timeout)
        session = await self._get_session()

        async def _invoke_with_key_wrapper(api_key: str | None) -> dict[str, Any]:
            if not api_key:
                raise ProviderError("Claude Provider 需要 api_key")
            response = await session.post(
                url,
                json=payload,
                headers=self._build_headers(api_key),
                timeout=timeout,
            )
            if response.status_code >= 400:
                raise ProviderError(
                    f"Claude 请求失败: {response.status_code} {response.text}"
                )
            return response.json()

        return await self._invoke_with_failover(
            _invoke_with_key_wrapper,
            require_api_key=True,
            error_message="Claude Provider 需要 api_key",
        )

    async def _get_json_with_failover(self, endpoint: str) -> dict[str, Any]:
        url = self._build_url_with_endpoint(endpoint)
        timeout = self.provider.settings.get("timeout", self.settings.default_timeout)
        session = await self._get_session()

        async def _invoke_with_key_wrapper(api_key: str | None) -> dict[str, Any]:
            if not api_key:
                raise ProviderError("Claude Provider 需要 api_key")
            response = await session.get(
                url,
                headers=self._build_headers(api_key),
                timeout=timeout,
            )
            if response.status_code >= 400:
                raise ProviderError(
                    f"Claude 请求失败: {response.status_code} {response.text}"
                )
            return response.json()

        return await self._invoke_with_failover(
            _invoke_with_key_wrapper,
            require_api_key=True,
            error_message="Claude Provider 需要 api_key",
        )


__all__ = ["AnthropicProviderClient"]


