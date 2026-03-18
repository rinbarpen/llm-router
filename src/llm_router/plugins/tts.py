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


class TTSPlugin(Protocol):
    plugin_id: str

    async def synthesize_speech(
        self,
        model_id: str,
        payload: dict[str, Any],
        config: dict[str, Any],
    ) -> tuple[bytes, str]:
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


def _as_float(value: Any, fallback: float) -> float:
    try:
        return float(value)
    except (TypeError, ValueError):
        return fallback


class OpenAICompatibleTTSPlugin(_BaseHTTPPlugin):
    """把 TTS 请求转发到 OpenAI 兼容接口。"""

    plugin_id = "openai_compatible"

    async def synthesize_speech(
        self,
        model_id: str,
        payload: dict[str, Any],
        config: dict[str, Any],
    ) -> tuple[bytes, str]:
        base_url = str(config.get("base_url") or "").strip()
        if not base_url:
            raise RuntimeError("TTS 插件 openai_compatible 缺少 base_url 配置")

        endpoint = str(config.get("audio_speech_endpoint") or "/v1/audio/speech").strip()
        timeout = _as_float(config.get("timeout"), 60.0)

        request_body = dict(payload)
        request_body["model"] = model_id

        api_key = self._resolve_secret(config, "api_key", "api_key_env")
        headers = {"Content-Type": "application/json"}
        if api_key:
            headers["Authorization"] = f"Bearer {api_key}"

        url = urljoin(base_url.rstrip("/") + "/", endpoint.lstrip("/"))
        session = await self._get_session(config)
        response = await session.post(url, json=request_body, headers=headers, timeout=timeout)
        if response.status_code >= 400:
            raise RuntimeError(
                f"TTS 插件 openai_compatible 请求失败: {response.status_code} {response.text}"
            )

        content_type = response.headers.get("content-type", "audio/mpeg")
        media_type = content_type.split(";", 1)[0].strip() or "audio/mpeg"
        return response.content, media_type


class ElevenLabsTTSPlugin(_BaseHTTPPlugin):
    plugin_id = "elevenlabs"

    async def synthesize_speech(self, model_id: str, payload: dict[str, Any], config: dict[str, Any]) -> tuple[bytes, str]:
        base_url = str(config.get("base_url") or "https://api.elevenlabs.io").strip()
        endpoint_template = str(config.get("tts_endpoint_template") or "/v1/text-to-speech/{voice_id}").strip()
        api_key = self._resolve_secret(config, "api_key", "api_key_env")
        if not api_key:
            raise RuntimeError("TTS 插件 elevenlabs 缺少 api_key/api_key_env")

        voice_id = model_id or str(payload.get("voice") or "").strip()
        if not voice_id:
            raise RuntimeError("TTS 插件 elevenlabs 需要 model_id 作为 voice_id")

        body = {
            "text": payload.get("input") or payload.get("text") or "",
            "model_id": payload.get("model_id") or config.get("model_id"),
            "voice_settings": payload.get("voice_settings") or config.get("voice_settings"),
        }
        body = {k: v for k, v in body.items() if v is not None}

        timeout = _as_float(config.get("timeout"), 60.0)
        endpoint = endpoint_template.format(voice_id=voice_id)
        url = urljoin(base_url.rstrip("/") + "/", endpoint.lstrip("/"))
        session = await self._get_session(config)
        response = await session.post(
            url,
            json=body,
            headers={
                "xi-api-key": api_key,
                "Content-Type": "application/json",
                "Accept": "audio/mpeg",
            },
            timeout=timeout,
        )
        if response.status_code >= 400:
            raise RuntimeError(f"TTS 插件 elevenlabs 请求失败: {response.status_code} {response.text}")
        media = response.headers.get("content-type", "audio/mpeg").split(";", 1)[0].strip() or "audio/mpeg"
        return response.content, media


