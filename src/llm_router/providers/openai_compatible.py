from __future__ import annotations

import json
from typing import AsyncIterator, Dict
from urllib.parse import urljoin

from ..db.models import Model, ProviderType
from ..schemas import ModelInvokeRequest, ModelInvokeResponse, ModelStreamChunk
from .base import BaseProviderClient, ProviderError


class OpenAICompatibleProviderClient(BaseProviderClient):
    DEFAULT_BASE_URLS: Dict[ProviderType, str] = {
        ProviderType.OPENAI: "https://api.openai.com",
        ProviderType.GROK: "https://api.x.ai",
        ProviderType.DEEPSEEK: "https://api.deepseek.com",
        ProviderType.QWEN: "https://dashscope.aliyuncs.com",
        ProviderType.KIMI: "https://api.moonshot.cn",
        ProviderType.GLM: "https://open.bigmodel.cn/api/paas/v4",
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
        messages = self._build_messages(request)
        if not messages:
            raise ProviderError("prompt 或 messages 至少需要提供一个")

        async def _invoke_with_key_wrapper(api_key: str | None) -> ModelInvokeResponse:
            return await self._invoke_with_key(api_key, model, request, messages)

        return await self._invoke_with_failover(
            _invoke_with_key_wrapper,
            require_api_key=False,
            error_message=f"{self.provider.type.value} 需要至少一个 API key",
        )

    async def _invoke_with_key(
        self, api_key: str | None, model: Model, request: ModelInvokeRequest, messages: list[dict[str, str]]
    ) -> ModelInvokeResponse:
        """使用指定的 API key 发起请求（内部方法）"""
        url = self._build_url()
        headers = self._build_headers(api_key)
        payload = self._build_payload(model, request, messages)

        timeout = self.provider.settings.get("timeout", self.settings.default_timeout)
        session = await self._get_session()
        response = await session.post(url, json=payload, headers=headers, timeout=timeout)

        if response.status_code >= 400:
            raise ProviderError(
                f"{self.provider.type.value} 请求失败: {response.status_code} {response.text}"
            )

        data = response.json()
        output_text = self._extract_output(data)
        return ModelInvokeResponse(output_text=output_text, raw=data)

    async def stream_invoke(
        self, model: Model, request: ModelInvokeRequest
    ) -> AsyncIterator[ModelStreamChunk]:
        messages = self._build_messages(request)
        if not messages:
            raise ProviderError("prompt 或 messages 至少需要提供一个")

        api_keys = self._get_api_keys()
        if not api_keys:
            # 如果没有配置 API key，尝试不使用 key（某些服务可能不需要）
            async for chunk in self._stream_invoke_with_key(None, model, request, messages):
                yield chunk
            return

        url = self._build_url()
        payload = self._build_payload(model, request, messages)
        payload["stream"] = True
        timeout = self.provider.settings.get("timeout", self.settings.default_timeout)
        session = await self._get_session()

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
                        error_text = await response.aread()
                        # 检查是否为可重试错误
                        if self._is_retryable_error(response.status_code) and api_key != api_keys[-1]:
                            last_error = ProviderError(
                                f"{self.provider.type.value} 请求失败: {response.status_code} {error_text.decode()}"
                            )
                            continue  # 尝试下一个 key
                        else:
                            raise ProviderError(
                                f"{self.provider.type.value} 请求失败: {response.status_code} {error_text.decode()}"
                            )

                    # 成功，流式返回结果
                    async for line in response.aiter_lines():
                        if not line:
                            continue
                        if line.startswith(":"):
                            continue
                        if not line.startswith("data:"):
                            continue
                        raw_data = line[5:].strip()
                        if not raw_data:
                            continue
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
                    return  # 成功返回
            except ProviderError as e:
                # 如果是最后一个 key，直接抛出错误
                if api_key == api_keys[-1]:
                    raise
                # 否则检查是否为可重试错误
                status_code = self._extract_status_code_from_error(e)
                if status_code and self._is_retryable_error(status_code):
                    last_error = e
                    continue
                else:
                    # 非可重试错误直接抛出
                    raise

        # 所有 key 都失败
        if last_error:
            raise last_error
        raise ProviderError("所有 API key 都不可用")

    async def _stream_invoke_with_key(
        self, api_key: str | None, model: Model, request: ModelInvokeRequest, messages: list[dict[str, str]]
    ) -> AsyncIterator[ModelStreamChunk]:
        """使用指定的 API key 发起流式请求（内部方法）"""
        url = self._build_url()
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
                error_text = await response.aread()
                raise ProviderError(
                    f"{self.provider.type.value} 请求失败: {response.status_code} {error_text.decode()}"
                )

            async for line in response.aiter_lines():
                if not line:
                    continue
                if line.startswith(":"):
                    continue
                if not line.startswith("data:"):
                    continue
                raw_data = line[5:].strip()
                if not raw_data:
                    continue
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
            "model": self._resolve_model_identifier(model, request),
            "messages": messages,
        }
        payload.update(self.merge_parameters(model, request))
        return payload

    def _build_headers(self, api_key: str | None = None) -> dict[str, str]:
        """构建请求头，支持指定 API key"""
        headers = {"Content-Type": "application/json"}
        custom = self.provider.settings.get("headers", {})
        headers.update(custom)

        # 如果指定了 api_key，使用指定的；否则使用 provider 的 api_key
        if api_key is None:
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

    def _resolve_model_identifier(self, model: Model, request: ModelInvokeRequest) -> str:
        # 优先使用请求中的 remote_identifier_override（用于 OpenAI 兼容 API）
        if request.remote_identifier_override:
            return request.remote_identifier_override
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

    def _extract_stream_text(self, delta: dict) -> str | None:
        text = delta.get("content")
        if isinstance(text, list):
            return "".join(part for part in text if isinstance(part, str)) or None
        if isinstance(text, str):
            return text
        if "text" in delta and isinstance(delta["text"], str):
            return delta["text"]
        return None


__all__ = ["OpenAICompatibleProviderClient"]


