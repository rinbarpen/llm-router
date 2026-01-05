"""独立的监控数据库模型 - 不依赖主数据库的外键关系"""

from __future__ import annotations

import enum
from datetime import datetime
from typing import Any, Dict, List, Optional

from sqlalchemy import (
    JSON,
    DateTime,
    Enum,
    Float,
    Integer,
    String,
    Text,
)
from sqlalchemy.orm import Mapped, mapped_column

from .base import Base


class InvocationStatus(str, enum.Enum):
    SUCCESS = "success"
    ERROR = "error"


class MonitorInvocation(Base):
    """监控数据库中的调用记录 - 独立于主数据库"""
    __tablename__ = "monitor_invocations"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    
    # 模型和Provider信息（存储为字符串，不依赖外键）
    model_id: Mapped[int] = mapped_column(Integer, nullable=False, index=True)
    provider_id: Mapped[int] = mapped_column(Integer, nullable=False, index=True)
    model_name: Mapped[str] = mapped_column(String(255), nullable=False, index=True)
    provider_name: Mapped[str] = mapped_column(String(255), nullable=False, index=True)
    
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
    
    # 成本信息（USD）
    cost: Mapped[Optional[float]] = mapped_column(Float, nullable=True)
    
    # 原始响应数据（用于调试）
    raw_response: Mapped[Optional[Dict[str, Any]]] = mapped_column(JSON, nullable=True)
    
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=datetime.utcnow, nullable=False
    )

