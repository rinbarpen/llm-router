"""CLI 会话映射：Router session/conversation_id -> Codex thread_id / Claude session_id"""

from __future__ import annotations

import time
from dataclasses import dataclass, field
from typing import Dict, Optional

from ..db.models import ProviderType


@dataclass
class CliSessionInfo:
    """CLI 会话信息"""

    cli_id: str  # Codex thread_id 或 Claude session_id
    provider_type: ProviderType
    created_at: float = field(default_factory=time.time)
    token_count: int = 0  # 粗略估算，用于判断是否超限


class CliConversationStore:
    """Router 会话键 -> CLI 会话映射

    键：(provider_type, conversation_key)
    conversation_key: session_token（登录态）或 conversation_id（请求体）或临时生成的 ID
    """

    def __init__(self, default_ttl: int = 3600 * 24) -> None:
        self._store: Dict[tuple[str, str], CliSessionInfo] = {}
        self.default_ttl = default_ttl

    def _key(self, provider_type: ProviderType, conversation_key: str) -> tuple[str, str]:
        return (provider_type.value, conversation_key)

    def get(
        self, provider_type: ProviderType, conversation_key: str
    ) -> Optional[CliSessionInfo]:
        """获取已有 CLI 会话"""
        info = self._store.get(self._key(provider_type, conversation_key))
        if info is None:
            return None
        return info

    def set(
        self,
        provider_type: ProviderType,
        conversation_key: str,
        cli_id: str,
        token_count: int = 0,
    ) -> None:
        """设置或更新 CLI 会话"""
        key = self._key(provider_type, conversation_key)
        self._store[key] = CliSessionInfo(
            cli_id=cli_id,
            provider_type=provider_type,
            created_at=time.time(),
            token_count=token_count,
        )

    def delete(self, provider_type: ProviderType, conversation_key: str) -> bool:
        """删除会话映射（上下文超限时调用）"""
        key = self._key(provider_type, conversation_key)
        if key in self._store:
            del self._store[key]
            return True
        return False

    def update_token_count(
        self, provider_type: ProviderType, conversation_key: str, token_count: int
    ) -> bool:
        """更新 token 计数"""
        info = self.get(provider_type, conversation_key)
        if info is None:
            return False
        info.token_count = token_count
        return True

    def cleanup_by_conversation_key(self, conversation_key: str) -> int:
        """按 conversation_key 清理（登出时调用）"""
        removed = 0
        keys_to_remove = [k for k in self._store if k[1] == conversation_key]
        for k in keys_to_remove:
            del self._store[k]
            removed += 1
        return removed


# 全局单例
_cli_conversation_store: Optional[CliConversationStore] = None


def get_cli_conversation_store() -> CliConversationStore:
    """获取全局 CliConversationStore 实例"""
    global _cli_conversation_store
    if _cli_conversation_store is None:
        _cli_conversation_store = CliConversationStore()
    return _cli_conversation_store


__all__ = ["CliConversationStore", "CliSessionInfo", "get_cli_conversation_store"]
