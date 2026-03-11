from __future__ import annotations

import json
from typing import AsyncIterator, List
from urllib.parse import urlencode

from ..db.models import Model
from ..schemas import ModelInvokeRequest, ModelInvokeResponse, ModelStreamChunk
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

        payload = {"contents": contents}
        payload.update(self.merge_parameters(model, request))

        timeout = self.provider.settings.get("timeout", self.settings.default_timeout)
        headers = {"Content-Type": "application/json"}
        session = await self._get_session()

        async def _invoke_with_key_wrapper(api_key: str | None) -> ModelInvokeResponse:
            if not api_key:
                raise ProviderError("Gemini Provider 需要 api_key")
            url = self._build_url(model, api_key)
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

        return await self._invoke_with_failover(
            _invoke_with_key_wrapper,
            require_api_key=True,
            error_message="Gemini Provider 需要 api_key",
        )

    def _build_contents(
        self, request: ModelInvokeRequest
    ) -> List[dict]:
        contents: List[dict] = []

        for message in request.messages or []:
            content = message.content
            if content is None or content == "":
                continue
            role = "user" if message.role in {"system", "user"} else "model"
            parts = self._message_content_to_parts(content)
            if parts:
                contents.append({"role": role, "parts": parts})

        if request.prompt:
            contents.append({"role": "user", "parts": [{"text": request.prompt}]})

        return contents

    def _message_content_to_parts(self, content: str | list) -> List[dict]:
        """将 content（str 或多模态列表）转为 Gemini parts"""
        if isinstance(content, str):
            if content.startswith("data:") and ";base64," in content:
                mime, data = self._parse_data_url(content)
                if data:
                    return [{"inlineData": {"mimeType": mime, "data": data}}]
            return [{"text": content}]
        if isinstance(content, list):
            parts = []
            for item in content:
                if not isinstance(item, dict):
                    continue
                t = item.get("type")
                if t == "text":
                    text = item.get("text", "")
                    if text:
                        parts.append({"text": text})
                elif t == "image_url":
                    url = item.get("url", "")
                    if url.startswith("data:") and ";base64," in url:
                        mime, data = self._parse_data_url(url)
                        if data:
                            parts.append({"inlineData": {"mimeType": mime, "data": data}})
                    elif url:
                        parts.append({"text": url})
            return parts if parts else [{"text": ""}]
        return [{"text": str(content)}]

    def _parse_data_url(self, url: str) -> tuple[str, str]:
        import re
        m = re.match(r"data:([^;]+);base64,(.+)", url.strip())
        if m:
            return m.group(1).strip(), m.group(2).strip()
        return "image/png", ""

    def _build_url(self, model: Model, api_key: str, stream: bool = False) -> str:
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

        template = self.DEFAULT_ENDPOINT_TEMPLATE
        if stream:
            template = "/v1beta/models/{model}:streamGenerateContent"
        endpoint_template = self.provider.settings.get(
            "endpoint_template", template
        )
        if stream and "streamGenerateContent" not in endpoint_template:
            endpoint_template = "/v1beta/models/{model}:streamGenerateContent"
        endpoint = endpoint_template.format(model=model_id)
        params = {"key": api_key}
        if stream:
            params["alt"] = "sse"
        query = urlencode(params)
        return f"{base}{endpoint}?{query}"

    async def stream_invoke(
        self, model: Model, request: ModelInvokeRequest
    ) -> AsyncIterator[ModelStreamChunk]:
        contents = self._build_contents(request)
        if not contents:
            raise ProviderError("Gemini 请求需要至少一个消息或提示")

        payload = {"contents": contents}
        params = self.merge_parameters(model, request)
        # generationConfig for Gemini
        gen_config = {}
        if "max_tokens" in params:
            gen_config["maxOutputTokens"] = params.pop("max_tokens")
        if "temperature" in params:
            gen_config["temperature"] = params.pop("temperature")
        if "top_p" in params:
            gen_config["topP"] = params.pop("top_p")
        if "top_k" in params:
            gen_config["topK"] = params.pop("top_k")
        if "stop" in params:
            gen_config["stopSequences"] = params.pop("stop", [])
        if gen_config:
            payload["generationConfig"] = gen_config
        payload.update(params)

        timeout = self.provider.settings.get("timeout", self.settings.default_timeout)
        session = await self._get_session()

        api_keys = self._get_api_keys()
        if not api_keys:
            raise ProviderError("Gemini Provider 需要 api_key")

        last_error = None
        for api_key in api_keys:
            try:
                url = self._build_url(model, api_key, stream=True)
                async with session.stream(
                    "POST",
                    url,
                    json=payload,
                    headers={"Content-Type": "application/json"},
                    timeout=timeout,
                ) as response:
                    if response.status_code >= 400:
                        error_text = await self._read_response_text(response)
                        err = ProviderError(
                            f"Gemini 请求失败: {response.status_code} {error_text.decode()}"
                        )
                        if api_key != api_keys[-1] and self._is_retryable_error(
                            response.status_code
                        ):
                            last_error = err
                            continue
                        raise err
                    buffer = ""
                    async for chunk in response.aiter_bytes():
                        buffer += chunk.decode("utf-8", errors="replace")
                        while "\n" in buffer or "\r\n" in buffer:
                            line, _, buffer = buffer.partition("\n")
                            line = line.strip()
                            if not line or line.startswith(":"):
                                continue
                            if not line.startswith("data:"):
                                continue
                            raw_data = line[5:].strip()
                            if not raw_data:
                                continue
                            try:
                                data = json.loads(raw_data)
                            except json.JSONDecodeError:
                                continue
                            text_piece = self._extract_stream_text(data)
                            usage = None
                            if "usageMetadata" in data:
                                um = data["usageMetadata"]
                                usage = {
                                    "prompt_tokens": um.get("promptTokenCount", 0),
                                    "completion_tokens": um.get(
                                        "candidatesTokenCount", 0
                                    ),
                                    "total_tokens": um.get("totalTokenCount", 0),
                                }
                            finish_reason = None
                            candidates = data.get("candidates") or []
                            if candidates:
                                c = candidates[0]
                                finish_reason = c.get("finishReason") or c.get(
                                    "finish_reason"
                                )
                            yield ModelStreamChunk(
                                text=text_piece,
                                raw=data,
                                finish_reason=finish_reason,
                                usage=usage,
                            )
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
        raise ProviderError("Gemini Provider 需要 api_key")

    def _extract_stream_text(self, data: dict) -> str | None:
        candidates = data.get("candidates") or []
        if not candidates:
            return None
        candidate = candidates[0]
        content = candidate.get("content") or {}
        parts = content.get("parts", [])
        texts = [
            p.get("text", "")
            for p in parts
            if isinstance(p, dict) and "text" in p
        ]
        return "".join(texts) if texts else None

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


