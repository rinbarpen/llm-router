from __future__ import annotations

from datetime import datetime
from typing import Any, Dict, List, Literal, Optional, Union

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


class ProviderUpdate(BaseModel):
    type: Optional[ProviderType] = None
    base_url: Optional[str] = None
    api_key: Optional[str] = None
    is_active: Optional[bool] = None
    settings: Optional[Dict[str, Any]] = None


class ProviderRead(BaseModel):
    id: int
    name: str
    type: ProviderType
    is_active: bool
    base_url: Optional[str] = None
    api_key: Optional[str] = None

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
    name: Optional[str] = None
    tags: List[str] = Field(default_factory=list)
    provider_types: List[ProviderType] = Field(default_factory=list)
    include_inactive: bool = False


class ChatMessage(BaseModel):
    """聊天消息，content 支持纯文本或多模态格式（OpenAI 风格列表）"""
    role: Literal["system", "user", "assistant"]
    content: Union[str, List[Dict[str, Any]]]  # 字符串或 [{type, text}, {type, image_url}]


class ModelInvokeRequest(BaseModel):
    prompt: Optional[str] = None
    messages: Optional[List[ChatMessage]] = None
    parameters: Dict[str, Any] = Field(default_factory=dict)
    stream: bool = False
    remote_identifier_override: Optional[str] = None  # 用于 OpenAI 兼容 API，覆盖数据库中的 remote_identifier
    batch: Optional[List[ModelInvokeRequest]] = None  # 批量请求

    @model_validator(mode="after")
    def _validate_input(self) -> "ModelInvokeRequest":
        if not self.prompt and not self.messages and not self.batch:
            raise ValueError("prompt, messages 或 batch 至少需要提供一个")
        return self


class ModelInvokeResponse(BaseModel):
    output_text: str
    raw: Dict[str, Any] = Field(default_factory=dict)
    cost: Optional[float] = None  # 成本（USD）
    batch: Optional[List[ModelInvokeResponse]] = None  # 批量响应结果


class BatchModelInvokeResponse(BaseModel):
    """批量调用的聚合响应格式"""
    responses: List[ModelInvokeResponse]
    total_cost: Optional[float] = None


class ModelStreamChunk(BaseModel):
    """Provider 流式输出的统一结构。"""

    delta: Dict[str, Any] = Field(default_factory=dict)
    text: Optional[str] = None
    raw: Optional[Dict[str, Any]] = None
    usage: Optional[Dict[str, Any]] = None
    cost: Optional[float] = None  # 成本（USD）
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
    cost: Optional[float]  # 成本（USD）
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
    total_cost: Optional[float] = None  # 总成本（USD）


class ProviderStatistics(BaseModel):
    provider_id: int
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
    total_cost: Optional[float] = None  # 总成本（USD）


class TimeRangeStatistics(BaseModel):
    time_range: str  # e.g., "1h", "24h", "7d"
    total_calls: int
    success_calls: int
    error_calls: int
    success_rate: float
    total_tokens: int
    avg_duration_ms: Optional[float]
    total_cost: Optional[float] = None  # 总成本（USD）


class StatisticsResponse(BaseModel):
    overall: TimeRangeStatistics
    by_model: List[ModelStatistics]
    by_provider: List[ProviderStatistics]
    recent_errors: List[InvocationRead]


class TimeSeriesDataPoint(BaseModel):
    timestamp: datetime
    total_calls: int
    success_calls: int
    error_calls: int
    total_tokens: int
    prompt_tokens: Optional[int] = 0
    completion_tokens: Optional[int] = 0
    total_cost: Optional[float] = None  # 总成本（USD）


class TimeSeriesResponse(BaseModel):
    granularity: Literal["hour", "day", "week", "month"]
    data: List[TimeSeriesDataPoint]


class GroupedTimeSeriesDataPoint(BaseModel):
    timestamp: datetime
    group_name: str  # 模型名称或provider名称
    total_calls: int
    success_calls: int
    error_calls: int
    total_tokens: int
    prompt_tokens: Optional[int] = 0
    completion_tokens: Optional[int] = 0
    total_cost: Optional[float] = None  # 总成本（USD）


class GroupedTimeSeriesResponse(BaseModel):
    granularity: Literal["hour", "day", "week", "month"]
    group_by: Literal["model", "provider"]
    data: List[GroupedTimeSeriesDataPoint]


# 定价相关Schema
class ModelPricingInfo(BaseModel):
    """模型定价信息"""
    model_name: str
    provider: str
    input_price_per_1k: float  # 每1k输入token价格（USD）
    output_price_per_1k: float  # 每1k输出token价格（USD）
    source: str  # 数据来源
    last_updated: datetime
    notes: Optional[str] = None


