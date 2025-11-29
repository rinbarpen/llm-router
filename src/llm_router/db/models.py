from __future__ import annotations

import enum
from datetime import datetime
from typing import Any, Dict, List, Optional

from sqlalchemy import (
    JSON,
    Boolean,
    DateTime,
    Enum,
    Float,
    ForeignKey,
    Integer,
    String,
    Text,
    UniqueConstraint,
)
from sqlalchemy.orm import Mapped, mapped_column, relationship

from .base import Base


class ProviderType(str, enum.Enum):
    REMOTE_HTTP = "remote_http"
    TRANSFORMERS = "transformers"
    OLLAMA = "ollama"
    VLLM = "vllm"
    CUSTOM_HTTP = "custom_http"
    OPENAI = "openai"
    GEMINI = "gemini"
    CLAUDE = "claude"
    GROK = "grok"
    DEEPSEEK = "deepseek"
    QWEN = "qwen"
    KIMI = "kimi"
    GLM = "glm"
    OPENROUTER = "openrouter"


class InvocationStatus(str, enum.Enum):
    SUCCESS = "success"
    ERROR = "error"


class Provider(Base):
    __tablename__ = "providers"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    name: Mapped[str] = mapped_column(String(255), unique=True, nullable=False)
    type: Mapped[ProviderType] = mapped_column(Enum(ProviderType), nullable=False)
    is_active: Mapped[bool] = mapped_column(Boolean, default=True, nullable=False)
    base_url: Mapped[Optional[str]] = mapped_column(String(512), nullable=True)
    api_key: Mapped[Optional[str]] = mapped_column(String(512), nullable=True)
    settings: Mapped[Dict[str, Any]] = mapped_column(JSON, default=dict, nullable=False)
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=datetime.utcnow, nullable=False
    )
    updated_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True),
        default=datetime.utcnow,
        onupdate=datetime.utcnow,
        nullable=False,
    )

    models: Mapped[List["Model"]] = relationship(
        back_populates="provider", cascade="all, delete-orphan"
    )


class Model(Base):
    __tablename__ = "models"
    __table_args__ = (
        UniqueConstraint("provider_id", "name", name="uq_models_provider_name"),
    )

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    provider_id: Mapped[int] = mapped_column(
        ForeignKey("providers.id", ondelete="CASCADE"), nullable=False
    )
    name: Mapped[str] = mapped_column(String(255), nullable=False)
    display_name: Mapped[Optional[str]] = mapped_column(String(255))
    description: Mapped[Optional[str]] = mapped_column(String(1024))
    is_active: Mapped[bool] = mapped_column(Boolean, default=True, nullable=False)
    remote_identifier: Mapped[Optional[str]] = mapped_column(String(255))
    default_params: Mapped[Dict[str, Any]] = mapped_column(JSON, default=dict, nullable=False)
    config: Mapped[Dict[str, Any]] = mapped_column(JSON, default=dict, nullable=False)
    download_uri: Mapped[Optional[str]] = mapped_column(String(1024))
    local_path: Mapped[Optional[str]] = mapped_column(String(1024))
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=datetime.utcnow, nullable=False
    )
    updated_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True),
        default=datetime.utcnow,
        onupdate=datetime.utcnow,
        nullable=False,
    )

    provider: Mapped["Provider"] = relationship(back_populates="models")
    tags: Mapped[List["Tag"]] = relationship(
        secondary="model_tags", back_populates="models", lazy="selectin"
    )
    rate_limit: Mapped[Optional["RateLimit"]] = relationship(
        back_populates="model", uselist=False, cascade="all, delete-orphan"
    )


class Tag(Base):
    __tablename__ = "tags"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    name: Mapped[str] = mapped_column(String(128), unique=True, nullable=False)
    description: Mapped[Optional[str]] = mapped_column(String(512))

    models: Mapped[List["Model"]] = relationship(
        secondary="model_tags", back_populates="tags", lazy="selectin"
    )


