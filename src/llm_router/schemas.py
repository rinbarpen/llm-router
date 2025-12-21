from __future__ import annotations

from datetime import datetime
from typing import Any, Dict, List, Literal, Optional

from pydantic import BaseModel, Field, ValidationInfo, field_validator, model_validator

from .api_key_config import ParameterLimits
from .db.models import InvocationStatus, ProviderType


class ProviderCreate(BaseModel):
    name: str
    type: ProviderType
    base_url: Optional[str] = None
    api_key: Optional[str] = None
    is_active: bool = True
    settings: Dict[str, Any] = Field(default_factory=dict)


class ProviderRead(BaseModel):
    id: int
    name: str
    type: ProviderType
    is_active: bool
    base_url: Optional[str]

    class Config:
        from_attributes = True


class RateLimitConfig(BaseModel):
    max_requests: int = Field(gt=0)
    per_seconds: int = Field(gt=0)
    burst_size: Optional[int] = Field(default=None, gt=0)
    notes: Optional[str] = None
    config: Dict[str, Any] = Field(default_factory=dict)

    @field_validator("burst_size")
    @classmethod
    def _validate_burst(cls, value: Optional[int], info: ValidationInfo) -> Optional[int]:
        if value is None:
            return None
        max_requests = info.data.get("max_requests")
        if max_requests and value < max_requests:
            raise ValueError("burst_size 必须大于或等于 max_requests")
        return value


class ModelCreate(BaseModel):
    name: str
    provider_id: Optional[int] = None
    provider_name: Optional[str] = None
    display_name: Optional[str] = None
    description: Optional[str] = None
    remote_identifier: Optional[str] = None
    is_active: bool = True
    tags: List[str] = Field(default_factory=list)
    default_params: Dict[str, Any] = Field(default_factory=dict)
    config: Dict[str, Any] = Field(default_factory=dict)
    download_uri: Optional[str] = None
    local_path: Optional[str] = None
    rate_limit: Optional[RateLimitConfig] = None

    @model_validator(mode="after")
    def _check_provider(self) -> "ModelCreate":
        if self.provider_id is None and self.provider_name is None:
            raise ValueError("provider_id 或 provider_name 必须至少提供一个")
        return self


class ModelUpdate(BaseModel):
    display_name: Optional[str] = None
    description: Optional[str] = None
    is_active: Optional[bool] = None
    tags: Optional[List[str]] = None
    default_params: Optional[Dict[str, Any]] = None
    config: Optional[Dict[str, Any]] = None
    download_uri: Optional[str] = None
    local_path: Optional[str] = None
    rate_limit: Optional[RateLimitConfig] = None


class TagRead(BaseModel):
    id: int
    name: str
    description: Optional[str] = None

    class Config:
        from_attributes = True


class ModelRead(BaseModel):
    id: int
    name: str
    display_name: Optional[str]
    description: Optional[str]
    provider_id: int
    provider_name: str
    provider_type: ProviderType
    tags: List[str]
    default_params: Dict[str, Any]
    config: Dict[str, Any]
    rate_limit: Optional[RateLimitConfig] = None
    local_path: Optional[str] = None

    class Config:
        from_attributes = True


class ModelQuery(BaseModel):
    tags: List[str] = Field(default_factory=list)
    provider_types: List[ProviderType] = Field(default_factory=list)
    include_inactive: bool = False


class ChatMessage(BaseModel):
    role: Literal["system", "user", "assistant"]
    content: str


class ModelInvokeRequest(BaseModel):
    prompt: Optional[str] = None
    messages: Optional[List[ChatMessage]] = None
    parameters: Dict[str, Any] = Field(default_factory=dict)
    stream: bool = False
    remote_identifier_override: Optional[str] = None  # 用于 OpenAI 兼容 API，覆盖数据库中的 remote_identifier

    @model_validator(mode="after")
    def _validate_input(self) -> "ModelInvokeRequest":
        if not self.prompt and not self.messages:
            raise ValueError("prompt 或 messages 至少需要提供一个")
        return self


class ModelInvokeResponse(BaseModel):
    output_text: str
    raw: Dict[str, Any] = Field(default_factory=dict)


class ModelStreamChunk(BaseModel):
    """Provider 流式输出的统一结构。"""

    delta: Dict[str, Any] = Field(default_factory=dict)
    text: Optional[str] = None
    raw: Optional[Dict[str, Any]] = None
    usage: Optional[Dict[str, Any]] = None
    finish_reason: Optional[str] = None
    is_final: bool = False


# Monitor schemas
class InvocationRead(BaseModel):
    id: int
    model_id: int
    provider_id: int
    model_name: str
    provider_name: str
    started_at: datetime
    completed_at: Optional[datetime]
    duration_ms: Optional[float]
    status: InvocationStatus
    error_message: Optional[str]
    request_prompt: Optional[str]
    request_messages: Optional[List[Dict[str, Any]]]
    request_parameters: Dict[str, Any]
    response_text: Optional[str]
    response_text_length: Optional[int]
    prompt_tokens: Optional[int]
    completion_tokens: Optional[int]
    total_tokens: Optional[int]
    raw_response: Optional[Dict[str, Any]]
    created_at: datetime

    class Config:
        from_attributes = True


class InvocationQuery(BaseModel):
    model_id: Optional[int] = None
    provider_id: Optional[int] = None
    model_name: Optional[str] = None
    provider_name: Optional[str] = None
    status: Optional[InvocationStatus] = None
    start_time: Optional[datetime] = None
    end_time: Optional[datetime] = None
    limit: int = Field(default=100, ge=1, le=1000)
    offset: int = Field(default=0, ge=0)
    order_by: Literal["started_at", "duration_ms", "total_tokens"] = "started_at"
    order_desc: bool = True