class PricingSuggestion(BaseModel):
    """定价更新建议"""
    model_id: int
    model_name: str
    provider_name: str
    current_input_price: Optional[float] = None
    current_output_price: Optional[float] = None
    latest_input_price: Optional[float] = None
    latest_output_price: Optional[float] = None
    has_update: bool = False
    pricing_info: Optional[ModelPricingInfo] = None


class PricingSyncRequest(BaseModel):
    """定价同步请求"""
    model_id: int
    auto_confirm: bool = False  # 是否自动确认更新


class PricingSyncResponse(BaseModel):
    """定价同步响应"""
    success: bool
    message: str
    updated_pricing: Optional[ModelPricingInfo] = None


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


class RouteDecisionRequest(BaseModel):
    """轻量路由决策请求：仅返回模型与调用参数，不执行推理。"""

    model: Optional[str] = None
    role: Optional[str] = None
    task: Optional[str] = None
    trace_id: Optional[str] = None
    model_hint: Optional[str] = None  # deprecated: use model
    routing_mode: Optional[Literal["auto", "strong", "weak", "stronge"]] = None
    prompt: Optional[str] = None
    messages: Optional[List[ChatMessage]] = None
    temperature: Optional[float] = None
    max_tokens: Optional[int] = None

    @field_validator("routing_mode")
    @classmethod
    def _normalize_routing_mode(cls, value: Optional[str]) -> Optional[str]:
        if value == "stronge":
            return "strong"
        return value


class RouteDecisionResponse(BaseModel):
    """轻量路由决策响应，供上游客户端直接实例化 LLM 调用器。"""

    model: str
    base_url: Optional[str] = None
    temperature: float = 0.0
    max_tokens: Optional[int] = None
    provider: str


class OpenAICompatibleMessage(BaseModel):
    """OpenAI 兼容的消息格式，content 支持多模态 [{type:text, text}, {type:image_url, image_url:{url}}]"""
    role: Literal["system", "user", "assistant", "tool", "function"]
    content: Optional[Union[str, List[Dict[str, Any]]]] = None
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
    routing_mode: Optional[Literal["auto", "strong", "weak", "stronge"]] = None
    # 扩展字段
    top_k: Optional[int] = None
    repetition_penalty: Optional[float] = None

    @field_validator("routing_mode")
    @classmethod
    def _normalize_routing_mode(cls, value: Optional[str]) -> Optional[str]:
        if value == "stronge":
            return "strong"
        return value


class OpenAIResponsesRequest(BaseModel):
    """OpenAI Responses API 请求（兼容子集）"""

    model: Optional[str] = None
    input: Any
    stream: Optional[bool] = False
    temperature: Optional[float] = None
    top_p: Optional[float] = None
    max_output_tokens: Optional[int] = None
    instructions: Optional[str] = None
    user: Optional[str] = None
    tools: Optional[List[Dict[str, Any]]] = None
    tool_choice: Optional[Any] = None
    metadata: Optional[Dict[str, Any]] = None
    extra_body: Optional[Dict[str, Any]] = None
    routing_mode: Optional[Literal["auto", "strong", "weak", "stronge"]] = None

    @field_validator("routing_mode")
    @classmethod
    def _normalize_responses_routing_mode(cls, value: Optional[str]) -> Optional[str]:
        if value == "stronge":
            return "strong"
        return value


class ClaudeCountTokensRequest(BaseModel):
    model: str
    messages: List[Dict[str, Any]]
    system: Optional[str] = None


class ClaudeCountTokensResponse(BaseModel):
    input_tokens: int


ModelCapability = Literal[
    "embedding",
    "tts",
    "asr",
    "realtime",
    "image_generation",
    "video_generation",
]


class OpenAIEmbeddingsRequest(BaseModel):
    model: str
    input: Union[str, List[str], List[int], List[List[int]]]
    encoding_format: Optional[Literal["float", "base64"]] = None
    dimensions: Optional[int] = None
    user: Optional[str] = None


class OpenAIAudioSpeechRequest(BaseModel):
    model: str
    input: str
    voice: Optional[str] = None          # instruct 模型可通过 instructions 指定音色，故为可选
    response_format: Optional[str] = "mp3"
    speed: Optional[float] = None
    instructions: Optional[str] = None   # 用于 instruct 模型的指令控制


class OpenAIAudioTranscriptionRequest(BaseModel):
    model: str
    prompt: Optional[str] = None
    response_format: Optional[str] = None
    temperature: Optional[float] = None
    language: Optional[str] = None


class OpenAIAudioTranslationRequest(BaseModel):
    model: str
    prompt: Optional[str] = None
    response_format: Optional[str] = None
    temperature: Optional[float] = None


