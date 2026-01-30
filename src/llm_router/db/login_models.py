"""登录记录数据模型"""

from __future__ import annotations

import uuid
from datetime import datetime
from typing import Optional

from pydantic import BaseModel, Field


class LoginRecord(BaseModel):
    """登录记录（基础信息）"""

    id: str = Field(default_factory=lambda: str(uuid.uuid4()), description="记录 ID")
    timestamp: datetime = Field(default_factory=datetime.utcnow, description="时间戳")
    ip_address: str = Field(..., description="客户端 IP")
    auth_type: str = Field(..., description="认证方式：api_key / session_token / none")
    is_success: bool = Field(..., description="是否成功")
    api_key_id: Optional[int] = Field(default=None, description="API Key ID（成功时）")
    session_token_hash: Optional[str] = Field(default=None, description="Session Token 哈希（不存完整 token）")
    is_local: bool = Field(default=False, description="是否本地请求")
