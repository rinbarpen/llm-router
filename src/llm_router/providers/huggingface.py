from __future__ import annotations

import json
from typing import AsyncIterator, Dict
from urllib.parse import urljoin

from ..db.models import Model, ProviderType
from ..schemas import ModelInvokeRequest, ModelInvokeResponse, ModelStreamChunk
from .base import ProviderError
from .openai_compatible import OpenAICompatibleProviderClient


class HuggingFaceProviderClient(OpenAICompatibleProviderClient):
    """Hugging Face Inference API Provider Client
    
    Supports both Serverless Inference API and Dedicated Endpoints.
    """
    
    DEFAULT_BASE_URL = "https://api-inference.huggingface.co/models/"

    def _build_url(self) -> str:
        base = (
            self.provider.base_url
            or self.provider.settings.get("base_url")
            or self.DEFAULT_BASE_URL
        ).rstrip("/")
        
        return base

    async def _invoke_with_key(
        self, api_key: str | None, model: Model, request: ModelInvokeRequest, messages: list[dict[str, str]]
    ) -> ModelInvokeResponse:
        base_url = self._build_url()
        if base_url.startswith("https://api-inference.huggingface.co"):
            model_id = model.remote_identifier or model.name
            url = f"{base_url}/{model_id}"
            if not url.endswith("/v1/chat/completions"):
                url = urljoin(url + "/", "v1/chat/completions")
        else:
            url = base_url
            if not url.endswith("/v1/chat/completions"):
                url = urljoin(url + "/", "v1/chat/completions")

        headers = self._build_headers(api_key)
        payload = self._build_payload(model, request, messages)

        timeout = self.provider.settings.get("timeout", self.settings.default_timeout)
        session = await self._get_session()
        response = await session.post(url, json=payload, headers=headers, timeout=timeout)

        if response.status_code >= 400:
            raise ProviderError(
                f"HuggingFace 请求失败: {response.status_code} {response.text}"
            )

        data = response.json()
        output_text = self._extract_output(data)
        return ModelInvokeResponse(output_text=output_text, raw=data)

    async def _stream_invoke_with_key(
        self, api_key: str | None, model: Model, request: ModelInvokeRequest, messages: list[dict[str, str]]
    ) -> AsyncIterator[ModelStreamChunk]:
        base_url = self._build_url()
        if base_url.startswith("https://api-inference.huggingface.co"):
            model_id = model.remote_identifier or model.name
            url = f"{base_url}/{model_id}"
            if not url.endswith("/v1/chat/completions"):
                url = urljoin(url + "/", "v1/chat/completions")
        else:
            url = base_url
            if not url.endswith("/v1/chat/completions"):
                url = urljoin(url + "/", "v1/chat/completions")

        headers = self._build_headers(api_key)
        payload = self._build_payload(model, request, messages)
        payload["stream"] = True

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
                    f"HuggingFace 请求失败: {response.status_code} {error_text.decode()}"
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
