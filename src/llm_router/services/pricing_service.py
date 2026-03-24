"""定价服务 - 统一从多来源获取模型定价信息。"""

from __future__ import annotations

import json
import logging
import os
from datetime import datetime, timedelta
from pathlib import Path
from typing import Any, Callable, Dict, List, Optional

from curl_cffi import requests
from pydantic import BaseModel

logger = logging.getLogger(__name__)


class ModelPricing(BaseModel):
    """模型定价信息"""

    model_name: str
    provider: str
    input_price_per_1k: float  # 每 1k 输入 token 价格（USD）
    output_price_per_1k: float  # 每 1k 输出 token 价格（USD）
    source: str  # 数据来源
    last_updated: datetime
    notes: Optional[str] = None  # 备注信息


class PricingCache:
    """定价缓存"""

    def __init__(self, cache_ttl_hours: int = 24):
        self.cache: Dict[str, tuple[ModelPricing, datetime]] = {}
        self.cache_ttl = timedelta(hours=cache_ttl_hours)

    def get(self, key: str) -> Optional[ModelPricing]:
        """获取缓存的定价信息"""
        if key not in self.cache:
            return None
        pricing, cached_at = self.cache[key]
        if datetime.utcnow() - cached_at > self.cache_ttl:
            del self.cache[key]
            return None
        return pricing

    def set(self, key: str, pricing: ModelPricing):
        """设置缓存"""
        self.cache[key] = (pricing, datetime.utcnow())

    def clear(self):
        """清空缓存"""
        self.cache.clear()