class AzureSpeechTTSPlugin(_BaseHTTPPlugin):
    plugin_id = "azure_speech"

    def _build_url(self, config: dict[str, Any]) -> str:
        endpoint = str(config.get("base_url") or "").strip()
        if endpoint:
            return endpoint
        region = str(config.get("region") or "").strip()
        if not region:
            raise RuntimeError("TTS 插件 azure_speech 缺少 region/base_url 配置")
        return f"https://{region}.tts.speech.microsoft.com/cognitiveservices/v1"

    async def synthesize_speech(self, model_id: str, payload: dict[str, Any], config: dict[str, Any]) -> tuple[bytes, str]:
        api_key = self._resolve_secret(config, "api_key", "api_key_env")
        bearer = self._resolve_secret(config, "bearer_token", "bearer_token_env")
        if not api_key and not bearer:
            raise RuntimeError("TTS 插件 azure_speech 缺少 api_key/api_key_env 或 bearer_token")

        text = str(payload.get("input") or payload.get("text") or "").strip()
        if not text:
            raise RuntimeError("TTS 插件 azure_speech 缺少 input 文本")

        voice_name = model_id or str(payload.get("voice") or config.get("voice") or "").strip()
        if not voice_name:
            raise RuntimeError("TTS 插件 azure_speech 需要 model_id 作为 voice")

        lang = str(payload.get("language") or config.get("language") or "en-US").strip()
        ssml = (
            "<speak version='1.0' xml:lang='{lang}'>"
            "<voice name='{voice}'>{text}</voice>"
            "</speak>"
        ).format(lang=lang, voice=voice_name, text=text)

        headers = {
            "Content-Type": "application/ssml+xml",
            "X-Microsoft-OutputFormat": str(
                payload.get("response_format") or config.get("output_format") or "audio-16khz-64kbitrate-mono-mp3"
            ),
            "User-Agent": "llm-router",
        }
        if api_key:
            headers["Ocp-Apim-Subscription-Key"] = api_key
        if bearer:
            headers["Authorization"] = f"Bearer {bearer}"

        timeout = _as_float(config.get("timeout"), 60.0)
        session = await self._get_session(config)
        response = await session.post(self._build_url(config), content=ssml.encode("utf-8"), headers=headers, timeout=timeout)
        if response.status_code >= 400:
            raise RuntimeError(f"TTS 插件 azure_speech 请求失败: {response.status_code} {response.text}")
        media = response.headers.get("content-type", "audio/mpeg").split(";", 1)[0].strip() or "audio/mpeg"
        return response.content, media


class GoogleCloudTTSPlugin(_BaseHTTPPlugin):
    plugin_id = "google_cloud"

    async def synthesize_speech(self, model_id: str, payload: dict[str, Any], config: dict[str, Any]) -> tuple[bytes, str]:
        access_token = self._resolve_secret(config, "access_token", "access_token_env")
        if not access_token:
            access_token = self._resolve_secret(config, "api_key", "api_key_env")
        if not access_token:
            raise RuntimeError("TTS 插件 google_cloud 需要 access_token/api_key")

        base_url = str(config.get("base_url") or "https://texttospeech.googleapis.com").strip()
        endpoint = str(config.get("synthesize_endpoint") or "/v1/text:synthesize").strip()
        url = urljoin(base_url.rstrip("/") + "/", endpoint.lstrip("/"))

        text = str(payload.get("input") or payload.get("text") or "").strip()
        if not text:
            raise RuntimeError("TTS 插件 google_cloud 缺少 input 文本")

        language_code = str(payload.get("language") or config.get("language_code") or "en-US")
        response_format = str(payload.get("response_format") or config.get("audio_encoding") or "MP3")
        body = {
            "input": {"text": text},
            "voice": {
                "languageCode": language_code,
                "name": model_id,
            },
            "audioConfig": {
                "audioEncoding": response_format,
                "speakingRate": payload.get("speed") if payload.get("speed") is not None else config.get("speaking_rate"),
            },
        }
        if body["audioConfig"].get("speakingRate") is None:
            body["audioConfig"].pop("speakingRate")

        timeout = _as_float(config.get("timeout"), 60.0)
        session = await self._get_session(config)
        response = await session.post(
            url,
            json=body,
            headers={"Authorization": f"Bearer {access_token}", "Content-Type": "application/json"},
            timeout=timeout,
        )
        if response.status_code >= 400:
            raise RuntimeError(f"TTS 插件 google_cloud 请求失败: {response.status_code} {response.text}")
        payload_data = response.json()
        audio = str(payload_data.get("audioContent") or "").strip()
        if not audio:
            raise RuntimeError("TTS 插件 google_cloud 未返回 audioContent")
        return base64.b64decode(audio), "audio/mpeg"


