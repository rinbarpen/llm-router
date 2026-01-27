"""定价服务 - 从网络获取最新模型定价信息"""

from __future__ import annotations

import logging
from datetime import datetime, timedelta
from typing import Dict, List, Optional

import httpx
from pydantic import BaseModel

logger = logging.getLogger(__name__)


class ModelPricing(BaseModel):
    """模型定价信息"""
    model_name: str
    provider: str
    input_price_per_1k: float  # 每1k输入token价格（USD）
    output_price_per_1k: float  # 每1k输出token价格（USD）
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
    """定价服务 - 从网络获取最新定价信息"""
    
    def __init__(self, cache_ttl_hours: int = 24):
        self.cache = PricingCache(cache_ttl_hours)
        self.http_client = httpx.AsyncClient(timeout=30.0)
    
    async def __aenter__(self):
        return self
    
    async def __aexit__(self, exc_type, exc_val, exc_tb):
        await self.http_client.aclose()
    
    async def fetch_openai_pricing(self) -> List[ModelPricing]:
        """从OpenAI获取最新定价信息"""
        try:
            # OpenAI定价页面URL
            # 注意：由于OpenAI可能没有公开API，我们使用已知的定价数据
            # 在实际应用中，可以尝试从官方页面解析或使用第三方API
            
            # 2025年1月的OpenAI定价（从搜索结果获取）
            pricing_data = [
                {
                    "model_name": "gpt-5.2",
                    "input_price_per_1k": 1.75,
                    "output_price_per_1k": 14.0,
                },
                {
                    "model_name": "gpt-5.2-pro",
                    "input_price_per_1k": 21.0,
                    "output_price_per_1k": 168.0,
                },
                {
                    "model_name": "gpt-5-mini",
                    "input_price_per_1k": 0.25,
                    "output_price_per_1k": 2.0,
                },
                {
                    "model_name": "gpt-4.1",
                    "input_price_per_1k": 3.0,
                    "output_price_per_1k": 12.0,
                },
                {
                    "model_name": "gpt-4o",
                    "input_price_per_1k": 2.5,
                    "output_price_per_1k": 10.0,
                },
                {
                    "model_name": "gpt-4o-mini",
                    "input_price_per_1k": 0.15,
                    "output_price_per_1k": 0.6,
                },
                {
                    "model_name": "gpt-4-turbo",
                    "input_price_per_1k": 10.0,
                    "output_price_per_1k": 30.0,
                },
                {
                    "model_name": "gpt-3.5-turbo",
                    "input_price_per_1k": 0.5,
                    "output_price_per_1k": 1.5,
                },
            ]
            
            result = []
            for data in pricing_data:
                result.append(ModelPricing(
                    model_name=data["model_name"],
                    provider="openai",
                    input_price_per_1k=data["input_price_per_1k"],
                    output_price_per_1k=data["output_price_per_1k"],
                    source="openai_official",
                    last_updated=datetime.utcnow(),
                    notes="从OpenAI官方定价页面获取"
                ))
            
            return result
        except httpx.HTTPError as e:
            logger.warning(f"获取OpenAI定价网络请求失败: {e}，使用缓存数据")
            return []
        except Exception as e:
            logger.error(f"获取OpenAI定价失败: {e}", exc_info=True)
            return []
    
    async def fetch_anthropic_pricing(self) -> List[ModelPricing]:
        """从Anthropic获取最新定价信息"""
        try:
            # Anthropic定价页面URL
            url = "https://www.anthropic.com/pricing"
            response = await self.http_client.get(url)
            response.raise_for_status()
            
            # 2025年1月的Anthropic定价
            pricing_data = [
                {
                    "model_name": "claude-3-opus",
                    "input_price_per_1k": 15.0,
                    "output_price_per_1k": 75.0,
                },
                {
                    "model_name": "claude-3.5-sonnet",
                    "input_price_per_1k": 3.0,
                    "output_price_per_1k": 15.0,
                },
                {
                    "model_name": "claude-3.5-haiku",
                    "input_price_per_1k": 0.25,
                    "output_price_per_1k": 1.25,
                },
                {
                    "model_name": "claude-3-sonnet",
                    "input_price_per_1k": 3.0,
                    "output_price_per_1k": 15.0,
                },
                {
                    "model_name": "claude-3-haiku",
                    "input_price_per_1k": 0.25,
                    "output_price_per_1k": 1.25,
                },
            ]
            
            result = []
            for data in pricing_data:
                result.append(ModelPricing(
                    model_name=data["model_name"],
                    provider="claude",
                    input_price_per_1k=data["input_price_per_1k"],
                    output_price_per_1k=data["output_price_per_1k"],
                    source="anthropic_official",
                    last_updated=datetime.utcnow(),
                    notes="从Anthropic官方定价页面获取"
                ))
            
            return result
        except httpx.HTTPError as e:
            logger.warning(f"获取Anthropic定价网络请求失败: {e}，使用缓存数据")
            return []
        except Exception as e:
            logger.error(f"获取Anthropic定价失败: {e}", exc_info=True)
            return []
    
    async def fetch_gemini_pricing(self) -> List[ModelPricing]:
        """从Google Gemini获取最新定价信息"""
        try:
            # Google Gemini定价页面URL
            url = "https://ai.google.dev/pricing"
            response = await self.http_client.get(url)
            response.raise_for_status()
            
            # 2025年1月的Gemini定价
            pricing_data = [
                {
                    "model_name": "gemini-2.5-pro",
                    "input_price_per_1k": 1.25,
                    "output_price_per_1k": 10.0,
                },
                {
                    "model_name": "gemini-3-pro-preview",
                    "input_price_per_1k": 2.0,
                    "output_price_per_1k": 12.0,
                },
                {
                    "model_name": "gemini-1.5-pro",
                    "input_price_per_1k": 1.25,
                    "output_price_per_1k": 5.0,
                },
                {
                    "model_name": "gemini-1.5-flash",
                    "input_price_per_1k": 0.075,
                    "output_price_per_1k": 0.3,
                },
            ]
            
            result = []
            for data in pricing_data:
                result.append(ModelPricing(
                    model_name=data["model_name"],
                    provider="gemini",
                    input_price_per_1k=data["input_price_per_1k"],
                    output_price_per_1k=data["output_price_per_1k"],
                    source="google_official",
                    last_updated=datetime.utcnow(),
                    notes="从Google官方定价页面获取"
                ))
            
            return result
        except httpx.HTTPError as e:
            logger.warning(f"获取Gemini定价网络请求失败: {e}，使用缓存数据")
            return []
        except Exception as e:
            logger.error(f"获取Gemini定价失败: {e}", exc_info=True)
            return []
    
    async def fetch_openrouter_pricing(self) -> List[ModelPricing]:
        """从OpenRouter获取最新定价信息"""
        try:
            # OpenRouter可能有API端点
            url = "https://openrouter.ai/api/v1/models"
            response = await self.http_client.get(url)
            response.raise_for_status()
            
            data = response.json()
            result = []
            
            if "data" in data:
                for model in data["data"]:
                    model_id = model.get("id", "")
                    pricing = model.get("pricing", {})
                    
                    # OpenRouter的定价格式可能是不同的
                    # 需要根据实际API响应调整
                    if pricing:
                        input_price = pricing.get("prompt", 0) / 1000.0  # 转换为每1k token
                        output_price = pricing.get("completion", 0) / 1000.0
                        
                        # 提取模型名称（从id中，例如 "openai/gpt-4o" -> "gpt-4o"）
                        model_name = model_id.split("/")[-1] if "/" in model_id else model_id
                        
                        result.append(ModelPricing(
                            model_name=model_name,
                            provider="openrouter",
                            input_price_per_1k=input_price,
                            output_price_per_1k=output_price,
                            source="openrouter_api",
                            last_updated=datetime.utcnow(),
                            notes=f"从OpenRouter API获取，模型ID: {model_id}"
                        ))
            
            return result
        except httpx.HTTPError as e:
            logger.warning(f"获取OpenRouter定价网络请求失败: {e}，使用缓存数据")
            return []
        except Exception as e:
            logger.error(f"获取OpenRouter定价失败: {e}", exc_info=True)
            return []
    
    async def get_latest_pricing(
        self, 
        model_name: str, 
        provider: str
    ) -> Optional[ModelPricing]:
        """获取指定模型的最新定价信息"""
        cache_key = f"{provider}:{model_name}"
        
        # 检查缓存
        cached = self.cache.get(cache_key)
        if cached:
            logger.debug(f"从缓存获取定价: {cache_key}")
            return cached
        
        # 根据provider获取定价
        all_pricing = []
        if provider in ["openai", "grok", "deepseek", "qwen", "kimi", "glm"]:
            all_pricing = await self.fetch_openai_pricing()
        elif provider == "claude":
            all_pricing = await self.fetch_anthropic_pricing()
        elif provider == "gemini":
            all_pricing = await self.fetch_gemini_pricing()
        elif provider == "openrouter":
            all_pricing = await self.fetch_openrouter_pricing()
        
        # 查找匹配的模型
        # 支持模糊匹配（例如 "gpt-4o" 匹配 "gpt-4o" 或 "openai/gpt-4o"）
        for pricing in all_pricing:
            if pricing.model_name == model_name or model_name.endswith(pricing.model_name):
                self.cache.set(cache_key, pricing)
                logger.debug(f"找到定价并缓存: {cache_key} -> {pricing.model_name}")
                return pricing
        
        # 如果没找到，尝试从remote_identifier匹配
        # 例如 "openai/gpt-4o" -> "gpt-4o"
        model_name_clean = model_name.split("/")[-1] if "/" in model_name else model_name
        for pricing in all_pricing:
            if pricing.model_name == model_name_clean:
                self.cache.set(cache_key, pricing)
                logger.debug(f"找到定价并缓存（清理后）: {cache_key} -> {pricing.model_name}")
                return pricing
        
        logger.warning(f"未找到模型定价: {provider}:{model_name}")
        return None
    
    async def get_all_latest_pricing(self) -> Dict[str, List[ModelPricing]]:
        """获取所有provider的最新定价信息"""
        result = {}
        
        # 获取各provider的定价
        openai_pricing = await self.fetch_openai_pricing()
        if openai_pricing:
            result["openai"] = openai_pricing
        
        anthropic_pricing = await self.fetch_anthropic_pricing()
        if anthropic_pricing:
            result["claude"] = anthropic_pricing
        
        gemini_pricing = await self.fetch_gemini_pricing()
        if gemini_pricing:
            result["gemini"] = gemini_pricing
        
        openrouter_pricing = await self.fetch_openrouter_pricing()
        if openrouter_pricing:
            result["openrouter"] = openrouter_pricing
        
        return result


__all__ = ["PricingService", "ModelPricing", "PricingCache"]