class PricingService:
    """定价服务 - 从网络和内置目录获取最新定价信息"""

    # 兼容 router.toml 中的 provider 命名
    PROVIDER_ALIASES: Dict[str, str] = {
        "claude": "claude",
        "anthropic": "claude",
        "gemini": "gemini",
        "openai": "openai",
        "openrouter": "openrouter",
        "deepseek": "deepseek",
        "deepseek (cn)": "deepseek",
        "qwen": "qwen",
        "qwen (cn)": "qwen",
        "kimi": "kimi",
        "kimi (cn)": "kimi",
        "glm": "glm",
        "glm (cn)": "glm",
        "groq": "groq",
        "grok": "groq",
    }

    STATIC_PRICING_CATALOG: Dict[str, Dict[str, object]] = {
        "openai": {
            "source": "openai_reference",
            "notes": "OpenAI 常用模型参考价",
            "models": [
                ("gpt-5", 1.25, 10.0),
                ("gpt-5-mini", 0.25, 2.0),
                ("gpt-5-nano", 0.05, 0.4),
                ("gpt-4.1", 3.0, 12.0),
                ("gpt-4o", 2.5, 10.0),
                ("gpt-4o-mini", 0.15, 0.6),
                ("gpt-3.5-turbo", 0.5, 1.5),
            ],
        },
        "claude": {
            "source": "anthropic_reference",
            "notes": "Anthropic 常用模型参考价",
            "models": [
                ("claude-opus-4", 15.0, 75.0),
                ("claude-sonnet-4", 3.0, 15.0),
                ("claude-3.7-sonnet", 3.0, 15.0),
                ("claude-3.5-haiku", 0.25, 1.25),
            ],
        },
        "gemini": {
            "source": "google_reference",
            "notes": "Google Gemini 常用模型参考价",
            "models": [
                ("gemini-2.5-pro", 1.25, 10.0),
                ("gemini-2.0-flash", 0.1, 0.4),
                ("gemini-1.5-pro", 1.25, 5.0),
                ("gemini-1.5-flash", 0.075, 0.3),
                # 免费层/实验模型
                ("gemini-2.0-flash-exp", 0.0, 0.0),
            ],
        },
        "deepseek": {
            "source": "deepseek_reference",
            "notes": "DeepSeek 常用模型与免费模型参考价",
            "models": [
                ("deepseek-chat", 0.14, 0.28),
                ("deepseek-reasoner", 0.55, 2.19),
                ("deepseek-v3", 0.14, 0.28),
                ("deepseek-r1", 0.55, 2.19),
                ("deepseek-r1-free", 0.0, 0.0),
            ],
        },
        "qwen": {
            "source": "qwen_reference",
            "notes": "Qwen 常用模型与免费模型参考价",
            "models": [
                ("qwen-plus", 0.4, 1.2),
                ("qwen-max", 1.6, 4.8),
                ("qwen-turbo", 0.08, 0.24),
                ("qwen2.5-72b-instruct", 0.9, 2.7),
                ("qwen2.5-7b-instruct-free", 0.0, 0.0),
            ],
        },
        "kimi": {
            "source": "moonshot_reference",
            "notes": "Kimi 常用模型与免费模型参考价",
            "models": [
                ("moonshot-v1-8k", 0.12, 0.12),
                ("moonshot-v1-32k", 0.24, 0.24),
                ("moonshot-v1-128k", 0.6, 0.6),
                ("kimi-k2-free", 0.0, 0.0),
            ],
        },
        "glm": {
            "source": "zhipu_reference",
            "notes": "GLM 常用模型与免费模型参考价",
            "models": [
                ("glm-4-plus", 0.5, 0.5),
                ("glm-4-air", 0.1, 0.1),
                ("glm-4-flash", 0.0, 0.0),
                ("glm-zero-preview", 0.0, 0.0),
            ],
        },
        "groq": {
            "source": "groq_reference",
            "notes": "Groq 常用模型与免费模型参考价",
            "models": [
                ("llama-3.3-70b-versatile", 0.59, 0.79),
                ("llama-3.1-8b-instant", 0.05, 0.08),
                ("mixtral-8x7b-32768", 0.24, 0.24),
                ("llama-3.1-8b-instant-free", 0.0, 0.0),
            ],
        },
    }

    def __init__(self, cache_ttl_hours: int = 24):
        self.cache = PricingCache(cache_ttl_hours)
        self.http_client = requests.AsyncSession(timeout=30.0, trust_env=False)
        self.remote_source_urls = self._load_remote_source_urls()
        self.source_fetchers: Dict[str, Callable[[], object]] = {
            "openai": self.fetch_openai_pricing,
            "claude": self.fetch_anthropic_pricing,
            "gemini": self.fetch_gemini_pricing,
            "deepseek": self.fetch_deepseek_pricing,
            "qwen": self.fetch_qwen_pricing,
            "kimi": self.fetch_kimi_pricing,
            "glm": self.fetch_glm_pricing,
            "groq": self.fetch_groq_pricing,
            "openrouter": self.fetch_openrouter_pricing,
        }

    async def __aenter__(self):
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        await self.http_client.close()

    @staticmethod
    def _load_remote_source_urls() -> Dict[str, str]:
        """从环境变量加载定价来源 URL 映射。

        格式：
        LLM_ROUTER_PRICING_SOURCE_URLS='{"openai":"https://...","qwen":"https://..."}'
        """
        raw_value = os.getenv("LLM_ROUTER_PRICING_SOURCE_URLS", "").strip()
        if not raw_value:
            return {}

        try:
            parsed = json.loads(raw_value)
        except json.JSONDecodeError:
            logger.warning("LLM_ROUTER_PRICING_SOURCE_URLS 不是合法 JSON，已忽略")
            return {}

        if not isinstance(parsed, dict):
            logger.warning("LLM_ROUTER_PRICING_SOURCE_URLS 需为 JSON object，已忽略")
            return {}

        result: Dict[str, str] = {}
        for provider, url in parsed.items():
            if isinstance(provider, str) and isinstance(url, str) and url.strip():
                result[provider.strip().lower()] = url.strip()
        return result

    @classmethod
    def normalize_provider_name(cls, provider: str) -> str:
        """将 provider 名称归一化为定价来源 key。"""
        normalized = provider.strip().lower()
        return cls.PROVIDER_ALIASES.get(normalized, normalized)

    def _build_static_pricing(self, provider: str) -> List[ModelPricing]:
        catalog = self.STATIC_PRICING_CATALOG.get(provider)
        if not catalog:
            return []
        source = str(catalog["source"])
        notes = str(catalog["notes"])
        models = catalog["models"]
        now = datetime.utcnow()
        result: List[ModelPricing] = []
        for model_name, input_price, output_price in models:  # type: ignore[misc]
            model_note = notes
            if float(input_price) == 0.0 and float(output_price) == 0.0:
                model_note = f"{notes}；免费模型/免费层"
            result.append(
                ModelPricing(
                    model_name=str(model_name),
                    provider=provider,
                    input_price_per_1k=float(input_price),
                    output_price_per_1k=float(output_price),
                    source=source,
                    last_updated=now,
                    notes=model_note,
                )
            )
        return result

    async def _fetch_remote_pricing(self, provider: str) -> List[ModelPricing]:
        """从外部配置的 URL 获取定价，失败时返回空列表。"""
        source_url = self.remote_source_urls.get(provider)
        if not source_url:
            return []

        try:
            payload = await self._load_remote_payload(source_url)
            return self._parse_remote_pricing_payload(provider, payload, source_url)
        except requests.RequestsError as e:
            logger.warning("远程定价拉取失败 provider=%s url=%s err=%s", provider, source_url, e)
            return []
        except Exception as e:
            logger.error("远程定价解析失败 provider=%s url=%s err=%s", provider, source_url, e, exc_info=True)
            return []

    async def _load_remote_payload(self, source_url: str) -> Any:
        """加载远程来源 payload，支持 http(s)、file:// 和绝对路径。"""
        if source_url.startswith("file://"):
            local_path = Path(source_url[7:])
            return json.loads(local_path.read_text(encoding="utf-8"))

        if source_url.startswith("/"):
            local_path = Path(source_url)
            return json.loads(local_path.read_text(encoding="utf-8"))

        response = await self.http_client.get(source_url)
        response.raise_for_status()
        return response.json()

    def _parse_remote_pricing_payload(
        self,
        provider: str,
        payload: Any,
        source_url: str,
    ) -> List[ModelPricing]:
        if isinstance(payload, list):
            items = payload
        elif isinstance(payload, dict):
            maybe_items = payload.get("models") or payload.get("data") or payload.get("items")
            if not isinstance(maybe_items, list):
                return []
            items = maybe_items
        else:
            return []

        now = datetime.utcnow()
        result: List[ModelPricing] = []
        for item in items:
            if not isinstance(item, dict):
                continue

            model_name = (
                item.get("model_name")
                or item.get("name")
                or item.get("model")
                or item.get("id")
            )
            if not isinstance(model_name, str) or not model_name.strip():
                continue

            input_price = item.get("input_price_per_1k")
            output_price = item.get("output_price_per_1k")
            if input_price is None:
                input_price = item.get("input")
            if output_price is None:
                output_price = item.get("output")
            if input_price is None:
                input_price = item.get("prompt")
            if output_price is None:
                output_price = item.get("completion")

            if input_price is None or output_price is None:
                continue

            unit = str(item.get("unit", "per_1k")).strip().lower()
            try:
                parsed_input = float(input_price)
                parsed_output = float(output_price)
            except (TypeError, ValueError):
                continue

            if unit == "per_token":
                parsed_input *= 1000.0
                parsed_output *= 1000.0

            notes = item.get("notes")
            if not isinstance(notes, str) or not notes.strip():
                notes = f"远程来源: {source_url}"
            if parsed_input == 0.0 and parsed_output == 0.0 and "免费" not in notes:
                notes = f"{notes}；免费模型/免费层"

            clean_name = model_name.split("/")[-1] if "/" in model_name else model_name
            result.append(
                ModelPricing(
                    model_name=clean_name,
                    provider=provider,
                    input_price_per_1k=parsed_input,
                    output_price_per_1k=parsed_output,
                    source=f"{provider}_remote",
                    last_updated=now,
                    notes=notes,
                )
            )
        return result

    async def _fetch_catalog_pricing(self, provider: str) -> List[ModelPricing]:
        """远程来源优先，失败时使用内置静态目录。"""
        remote = await self._fetch_remote_pricing(provider)
        if remote:
            return remote
        return self._build_static_pricing(provider)

    async def fetch_openai_pricing(self) -> List[ModelPricing]:
        return await self._fetch_catalog_pricing("openai")

    async def fetch_anthropic_pricing(self) -> List[ModelPricing]:
        return await self._fetch_catalog_pricing("claude")

    async def fetch_gemini_pricing(self) -> List[ModelPricing]:
        return await self._fetch_catalog_pricing("gemini")

    async def fetch_deepseek_pricing(self) -> List[ModelPricing]:
        return await self._fetch_catalog_pricing("deepseek")

    async def fetch_qwen_pricing(self) -> List[ModelPricing]:
        return await self._fetch_catalog_pricing("qwen")

    async def fetch_kimi_pricing(self) -> List[ModelPricing]:
        return await self._fetch_catalog_pricing("kimi")

    async def fetch_glm_pricing(self) -> List[ModelPricing]:
        return await self._fetch_catalog_pricing("glm")

    async def fetch_groq_pricing(self) -> List[ModelPricing]:
        return await self._fetch_catalog_pricing("groq")

    async def fetch_openrouter_pricing(self) -> List[ModelPricing]:
        """从 OpenRouter API 获取最新定价信息。"""
        try:
            url = "https://openrouter.ai/api/v1/models"
            response = await self.http_client.get(url)
            response.raise_for_status()

            data = response.json()
            result = []

            if "data" in data:
                for model in data["data"]:
                    model_id = model.get("id", "")
                    pricing = model.get("pricing", {})
                    if not pricing:
                        continue

                    # OpenRouter pricing: 每 1 token 的美元价格，转换为每 1k token。
                    input_price = float(pricing.get("prompt", 0) or 0) * 1000.0
                    output_price = float(pricing.get("completion", 0) or 0) * 1000.0
                    model_name = model_id.split("/")[-1] if "/" in model_id else model_id

                    notes = f"从 OpenRouter API 获取，模型ID: {model_id}"
                    if input_price == 0.0 and output_price == 0.0:
                        notes = f"{notes}；免费模型/免费层"

                    result.append(
                        ModelPricing(
                            model_name=model_name,
                            provider="openrouter",
                            input_price_per_1k=input_price,
                            output_price_per_1k=output_price,
                            source="openrouter_api",
                            last_updated=datetime.utcnow(),
                            notes=notes,
                        )
                    )

            return result
        except requests.RequestsError as e:
            logger.warning(f"获取OpenRouter定价网络请求失败: {e}")
            return []
        except Exception as e:
            logger.error(f"获取OpenRouter定价失败: {e}", exc_info=True)
            return []

    async def _fetch_pricing_by_provider(self, provider: str) -> List[ModelPricing]:
        normalized_provider = self.normalize_provider_name(provider)
        fetcher = self.source_fetchers.get(normalized_provider)
        if not fetcher:
            return []

        try:
            pricing_list = await fetcher()  # type: ignore[misc]
            return pricing_list if isinstance(pricing_list, list) else []
        except Exception as e:
            logger.error("获取定价失败 provider=%s: %s", normalized_provider, e, exc_info=True)
            return []

    async def get_latest_pricing(self, model_name: str, provider: str) -> Optional[ModelPricing]:
        """获取指定模型的最新定价信息。"""
        normalized_provider = self.normalize_provider_name(provider)
        cache_key = f"{normalized_provider}:{model_name}"

        cached = self.cache.get(cache_key)
        if cached:
            logger.debug("从缓存获取定价: %s", cache_key)
            return cached

        all_pricing = await self._fetch_pricing_by_provider(normalized_provider)

        for pricing in all_pricing:
            if pricing.model_name == model_name or model_name.endswith(pricing.model_name):
                self.cache.set(cache_key, pricing)
                logger.debug("找到定价并缓存: %s -> %s", cache_key, pricing.model_name)
                return pricing

        model_name_clean = model_name.split("/")[-1] if "/" in model_name else model_name
        for pricing in all_pricing:
            if pricing.model_name == model_name_clean:
                self.cache.set(cache_key, pricing)
                logger.debug("找到定价并缓存（清理后）: %s -> %s", cache_key, pricing.model_name)
                return pricing

        logger.warning("未找到模型定价: %s:%s", normalized_provider, model_name)
        return None

    async def get_all_latest_pricing(self) -> Dict[str, List[ModelPricing]]:
        """获取所有支持 provider 的最新定价信息。"""
        result: Dict[str, List[ModelPricing]] = {}

        for provider in self.source_fetchers:
            pricing = await self._fetch_pricing_by_provider(provider)
            if pricing:
                result[provider] = pricing

        return result


__all__ = ["PricingService", "ModelPricing", "PricingCache"]
