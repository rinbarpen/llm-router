from __future__ import annotations

from pathlib import Path

import pytest
import respx
from httpx import Response

from llm_router.config import RouterSettings
from llm_router.db.models import Model, Provider, ProviderType
from llm_router.providers.anthropic import AnthropicProviderClient
from llm_router.providers.gemini import GeminiProviderClient
from llm_router.providers.openai_compatible import OpenAICompatibleProviderClient
from llm_router.schemas import ModelInvokeRequest


def _settings(tmp_path: Path) -> RouterSettings:
    return RouterSettings(
        database_url=f"sqlite+aiosqlite:///{tmp_path / 'provider.db'}",
        model_store_dir=tmp_path / "models",
    )


def _provider(provider_type: ProviderType, **kwargs) -> Provider:
    provider = Provider(
        name=provider_type.value,
        type=provider_type,
        is_active=True,
        base_url=kwargs.get("base_url"),
        api_key=kwargs.get("api_key"),
        settings=kwargs.get("settings") or {},
    )
    provider.id = 1
    return provider


def _model(provider: Provider, **kwargs) -> Model:
    model = Model(
        provider_id=provider.id or 1,
        name=kwargs.get("name", "test-model"),
        default_params=kwargs.get("default_params", {}),
        config=kwargs.get("config", {}),
        is_active=True,
    )
    model.provider = provider
    model.remote_identifier = kwargs.get("remote_identifier")
    return model


@pytest.mark.asyncio
@respx.mock
async def test_openai_compatible_invocation(tmp_path: Path) -> None:
    route = respx.post("https://api.openai.com/v1/chat/completions").mock(
        return_value=Response(
            200,
            json={
                "choices": [
                    {
                        "message": {"content": "hi there"},
                    }
                ]
            },
        )
    )

    provider = _provider(ProviderType.OPENAI, api_key="sk-test")
    model = _model(provider, remote_identifier="gpt-4o-mini")
    client = OpenAICompatibleProviderClient(provider, _settings(tmp_path))

    response = await client.invoke(model, ModelInvokeRequest(prompt="hello"))

    assert route.called
    assert response.output_text == "hi there"


@pytest.mark.asyncio
@respx.mock
async def test_gemini_invocation(tmp_path: Path) -> None:
    route = respx.post(
        "https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:generateContent?key=gm-key"
    ).mock(
        return_value=Response(
            200,
            json={
                "candidates": [
                    {"content": {"parts": [{"text": "gemini-response"}]}}
                ]
            },
        )
    )

    provider = _provider(ProviderType.GEMINI, api_key="gm-key")
    model = _model(provider, remote_identifier="gemini-pro")
    client = GeminiProviderClient(provider, _settings(tmp_path))

    await client.invoke(model, ModelInvokeRequest(prompt="hi"))
    assert route.called


@pytest.mark.asyncio
@respx.mock
async def test_anthropic_invocation(tmp_path: Path) -> None:
    route = respx.post("https://api.anthropic.com/v1/messages").mock(
        return_value=Response(
            200,
            json={
                "content": [
                    {"type": "text", "text": "claude says hi"},
                ]
            },
        )
    )

    provider = _provider(ProviderType.CLAUDE, api_key="anthropic-key")
    model = _model(provider, remote_identifier="claude-3")
    client = AnthropicProviderClient(provider, _settings(tmp_path))

    response = await client.invoke(model, ModelInvokeRequest(prompt="ping"))

    assert route.called
    assert response.output_text == "claude says hi"


