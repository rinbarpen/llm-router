from __future__ import annotations

from typing import List
from urllib.parse import urlencode

from ..db.models import Model
from ..schemas import ModelInvokeRequest, ModelInvokeResponse
from .base import BaseProviderClient, ProviderError


class GeminiProviderClient(BaseProviderClient):
    DEFAULT_BASE_URL = "https://generativelanguage.googleapis.com"
    DEFAULT_ENDPOINT_TEMPLATE = "/v1beta/models/{model}:generateContent"

    async def invoke(
        self, model: Model, request: ModelInvokeRequest
    ) -> ModelInvokeResponse:
        contents = self._build_contents(request)
        if not contents:
            raise ProviderError("Gemini 请求需要至少一个消息或提示")

        api_key = self.provider.api_key or self.provider.settings.get("api_key")
        if not api_key:
            raise ProviderError("Gemini Provider 需要 api_key")

        url = self._build_url(model, api_key)
        payload = {"contents": contents}
        payload.update(self.merge_parameters(model, request))

        timeout = self.provider.settings.get("timeout", self.settings.default_timeout)
        headers = {"Content-Type": "application/json"}
        session = await self._get_session()
        response = await session.post(
            url,
            json=payload,
            headers=headers,
            timeout=timeout,
        )

        if response.status_code >= 400:
            raise ProviderError(
                f"Gemini 请求失败: {response.status_code} {response.text}"
            )

        data = response.json()
        output_text = self._extract_output(data)
        return ModelInvokeResponse(output_text=output_text, raw=data)

    def _build_contents(
        self, request: ModelInvokeRequest
    ) -> List[dict]:
        contents: List[dict] = []

        for message in request.messages or []:
            if not message.content:
                continue
            role = "user" if message.role in {"system", "user"} else "model"
            contents.append({"role": role, "parts": [{"text": message.content}]})

        if request.prompt:
            contents.append({"role": "user", "parts": [{"text": request.prompt}]})

        return contents

    def _build_url(self, model: Model, api_key: str) -> str:
        base = (
            self.provider.base_url
            or self.provider.settings.get("base_url")
            or self.DEFAULT_BASE_URL
        ).rstrip("/")

        model_id = (
            model.remote_identifier
            or model.config.get("model")
            or model.name
        )

        endpoint_template = self.provider.settings.get(
            "endpoint_template", self.DEFAULT_ENDPOINT_TEMPLATE
        )
        endpoint = endpoint_template.format(model=model_id)
        query = urlencode({"key": api_key})
        return f"{base}{endpoint}?{query}"

    def _extract_output(self, data: dict) -> str:
        candidates = data.get("candidates") or []
        if not candidates:
            return ""
        candidate = candidates[0]

        parts = []
        if "content" in candidate:
            parts = candidate["content"].get("parts", [])
        elif "parts" in candidate:
            parts = candidate.get("parts", [])

        texts = [
            part.get("text", "")
            for part in parts
            if isinstance(part, dict)
        ]
        return "".join(texts)


__all__ = ["GeminiProviderClient"]


