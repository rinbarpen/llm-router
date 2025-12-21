from __future__ import annotations

import asyncio
import time
from typing import Dict, Iterable, Optional

from ..db.models import RateLimit as RateLimitModel
from ..schemas import RateLimitConfig


class TokenBucket:
    def __init__(self, config: RateLimitConfig) -> None:
        self.max_requests = config.max_requests
        self.per_seconds = config.per_seconds
        self.burst_size = config.burst_size or config.max_requests
        self._refill_rate = self.max_requests / self.per_seconds
        self._tokens = float(self.burst_size)
        self._last_refill = time.monotonic()
        self._lock = asyncio.Lock()

    def _refill(self) -> None:
        now = time.monotonic()
        elapsed = now - self._last_refill
        self._last_refill = now
        refill = elapsed * self._refill_rate
        if refill > 0:
            self._tokens = min(self.burst_size, self._tokens + refill)

    async def acquire(self, tokens: int = 1) -> None:
        if tokens <= 0:
            return

        while True:
            async with self._lock:
                self._refill()
                if self._tokens >= tokens:
                    self._tokens -= tokens
                    return
                deficit = tokens - self._tokens
                wait_time = deficit / self._refill_rate if self._refill_rate > 0 else self.per_seconds
            await asyncio.sleep(wait_time)


class RateLimiterManager:
    def __init__(self) -> None:
        self._buckets: Dict[int, TokenBucket] = {}
        self._lock = asyncio.Lock()

    def upsert(self, model_id: int, config: RateLimitConfig) -> None:
        self._buckets[model_id] = TokenBucket(config)

    def remove(self, model_id: int) -> None:
        self._buckets.pop(model_id, None)

    def load_from_records(self, records: Iterable[RateLimitModel]) -> None:
        for record in records:
            config = RateLimitConfig(
                max_requests=record.max_requests,
                per_seconds=record.per_seconds,
                burst_size=record.burst_size,
                notes=record.notes,
                config=record.config,
            )
            self.upsert(record.model_id, config)

    async def acquire(self, model_id: int, tokens: int = 1) -> None:
        bucket = self._buckets.get(model_id)
        if not bucket:
            return
        await bucket.acquire(tokens)

    def get_bucket(self, model_id: int) -> Optional[TokenBucket]:
        return self._buckets.get(model_id)