class OpenAIImagesGenerationsRequest(BaseModel):
    model: str
    prompt: str
    n: Optional[int] = 1
    size: Optional[str] = None
    quality: Optional[str] = None
    response_format: Optional[Literal["url", "b64_json"]] = "url"
    style: Optional[str] = None
    user: Optional[str] = None


class OpenAIVideosGenerationsRequest(BaseModel):
    model: str
    prompt: str
    size: Optional[str] = None
    duration: Optional[int] = None
    fps: Optional[int] = None
    response_format: Optional[Literal["url", "b64_json"]] = "url"
    user: Optional[str] = None


class BindModelRequest(BaseModel):
    provider_name: str
    model_name: str
    binding_type: Literal["default", "strong", "weak"] = "default"


class OpenAICompatibleUsage(BaseModel):
    """OpenAI 兼容的使用统计"""
    prompt_tokens: int
    completion_tokens: int
    total_tokens: int
    cost: Optional[float] = None  # 成本（USD）


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


class OpenAIChatCompletionRequest(BaseModel):
    model: str
    messages: List[ChatMessage]
    temperature: Optional[float] = None
    top_p: Optional[float] = None
    n: Optional[int] = None
    stream: Optional[bool] = False
    stop: Optional[Any] = None
    max_tokens: Optional[int] = None
    presence_penalty: Optional[float] = None
    frequency_penalty: Optional[float] = None
    logit_bias: Optional[Dict[str, float]] = None
    user: Optional[str] = None
    # 允许额外参数透传
    extra_body: Optional[Dict[str, Any]] = None

    def to_model_invoke_request(self) -> ModelInvokeRequest:
        parameters = {}
        # 提取标准参数
        for field in [
            "temperature",
            "top_p",
            "n",
            "stop",
            "max_tokens",
            "presence_penalty",
            "frequency_penalty",
            "logit_bias",
            "user",
        ]:
            val = getattr(self, field)
            if val is not None:
                parameters[field] = val

        # 合并额外参数
        if self.extra_body:
            parameters.update(self.extra_body)

        return ModelInvokeRequest(
            messages=self.messages,
            parameters=parameters,
            stream=self.stream or False,
        )


class OpenAIChatCompletionResponse(BaseModel):
    id: str
    object: str = "chat.completion"
    created: int
    model: str
    choices: List[Dict[str, Any]]
    usage: Optional[Dict[str, int]] = None


class OpenAIModelInfo(BaseModel):
    id: str
    object: str = "model"
    created: int = 0
    owned_by: str = "llm-router"


class OpenAIModelList(BaseModel):
    object: str = "list"
    data: List[OpenAIModelInfo]


__all__ = [
    "ProviderCreate",
    "ProviderRead",
    "RateLimitConfig",
    "GroupedTimeSeriesDataPoint",
    "GroupedTimeSeriesResponse",
    "ModelCreate",
    "ModelUpdate",
    "TagRead",
    "ModelRead",
    "ModelQuery",
    "ChatMessage",
    "ModelInvokeRequest",
    "ModelInvokeResponse",
    "BatchModelInvokeResponse",
    "ModelStreamChunk",
    "InvocationRead",
    "InvocationQuery",
    "ModelStatistics",
    "ProviderStatistics",
    "TimeRangeStatistics",
    "StatisticsResponse",
    "TimeSeriesDataPoint",
    "TimeSeriesResponse",
    "GroupedTimeSeriesDataPoint",
    "GroupedTimeSeriesResponse",
    "APIKeyCreate",
    "APIKeyUpdate",
    "APIKeyRead",
    "BindModelRequest",
    "RouteDecisionRequest",
    "RouteDecisionResponse",
    "OpenAICompatibleMessage",
    "OpenAICompatibleChatCompletionRequest",
    "OpenAIResponsesRequest",
    "ClaudeCountTokensRequest",
    "ClaudeCountTokensResponse",
    "ModelCapability",
    "OpenAIEmbeddingsRequest",
    "OpenAIAudioSpeechRequest",
    "OpenAIAudioTranscriptionRequest",
    "OpenAIAudioTranslationRequest",
    "OpenAIImagesGenerationsRequest",
    "OpenAIVideosGenerationsRequest",
    "OpenAICompatibleUsage",
    "OpenAICompatibleChoice",
    "OpenAICompatibleChatCompletionResponse",
    "OpenAIChatCompletionRequest",
    "OpenAIChatCompletionResponse",
    "OpenAIModelInfo",
    "OpenAIModelList",
    "ModelPricingInfo",
    "PricingSuggestion",
    "PricingSyncRequest",
    "PricingSyncResponse",
]
