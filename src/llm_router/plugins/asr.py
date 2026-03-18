"""ASR (语音识别/转写) 能力级插件。"""

from __future__ import annotations

import asyncio
import base64
import hashlib
import hmac
import inspect
import json
import logging
import os
import time
from datetime import datetime, timezone
from email.utils import format_datetime
from importlib.metadata import entry_points
from typing import Any, Dict, Optional, Protocol
from urllib.parse import urlencode, urljoin, urlparse

from curl_cffi import requests

logger = logging.getLogger(__name__)


class ASRPlugin(Protocol):
    """ASR 插件协议。"""

    plugin_id: str

    async def transcribe_audio(
        self,
        model_id: str,
        data: bytes,
        filename: str,
        mime_type: str,
        extra_payload: dict[str, Any],
        config: dict[str, Any],
    ) -> dict[str, Any]:
        ...

    async def translate_audio(
        self,
        model_id: str,
        data: bytes,
        filename: str,
        mime_type: str,
        extra_payload: dict[str, Any],
        config: dict[str, Any],
    ) -> dict[str, Any]:
        ...


class _BaseHTTPPlugin:
    plugin_id = "base"

    def __init__(self) -> None:
        self._session: Optional[Any] = None
        self._session_key: Optional[tuple[Any, ...]] = None

    def _build_session_key(self, config: dict[str, Any]) -> tuple[Any, ...]:
        timeout = float(config.get("timeout", 60.0))
        proxy = config.get("proxy")
        return (timeout, proxy)

    async def _get_session(self, config: dict[str, Any]) -> Any:
        session_key = self._build_session_key(config)
        if self._session is not None and self._session_key != session_key:
            await self._session.close()
            self._session = None

        if self._session is None:
            timeout = float(config.get("timeout", 60.0))
            options: dict[str, Any] = {
                "timeout": timeout,
                "allow_redirects": True,
                "trust_env": False,
            }
            proxy = config.get("proxy")
            if proxy:
                options["proxy"] = proxy
            self._session = requests.AsyncSession(**options)
            self._session_key = session_key

        return self._session

    async def aclose(self) -> None:
        if self._session is not None:
            await self._session.close()
            self._session = None

    def _resolve_secret(
        self,
        config: dict[str, Any],
        value_key: str,
        env_key: str,
    ) -> str:
        value = str(config.get(value_key) or "").strip()
        if value:
            return value
        env_name = str(config.get(env_key) or "").strip()
        if env_name:
            return str(os.getenv(env_name, "")).strip()
        return ""

    def _require_base_url(self, config: dict[str, Any], default_url: str = "") -> str:
        base_url = str(config.get("base_url") or default_url).strip()
        if not base_url:
            raise RuntimeError(f"ASR 插件 {self.plugin_id} 缺少 base_url 配置")
        return base_url


def _as_float(value: Any, fallback: float) -> float:
    try:
        return float(value)
    except (TypeError, ValueError):
        return fallback


def _extract_text(value: Any) -> str:
    if isinstance(value, str):
        return value
    if isinstance(value, dict):
        for key in ("text", "transcript", "result", "display_text"):
            text = _extract_text(value.get(key))
            if text:
                return text
        combined = []
        for key in ("combinedPhrases", "phrases", "segments", "alternatives", "channels", "results"):
            child = value.get(key)
            text = _extract_text(child)
            if text:
                combined.append(text)
        return " ".join(s for s in combined if s).strip()
    if isinstance(value, list):
        parts = [_extract_text(item) for item in value]
        return " ".join(s for s in parts if s).strip()
    return ""