class ModelTag(Base):
    __tablename__ = "model_tags"
    __table_args__ = (
        UniqueConstraint("model_id", "tag_id", name="uq_model_tag"),
    )

    model_id: Mapped[int] = mapped_column(
        ForeignKey("models.id", ondelete="CASCADE"), primary_key=True
    )
    tag_id: Mapped[int] = mapped_column(
        ForeignKey("tags.id", ondelete="CASCADE"), primary_key=True
    )


class RateLimit(Base):
    __tablename__ = "rate_limits"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    model_id: Mapped[int] = mapped_column(
        ForeignKey("models.id", ondelete="CASCADE"), unique=True, nullable=False
    )
    max_requests: Mapped[int] = mapped_column(Integer, nullable=False)
    per_seconds: Mapped[int] = mapped_column(Integer, nullable=False)
    burst_size: Mapped[Optional[int]] = mapped_column(Integer)
    notes: Mapped[Optional[str]] = mapped_column(String(512))
    config: Mapped[Dict[str, Any]] = mapped_column(JSON, default=dict, nullable=False)

    model: Mapped["Model"] = relationship(back_populates="rate_limit")


class ModelInvocation(Base):
    __tablename__ = "model_invocations"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    model_id: Mapped[int] = mapped_column(
        ForeignKey("models.id", ondelete="CASCADE"), nullable=False, index=True
    )
    provider_id: Mapped[int] = mapped_column(
        ForeignKey("providers.id", ondelete="CASCADE"), nullable=False, index=True
    )
    
    # 时间信息
    started_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), nullable=False, index=True
    )
    completed_at: Mapped[Optional[datetime]] = mapped_column(
        DateTime(timezone=True), nullable=True
    )
    duration_ms: Mapped[Optional[float]] = mapped_column(Float, nullable=True)
    
    # 状态信息
    status: Mapped[InvocationStatus] = mapped_column(
        Enum(InvocationStatus), nullable=False, index=True
    )
    error_message: Mapped[Optional[str]] = mapped_column(Text, nullable=True)
    
    # 请求信息
    request_prompt: Mapped[Optional[str]] = mapped_column(Text, nullable=True)
    request_messages: Mapped[Optional[List[Dict[str, Any]]]] = mapped_column(
        JSON, nullable=True
    )
    request_parameters: Mapped[Dict[str, Any]] = mapped_column(
        JSON, default=dict, nullable=False
    )
    
    # 响应信息
    response_text: Mapped[Optional[str]] = mapped_column(Text, nullable=True)
    response_text_length: Mapped[Optional[int]] = mapped_column(Integer, nullable=True)
    
    # Token使用信息
    prompt_tokens: Mapped[Optional[int]] = mapped_column(Integer, nullable=True)
    completion_tokens: Mapped[Optional[int]] = mapped_column(Integer, nullable=True)
    total_tokens: Mapped[Optional[int]] = mapped_column(Integer, nullable=True)
    
    # 原始响应数据（用于调试）
    raw_response: Mapped[Optional[Dict[str, Any]]] = mapped_column(JSON, nullable=True)
    
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=datetime.utcnow, nullable=False
    )
    
    # 关系
    model: Mapped["Model"] = relationship()
    provider: Mapped["Provider"] = relationship()


class APIKey(Base):
    __tablename__ = "api_keys"
    __table_args__ = (
        UniqueConstraint("key", name="uq_api_key"),
    )

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    key: Mapped[str] = mapped_column(String(512), unique=True, nullable=False, index=True)
    name: Mapped[Optional[str]] = mapped_column(String(255), nullable=True)
    is_active: Mapped[bool] = mapped_column(Boolean, default=True, nullable=False, index=True)
    
    # 模型和 Provider 限制（JSON 数组）
    allowed_models: Mapped[Optional[List[str]]] = mapped_column(JSON, nullable=True)
    allowed_providers: Mapped[Optional[List[str]]] = mapped_column(JSON, nullable=True)
    
    # 参数限制（JSON 对象）
    parameter_limits: Mapped[Optional[Dict[str, Any]]] = mapped_column(JSON, nullable=True)
    
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=datetime.utcnow, nullable=False
    )
    updated_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True),
        default=datetime.utcnow,
        onupdate=datetime.utcnow,
        nullable=False,
    )


