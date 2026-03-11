from __future__ import annotations

import json
from typing import AsyncIterator, Dict
from urllib.parse import urljoin

from ..db.models import Model, ProviderType
from ..schemas import ModelInvokeRequest, ModelInvokeResponse, ModelStreamChunk
from .base import ProviderError
from .openai_compatible import OpenAICompatibleProviderClient


class AzureOpenAIProviderClient(OpenAICompatibleProviderClient):
    """Azure OpenAI Provider Client
    
    Settings:
        api_version: Azure API version (default: 2024-02-15-preview)
        deployment_id: Azure deployment name (can also be model.remote_identifier)
    """
    
    DEFAULT_API_VERSION = "2024-02-15-preview"

    def _build_headers(self, api_key: str | None = None) -> dict[str, str]:
        headers = {"Content-Type": "application/json"}
        custom = self.provider.settings.get("headers", {})
        headers.update(custom)

        if api_key is None:
            api_key = self.provider.api_key or self.provider.settings.get("api_key")
        
        if api_key:
            # Azure uses api-key header instead of Authorization
            headers["api-key"] = api_key

        return headers

    def _build_url(self) -> str:
        base = (
            self.provider.base_url
            or self.provider.settings.get("base_url")
        )
        if not base:
            raise ProviderError("Azure OpenAI 需要提供 base_url (endpoint)")
        
        base = base.rstrip("/")
        # Azure URL format: {endpoint}/openai/deployments/{deployment_id}/chat/completions?api-version={api_version}
        
        # If the user provided a full deployment URL, use it
        if "/openai/deployments/" in base:
            url = base
        else:
            # We need deployment_id from settings or model
            deployment_id = self.provider.settings.get("deployment_id")
            if not deployment_id:
                # We'll use a placeholder and replace it during actual call if possible
                url = f"{base}/openai/deployments/DEPLOYMENT_ID/chat/completions"
            else:
                url = f"{base}/openai/deployments/{deployment_id}/chat/completions"

        api_version = self.provider.settings.get("api_version", self.DEFAULT_API_VERSION)
        if "?" in url:
            url = f"{url}&api-version={api_version}"
        else:
            url = f"{url}?api-version={api_version}"
            
        return url

    async def _invoke_with_key(
        self, api_key: str | None, model: Model, request: ModelInvokeRequest, messages: list[dict[str, str]]
    ) -> ModelInvokeResponse:
        url = self._build_url()
        if "DEPLOYMENT_ID" in url:
            deployment_id = model.remote_identifier or model.name
            url = url.replace("DEPLOYMENT_ID", deployment_id)
            
        headers = self._build_headers(api_key)
        payload = self._build_payload(model, request, messages)
        if "model" in payload:
            del payload["model"]

        timeout = self.provider.settings.get("timeout", self.settings.default_timeout)
        session = await self._get_session()
        response = await session.post(url, json=payload, headers=headers, timeout=timeout)

        if response.status_code >= 400:
            raise ProviderError(
                f"Azure OpenAI 请求失败: {response.status_code} {response.text}"
            )

        data = response.json()
        output_text = self._extract_output(data)
        return ModelInvokeResponse(output_text=output_text, raw=data)

    async def _stream_invoke_with_key(
        self, api_key: str | None, model: Model, request: ModelInvokeRequest, messages: list[dict[str, str]]
    ) -> AsyncIterator[ModelStreamChunk]:
        url = self._build_url()
        if "DEPLOYMENT_ID" in url:
            deployment_id = model.remote_identifier or model.name
            url = url.replace("DEPLOYMENT_ID", deployment_id)

        headers = self._build_headers(api_key)
        payload = self._build_payload(model, request, messages)
        payload["stream"] = True
        if "model" in payload:
            del payload["model"]

        timeout = self.provider.settings.get("timeout", self.settings.default_timeout)
        session = await self._get_session()
        async with session.stream(
            "POST",
            url,
            json=payload,
            headers=headers,
            timeout=timeout,
        ) as response:
            if response.status_code >= 400:
                error_text = await self._read_response_text(response)
                raise ProviderError(
                    f"Azure OpenAI 请求失败: {response.status_code} {error_text.decode()}"
                )

            async for line in response.aiter_lines():
                if not line: continue
                if line.startswith(":"): continue
                if not line.startswith("data:"): continue
                raw_data = line[5:].strip()
                if not raw_data: continue
                if raw_data == "[DONE]":
                    yield ModelStreamChunk(is_final=True)
                    return

                try:
                    chunk_payload = json.loads(raw_data)
                except json.JSONDecodeError:
                    continue

                choices = chunk_payload.get("choices") or []
                delta = choices[0].get("delta", {}) if choices else {}
                finish_reason = choices[0].get("finish_reason") if choices else None
                text_piece = self._extract_stream_text(delta)

                yield ModelStreamChunk(
                    delta=delta,
                    text=text_piece,
                    raw=chunk_payload,
                    finish_reason=finish_reason,
                    usage=chunk_payload.get("usage"),
                )