class OpenAICompatibleASRPlugin(_BaseHTTPPlugin):
    """将 ASR 请求转发到 OpenAI 兼容接口。"""

    plugin_id = "openai_compatible"

    async def _audio_request(
        self,
        model_id: str,
        data: bytes,
        filename: str,
        mime_type: str,
        extra_payload: dict[str, Any],
        config: dict[str, Any],
        endpoint_key: str,
        default_endpoint: str,
    ) -> dict[str, Any]:
        base_url = self._require_base_url(config)
        endpoint = str(config.get(endpoint_key) or default_endpoint).strip()
        timeout = _as_float(config.get("timeout"), 60.0)

        api_key = self._resolve_secret(config, "api_key", "api_key_env")
        headers: dict[str, str] = {}
        if api_key:
            headers["Authorization"] = f"Bearer {api_key}"

        url = urljoin(base_url.rstrip("/") + "/", endpoint.lstrip("/"))

        form_data = {k: str(v) for k, v in extra_payload.items() if v is not None}
        form_data["model"] = model_id
        files = {"file": (filename, data, mime_type)}

        session = await self._get_session(config)
        response = await session.post(
            url,
            data=form_data,
            files=files,
            headers=headers,
            timeout=timeout,
        )
        if response.status_code >= 400:
            raise RuntimeError(
                f"ASR 插件 {self.plugin_id} 请求失败: {response.status_code} {response.text}"
            )
        payload = response.json()
        if isinstance(payload, dict):
            return payload
        return {"text": _extract_text(payload)}

    async def transcribe_audio(
        self,
        model_id: str,
        data: bytes,
        filename: str,
        mime_type: str,
        extra_payload: dict[str, Any],
        config: dict[str, Any],
    ) -> dict[str, Any]:
        return await self._audio_request(
            model_id,
            data,
            filename,
            mime_type,
            extra_payload,
            config,
            "audio_transcriptions_endpoint",
            "/v1/audio/transcriptions",
        )

    async def translate_audio(
        self,
        model_id: str,
        data: bytes,
        filename: str,
        mime_type: str,
        extra_payload: dict[str, Any],
        config: dict[str, Any],
    ) -> dict[str, Any]:
        return await self._audio_request(
            model_id,
            data,
            filename,
            mime_type,
            extra_payload,
            config,
            "audio_translations_endpoint",
            "/v1/audio/translations",
        )


class DeepgramASRPlugin(_BaseHTTPPlugin):
    plugin_id = "deepgram"

    async def _run_listen(
        self,
        model_id: str,
        data: bytes,
        mime_type: str,
        extra_payload: dict[str, Any],
        config: dict[str, Any],
    ) -> dict[str, Any]:
        base_url = self._require_base_url(config, "https://api.deepgram.com")
        endpoint = str(config.get("listen_endpoint") or "/v1/listen").strip()
        timeout = _as_float(config.get("timeout"), 60.0)
        api_key = self._resolve_secret(config, "api_key", "api_key_env")
        if not api_key:
            raise RuntimeError("ASR 插件 deepgram 缺少 api_key 或 api_key_env")

        query: dict[str, Any] = {"model": model_id}
        for key in ("language", "punctuate", "diarize", "smart_format"):
            value = extra_payload.get(key)
            if value is not None:
                query[key] = value

        url = urljoin(base_url.rstrip("/") + "/", endpoint.lstrip("/"))
        session = await self._get_session(config)
        response = await session.post(
            url,
            params=query,
            content=data,
            headers={
                "Authorization": f"Token {api_key}",
                "Content-Type": mime_type or "application/octet-stream",
            },
            timeout=timeout,
        )
        if response.status_code >= 400:
            raise RuntimeError(f"ASR 插件 deepgram 请求失败: {response.status_code} {response.text}")
        payload = response.json()
        transcript = _extract_text(payload)
        return {"text": transcript}

    async def transcribe_audio(self, model_id: str, data: bytes, filename: str, mime_type: str, extra_payload: dict[str, Any], config: dict[str, Any]) -> dict[str, Any]:
        return await self._run_listen(model_id, data, mime_type, extra_payload, config)

    async def translate_audio(self, model_id: str, data: bytes, filename: str, mime_type: str, extra_payload: dict[str, Any], config: dict[str, Any]) -> dict[str, Any]:
        return await self._run_listen(model_id, data, mime_type, extra_payload, config)


