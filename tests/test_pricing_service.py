from __future__ import annotations

import json
from pathlib import Path

import pytest

from llm_router.services.pricing_service import PricingService


@pytest.mark.parametrize(
    ("raw", "normalized"),
    [
        ("openai", "openai"),
        ("claude", "claude"),
        ("qwen (cn)", "qwen"),
        ("deepseek (cn)", "deepseek"),
        ("kimi (cn)", "kimi"),
        ("glm (cn)", "glm"),
        ("grok", "groq"),
        ("groq", "groq"),
    ],
)
def test_normalize_provider_name(raw: str, normalized: str) -> None:
    assert PricingService.normalize_provider_name(raw) == normalized


@pytest.mark.asyncio
async def test_get_latest_pricing_supports_provider_alias_and_model_suffix() -> None:
    service = PricingService()

    pricing = await service.get_latest_pricing("qwen-plus", "qwen (cn)")
    assert pricing is not None
    assert pricing.provider == "qwen"
    assert pricing.model_name == "qwen-plus"

    pricing_from_remote_identifier = await service.get_latest_pricing(
        "dashscope/qwen-plus", "qwen (cn)"
    )
    assert pricing_from_remote_identifier is not None
    assert pricing_from_remote_identifier.model_name == "qwen-plus"


@pytest.mark.asyncio
async def test_free_models_are_included_and_marked() -> None:
    service = PricingService()

    pricing = await service.get_latest_pricing("glm-4-flash", "glm (cn)")
    assert pricing is not None
    assert pricing.input_price_per_1k == 0.0
    assert pricing.output_price_per_1k == 0.0
    assert pricing.notes is not None
    assert "免费模型" in pricing.notes


@pytest.mark.asyncio
async def test_get_all_latest_pricing_contains_expected_providers() -> None:
    service = PricingService()

    async def _fake_openrouter() -> list:
        return []

    service.source_fetchers["openrouter"] = _fake_openrouter

    all_pricing = await service.get_all_latest_pricing()
    assert "openai" in all_pricing
    assert "claude" in all_pricing
    assert "gemini" in all_pricing
    assert "deepseek" in all_pricing
    assert "qwen" in all_pricing
    assert "kimi" in all_pricing
    assert "glm" in all_pricing
    assert "groq" in all_pricing


@pytest.mark.asyncio
async def test_cache_key_uses_normalized_provider() -> None:
    service = PricingService()

    first = await service.get_latest_pricing("deepseek-chat", "deepseek (cn)")
    second = await service.get_latest_pricing("deepseek-chat", "deepseek")

    assert first is not None
    assert second is not None
    assert first.model_dump() == second.model_dump()


@pytest.mark.asyncio
async def test_remote_source_override_static_catalog(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv(
        "LLM_ROUTER_PRICING_SOURCE_URLS",
        json.dumps({"openai": "https://example.com/openai-pricing.json"}),
    )
    service = PricingService()

    async def _fake_remote(provider: str):
        if provider == "openai":
            return [
                service._parse_remote_pricing_payload(
                    provider="openai",
                    payload=[
                        {
                            "model_name": "gpt-4o",
                            "input_price_per_1k": 9.9,
                            "output_price_per_1k": 19.9,
                            "notes": "remote override",
                        }
                    ],
                    source_url="https://example.com/openai-pricing.json",
                )[0]
            ]
        return []

    async def _fake_fetch_remote(provider: str):
        return await _fake_remote(provider)

    service._fetch_remote_pricing = _fake_fetch_remote  # type: ignore[method-assign]
    pricing = await service.get_latest_pricing("gpt-4o", "openai")
    assert pricing is not None
    assert pricing.input_price_per_1k == 9.9
    assert pricing.output_price_per_1k == 19.9
    assert pricing.source == "openai_remote"


def test_parse_remote_payload_supports_per_token_unit() -> None:
    service = PricingService()
    parsed = service._parse_remote_pricing_payload(
        provider="qwen",
        payload=[
            {
                "model": "qwen-plus",
                "prompt": 0.0004,
                "completion": 0.0012,
                "unit": "per_token",
            }
        ],
        source_url="https://example.com/qwen.json",
    )

    assert len(parsed) == 1
    assert parsed[0].model_name == "qwen-plus"
    assert parsed[0].input_price_per_1k == pytest.approx(0.4)
    assert parsed[0].output_price_per_1k == pytest.approx(1.2)


@pytest.mark.asyncio
async def test_remote_source_supports_file_scheme(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    pricing_file = tmp_path / "openai-pricing.json"
    pricing_file.write_text(
        json.dumps(
            [
                {
                    "model_name": "gpt-4o-mini",
                    "input_price_per_1k": 0.123,
                    "output_price_per_1k": 0.456,
                }
            ]
        ),
        encoding="utf-8",
    )
    monkeypatch.setenv(
        "LLM_ROUTER_PRICING_SOURCE_URLS",
        json.dumps({"openai": f"file://{pricing_file}"}),
    )

    service = PricingService()
    pricing = await service.get_latest_pricing("gpt-4o-mini", "openai")
    assert pricing is not None
    assert pricing.input_price_per_1k == pytest.approx(0.123)
    assert pricing.output_price_per_1k == pytest.approx(0.456)
    assert pricing.source == "openai_remote"


@pytest.mark.asyncio
async def test_remote_source_supports_absolute_local_path(
    monkeypatch: pytest.MonkeyPatch, tmp_path: Path
) -> None:
    pricing_file = tmp_path / "qwen-pricing.json"
    pricing_file.write_text(
        json.dumps(
            {
                "models": [
                    {
                        "name": "qwen-plus",
                        "input": 0.4,
                        "output": 1.2,
                        "unit": "per_1k",
                    }
                ]
            }
        ),
        encoding="utf-8",
    )
    monkeypatch.setenv(
        "LLM_ROUTER_PRICING_SOURCE_URLS",
        json.dumps({"qwen": str(pricing_file)}),
    )

    service = PricingService()
    pricing = await service.get_latest_pricing("qwen-plus", "qwen (cn)")
    assert pricing is not None
    assert pricing.input_price_per_1k == pytest.approx(0.4)
    assert pricing.output_price_per_1k == pytest.approx(1.2)
    assert pricing.source == "qwen_remote"
