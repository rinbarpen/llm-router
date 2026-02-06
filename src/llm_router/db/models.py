from __future__ import annotations

import enum
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional

from sqlalchemy import (
    JSON,
    Boolean,
    Column,
    DateTime,
    Enum,
    Float,
    ForeignKey,
    Integer,
    String,
    Table,
    Text,
    UniqueConstraint,
)
from sqlalchemy.orm import Mapped, mapped_column, relationship

from .base import Base

# Model <-> Tag 多对多关联表
model_tags = Table(
    "model_tags",
    Base.metadata,
    Column("model_id", ForeignKey("models.id", ondelete="CASCADE"), primary_key=True),
    Column("tag_id", ForeignKey("api_keys.id", ondelete="CASCADE"), primary_key=True),
)


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
        DateTime(timezone=True), default=lambda: datetime.now(timezone.utc), nullable=False
    )
    updated_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True),
        default=lambda: datetime.now(timezone.utc),
        onupdate=lambda: datetime.now(timezone.utc),
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
        DateTime(timezone=True), default=lambda: datetime.now(timezone.utc), nullable=False
    )
    updated_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True),
        default=lambda: datetime.now(timezone.utc),
        onupdate=lambda: datetime.now(timezone.utc),
        nullable=False,
    )

    provider: Mapped["Provider"] = relationship(back_populates="models")
    tags: Mapped[List["Tag"]] = relationship(
        secondary=model_tags, back_populates="models", lazy="selectin"
    )
    rate_limit: Mapped[Optional["RateLimit"]] = relationship(
        back_populates="model", uselist=False, cascade="all, delete-orphan"
    )


class RateLimit(Base):
    __tablename__ = "rate_limits"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    model_id: Mapped[int] = mapped_column(
        ForeignKey("models.id", ondelete="CASCADE"), unique=True, nullable=False
    )
    max_requests: Mapped[int] = mapped_column(Integer, nullable=False)
    per_seconds: Mapped[int] = mapped_column(Integer, nullable=False)
    burst_size: Mapped[Optional[int]] = mapped_column(Integer, nullable=True)
    notes: Mapped[Optional[str]] = mapped_column(String(512), nullable=True)
    config: Mapped[Dict[str, Any]] = mapped_column(JSON, default=dict, nullable=False)

    model: Mapped["Model"] = relationship(back_populates="rate_limit")


class Tag(Base):
    __tablename__ = "api_keys"
    __table_args__ = (
        UniqueConstraint("key", name="uq_api_key"),
    )

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    # key 为 None 时表示模型标签（仅用 name）；有值时表示 API Key
    key: Mapped[Optional[str]] = mapped_column(
        String(512), unique=True, nullable=True, index=True
    )
    name: Mapped[Optional[str]] = mapped_column(String(255), nullable=True)
    is_active: Mapped[bool] = mapped_column(Boolean, default=True, nullable=False, index=True)
    
    # 模型和 Provider 限制（JSON 数组）
    allowed_models: Mapped[Optional[List[str]]] = mapped_column(JSON, nullable=True)
    allowed_providers: Mapped[Optional[List[str]]] = mapped_column(JSON, nullable=True)
    
    # 参数限制（JSON 对象）
    parameter_limits: Mapped[Optional[Dict[str, Any]]] = mapped_column(JSON, nullable=True)
    
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=lambda: datetime.now(timezone.utc), nullable=False
    )
    updated_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True),
        default=lambda: datetime.now(timezone.utc),
        onupdate=lambda: datetime.now(timezone.utc),
        nullable=False,
    )

    models: Mapped[List["Model"]] = relationship(
        back_populates="tags", secondary=model_tags
    )


# 表 api_keys 的 ORM 类，别名为 APIKey 供 APIKeyService 等使用
APIKey = Tag