class ModelStatistics(BaseModel):
    model_id: int
    model_name: str
    provider_name: str
    total_calls: int
    success_calls: int
    error_calls: int
    success_rate: float
    total_tokens: int
    prompt_tokens: int
    completion_tokens: int
    avg_duration_ms: Optional[float]
    total_duration_ms: float


class TimeRangeStatistics(BaseModel):
    time_range: str  # e.g., "1h", "24h", "7d"
    total_calls: int
    success_calls: int
    error_calls: int
    success_rate: float
    total_tokens: int
    avg_duration_ms: Optional[float]


class StatisticsResponse(BaseModel):
    overall: TimeRangeStatistics
    by_model: List[ModelStatistics]
    recent_errors: List[InvocationRead]


class TimeSeriesDataPoint(BaseModel):
    timestamp: datetime
    total_calls: int
    success_calls: int
    error_calls: int
    total_tokens: int


class TimeSeriesResponse(BaseModel):
    granularity: Literal["hour", "day", "week", "month"]
    data: List[TimeSeriesDataPoint]


class APIKeyCreate(BaseModel):
    key: str
    name: Optional[str] = None
    is_active: bool = True
    allowed_models: Optional[List[str]] = None
    allowed_providers: Optional[List[str]] = None
    parameter_limits: Optional[ParameterLimits] = None


class APIKeyUpdate(BaseModel):
    name: Optional[str] = None
    is_active: Optional[bool] = None
    allowed_models: Optional[List[str]] = None
    allowed_providers: Optional[List[str]] = None
    parameter_limits: Optional[ParameterLimits] = None


class APIKeyRead(BaseModel):
    id: int
    key: str
    name: Optional[str]
    is_active: bool
    allowed_models: Optional[List[str]]
    allowed_providers: Optional[List[str]]
    parameter_limits: Optional[ParameterLimits] = None
    created_at: datetime
    updated_at: datetime

    @classmethod
    def model_validate(cls, obj, **kwargs):
        """自定义 model_validate 以处理 parameter_limits 的转换"""
        if hasattr(obj, "__dict__"):
            # SQLAlchemy 模型对象
            data = {
                "id": obj.id,
                "key": obj.key,
                "name": obj.name,
                "is_active": obj.is_active,
                "allowed_models": obj.allowed_models,
                "allowed_providers": obj.allowed_providers,
                "created_at": obj.created_at,
                "updated_at": obj.updated_at,
            }
            if obj.parameter_limits:
                data["parameter_limits"] = ParameterLimits(**obj.parameter_limits)
            return cls(**data)
        return super().model_validate(obj, **kwargs)

    class Config:
        from_attributes = True


# OpenAI 兼容的 Schema
class OpenAICompatibleMessage(BaseModel):
    """OpenAI 兼容的消息格式"""
    role: Literal["system", "user", "assistant", "tool", "function"]
    content: Optional[str] = None
    name: Optional[str] = None
    tool_calls: Optional[List[Dict[str, Any]]] = None
    tool_call_id: Optional[str] = None


class OpenAICompatibleChatCompletionRequest(BaseModel):
    """OpenAI 兼容的聊天完成请求"""
    model: Optional[str] = None  # 如果为 None，则从 session 中获取
    messages: List[OpenAICompatibleMessage]
    temperature: Optional[float] = None
    top_p: Optional[float] = None
    n: Optional[int] = 1
    stream: Optional[bool] = False
    stop: Optional[List[str] | str] = None
    max_tokens: Optional[int] = None
    presence_penalty: Optional[float] = None
    frequency_penalty: Optional[float] = None
    logit_bias: Optional[Dict[str, float]] = None
    user: Optional[str] = None
    # 扩展字段
    top_k: Optional[int] = None
    repetition_penalty: Optional[float] = None


class OpenAICompatibleUsage(BaseModel):
    """OpenAI 兼容的使用统计"""
    prompt_tokens: int
    completion_tokens: int
    total_tokens: int


class OpenAICompatibleChoice(BaseModel):
    """OpenAI 兼容的选择项"""
    index: int
    message: OpenAICompatibleMessage
    finish_reason: Optional[str] = None  # stop, length, tool_calls, content_filter, null


class OpenAICompatibleChatCompletionResponse(BaseModel):
    """OpenAI 兼容的聊天完成响应"""
    id: str
    object: str = "chat.completion"
    created: int
    model: str
    choices: List[OpenAICompatibleChoice]
    usage: Optional[OpenAICompatibleUsage] = None


__all__ = [
    "ProviderCreate",
    "ProviderRead",
    "RateLimitConfig",
    "ModelCreate",
    "ModelUpdate",
    "TagRead",
    "ModelRead",
    "ModelQuery",
    "ChatMessage",
    "ModelInvokeRequest",
    "ModelInvokeResponse",
    "ModelStreamChunk",
    "InvocationRead",
    "InvocationQuery",
    "ModelStatistics",
    "TimeRangeStatistics",
    "StatisticsResponse",
    "TimeSeriesDataPoint",
    "TimeSeriesResponse",
    "APIKeyCreate",
    "APIKeyUpdate",
    "APIKeyRead",
    "OpenAICompatibleMessage",
    "OpenAICompatibleChatCompletionRequest",
    "OpenAICompatibleUsage",
    "OpenAICompatibleChoice",
    "OpenAICompatibleChatCompletionResponse",
]


