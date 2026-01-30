"""Redis 客户端 - 用于存储登录记录等数据"""

from __future__ import annotations

from typing import Optional

from redis.asyncio import Redis, from_url

from ..config import load_settings

_redis_client: Optional[Redis] = None


async def get_redis() -> Redis:
    """获取全局 Redis 客户端（单例，懒初始化）"""
    global _redis_client
    if _redis_client is None:
        redis_url = load_settings().redis_url
        _redis_client = from_url(redis_url, decode_responses=True)
    return _redis_client


async def close_redis() -> None:
    """关闭 Redis 连接"""
    global _redis_client
    if _redis_client is not None:
        await _redis_client.close()
        _redis_client = None


__all__ = ["get_redis", "close_redis"]
