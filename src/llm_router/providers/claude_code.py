from __future__ import annotations

from typing import Any
from urllib.parse import urljoin

from ..db.models import Model
from .anthropic import AnthropicProviderClient
from .base import ProviderError


class ClaudeCodeProviderClient(AnthropicProviderClient):
    """Claude Code provider client with extra native Anthropic endpoints."""

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
                raise ProviderError("Claude Code Provider 需要 api_key")
            response = await session.post(
                url,
                json=payload,
                headers=self._build_headers(api_key),
                timeout=timeout,
            )
            if response.status_code >= 400:
                raise ProviderError(
                    f"Claude Code 请求失败: {response.status_code} {response.text}"
                )
            return response.json()

        return await self._invoke_with_failover(
            _invoke_with_key_wrapper,
            require_api_key=True,
            error_message="Claude Code Provider 需要 api_key",
        )

    async def _get_json_with_failover(self, endpoint: str) -> dict[str, Any]:
        url = self._build_url_with_endpoint(endpoint)
        timeout = self.provider.settings.get("timeout", self.settings.default_timeout)
        session = await self._get_session()

        async def _invoke_with_key_wrapper(api_key: str | None) -> dict[str, Any]:
            if not api_key:
                raise ProviderError("Claude Code Provider 需要 api_key")
            response = await session.get(
                url,
                headers=self._build_headers(api_key),
                timeout=timeout,
            )
            if response.status_code >= 400:
                raise ProviderError(
                    f"Claude Code 请求失败: {response.status_code} {response.text}"
                )
            return response.json()

        return await self._invoke_with_failover(
            _invoke_with_key_wrapper,
            require_api_key=True,
            error_message="Claude Code Provider 需要 api_key",
        )

    def _build_url_with_endpoint(self, endpoint: str) -> str:
        base = (
            self.provider.base_url
            or self.provider.settings.get("base_url")
            or self.DEFAULT_BASE_URL
        ).rstrip("/")
        return urljoin(base + "/", endpoint.lstrip("/"))


__all__ = ["ClaudeCodeProviderClient"]