class AssemblyAIASRPlugin(_BaseHTTPPlugin):
    plugin_id = "assemblyai"

    async def _upload_audio(self, data: bytes, config: dict[str, Any], api_key: str) -> str:
        base_url = self._require_base_url(config, "https://api.assemblyai.com")
        upload_endpoint = str(config.get("upload_endpoint") or "/v2/upload").strip()
        timeout = _as_float(config.get("timeout"), 60.0)
        upload_url = urljoin(base_url.rstrip("/") + "/", upload_endpoint.lstrip("/"))
        session = await self._get_session(config)
        response = await session.post(
            upload_url,
            content=data,
            headers={"Authorization": api_key, "Content-Type": "application/octet-stream"},
            timeout=timeout,
        )
        if response.status_code >= 400:
            raise RuntimeError(f"ASR 插件 assemblyai 上传失败: {response.status_code} {response.text}")
        payload = response.json()
        audio_url = str(payload.get("upload_url") or "").strip()
        if not audio_url:
            raise RuntimeError("ASR 插件 assemblyai 上传成功但未返回 upload_url")
        return audio_url

    async def _create_and_poll(
        self,
        model_id: str,
        audio_url: str,
        extra_payload: dict[str, Any],
        config: dict[str, Any],
        api_key: str,
    ) -> dict[str, Any]:
        base_url = self._require_base_url(config, "https://api.assemblyai.com")
        transcript_endpoint = str(config.get("transcript_endpoint") or "/v2/transcript").strip()
        timeout = _as_float(config.get("timeout"), 60.0)
        poll_interval = _as_float(config.get("poll_interval"), 1.5)
        poll_attempts = int(config.get("poll_attempts", 60))

        session = await self._get_session(config)
        start_url = urljoin(base_url.rstrip("/") + "/", transcript_endpoint.lstrip("/"))
        start_body: dict[str, Any] = {
            "audio_url": audio_url,
            "speech_model": model_id,
        }
        for key in ("language_code", "punctuate", "format_text", "speaker_labels"):
            if extra_payload.get(key) is not None:
                start_body[key] = extra_payload[key]

        start_resp = await session.post(
            start_url,
            json=start_body,
            headers={"Authorization": api_key, "Content-Type": "application/json"},
            timeout=timeout,
        )
        if start_resp.status_code >= 400:
            raise RuntimeError(f"ASR 插件 assemblyai 创建任务失败: {start_resp.status_code} {start_resp.text}")
        start_payload = start_resp.json()
        task_id = str(start_payload.get("id") or "").strip()
        if not task_id:
            raise RuntimeError("ASR 插件 assemblyai 创建任务成功但未返回 id")

        status_url = urljoin(base_url.rstrip("/") + "/", f"v2/transcript/{task_id}")
        for _ in range(max(poll_attempts, 1)):
            status_resp = await session.get(status_url, headers={"Authorization": api_key}, timeout=timeout)
            if status_resp.status_code >= 400:
                raise RuntimeError(
                    f"ASR 插件 assemblyai 查询任务失败: {status_resp.status_code} {status_resp.text}"
                )
            status_payload = status_resp.json()
            status = str(status_payload.get("status") or "").lower()
            if status == "completed":
                return {"text": str(status_payload.get("text") or "")}
            if status == "error":
                raise RuntimeError(f"ASR 插件 assemblyai 转写失败: {status_payload.get('error')}")
            await asyncio.sleep(max(poll_interval, 0.2))

        raise RuntimeError("ASR 插件 assemblyai 轮询超时")

    async def transcribe_audio(self, model_id: str, data: bytes, filename: str, mime_type: str, extra_payload: dict[str, Any], config: dict[str, Any]) -> dict[str, Any]:
        api_key = self._resolve_secret(config, "api_key", "api_key_env")
        if not api_key:
            raise RuntimeError("ASR 插件 assemblyai 缺少 api_key 或 api_key_env")
        audio_url = await self._upload_audio(data, config, api_key)
        return await self._create_and_poll(model_id, audio_url, extra_payload, config, api_key)

    async def translate_audio(self, model_id: str, data: bytes, filename: str, mime_type: str, extra_payload: dict[str, Any], config: dict[str, Any]) -> dict[str, Any]:
        return await self.transcribe_audio(model_id, data, filename, mime_type, extra_payload, config)


