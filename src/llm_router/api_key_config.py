from __future__ import annotations

import os
from typing import Any, Dict, List, Optional

from pydantic import BaseModel, Field, model_validator


class ParameterLimits(BaseModel):
    """API Key 的参数限制"""

    max_tokens: Optional[int] = Field(default=None, ge=1, description="最大 token 数")
    temperature: Optional[float] = Field(default=None, ge=0.0, le=2.0, description="温度参数范围")
    top_p: Optional[float] = Field(default=None, ge=0.0, le=1.0, description="top_p 参数范围")
    frequency_penalty: Optional[float] = Field(default=None, ge=-2.0, le=2.0, description="频率惩罚范围")
    presence_penalty: Optional[float] = Field(default=None, ge=-2.0, le=2.0, description="存在惩罚范围")
    # 其他自定义限制
    custom_limits: Dict[str, Any] = Field(default_factory=dict, description="自定义参数限制")


class APIKeyConfig(BaseModel):
    """API Key 配置"""

    key: Optional[str] = Field(default=None, description="API Key 值（直接指定）")
    key_env: Optional[str] = Field(default=None, description="环境变量名（从环境变量读取 key）")
    name: Optional[str] = Field(default=None, description="API Key 名称/描述")
    allowed_models: Optional[List[str]] = Field(
        default=None, description="允许调用的模型列表（None 表示无限制）"
    )
    allowed_providers: Optional[List[str]] = Field(
        default=None, description="允许调用的 Provider 列表（None 表示无限制）"
    )
    parameter_limits: Optional[ParameterLimits] = Field(
        default=None, description="参数限制（None 表示无限制）"
    )
    is_active: bool = Field(default=True, description="是否启用")

    @model_validator(mode="after")
    def _check_key(self) -> "APIKeyConfig":
        """确保至少提供了 key 或 key_env 之一"""
        if not self.key and not self.key_env:
            raise ValueError("必须提供 key 或 key_env 之一")
        return self

    def resolved_key(self) -> Optional[str]:
        """从环境变量或直接值解析 API Key（单个值，向后兼容）"""
        keys = self.resolved_keys()
        return keys[0] if keys else None

    def resolved_keys(self) -> List[str]:
        """从环境变量或直接值解析 API Key（支持多个，逗号分隔）"""
        if self.key:
            # 支持逗号分隔的多个 key
            keys = [k.strip() for k in self.key.split(",") if k.strip()]
            return keys
        if self.key_env:
            env_value = os.getenv(self.key_env)
            if env_value:
                # 支持逗号分隔的多个 key
                keys = [k.strip() for k in env_value.split(",") if k.strip()]
                return keys
        return []

    def is_model_allowed(self, provider_name: str, model_name: str) -> bool:
        """检查是否允许调用指定模型"""
        if not self.is_active:
            return False

        # 检查 Provider 限制
        if self.allowed_providers is not None:
            if provider_name not in self.allowed_providers:
                return False

        # 检查模型限制
        if self.allowed_models is not None:
            # 支持完整模型名（provider/model）或仅模型名
            full_name = f"{provider_name}/{model_name}"
            if full_name not in self.allowed_models and model_name not in self.allowed_models:
                return False

        return True

    def validate_parameters(self, parameters: Dict[str, Any]) -> Dict[str, Any]:
        """验证并限制参数，返回调整后的参数"""
        if self.parameter_limits is None:
            return parameters

        limits = self.parameter_limits
        validated = dict(parameters)

        # 验证 max_tokens
        if limits.max_tokens is not None:
            if "max_tokens" in validated:
                validated["max_tokens"] = min(validated["max_tokens"], limits.max_tokens)
            else:
                validated["max_tokens"] = limits.max_tokens

        # 验证 temperature
        if limits.temperature is not None:
            if "temperature" in validated:
                validated["temperature"] = min(validated["temperature"], limits.temperature)
            else:
                validated["temperature"] = limits.temperature

        # 验证 top_p
        if limits.top_p is not None:
            if "top_p" in validated:
                validated["top_p"] = min(validated["top_p"], limits.top_p)
            else:
                validated["top_p"] = limits.top_p

        # 验证 frequency_penalty
        if limits.frequency_penalty is not None:
            if "frequency_penalty" in validated:
                validated["frequency_penalty"] = min(
                    validated["frequency_penalty"], limits.frequency_penalty
                )
            else:
                validated["frequency_penalty"] = limits.frequency_penalty

        # 验证 presence_penalty
        if limits.presence_penalty is not None:
            if "presence_penalty" in validated:
                validated["presence_penalty"] = min(
                    validated["presence_penalty"], limits.presence_penalty
                )
            else:
                validated["presence_penalty"] = limits.presence_penalty

        # 验证自定义限制
        for key, limit_value in limits.custom_limits.items():
            if key in validated:
                if isinstance(limit_value, (int, float)):
                    validated[key] = min(validated[key], limit_value)
                elif isinstance(limit_value, dict):
                    # 支持范围限制，如 {"max": 100, "min": 0}
                    if "max" in limit_value:
                        validated[key] = min(validated[key], limit_value["max"])
                    if "min" in limit_value:
                        validated[key] = max(validated[key], limit_value["min"])

        return validated


__all__ = ["APIKeyConfig", "ParameterLimits"]