class AliyunTTSPlugin(_BaseHTTPPlugin):
    plugin_id = "aliyun"

    async def synthesize_speech(self, model_id: str, payload: dict[str, Any], config: dict[str, Any]) -> tuple[bytes, str]:
        token = self._resolve_secret(config, "token", "token_env")
        appkey = self._resolve_secret(config, "appkey", "appkey_env")
        if not token or not appkey:
            raise RuntimeError("TTS 插件 aliyun 需要 token/token_env 与 appkey/appkey_env")

        text = str(payload.get("input") or payload.get("text") or "").strip()
        if not text:
            raise RuntimeError("TTS 插件 aliyun 缺少 input 文本")

        base_url = str(config.get("base_url") or "https://nls-gateway-cn-shanghai.aliyuncs.com").strip()
        endpoint = str(config.get("tts_endpoint") or "/stream/v1/tts").strip()
        body = {
            "appkey": appkey,
            "token": token,
            "text": text,
            "format": payload.get("response_format") or config.get("format") or "mp3",
            "sample_rate": int(config.get("sample_rate") or 16000),
            "voice": model_id,
        }

        timeout = _as_float(config.get("timeout"), 60.0)
        url = urljoin(base_url.rstrip("/") + "/", endpoint.lstrip("/"))
        session = await self._get_session(config)
        response = await session.post(url, json=body, headers={"Content-Type": "application/json"}, timeout=timeout)
        if response.status_code >= 400:
            raise RuntimeError(f"TTS 插件 aliyun 请求失败: {response.status_code} {response.text}")
        media = response.headers.get("content-type", "audio/mpeg").split(";", 1)[0].strip() or "audio/mpeg"
        return response.content, media


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


class TencentTTSPlugin(_BaseHTTPPlugin):
    plugin_id = "tencent"

    async def synthesize_speech(self, model_id: str, payload: dict[str, Any], config: dict[str, Any]) -> tuple[bytes, str]:
        secret_id = self._resolve_secret(config, "secret_id", "secret_id_env")
        secret_key = self._resolve_secret(config, "secret_key", "secret_key_env")
        if not secret_id or not secret_key:
            raise RuntimeError("TTS 插件 tencent 缺少 secret_id/secret_key")

        text = str(payload.get("input") or payload.get("text") or "").strip()
        if not text:
            raise RuntimeError("TTS 插件 tencent 缺少 input 文本")

        host = str(config.get("host") or "tts.tencentcloudapi.com").strip()
        region = str(config.get("region") or "ap-guangzhou").strip()
        version = str(config.get("version") or "2019-08-23").strip()
        action = str(config.get("action") or "TextToVoice").strip()
        service = str(config.get("service") or "tts").strip()

        body = {
            "Text": text,
            "SessionId": str(payload.get("session_id") or f"llm-router-{int(time.time())}"),
            "ModelType": 1,
            "VoiceType": int(model_id or config.get("voice_type") or 101001),
            "Codec": str(payload.get("response_format") or config.get("codec") or "mp3"),
            "Speed": int(payload.get("speed") or config.get("speed") or 0),
            "Volume": int(payload.get("volume") or config.get("volume") or 0),
        }

        headers = _tc3_headers(host, service, action, version, region, body, secret_id, secret_key)
        timeout = _as_float(config.get("timeout"), 60.0)
        session = await self._get_session(config)
        response = await session.post(f"https://{host}", headers=headers, json=body, timeout=timeout)
        if response.status_code >= 400:
            raise RuntimeError(f"TTS 插件 tencent 请求失败: {response.status_code} {response.text}")

        payload_data = response.json()
        if isinstance(payload_data, dict) and payload_data.get("Response", {}).get("Error"):
            raise RuntimeError(f"TTS 插件 tencent 失败: {payload_data['Response']['Error']}")
        audio = str(payload_data.get("Response", {}).get("Audio") or "").strip()
        if not audio:
            raise RuntimeError("TTS 插件 tencent 未返回 Audio")
        return base64.b64decode(audio), "audio/mpeg"