class AzureSpeechASRPlugin(_BaseHTTPPlugin):
    plugin_id = "azure_speech"

    def _build_url(self, config: dict[str, Any]) -> str:
        endpoint = str(config.get("base_url") or "").strip()
        if endpoint:
            return endpoint
        region = str(config.get("region") or "").strip()
        if not region:
            raise RuntimeError("ASR 插件 azure_speech 缺少 region/base_url 配置")
        api_version = str(config.get("api_version") or "2024-11-15").strip()
        return f"https://{region}.api.cognitive.microsoft.com/speechtotext/transcriptions:transcribe?api-version={api_version}"

    async def _run(self, model_id: str, data: bytes, filename: str, mime_type: str, extra_payload: dict[str, Any], config: dict[str, Any]) -> dict[str, Any]:
        api_key = self._resolve_secret(config, "api_key", "api_key_env")
        bearer = self._resolve_secret(config, "bearer_token", "bearer_token_env")
        if not api_key and not bearer:
            raise RuntimeError("ASR 插件 azure_speech 缺少 api_key/api_key_env 或 bearer_token")

        locale = str(extra_payload.get("language") or model_id or config.get("default_locale") or "en-US")
        definition = {
            "locales": [locale],
            "profanityFilterMode": str(config.get("profanity_filter_mode") or "Masked"),
        }
        channels_value = config.get("channels")
        if channels_value is not None:
            definition["channels"] = [int(str(channels_value))]

        headers: dict[str, str] = {}
        if api_key:
            headers["Ocp-Apim-Subscription-Key"] = api_key
        if bearer:
            headers["Authorization"] = f"Bearer {bearer}"

        timeout = _as_float(config.get("timeout"), 90.0)
        files = {
            "audio": (filename or "audio.wav", data, mime_type or "application/octet-stream"),
            "definition": (None, json.dumps(definition), "application/json"),
        }
        session = await self._get_session(config)
        response = await session.post(self._build_url(config), files=files, headers=headers, timeout=timeout)
        if response.status_code >= 400:
            raise RuntimeError(f"ASR 插件 azure_speech 请求失败: {response.status_code} {response.text}")
        payload = response.json()
        return {"text": _extract_text(payload)}

    async def transcribe_audio(self, model_id: str, data: bytes, filename: str, mime_type: str, extra_payload: dict[str, Any], config: dict[str, Any]) -> dict[str, Any]:
        return await self._run(model_id, data, filename, mime_type, extra_payload, config)

    async def translate_audio(self, model_id: str, data: bytes, filename: str, mime_type: str, extra_payload: dict[str, Any], config: dict[str, Any]) -> dict[str, Any]:
        return await self._run(model_id, data, filename, mime_type, extra_payload, config)


