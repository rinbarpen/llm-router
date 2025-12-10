from __future__ import annotations

import secrets
import time
from typing import Dict, Optional

from ..api_key_config import APIKeyConfig


class SessionStore:
    """简单的内存 Session 存储
    
    注意：在生产环境中，建议使用 Redis 等持久化存储
    """
    
    def __init__(self, default_ttl: int = 3600 * 24) -> None:
        """初始化 Session 存储
        
        Args:
            default_ttl: 默认 session 过期时间（秒），默认 24 小时
        """
        self._sessions: Dict[str, SessionData] = {}
        self.default_ttl = default_ttl
    
    def create_session(
        self, 
        api_key_config: APIKeyConfig,
        provider_name: Optional[str] = None,
        model_name: Optional[str] = None,
    ) -> str:
        """创建新的 session
        
        Args:
            api_key_config: API Key 配置
            provider_name: 可选的 provider 名称
            model_name: 可选的 model 名称
            
        Returns:
            session token
        """
        token = secrets.token_urlsafe(32)
        session_data = SessionData(
            api_key_config=api_key_config,
            created_at=time.time(),
            expires_at=time.time() + self.default_ttl,
            provider_name=provider_name,
            model_name=model_name,
        )
        self._sessions[token] = session_data
        return token
    
    def get_session(self, token: str) -> Optional[SessionData]:
        """获取 session 数据
        
        Args:
            token: session token
            
        Returns:
            SessionData，如果 session 不存在或已过期则返回 None
        """
        session_data = self._sessions.get(token)
        if session_data is None:
            return None
        
        # 检查是否过期
        if time.time() > session_data.expires_at:
            del self._sessions[token]
            return None
        
        return session_data
    
    def delete_session(self, token: str) -> bool:
        """删除 session
        
        Args:
            token: session token
            
        Returns:
            是否成功删除
        """
        if token in self._sessions:
            del self._sessions[token]
            return True
        return False
    
    def cleanup_expired(self) -> int:
        """清理过期的 session
        
        Returns:
            清理的 session 数量
        """
        now = time.time()
        expired_tokens = [
            token for token, session_data in self._sessions.items()
            if now > session_data.expires_at
        ]
        for token in expired_tokens:
            del self._sessions[token]
        return len(expired_tokens)
    
    def extend_session(self, token: str, ttl: Optional[int] = None) -> bool:
        """延长 session 过期时间
        
        Args:
            token: session token
            ttl: 新的过期时间（秒），如果为 None 则使用默认值
            
        Returns:
            是否成功延长
        """
        session_data = self._sessions.get(token)
        if session_data is None:
            return False
        
        ttl = ttl or self.default_ttl
        session_data.expires_at = time.time() + ttl
        return True
    
    def bind_model(self, token: str, provider_name: str, model_name: str) -> bool:
        """绑定模型到 session
        
        Args:
            token: session token
            provider_name: provider 名称
            model_name: model 名称
            
        Returns:
            是否成功绑定
        """
        session_data = self._sessions.get(token)
        if session_data is None:
            return False
        
        # 检查是否过期
        if time.time() > session_data.expires_at:
            del self._sessions[token]
            return False
        
        session_data.provider_name = provider_name
        session_data.model_name = model_name
        return True


class SessionData:
    """Session 数据"""
    
    def __init__(
        self,
        api_key_config: APIKeyConfig,
        created_at: float,
        expires_at: float,
        provider_name: Optional[str] = None,
        model_name: Optional[str] = None,
    ) -> None:
        self.api_key_config = api_key_config
        self.created_at = created_at
        self.expires_at = expires_at
        self.provider_name = provider_name
        self.model_name = model_name


# 全局 session 存储实例
_session_store: Optional[SessionStore] = None


def get_session_store() -> SessionStore:
    """获取全局 session 存储实例"""
    global _session_store
    if _session_store is None:
        _session_store = SessionStore()
    return _session_store


__all__ = ["SessionStore", "get_session_store"]