class XunfeiTTSPlugin(_BaseHTTPPlugin):
    plugin_id = "xunfei"

    def _build_ws_url(self, config: dict[str, Any]) -> str:
        ws_url = str(config.get("ws_url") or "wss://tts-api.xfyun.cn/v2/tts").strip()
        parsed = urlparse(ws_url)
        host = parsed.netloc
        path = parsed.path
        api_key = self._resolve_secret(config, "api_key", "api_key_env")
        api_secret = self._resolve_secret(config, "api_secret", "api_secret_env")
        if not api_key or not api_secret:
            raise RuntimeError("TTS 插件 xunfei 缺少 api_key/api_secret")

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

    async def synthesize_speech(self, model_id: str, payload: dict[str, Any], config: dict[str, Any]) -> tuple[bytes, str]:
        try:
            import websockets
        except ImportError as exc:
            raise RuntimeError("TTS 插件 xunfei 需要安装 websockets") from exc

        app_id = self._resolve_secret(config, "app_id", "app_id_env")
        if not app_id:
            raise RuntimeError("TTS 插件 xunfei 缺少 app_id")

        text = str(payload.get("input") or payload.get("text") or "").strip()
        if not text:
            raise RuntimeError("TTS 插件 xunfei 缺少 input 文本")

        frame = {
            "common": {"app_id": app_id},
            "business": {
                "aue": str(config.get("aue") or "lame"),
                "auf": str(config.get("auf") or "audio/L16;rate=16000"),
                "vcn": model_id or str(config.get("voice") or "xiaoyan"),
                "tte": str(config.get("tte") or "utf8"),
            },
            "data": {
                "status": 2,
                "text": base64.b64encode(text.encode("utf-8")).decode("utf-8"),
            },
        }

        audio_chunks: list[bytes] = []
        timeout = _as_float(config.get("timeout"), 30.0)
        async with websockets.connect(self._build_ws_url(config), close_timeout=timeout) as websocket:
            await websocket.send(json.dumps(frame, ensure_ascii=False))
            while True:
                message = await asyncio.wait_for(websocket.recv(), timeout=timeout)
                payload_data = json.loads(message)
                code = int(payload_data.get("code", 0))
                if code != 0:
                    raise RuntimeError(f"TTS 插件 xunfei 失败: {payload_data.get('message')}")

                audio_base64 = str(payload_data.get("data", {}).get("audio") or "")
                if audio_base64:
                    audio_chunks.append(base64.b64decode(audio_base64))
                if int(payload_data.get("data", {}).get("status", 0)) == 2:
                    break

        if not audio_chunks:
            raise RuntimeError("TTS 插件 xunfei 未返回音频数据")
        return b"".join(audio_chunks), "audio/mpeg"


class TTSPluginRegistry:
    """TTS 能力级插件注册表。"""

    def __init__(self, plugin_configs: Optional[Dict[str, Dict[str, Any]]] = None) -> None:
        self._plugins: Dict[str, TTSPlugin] = {}
        self._plugin_configs: Dict[str, Dict[str, Any]] = {}
        self._register_builtin_plugins()
        self._load_entrypoint_plugins()
        self.update_configs(plugin_configs or {})

    def _register_builtin_plugins(self) -> None:
        self.register(OpenAICompatibleTTSPlugin())
        self.register(ElevenLabsTTSPlugin())
        self.register(AzureSpeechTTSPlugin())
        self.register(GoogleCloudTTSPlugin())
        self.register(AliyunTTSPlugin())
        self.register(TencentTTSPlugin())
        self.register(XunfeiTTSPlugin())

    def _load_entrypoint_plugins(self) -> None:
        try:
            eps = entry_points(group="llm_router.tts_plugins")
        except TypeError:
            eps = entry_points().get("llm_router.tts_plugins", [])
        except Exception as exc:  # pragma: no cover - runtime env specific
            logger.warning("加载 TTS 插件 entry points 失败: %s", exc)
            return

        for ep in eps:
            try:
                loaded = ep.load()
                plugin = loaded() if inspect.isclass(loaded) else loaded
                plugin_id = str(getattr(plugin, "plugin_id", ep.name)).strip()
                if not plugin_id:
                    logger.warning("跳过无 plugin_id 的 TTS 插件: %s", ep.name)
                    continue
                self._plugins[plugin_id] = plugin
            except Exception as exc:
                logger.warning("加载 TTS 插件失败 %s: %s", ep.name, exc)

    def register(self, plugin: TTSPlugin) -> None:
        plugin_id = str(getattr(plugin, "plugin_id", "")).strip()
        if not plugin_id:
            raise ValueError("TTS 插件必须定义非空 plugin_id")
        self._plugins[plugin_id] = plugin

    def get(self, plugin_id: str) -> Optional[TTSPlugin]:
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