class AliyunASRPlugin(_BaseHTTPPlugin):
    plugin_id = "aliyun"

    async def _run(self, model_id: str, data: bytes, mime_type: str, extra_payload: dict[str, Any], config: dict[str, Any]) -> dict[str, Any]:
        base_url = self._require_base_url(config, "https://nls-gateway-cn-shanghai.aliyuncs.com")
        endpoint = str(config.get("asr_endpoint") or "/stream/v1/asr").strip()
        token = self._resolve_secret(config, "token", "token_env")
        appkey = self._resolve_secret(config, "appkey", "appkey_env")
        if not token or not appkey:
            raise RuntimeError("ASR 插件 aliyun 需要 token/token_env 与 appkey/appkey_env")

        params = {
            "appkey": appkey,
            "format": str(config.get("format") or extra_payload.get("format") or "wav"),
            "sample_rate": int(config.get("sample_rate") or extra_payload.get("sample_rate") or 16000),
        }
        if model_id:
            params["model"] = model_id

        timeout = _as_float(config.get("timeout"), 60.0)
        url = urljoin(base_url.rstrip("/") + "/", endpoint.lstrip("/"))
        session = await self._get_session(config)
        response = await session.post(
            url,
            params=params,
            content=data,
            headers={
                "X-NLS-Token": token,
                "Content-Type": mime_type or "application/octet-stream",
            },
            timeout=timeout,
        )
        if response.status_code >= 400:
            raise RuntimeError(f"ASR 插件 aliyun 请求失败: {response.status_code} {response.text}")
        payload = response.json()
        return {"text": _extract_text(payload)}

    async def transcribe_audio(self, model_id: str, data: bytes, filename: str, mime_type: str, extra_payload: dict[str, Any], config: dict[str, Any]) -> dict[str, Any]:
        return await self._run(model_id, data, mime_type, extra_payload, config)

    async def translate_audio(self, model_id: str, data: bytes, filename: str, mime_type: str, extra_payload: dict[str, Any], config: dict[str, Any]) -> dict[str, Any]:
        return await self._run(model_id, data, mime_type, extra_payload, config)


def _tc3_headers(host: str, service: str, action: str, version: str, region: str, body: dict[str, Any], secret_id: str, secret_key: str) -> dict[str, str]:
    timestamp = int(time.time())
    date = datetime.fromtimestamp(timestamp, tz=timezone.utc).strftime("%Y-%m-%d")
    payload = json.dumps(body, ensure_ascii=False, separators=(",", ":"))

    canonical_headers = f"content-type:application/json; charset=utf-8\nhost:{host}\n"
    signed_headers = "content-type;host"
    hashed_payload = hashlib.sha256(payload.encode("utf-8")).hexdigest()
    canonical_request = "\n".join(["POST", "/", "", canonical_headers, signed_headers, hashed_payload])

    credential_scope = f"{date}/{service}/tc3_request"
    string_to_sign = "\n".join([
        "TC3-HMAC-SHA256",
        str(timestamp),
        credential_scope,
        hashlib.sha256(canonical_request.encode("utf-8")).hexdigest(),
    ])

    secret_date = hmac.new(("TC3" + secret_key).encode("utf-8"), date.encode("utf-8"), hashlib.sha256).digest()
    secret_service = hmac.new(secret_date, service.encode("utf-8"), hashlib.sha256).digest()
    secret_signing = hmac.new(secret_service, b"tc3_request", hashlib.sha256).digest()
    signature = hmac.new(secret_signing, string_to_sign.encode("utf-8"), hashlib.sha256).hexdigest()

    authorization = (
        "TC3-HMAC-SHA256 "
        f"Credential={secret_id}/{credential_scope}, "
        f"SignedHeaders={signed_headers}, Signature={signature}"
    )

    return {
        "Authorization": authorization,
        "Content-Type": "application/json; charset=utf-8",
        "Host": host,
        "X-TC-Action": action,
        "X-TC-Version": version,
        "X-TC-Region": region,
        "X-TC-Timestamp": str(timestamp),
    }


