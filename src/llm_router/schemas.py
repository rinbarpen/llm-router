from __future__ import annotations

from typing import Any, Dict, List, Literal, Optional

from pydantic import BaseModel, Field, ValidationInfo, field_validator, model_validator

from .db.models import ProviderType


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

    @model_validator(mode="after")
    def _validate_input(self) -> "ModelInvokeRequest":
        if not self.prompt and not self.messages:
            raise ValueError("prompt 或 messages 至少需要提供一个")
        return self


class ModelInvokeResponse(BaseModel):
    output_text: str
    raw: Dict[str, Any] = Field(default_factory=dict)


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
]


