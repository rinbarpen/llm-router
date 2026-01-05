"""数据缓存服务 - 用于优化monitor API查询性能"""
from __future__ import annotations

import asyncio
import logging
from datetime import datetime, timedelta
from typing import Any, Dict, List, Optional, Tuple

from ..schemas import (
    GroupedTimeSeriesDataPoint,
    GroupedTimeSeriesResponse,
    InvocationQuery,
    InvocationRead,
    ModelStatistics,
    StatisticsResponse,
    TimeSeriesDataPoint,
    TimeSeriesResponse,
)

logger = logging.getLogger(__name__)


class CacheEntry:
    """缓存条目"""

    def __init__(self, data: Any, ttl: int):
        self.data = data
        self.expires_at = datetime.utcnow() + timedelta(seconds=ttl)
        self.created_at = datetime.utcnow()

    def is_expired(self) -> bool:
        return datetime.utcnow() > self.expires_at

    def age_seconds(self) -> float:
        return (datetime.utcnow() - self.created_at).total_seconds()


class CacheService:
    """内存缓存服务"""

    def __init__(
        self,
        default_ttl: int = 30,  # 默认缓存30秒
        stats_ttl: int = 60,  # 统计数据缓存60秒
        time_series_ttl: int = 60,  # 时间序列数据缓存60秒
    ):
        self._cache: Dict[str, CacheEntry] = {}
        self._lock = asyncio.Lock()
        self.default_ttl = default_ttl
        self.stats_ttl = stats_ttl
        self.time_series_ttl = time_series_ttl

        # 缓存统计
        self._hits = 0
        self._misses = 0

    def _generate_key(self, prefix: str, **kwargs) -> str:
        """生成缓存键"""
        # 将参数转换为字符串并排序
        params = []
        for key in sorted(kwargs.keys()):
            value = kwargs[key]
            if isinstance(value, list):
                params.append(f"{key}={','.join(sorted(map(str, value)))}")
            elif isinstance(value, datetime):
                params.append(f"{key}={value.isoformat()}")
            else:
                params.append(f"{key}={value}")
        return f"{prefix}:{'|'.join(params)}"

    def _generate_invocations_key(self, query: InvocationQuery) -> str:
        """生成invocations缓存键"""
        return self._generate_key(
            "invocations",
            model_id=query.model_id,
            provider_id=query.provider_id,
            model_name=query.model_name,
            provider_name=query.provider_name,
            status=query.status.value if query.status else None,
            start_time=query.start_time,
            end_time=query.end_time,
            limit=query.limit,
            offset=query.offset,
            order_by=query.order_by,
            order_desc=query.order_desc,
        )

    def _generate_stats_key(self, time_range_hours: int, limit: int) -> str:
        """生成统计数据缓存键"""
        return self._generate_key("stats", time_range_hours=time_range_hours, limit=limit)

    def _generate_time_series_key(self, granularity: str, time_range_hours: int) -> str:
        """生成时间序列缓存键"""
        return self._generate_key(
            "timeseries", granularity=granularity, time_range_hours=time_range_hours
        )

    def _generate_grouped_time_series_key(
        self, group_by: str, granularity: str, time_range_hours: int
    ) -> str:
        """生成分组时间序列缓存键"""
        return self._generate_key(
            "grouped_timeseries",
            group_by=group_by,
            granularity=granularity,
            time_range_hours=time_range_hours,
        )

    async def get_invocations(
        self, query: InvocationQuery
    ) -> Optional[Tuple[List[InvocationRead], int]]:
        """获取缓存的invocations数据"""
        key = self._generate_invocations_key(query)
        async with self._lock:
            entry = self._cache.get(key)
            if entry and not entry.is_expired():
                self._hits += 1
                logger.debug(f"Cache HIT for invocations (age: {entry.age_seconds():.1f}s)")
                return entry.data
            self._misses += 1
            return None

    async def set_invocations(
        self, query: InvocationQuery, data: Tuple[List[InvocationRead], int]
    ) -> None:
        """缓存invocations数据"""
        key = self._generate_invocations_key(query)
        async with self._lock:
            self._cache[key] = CacheEntry(data, self.default_ttl)

    async def get_statistics(
        self, time_range_hours: int, limit: int
    ) -> Optional[StatisticsResponse]:
        """获取缓存的统计数据"""
        key = self._generate_stats_key(time_range_hours, limit)
        async with self._lock:
            entry = self._cache.get(key)
            if entry and not entry.is_expired():
                self._hits += 1
                logger.debug(f"Cache HIT for statistics (age: {entry.age_seconds():.1f}s)")
                return entry.data
            self._misses += 1
            return None

    async def set_statistics(
        self, time_range_hours: int, limit: int, data: StatisticsResponse
    ) -> None:
        """缓存统计数据"""
        key = self._generate_stats_key(time_range_hours, limit)
        async with self._lock:
            self._cache[key] = CacheEntry(data, self.stats_ttl)

    async def get_time_series(
        self, granularity: str, time_range_hours: int
    ) -> Optional[TimeSeriesResponse]:
        """获取缓存的时间序列数据"""
        key = self._generate_time_series_key(granularity, time_range_hours)
        async with self._lock:
            entry = self._cache.get(key)
            if entry and not entry.is_expired():
                self._hits += 1
                logger.debug(f"Cache HIT for time_series (age: {entry.age_seconds():.1f}s)")
                return entry.data
            self._misses += 1
            return None

    async def set_time_series(
        self, granularity: str, time_range_hours: int, data: TimeSeriesResponse
    ) -> None:
        """缓存时间序列数据"""
        key = self._generate_time_series_key(granularity, time_range_hours)
        async with self._lock:
            self._cache[key] = CacheEntry(data, self.time_series_ttl)

    async def get_grouped_time_series(
        self, group_by: str, granularity: str, time_range_hours: int
    ) -> Optional[GroupedTimeSeriesResponse]:
        """获取缓存的分组时间序列数据"""
        key = self._generate_grouped_time_series_key(group_by, granularity, time_range_hours)
        async with self._lock:
            entry = self._cache.get(key)
            if entry and not entry.is_expired():
                self._hits += 1
                logger.debug(f"Cache HIT for grouped_time_series (age: {entry.age_seconds():.1f}s)")
                return entry.data
            self._misses += 1
            return None

    async def set_grouped_time_series(
        self,
        group_by: str,
        granularity: str,
        time_range_hours: int,
        data: GroupedTimeSeriesResponse,
    ) -> None:
        """缓存分组时间序列数据"""
        key = self._generate_grouped_time_series_key(group_by, granularity, time_range_hours)
        async with self._lock:
            self._cache[key] = CacheEntry(data, self.time_series_ttl)

    async def invalidate_all(self) -> None:
        """使所有缓存失效"""
        async with self._lock:
            self._cache.clear()
            logger.info("All cache invalidated")

    async def invalidate_expired(self) -> None:
        """清理过期缓存"""
        async with self._lock:
            expired_keys = [k for k, v in self._cache.items() if v.is_expired()]
            for key in expired_keys:
                del self._cache[key]
            if expired_keys:
                logger.debug(f"Cleaned up {len(expired_keys)} expired cache entries")

    async def get_stats(self) -> Dict[str, Any]:
        """获取缓存统计信息"""
        async with self._lock:
            total_requests = self._hits + self._misses
            hit_rate = (self._hits / total_requests * 100) if total_requests > 0 else 0
            return {
                "hits": self._hits,
                "misses": self._misses,
                "hit_rate": round(hit_rate, 2),
                "total_entries": len(self._cache),
                "entries": [
                    {
                        "key": k,
                        "age_seconds": v.age_seconds(),
                        "expires_in_seconds": (v.expires_at - datetime.utcnow()).total_seconds(),
                    }
                    for k, v in self._cache.items()
                ],
            }

    def start_cleanup_task(self, interval_seconds: int = 60) -> asyncio.Task:
        """启动定期清理任务"""
        async def cleanup():
            while True:
                try:
                    await asyncio.sleep(interval_seconds)
                    await self.invalidate_expired()
                except asyncio.CancelledError:
                    logger.info("Cache cleanup task cancelled")
                    break
                except Exception as e:
                    logger.error(f"Cache cleanup error: {e}", exc_info=True)

        task = asyncio.create_task(cleanup())
        task.add_done_callback(lambda t: logger.debug("Cache cleanup task stopped"))
        return task


__all__ = ["CacheService"]