class TencentASRPlugin(_BaseHTTPPlugin):
    plugin_id = "tencent"

    async def _run(self, model_id: str, data: bytes, config: dict[str, Any]) -> dict[str, Any]:
        secret_id = self._resolve_secret(config, "secret_id", "secret_id_env")
        secret_key = self._resolve_secret(config, "secret_key", "secret_key_env")
        if not secret_id or not secret_key:
            raise RuntimeError("ASR 插件 tencent 缺少 secret_id/secret_key")

        host = str(config.get("host") or "asr.tencentcloudapi.com").strip()
        region = str(config.get("region") or "ap-guangzhou").strip()
        version = str(config.get("version") or "2019-06-14").strip()
        action = str(config.get("action") or "SentenceRecognition").strip()
        service = str(config.get("service") or "asr").strip()

        body = {
            "EngSerViceType": model_id or config.get("default_engine") or "16k_zh",
            "SourceType": 1,
            "VoiceFormat": str(config.get("voice_format") or "wav"),
            "Data": base64.b64encode(data).decode("utf-8"),
            "DataLen": len(data),
        }

        headers = _tc3_headers(host, service, action, version, region, body, secret_id, secret_key)
        timeout = _as_float(config.get("timeout"), 60.0)
        session = await self._get_session(config)
        response = await session.post(f"https://{host}", headers=headers, json=body, timeout=timeout)
        if response.status_code >= 400:
            raise RuntimeError(f"ASR 插件 tencent 请求失败: {response.status_code} {response.text}")

        payload = response.json()
        if isinstance(payload, dict) and payload.get("Response", {}).get("Error"):
            raise RuntimeError(f"ASR 插件 tencent 失败: {payload['Response']['Error']}")
        text = _extract_text(payload)
        return {"text": text}

    async def transcribe_audio(self, model_id: str, data: bytes, filename: str, mime_type: str, extra_payload: dict[str, Any], config: dict[str, Any]) -> dict[str, Any]:
        return await self._run(model_id, data, config)

    async def translate_audio(self, model_id: str, data: bytes, filename: str, mime_type: str, extra_payload: dict[str, Any], config: dict[str, Any]) -> dict[str, Any]:
        return await self._run(model_id, data, config)


class XunfeiASRPlugin(_BaseHTTPPlugin):
    plugin_id = "xunfei"

    def _build_ws_url(self, config: dict[str, Any]) -> str:
        ws_url = str(config.get("ws_url") or "wss://iat-api.xfyun.cn/v2/iat").strip()
        parsed = urlparse(ws_url)
        host = parsed.netloc
        path = parsed.path
        api_key = self._resolve_secret(config, "api_key", "api_key_env")
        api_secret = self._resolve_secret(config, "api_secret", "api_secret_env")
        if not api_key or not api_secret:
            raise RuntimeError("ASR 插件 xunfei 缺少 api_key/api_secret")

        now = format_datetime(datetime.now(timezone.utc), usegmt=True)
        sign_origin = f"host: {host}\ndate: {now}\nGET {path} HTTP/1.1"
        sign_sha = hmac.new(api_secret.encode("utf-8"), sign_origin.encode("utf-8"), hashlib.sha256).digest()
        signature = base64.b64encode(sign_sha).decode("utf-8")
        auth_origin = (
            f'api_key="{api_key}", algorithm="hmac-sha256", '
            f'headers="host date request-line", signature="{signature}"'
        )
        authorization = base64.b64encode(auth_origin.encode("utf-8")).decode("utf-8")
        query = urlencode({"authorization": authorization, "date": now, "host": host})
        return f"{ws_url}?{query}"

    async def _run(self, model_id: str, data: bytes, config: dict[str, Any]) -> dict[str, Any]:
        try:
            import websockets
        except ImportError as exc:
            raise RuntimeError("ASR 插件 xunfei 需要安装 websockets") from exc

        app_id = self._resolve_secret(config, "app_id", "app_id_env")
        if not app_id:
            raise RuntimeError("ASR 插件 xunfei 缺少 app_id")

        ws_url = self._build_ws_url(config)
        business: dict[str, Any] = {
            "language": str(config.get("language") or "zh_cn"),
            "domain": str(config.get("domain") or "iat"),
            "accent": str(config.get("accent") or "mandarin"),
        }
        if model_id:
            business["vad_eos"] = int(config.get("vad_eos") or 5000)

        frame = {
            "common": {"app_id": app_id},
            "business": business,
            "data": {
                "status": 2,
                "format": str(config.get("audio_format") or "audio/L16;rate=16000"),
                "encoding": str(config.get("encoding") or "raw"),
                "audio": base64.b64encode(data).decode("utf-8"),
            },
        }

        results: list[str] = []
        timeout = _as_float(config.get("timeout"), 30.0)
        async with websockets.connect(ws_url, close_timeout=timeout) as websocket:
            await websocket.send(json.dumps(frame, ensure_ascii=False))
            while True:
                message = await asyncio.wait_for(websocket.recv(), timeout=timeout)
                payload = json.loads(message)
                code = int(payload.get("code", 0))
                if code != 0:
                    raise RuntimeError(f"ASR 插件 xunfei 失败: {payload.get('message')}")
                words = payload.get("data", {}).get("result", {}).get("ws", [])
                for item in words:
                    for cw in item.get("cw", []):
                        text = str(cw.get("w") or "")
                        if text:
                            results.append(text)
                if int(payload.get("data", {}).get("status", 0)) == 2:
                    break

        return {"text": "".join(results).strip()}

    async def transcribe_audio(self, model_id: str, data: bytes, filename: str, mime_type: str, extra_payload: dict[str, Any], config: dict[str, Any]) -> dict[str, Any]:
        return await self._run(model_id, data, config)

    async def translate_audio(self, model_id: str, data: bytes, filename: str, mime_type: str, extra_payload: dict[str, Any], config: dict[str, Any]) -> dict[str, Any]:
        return await self._run(model_id, data, config)


