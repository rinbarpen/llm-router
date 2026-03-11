from __future__ import annotations

import base64
import mimetypes
import json
from typing import Any, AsyncIterator, Dict
from urllib.parse import urljoin

from ..db.models import Model, ProviderType
from ..schemas import ModelInvokeRequest, ModelInvokeResponse, ModelStreamChunk
from .base import BaseProviderClient, ProviderError


class OpenAICompatibleProviderClient(BaseProviderClient):
    QWEN_NATIVE_TTS_ENDPOINT = "/api/v1/services/aigc/multimodal-generation/generation"
    DEFAULT_BASE_URLS: Dict[ProviderType, str] = {
        ProviderType.OPENAI: "https://api.openai.com/v1",
        ProviderType.GROK: "https://api.x.ai",
        ProviderType.DEEPSEEK: "https://api.deepseek.com",
        ProviderType.QWEN: "https://dashscope.aliyuncs.com",
        ProviderType.KIMI: "https://api.moonshot.cn",
        ProviderType.GLM: "https://open.bigmodel.cn/api/paas/v4",
        ProviderType.OPENROUTER: "https://openrouter.ai/api",
        ProviderType.MINIMAX: "https://api.minimax.chat/v1",
        ProviderType.DOUBAO: "https://ark.cn-beijing.volces.com/api/v3",
        ProviderType.GROQ: "https://api.groq.com/openai/v1",
        ProviderType.SILICONFLOW: "https://api.siliconflow.cn/v1",
        ProviderType.AIHUBMIX: "https://aihubmix.com/v1",
        ProviderType.VOLCENGINE: "https://ark.cn-beijing.volces.com/api/v3",
    }

    ENDPOINT_OVERRIDES: Dict[ProviderType, str] = {
        ProviderType.QWEN: "/compatible-mode/v1/chat/completions",
        ProviderType.GLM: "/chat/completions",
        ProviderType.MINIMAX: "/text/chatcompletion_v2",
        ProviderType.DOUBAO: "/chat/completions",
        ProviderType.VOLCENGINE: "/chat/completions",
    }

    AUDIO_SPEECH_ENDPOINT_OVERRIDES: Dict[ProviderType, str] = {
        ProviderType.QWEN: "/compatible-mode/v1/audio/speech",
    }

    DEFAULT_ENDPOINT = "/v1/chat/completions"
    DEFAULT_EMBEDDINGS_ENDPOINT = "/v1/embeddings"
    DEFAULT_AUDIO_SPEECH_ENDPOINT = "/v1/audio/speech"
    DEFAULT_AUDIO_TRANSCRIPTIONS_ENDPOINT = "/v1/audio/transcriptions"
    DEFAULT_AUDIO_TRANSLATIONS_ENDPOINT = "/v1/audio/translations"
    DEFAULT_IMAGES_GENERATIONS_ENDPOINT = "/v1/images/generations"
    DEFAULT_VIDEOS_GENERATIONS_ENDPOINT = "/v1/videos/generations"

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
                        error_text = await self._read_response_text(response)
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
                error_text = await self._read_response_text(response)
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
        ).rstrip("/")
        endpoint = (
            self.provider.settings.get("endpoint")
            or self.ENDPOINT_OVERRIDES.get(self.provider.type)
            or self.DEFAULT_ENDPOINT
        ).lstrip("/")
        # base 已含 /v1 且 endpoint 为 v1/chat/completions 时只拼 chat/completions，避免 /v1/v1/
        if base.endswith("/v1") and endpoint == "v1/chat/completions":
            endpoint = "chat/completions"
        return urljoin(base + "/", endpoint)

    def _build_url_with_endpoint(self, endpoint: str) -> str:
        base = (
            self.provider.base_url
            or self.provider.settings.get("base_url")
            or self.DEFAULT_BASE_URLS.get(self.provider.type)
            or self.DEFAULT_BASE_URLS[ProviderType.OPENAI]
        ).rstrip("/")
        return urljoin(base + "/", endpoint.lstrip("/"))

    def _resolve_model_identifier(self, model: Model, request: ModelInvokeRequest) -> str:
        # 优先使用请求中的 remote_identifier_override（用于 OpenAI 兼容 API）
        if request.remote_identifier_override:
            return request.remote_identifier_override
        return (
            model.remote_identifier
            or model.config.get("model")
            or model.name
        )

    def _resolve_model_identifier_without_request(self, model: Model) -> str:
        return (
            model.remote_identifier
            or model.config.get("model")
            or model.name
        )

    async def _post_json_with_failover(
        self, endpoint: str, payload: dict[str, Any], *, require_api_key: bool = False
    ) -> dict[str, Any]:
        url = self._build_url_with_endpoint(endpoint)
        timeout = self.provider.settings.get("timeout", self.settings.default_timeout)
        session = await self._get_session()

        async def _invoke_with_key_wrapper(api_key: str | None) -> dict[str, Any]:
            headers = self._build_headers(api_key)
            response = await session.post(url, json=payload, headers=headers, timeout=timeout)
            if response.status_code >= 400:
                raise ProviderError(
                    f"{self.provider.type.value} 请求失败: {response.status_code} {response.text}"
                )
            return response.json()

        return await self._invoke_with_failover(
            _invoke_with_key_wrapper,
            require_api_key=require_api_key,
            error_message=f"{self.provider.type.value} 需要至少一个 API key",
        )

    async def embed(self, model: Model, payload: dict[str, Any]) -> dict[str, Any]:
        endpoint = self.provider.settings.get("embeddings_endpoint", self.DEFAULT_EMBEDDINGS_ENDPOINT)
        body = dict(payload)
        body.setdefault("model", self._resolve_model_identifier_without_request(model))
        return await self._post_json_with_failover(endpoint, body, require_api_key=False)

    async def synthesize_speech(self, model: Model, payload: dict[str, Any]) -> tuple[bytes, str]:
        if self._use_qwen_native_tts():
            return await self._synthesize_qwen_native_speech(model, payload)

        endpoint = self.provider.settings.get(
            "audio_speech_endpoint",
            self.AUDIO_SPEECH_ENDPOINT_OVERRIDES.get(self.provider.type, self.DEFAULT_AUDIO_SPEECH_ENDPOINT),
        )
        body = dict(payload)
        body.setdefault("model", self._resolve_model_identifier_without_request(model))
        url = self._build_url_with_endpoint(endpoint)
        timeout = self.provider.settings.get("timeout", self.settings.default_timeout)
        session = await self._get_session()

        async def _invoke_with_key_wrapper(api_key: str | None) -> tuple[bytes, str]:
            headers = self._build_headers(api_key)
            response = await session.post(url, json=body, headers=headers, timeout=timeout)
            if response.status_code >= 400:
                raise ProviderError(
                    f"{self.provider.type.value} 请求失败: {response.status_code} {response.text}"
                )
            content_type = response.headers.get("content-type", "audio/mpeg")
            return response.content, content_type

        return await self._invoke_with_failover(
            _invoke_with_key_wrapper,
            require_api_key=False,
            error_message=f"{self.provider.type.value} 需要至少一个 API key",
        )

    def _use_qwen_native_tts(self) -> bool:
        mode = str(self.provider.settings.get("audio_mode", "qwen_native_tts")).strip().lower()
        return self.provider.type == ProviderType.QWEN and mode == "qwen_native_tts"

    async def _synthesize_qwen_native_speech(
        self, model: Model, payload: dict[str, Any]
    ) -> tuple[bytes, str]:
        voice = payload.get("voice")
        input_block: dict = {"text": payload.get("input", "")}
        if voice:
            input_block["voice"] = voice
        body = {
            "model": self._resolve_model_identifier_without_request(model),
            "input": input_block,
            "parameters": {},
        }
        for field in ("instructions", "optimize_instructions", "language_type", "stream"):
            value = payload.get(field)
            if value is not None:
                body["parameters"][field] = value
        if not body["input"]["text"]:
            raise ProviderError("Qwen TTS 需要 input 文本")
        if not voice and not body["parameters"].get("instructions"):
            raise ProviderError("Qwen TTS 需要 voice 或 instructions")

        url = self._build_qwen_native_tts_url()
        timeout = self.provider.settings.get("timeout", self.settings.default_timeout)
        fetch_timeout = self.provider.settings.get("audio_fetch_timeout", timeout)
        session = await self._get_session()

        async def _invoke_with_key_wrapper(api_key: str | None) -> tuple[bytes, str]:
            headers = self._build_headers(api_key)
            response = await session.post(url, json=body, headers=headers, timeout=timeout)
            if response.status_code >= 400:
                raise ProviderError(
                    f"{self.provider.type.value} 请求失败: {response.status_code} {response.text}"
                )

            data = response.json()
            audio_payload = ((data.get("output") or {}).get("audio") or {}) if isinstance(data, dict) else {}

            audio_data = audio_payload.get("data")
            if isinstance(audio_data, str) and audio_data:
                try:
                    decoded = base64.b64decode(audio_data)
                except Exception as exc:
                    raise ProviderError("Qwen TTS 返回了无效的 base64 音频数据") from exc
                return decoded, self._guess_audio_media_type(
                    audio_payload.get("mime_type"),
                    None,
                    payload.get("response_format"),
                )

            audio_url = audio_payload.get("url")
            if isinstance(audio_url, str) and audio_url:
                download = await session.get(audio_url, timeout=fetch_timeout)
                if download.status_code >= 400:
                    raise ProviderError(
                        f"{self.provider.type.value} 音频下载失败: {download.status_code} {download.text}"
                    )
                return download.content, self._guess_audio_media_type(
                    download.headers.get("content-type"),
                    audio_url,
                    payload.get("response_format"),
                )

            raise ProviderError("Qwen TTS 响应中缺少 output.audio.url 或 output.audio.data")

        return await self._invoke_with_failover(
            _invoke_with_key_wrapper,
            require_api_key=False,
            error_message=f"{self.provider.type.value} 需要至少一个 API key",
        )

    def _build_qwen_native_tts_url(self) -> str:
        base = (
            self.provider.base_url
            or self.provider.settings.get("base_url")
            or self.DEFAULT_BASE_URLS.get(self.provider.type)
            or self.DEFAULT_BASE_URLS[ProviderType.OPENAI]
        ).rstrip("/")
        for suffix in ("/compatible-mode/v1", "/v1", "/api/v1"):
            if base.endswith(suffix):
                base = base[: -len(suffix)]
                break
        return urljoin(base + "/", self.QWEN_NATIVE_TTS_ENDPOINT.lstrip("/"))

    def _guess_audio_media_type(
        self,
        content_type: Any,
        source_url: str | None,
        response_format: Any,
    ) -> str:
        if isinstance(content_type, str):
            normalized = content_type.split(";", 1)[0].strip()
            if normalized:
                return normalized

        if isinstance(response_format, str):
            guessed = mimetypes.guess_type(f"file.{response_format.strip('.')}")[0]
            if guessed:
                return guessed

        if source_url:
            guessed = mimetypes.guess_type(source_url)[0]
            if guessed:
                return guessed

        return "audio/mpeg"

    async def transcribe_audio(
        self,
        model: Model,
        data: bytes,
        filename: str,
        mime_type: str,
        extra_payload: dict[str, Any],
    ) -> dict[str, Any]:
        endpoint = self.provider.settings.get(
            "audio_transcriptions_endpoint", self.DEFAULT_AUDIO_TRANSCRIPTIONS_ENDPOINT
        )
        return await self._audio_multipart_request(endpoint, model, data, filename, mime_type, extra_payload)

    async def translate_audio(
        self,
        model: Model,
        data: bytes,
        filename: str,
        mime_type: str,
        extra_payload: dict[str, Any],
    ) -> dict[str, Any]:
        endpoint = self.provider.settings.get(
            "audio_translations_endpoint", self.DEFAULT_AUDIO_TRANSLATIONS_ENDPOINT
        )
        return await self._audio_multipart_request(endpoint, model, data, filename, mime_type, extra_payload)

    async def _audio_multipart_request(
        self,
        endpoint: str,
        model: Model,
        data: bytes,
        filename: str,
        mime_type: str,
        extra_payload: dict[str, Any],
    ) -> dict[str, Any]:
        url = self._build_url_with_endpoint(endpoint)
        timeout = self.provider.settings.get("timeout", self.settings.default_timeout)
        session = await self._get_session()

        form_data = {k: str(v) for k, v in extra_payload.items() if v is not None}
        form_data["model"] = self._resolve_model_identifier_without_request(model)

        async def _invoke_with_key_wrapper(api_key: str | None) -> dict[str, Any]:
            headers = self._build_headers(api_key)
            headers.pop("Content-Type", None)
            files = {"file": (filename, data, mime_type)}
            response = await session.post(
                url,
                data=form_data,
                files=files,
                headers=headers,
                timeout=timeout,
            )
            if response.status_code >= 400:
                raise ProviderError(
                    f"{self.provider.type.value} 请求失败: {response.status_code} {response.text}"
                )
            return response.json()

        return await self._invoke_with_failover(
            _invoke_with_key_wrapper,
            require_api_key=False,
            error_message=f"{self.provider.type.value} 需要至少一个 API key",
        )

    async def generate_image(self, model: Model, payload: dict[str, Any]) -> dict[str, Any]:
        endpoint = self.provider.settings.get(
            "images_generations_endpoint", self.DEFAULT_IMAGES_GENERATIONS_ENDPOINT
        )
        body = dict(payload)
        body.setdefault("model", self._resolve_model_identifier_without_request(model))
        return await self._post_json_with_failover(endpoint, body, require_api_key=False)

    async def generate_video(self, model: Model, payload: dict[str, Any]) -> dict[str, Any]:
        endpoint = self.provider.settings.get(
            "videos_generations_endpoint", self.DEFAULT_VIDEOS_GENERATIONS_ENDPOINT
        )
        body = dict(payload)
        body.setdefault("model", self._resolve_model_identifier_without_request(model))
        return await self._post_json_with_failover(endpoint, body, require_api_key=False)

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
