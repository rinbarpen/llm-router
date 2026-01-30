"""登录记录服务 - 使用 Redis 存储"""

from __future__ import annotations

import json
import logging
from datetime import datetime
from typing import List, Optional

from ..db.login_models import LoginRecord
from ..db.redis_client import get_redis

logger = logging.getLogger(__name__)

LOGIN_RECORDS_KEY = "login_records"
MAX_RECORDS = 10000


class LoginRecordService:
    """登录记录服务"""

    async def create_login_record(self, record: LoginRecord) -> None:
        """将登录记录写入 Redis List（左侧插入，保留最近 N 条）"""
        try:
            redis = await get_redis()
            data = record.model_dump(mode="json")
            value = json.dumps(data, ensure_ascii=False)
            await redis.lpush(LOGIN_RECORDS_KEY, value)
            await redis.ltrim(LOGIN_RECORDS_KEY, 0, MAX_RECORDS - 1)
        except Exception as e:
            logger.warning("写入登录记录失败: %s", e)

    async def get_login_records(
        self,
        limit: int = 100,
        offset: int = 0,
        auth_type: Optional[str] = None,
        is_success: Optional[bool] = None,
    ) -> tuple[List[LoginRecord], int]:
        """查询登录记录，支持分页与筛选。返回 (records, total)。"""
        try:
            redis = await get_redis()
            raw_list = await redis.lrange(LOGIN_RECORDS_KEY, 0, -1)
        except Exception as e:
            logger.warning("读取登录记录失败: %s", e)
            return [], 0

        records: List[LoginRecord] = []
        for raw in raw_list:
            try:
                data = json.loads(raw)
                if isinstance(data.get("timestamp"), str):
                    data["timestamp"] = datetime.fromisoformat(
                        data["timestamp"].replace("Z", "+00:00")
                    )
                rec = LoginRecord.model_validate(data)
                if auth_type is not None and rec.auth_type != auth_type:
                    continue
                if is_success is not None and rec.is_success != is_success:
                    continue
                records.append(rec)
            except (json.JSONDecodeError, Exception):
                continue

        total = len(records)
        page = records[offset : offset + limit]
        return page, total


_login_record_service: Optional[LoginRecordService] = None


def get_login_record_service() -> LoginRecordService:
    """获取全局登录记录服务实例"""
    global _login_record_service
    if _login_record_service is None:
        _login_record_service = LoginRecordService()
    return _login_record_service