class ASRPluginRegistry:
    """ASR 能力级插件注册表。"""

    def __init__(self, plugin_configs: Optional[Dict[str, Dict[str, Any]]] = None) -> None:
        self._plugins: Dict[str, ASRPlugin] = {}
        self._plugin_configs: Dict[str, Dict[str, Any]] = {}
        self._register_builtin_plugins()
        self._load_entrypoint_plugins()
        self.update_configs(plugin_configs or {})

    def _register_builtin_plugins(self) -> None:
        self.register(OpenAICompatibleASRPlugin())
        self.register(DeepgramASRPlugin())
        self.register(AssemblyAIASRPlugin())
        self.register(AzureSpeechASRPlugin())
        self.register(AliyunASRPlugin())
        self.register(TencentASRPlugin())
        self.register(XunfeiASRPlugin())

    def _load_entrypoint_plugins(self) -> None:
        try:
            eps = entry_points(group="llm_router.asr_plugins")
        except TypeError:
            eps = entry_points().get("llm_router.asr_plugins", [])
        except Exception as exc:  # pragma: no cover - runtime env specific
            logger.warning("加载 ASR 插件 entry points 失败: %s", exc)
            return

        for ep in eps:
            try:
                loaded = ep.load()
                plugin = loaded() if inspect.isclass(loaded) else loaded
                plugin_id = str(getattr(plugin, "plugin_id", ep.name)).strip()
                if not plugin_id:
                    logger.warning("跳过无 plugin_id 的 ASR 插件: %s", ep.name)
                    continue
                self._plugins[plugin_id] = plugin
            except Exception as exc:
                logger.warning("加载 ASR 插件失败 %s: %s", ep.name, exc)

    def register(self, plugin: ASRPlugin) -> None:
        plugin_id = str(getattr(plugin, "plugin_id", "")).strip()
        if not plugin_id:
            raise ValueError("ASR 插件必须定义非空 plugin_id")
        self._plugins[plugin_id] = plugin

    def get(self, plugin_id: str) -> Optional[ASRPlugin]:
        return self._plugins.get(plugin_id.strip())

    def update_configs(self, plugin_configs: Dict[str, Dict[str, Any]]) -> None:
        cleaned: Dict[str, Dict[str, Any]] = {}
        for plugin_id, config in (plugin_configs or {}).items():
            key = str(plugin_id).strip()
            if not key:
                continue
            cleaned[key] = dict(config or {})
        self._plugin_configs = cleaned

    def get_config(self, plugin_id: str) -> Dict[str, Any]:
        return dict(self._plugin_configs.get(plugin_id.strip(), {}))

    async def aclose(self) -> None:
        for plugin in self._plugins.values():
            close_fn = getattr(plugin, "aclose", None)
            if close_fn is None:
                continue
            try:
                result = close_fn()
                if inspect.isawaitable(result):
                    await result
            except Exception:
                pass
