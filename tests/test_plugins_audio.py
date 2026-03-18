from __future__ import annotations

import base64
import json
import types

import pytest

from llm_router.plugins.asr import (
    ASRPluginRegistry,
    AliyunASRPlugin,
    AssemblyAIASRPlugin,
    AzureSpeechASRPlugin,
    DeepgramASRPlugin,
    TencentASRPlugin,
    XunfeiASRPlugin,
)
from llm_router.plugins.tts import (
    AliyunTTSPlugin,
    AzureSpeechTTSPlugin,
    ElevenLabsTTSPlugin,
    GoogleCloudTTSPlugin,
    TencentTTSPlugin,
    TTSPluginRegistry,
    XunfeiTTSPlugin,
)


class FakeResponse:
    def __init__(
        self,
        status_code: int = 200,
        *,
        json_data=None,
        text: str = "",
        headers: dict[str, str] | None = None,
        content: bytes = b"",
    ) -> None:
        self.status_code = status_code
        self._json_data = json_data
        self.text = text
        self.headers = headers or {}
        self.content = content

    def json(self):
        return self._json_data


class FakeSession:
    def __init__(self, responses: list[FakeResponse]) -> None:
        self._responses = responses
        self.calls: list[tuple[str, str, dict]] = []

    async def post(self, url: str, **kwargs):
        self.calls.append(("post", url, kwargs))
        if not self._responses:
            raise AssertionError("no fake response left")
        return self._responses.pop(0)

    async def get(self, url: str, **kwargs):
        self.calls.append(("get", url, kwargs))
        if not self._responses:
            raise AssertionError("no fake response left")
        return self._responses.pop(0)


class _AsyncCM:
    def __init__(self, ws):
        self._ws = ws

    async def __aenter__(self):
        return self._ws

    async def __aexit__(self, exc_type, exc, tb):
        return False


@pytest.mark.asyncio
async def test_audio_plugin_registries_include_builtin_providers() -> None:
    asr_ids = set(ASRPluginRegistry()._plugins.keys())
    tts_ids = set(TTSPluginRegistry()._plugins.keys())

    assert {"openai_compatible", "deepgram", "assemblyai", "azure_speech", "aliyun", "tencent", "xunfei"}.issubset(asr_ids)
    assert {"openai_compatible", "elevenlabs", "azure_speech", "google_cloud", "aliyun", "tencent", "xunfei"}.issubset(tts_ids)


@pytest.mark.asyncio
async def test_deepgram_asr_transcribe_uses_model_query_and_extracts_text(monkeypatch) -> None:
    plugin = DeepgramASRPlugin()
    fake_session = FakeSession(
        [
            FakeResponse(
                json_data={
                    "results": {
                        "channels": [
                            {
                                "alternatives": [
                                    {"transcript": "hello world"},
                                ]
                            }
                        ]
                    }
                }
            )
        ]
    )

    async def _fake_get_session(_config):
        return fake_session

    monkeypatch.setattr(plugin, "_get_session", _fake_get_session)

    result = await plugin.transcribe_audio(
        "nova-3",
        b"abc",
        "a.wav",
        "audio/wav",
        {"language": "en"},
        {"api_key": "k", "base_url": "https://api.deepgram.com"},
    )

    assert result["text"] == "hello world"
    _, _, kwargs = fake_session.calls[0]
    assert kwargs["params"]["model"] == "nova-3"
    assert kwargs["headers"]["Authorization"].startswith("Token ")


@pytest.mark.asyncio
async def test_assemblyai_asr_upload_then_poll(monkeypatch) -> None:
    plugin = AssemblyAIASRPlugin()
    fake_session = FakeSession(
        [
            FakeResponse(json_data={"upload_url": "https://files.example.com/a.wav"}),
            FakeResponse(json_data={"id": "task-1"}),
            FakeResponse(json_data={"status": "processing"}),
            FakeResponse(json_data={"status": "completed", "text": "final transcript"}),
        ]
    )

    async def _fake_get_session(_config):
        return fake_session

    monkeypatch.setattr(plugin, "_get_session", _fake_get_session)

    result = await plugin.transcribe_audio(
        "universal",
        b"abc",
        "a.wav",
        "audio/wav",
        {},
        {
            "api_key": "k",
            "base_url": "https://api.assemblyai.com",
            "poll_interval": 0,
            "poll_attempts": 5,
        },
    )

    assert result["text"] == "final transcript"
    assert [m for m, _, _ in fake_session.calls] == ["post", "post", "get", "get"]


@pytest.mark.asyncio
async def test_elevenlabs_tts_uses_voice_id_path(monkeypatch) -> None:
    plugin = ElevenLabsTTSPlugin()
    fake_session = FakeSession(
        [
            FakeResponse(content=b"AUDIO", headers={"content-type": "audio/mpeg"}),
        ]
    )

    async def _fake_get_session(_config):
        return fake_session

    monkeypatch.setattr(plugin, "_get_session", _fake_get_session)

    audio, media_type = await plugin.synthesize_speech(
        "voice-123",
        {"input": "hello"},
        {"api_key": "k", "base_url": "https://api.elevenlabs.io"},
    )

    assert audio == b"AUDIO"
    assert media_type == "audio/mpeg"
    assert fake_session.calls[0][1].endswith("/v1/text-to-speech/voice-123")


@pytest.mark.asyncio
async def test_google_tts_decodes_audio_content(monkeypatch) -> None:
    plugin = GoogleCloudTTSPlugin()
    b64_audio = base64.b64encode(b"xyz").decode("ascii")
    fake_session = FakeSession([FakeResponse(json_data={"audioContent": b64_audio})])

    async def _fake_get_session(_config):
        return fake_session

    monkeypatch.setattr(plugin, "_get_session", _fake_get_session)

    audio, media_type = await plugin.synthesize_speech(
        "en-US-Standard-A",
        {"input": "hello"},
        {"api_key": "token", "base_url": "https://texttospeech.googleapis.com"},
    )

    assert audio == b"xyz"
    assert media_type == "audio/mpeg"


@pytest.mark.asyncio
async def test_azure_tts_sends_ssml_and_output_format(monkeypatch) -> None:
    plugin = AzureSpeechTTSPlugin()
    fake_session = FakeSession([FakeResponse(content=b"AUDIO", headers={"content-type": "audio/mpeg"})])

    async def _fake_get_session(_config):
        return fake_session

    monkeypatch.setattr(plugin, "_get_session", _fake_get_session)

    audio, media_type = await plugin.synthesize_speech(
        "zh-CN-YunxiNeural",
        {"input": "你好", "response_format": "audio-16khz-64kbitrate-mono-mp3", "language": "zh-CN"},
        {"api_key": "k", "region": "eastasia"},
    )
    assert audio == b"AUDIO"
    assert media_type == "audio/mpeg"
    _, url, kwargs = fake_session.calls[0]
    assert url.startswith("https://eastasia.tts.speech.microsoft.com/")
    assert kwargs["headers"]["X-Microsoft-OutputFormat"] == "audio-16khz-64kbitrate-mono-mp3"
    assert b"<voice name='zh-CN-YunxiNeural'>" in kwargs["content"]


@pytest.mark.asyncio
async def test_aliyun_asr_sets_appkey_token_and_query(monkeypatch) -> None:
    plugin = AliyunASRPlugin()
    fake_session = FakeSession([FakeResponse(json_data={"result": "ok", "text": "hello"})])

    async def _fake_get_session(_config):
        return fake_session

    monkeypatch.setattr(plugin, "_get_session", _fake_get_session)

    result = await plugin.transcribe_audio(
        "model-1",
        b"abc",
        "a.wav",
        "audio/wav",
        {"format": "wav", "sample_rate": 16000},
        {"token": "t", "appkey": "a", "base_url": "https://nls.example.com"},
    )
    assert result["text"]
    _, _, kwargs = fake_session.calls[0]
    assert kwargs["headers"]["X-NLS-Token"] == "t"
    assert kwargs["params"]["appkey"] == "a"
    assert kwargs["params"]["model"] == "model-1"


@pytest.mark.asyncio
async def test_aliyun_tts_posts_json_and_returns_audio(monkeypatch) -> None:
    plugin = AliyunTTSPlugin()
    fake_session = FakeSession([FakeResponse(content=b"AUDIO", headers={"content-type": "audio/mpeg"})])

    async def _fake_get_session(_config):
        return fake_session

    monkeypatch.setattr(plugin, "_get_session", _fake_get_session)

    audio, media_type = await plugin.synthesize_speech(
        "xiaoyun",
        {"input": "hello", "response_format": "mp3"},
        {"token": "t", "appkey": "a", "base_url": "https://nls.example.com"},
    )
    assert audio == b"AUDIO"
    assert media_type == "audio/mpeg"
    _, _, kwargs = fake_session.calls[0]
    assert kwargs["json"]["voice"] == "xiaoyun"
    assert kwargs["json"]["token"] == "t"
    assert kwargs["json"]["appkey"] == "a"


@pytest.mark.asyncio
async def test_tencent_asr_signs_request_and_base64_encodes_audio(monkeypatch) -> None:
    plugin = TencentASRPlugin()
    fake_session = FakeSession([FakeResponse(json_data={"Response": {"Result": "hi"}})])

    async def _fake_get_session(_config):
        return fake_session

    monkeypatch.setattr(plugin, "_get_session", _fake_get_session)

    result = await plugin.transcribe_audio(
        "16k_zh",
        b"abc",
        "a.wav",
        "audio/wav",
        {},
        {"secret_id": "sid", "secret_key": "skey", "region": "ap-guangzhou"},
    )
    assert "text" in result
    _, url, kwargs = fake_session.calls[0]
    assert url.startswith("https://asr.tencentcloudapi.com")
    assert kwargs["headers"]["Authorization"].startswith("TC3-HMAC-SHA256 ")
    assert kwargs["headers"]["X-TC-Action"] == "SentenceRecognition"
    assert kwargs["json"]["Data"]  # base64


@pytest.mark.asyncio
async def test_tencent_tts_signs_request_and_decodes_audio(monkeypatch) -> None:
    plugin = TencentTTSPlugin()
    audio_b64 = base64.b64encode(b"wav").decode("ascii")
    fake_session = FakeSession([FakeResponse(json_data={"Response": {"Audio": audio_b64}})])

    async def _fake_get_session(_config):
        return fake_session

    monkeypatch.setattr(plugin, "_get_session", _fake_get_session)

    audio, media_type = await plugin.synthesize_speech(
        "101001",
        {"input": "hello", "response_format": "mp3"},
        {"secret_id": "sid", "secret_key": "skey", "region": "ap-guangzhou"},
    )
    assert audio == b"wav"
    assert media_type == "audio/mpeg"
    _, url, kwargs = fake_session.calls[0]
    assert url.startswith("https://tts.tencentcloudapi.com")
    assert kwargs["headers"]["X-TC-Action"] == "TextToVoice"


@pytest.mark.asyncio
async def test_xunfei_asr_requires_websockets(monkeypatch) -> None:
    plugin = XunfeiASRPlugin()
    # simulate no websockets installed by temporarily removing module if present
    monkeypatch.setitem(__import__("sys").modules, "websockets", None)
    with pytest.raises(RuntimeError):
        await plugin.transcribe_audio("iat", b"a", "a.wav", "audio/wav", {}, {"app_id": "x", "api_key": "k", "api_secret": "s"})


@pytest.mark.asyncio
async def test_xunfei_tts_websocket_flow(monkeypatch) -> None:
    plugin = XunfeiTTSPlugin()

    class FakeWS:
        def __init__(self):
            self.sent = []
            self._messages = [
                json.dumps({"code": 0, "data": {"audio": base64.b64encode(b"ab").decode("ascii"), "status": 1}}),
                json.dumps({"code": 0, "data": {"audio": base64.b64encode(b"cd").decode("ascii"), "status": 2}}),
            ]

        async def send(self, msg: str):
            self.sent.append(msg)

        async def recv(self):
            return self._messages.pop(0)

    fake_ws = FakeWS()
    fake_module = types.SimpleNamespace(connect=lambda *a, **k: _AsyncCM(fake_ws))
    monkeypatch.setitem(__import__("sys").modules, "websockets", fake_module)

    audio, media_type = await plugin.synthesize_speech(
        "xiaoyan",
        {"input": "hello"},
        {"app_id": "appid", "api_key": "k", "api_secret": "s"},
    )
    assert audio == b"abcd"
    assert media_type == "audio/mpeg"
