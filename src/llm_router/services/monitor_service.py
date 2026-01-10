from __future__ import annotations

from datetime import datetime, timedelta
from typing import List, Optional, Tuple

from sqlalchemy import and_, case, func, select
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy.orm import selectinload

from ..db.models import InvocationStatus, Model, Provider
from ..db.monitor_models import MonitorInvocation
from sqlalchemy.ext.asyncio import async_sessionmaker
from ..schemas import (
    GroupedTimeSeriesDataPoint,
    GroupedTimeSeriesResponse,
    InvocationQuery,
    InvocationRead,
    ModelStatistics,
    StatisticsResponse,
    TimeRangeStatistics,
    TimeSeriesDataPoint,
    TimeSeriesResponse,
)
from .cache_service import CacheService


class MonitorService:
    def __init__(
        self,
        monitor_session_factory: async_sessionmaker[AsyncSession],
        cache_service: Optional[CacheService] = None,
    ):
        self.monitor_session_factory = monitor_session_factory
        self.cache_service = cache_service

    @staticmethod
    def calculate_cost(
        model: Model,
        prompt_tokens: Optional[int],
        completion_tokens: Optional[int],
    ) -> Optional[float]:
        """计算调用成本（USD）
        
        成本信息可以从以下位置获取：
        1. model.config.get("cost_per_1k_tokens")
        2. model.config.get("cost_per_1k_completion_tokens")
        
        如果两者都存在，分别计算prompt和completion的成本
        如果只有cost_per_1k_tokens，使用它计算总成本
        """
        if not prompt_tokens and not completion_tokens:
            return None
        
        config = model.config or {}
        cost_per_1k_tokens = config.get("cost_per_1k_tokens")
        cost_per_1k_completion_tokens = config.get("cost_per_1k_completion_tokens")
        
        if not cost_per_1k_tokens and not cost_per_1k_completion_tokens:
            return None
        
        cost = 0.0
        
        # 如果有completion tokens的单独定价
        if cost_per_1k_completion_tokens and completion_tokens:
            cost += (completion_tokens / 1000.0) * cost_per_1k_completion_tokens
            # 如果有prompt tokens的单独定价
            if cost_per_1k_tokens and prompt_tokens:
                cost += (prompt_tokens / 1000.0) * cost_per_1k_tokens
        # 否则使用统一的价格
        elif cost_per_1k_tokens:
            total = (prompt_tokens or 0) + (completion_tokens or 0)
            if total > 0:
                cost = (total / 1000.0) * cost_per_1k_tokens
        
        return round(cost, 6) if cost > 0 else None

    async def record_invocation(
        self,
        session: AsyncSession,  # 主数据库会话（用于获取model和provider information）
        model: Model,
        provider: Provider,
        started_at: datetime,
        completed_at: Optional[datetime],
        status: InvocationStatus,
        request_prompt: Optional[str] = None,
        request_messages: Optional[List[dict]] = None,
        request_parameters: Optional[dict] = None,
        response_text: Optional[str] = None,
        error_message: Optional[str] = None,
        prompt_tokens: Optional[int] = None,
        completion_tokens: Optional[int] = None,
        total_tokens: Optional[int] = None,
        raw_response: Optional[dict] = None,
    ) -> MonitorInvocation:
        """记录一次模型调用到独立的监控数据库"""
        duration_ms = None
        if completed_at:
            duration_ms = (completed_at - started_at).total_seconds() * 1000

        # 计算成本
        cost = self.calculate_cost(model, prompt_tokens, completion_tokens)

        # 使用独立的监控数据库会话
        async with self.monitor_session_factory() as monitor_session:
            invocation = MonitorInvocation(
                model_id=model.id,
                provider_id=provider.id,
                model_name=model.name,
                provider_name=provider.name,
                started_at=started_at,
                completed_at=completed_at,
                duration_ms=duration_ms,
                status=status,
                error_message=error_message,
                request_prompt=request_prompt,
                request_messages=request_messages,
                request_parameters=request_parameters or {},
                response_text=response_text,
                response_text_length=len(response_text) if response_text else None,
                prompt_tokens=prompt_tokens,
                completion_tokens=completion_tokens,
                total_tokens=total_tokens,
                cost=cost,
                raw_response=raw_response,
            )
            monitor_session.add(invocation)
            await monitor_session.commit()
            return invocation

    async def get_invocations(
        self, session: AsyncSession, query: InvocationQuery
    ) -> Tuple[List[InvocationRead], int]:
        """查询调用历史（从独立的监控数据库）"""
        # 尝试从缓存获取
        if self.cache_service:
            cached = await self.cache_service.get_invocations(query)
            if cached:
                return cached

        # 使用监控数据库会话
        async with self.monitor_session_factory() as monitor_session:
            stmt = select(MonitorInvocation)

            conditions = []
            if query.model_id:
                conditions.append(MonitorInvocation.model_id == query.model_id)
            if query.provider_id:
                conditions.append(MonitorInvocation.provider_id == query.provider_id)
            if query.model_name:
                conditions.append(MonitorInvocation.model_name == query.model_name)
            if query.provider_name:
                conditions.append(MonitorInvocation.provider_name == query.provider_name)
            if query.status:
                conditions.append(MonitorInvocation.status == query.status)
            if query.start_time:
                conditions.append(MonitorInvocation.started_at >= query.start_time)
            if query.end_time:
                conditions.append(MonitorInvocation.started_at <= query.end_time)

            if conditions:
                stmt = stmt.where(and_(*conditions))

            # 总数查询
            count_stmt = select(func.count()).select_from(stmt.subquery())
            total = await monitor_session.scalar(count_stmt)

            # 排序
            order_column = getattr(MonitorInvocation, query.order_by, MonitorInvocation.started_at)
            if query.order_desc:
                stmt = stmt.order_by(order_column.desc())
            else:
                stmt = stmt.order_by(order_column.asc())

            # 分页
            stmt = stmt.offset(query.offset).limit(query.limit)

            result = await monitor_session.scalars(stmt)
            invocations = result.all()

            # 转换为InvocationRead
            invocation_reads = []
            for inv in invocations:
                inv_read = InvocationRead(
                    id=inv.id,
                    model_id=inv.model_id,
                    provider_id=inv.provider_id,
                    model_name=inv.model_name,
                    provider_name=inv.provider_name,
                    started_at=inv.started_at,
                    completed_at=inv.completed_at,
                    duration_ms=inv.duration_ms,
                    status=inv.status,
                    error_message=inv.error_message,
                    request_prompt=inv.request_prompt,
                    request_messages=inv.request_messages,
                    request_parameters=inv.request_parameters,
                    response_text=inv.response_text,
                    response_text_length=inv.response_text_length,
                    prompt_tokens=inv.prompt_tokens,
                    completion_tokens=inv.completion_tokens,
                    total_tokens=inv.total_tokens,
                    cost=inv.cost,
                    raw_response=inv.raw_response,
                    created_at=inv.created_at,
                )
                invocation_reads.append(inv_read)

            result_data = (invocation_reads, total)

            # 缓存结果
            if self.cache_service:
                await self.cache_service.set_invocations(query, result_data)

            return result_data

    async def get_invocation_by_id(
        self, session: AsyncSession, invocation_id: int
    ) -> Optional[InvocationRead]:
        """根据ID获取单次调用详情（从独立的监控数据库）"""
        async with self.monitor_session_factory() as monitor_session:
            stmt = select(MonitorInvocation).where(MonitorInvocation.id == invocation_id)
            inv = await monitor_session.scalar(stmt)
            if not inv:
                return None

            return InvocationRead(
                id=inv.id,
                model_id=inv.model_id,
                provider_id=inv.provider_id,
                model_name=inv.model_name,
                provider_name=inv.provider_name,
                started_at=inv.started_at,
                completed_at=inv.completed_at,
                duration_ms=inv.duration_ms,
                status=inv.status,
                error_message=inv.error_message,
                request_prompt=inv.request_prompt,
                request_messages=inv.request_messages,
                request_parameters=inv.request_parameters,
                response_text=inv.response_text,
                response_text_length=inv.response_text_length,
                prompt_tokens=inv.prompt_tokens,
                completion_tokens=inv.completion_tokens,
                total_tokens=inv.total_tokens,
                cost=inv.cost,
                raw_response=inv.raw_response,
                created_at=inv.created_at,
            )

    async def get_statistics(
        self,
        session: AsyncSession,
        time_range_hours: int = 24,
        limit: int = 10,
    ) -> StatisticsResponse:
        """获取统计信息"""
        # 尝试从缓存获取
        if self.cache_service:
            cached = await self.cache_service.get_statistics(time_range_hours, limit)
            if cached:
                return cached

        now = datetime.utcnow()
        start_time = now - timedelta(hours=time_range_hours)

        # 总体统计
        overall_stmt = (
            select(
                func.count(ModelInvocation.id).label("total_calls"),
                func.sum(
                    case((ModelInvocation.status == InvocationStatus.SUCCESS, 1), else_=0)
                ).label("success_calls"),
                func.sum(
                    case((ModelInvocation.status == InvocationStatus.ERROR, 1), else_=0)
                ).label("error_calls"),
                func.sum(ModelInvocation.total_tokens).label("total_tokens"),
                func.avg(ModelInvocation.duration_ms).label("avg_duration_ms"),
                func.sum(ModelInvocation.cost).label("total_cost"),
            )
            .where(ModelInvocation.started_at >= start_time)
        )
        overall_result = await session.execute(overall_stmt)
        overall_row = overall_result.first()

        total_calls = overall_row.total_calls or 0
        success_calls = overall_row.success_calls or 0
        error_calls = overall_row.error_calls or 0
        success_rate = (success_calls / total_calls * 100) if total_calls > 0 else 0.0

        overall = TimeRangeStatistics(
            time_range=f"{time_range_hours}h",
            total_calls=total_calls,
            success_calls=success_calls,
            error_calls=error_calls,
            success_rate=round(success_rate, 2),
            total_tokens=overall_row.total_tokens or 0,
            avg_duration_ms=round(overall_row.avg_duration_ms, 2) if overall_row.avg_duration_ms else None,
            total_cost=round(overall_row.total_cost, 6) if overall_row.total_cost else None,
        )

        # 按模型统计
        model_stats_stmt = (
            select(
                ModelInvocation.model_id,
                Model.name.label("model_name"),
                Provider.name.label("provider_name"),
                func.count(ModelInvocation.id).label("total_calls"),
                func.sum(
                    case((ModelInvocation.status == InvocationStatus.SUCCESS, 1), else_=0)
                ).label("success_calls"),
                func.sum(
                    case((ModelInvocation.status == InvocationStatus.ERROR, 1), else_=0)
                ).label("error_calls"),
                func.sum(ModelInvocation.total_tokens).label("total_tokens"),
                func.sum(ModelInvocation.prompt_tokens).label("prompt_tokens"),
                func.sum(ModelInvocation.completion_tokens).label("completion_tokens"),
                func.avg(ModelInvocation.duration_ms).label("avg_duration_ms"),
                func.sum(ModelInvocation.duration_ms).label("total_duration_ms"),
                func.sum(ModelInvocation.cost).label("total_cost"),
            )
            .join(Model, ModelInvocation.model_id == Model.id)
            .join(Provider, ModelInvocation.provider_id == Provider.id)
            .where(ModelInvocation.started_at >= start_time)
            .group_by(ModelInvocation.model_id, Model.name, Provider.name)
            .order_by(func.count(ModelInvocation.id).desc())
            .limit(limit)
        )
        model_stats_result = await session.execute(model_stats_stmt)
        model_stats_rows = model_stats_result.all()

        by_model = []
        for row in model_stats_rows:
            model_total = row.total_calls or 0
            model_success = row.success_calls or 0
            model_success_rate = (model_success / model_total * 100) if model_total > 0 else 0.0

            by_model.append(
                ModelStatistics(
                    model_id=row.model_id,
                    model_name=row.model_name,
                    provider_name=row.provider_name,
                    total_calls=model_total,
                    success_calls=model_success,
                    error_calls=row.error_calls or 0,
                    success_rate=round(model_success_rate, 2),
                    total_tokens=row.total_tokens or 0,
                    prompt_tokens=row.prompt_tokens or 0,
                    completion_tokens=row.completion_tokens or 0,
                    avg_duration_ms=round(row.avg_duration_ms, 2) if row.avg_duration_ms else None,
                    total_duration_ms=row.total_duration_ms or 0.0,
                    total_cost=round(row.total_cost, 6) if row.total_cost else None,
                )
            )

        # 最近的错误
        error_stmt = (
            select(ModelInvocation)
            .where(
                and_(
                    ModelInvocation.status == InvocationStatus.ERROR,
                    ModelInvocation.started_at >= start_time,
                )
            )
            .order_by(ModelInvocation.started_at.desc())
            .limit(5)
            .options(
                selectinload(ModelInvocation.model),
                selectinload(ModelInvocation.provider),
            )
        )
        error_result = await session.scalars(error_stmt)
        error_invocations = error_result.all()

        recent_errors = []
        for inv in error_invocations:
            recent_errors.append(
                InvocationRead(
                    id=inv.id,
                    model_id=inv.model_id,
                    provider_id=inv.provider_id,
                    model_name=inv.model.name,
                    provider_name=inv.provider.name,
                    started_at=inv.started_at,
                    completed_at=inv.completed_at,
                    duration_ms=inv.duration_ms,
                    status=inv.status,
                    error_message=inv.error_message,
                    request_prompt=inv.request_prompt,
                    request_messages=inv.request_messages,
                    request_parameters=inv.request_parameters,
                    response_text=inv.response_text,
                    response_text_length=inv.response_text_length,
                    prompt_tokens=inv.prompt_tokens,
                    completion_tokens=inv.completion_tokens,
                    total_tokens=inv.total_tokens,
                    cost=inv.cost,
                    raw_response=inv.raw_response,
                    created_at=inv.created_at,
                )
            )

        result = StatisticsResponse(
            overall=overall,
            by_model=by_model,
            recent_errors=recent_errors,
        )

        # 缓存结果
        if self.cache_service:
            await self.cache_service.set_statistics(time_range_hours, limit, result)

        return result

    async def get_time_series(
        self,
        session: AsyncSession,
        granularity: str = "day",
        time_range_hours: int = 168,  # 默认7天
    ) -> TimeSeriesResponse:
        """获取时间序列数据

        Args:
            granularity: 聚合粒度，可选值: "hour", "day", "week", "month"
            time_range_hours: 时间范围（小时）
        """
        # 尝试从缓存获取
        if self.cache_service:
            cached = await self.cache_service.get_time_series(granularity, time_range_hours)
            if cached:
                return cached

        now = datetime.utcnow()
        start_time = now - timedelta(hours=time_range_hours)

        # 根据粒度确定时间格式字符串（SQLite strftime 格式）
        if granularity == "hour":
            # 按小时聚合: YYYY-MM-DD HH:00:00
            time_format = func.strftime("%Y-%m-%d %H:00:00", ModelInvocation.started_at)
        elif granularity == "day":
            # 按天聚合: YYYY-MM-DD 00:00:00
            time_format = func.strftime("%Y-%m-%d 00:00:00", ModelInvocation.started_at)
        elif granularity == "week":
            # 按周聚合: 简化处理，使用年份和周数
            # SQLite 中，strftime('%W', date) 返回周数（0-53，从周一开始）
            # 我们使用年份和周数来分组
            time_format = func.strftime("%Y-W%W", ModelInvocation.started_at)
        elif granularity == "month":
            # 按月聚合: YYYY-MM-01 00:00:00
            time_format = func.strftime("%Y-%m-01 00:00:00", ModelInvocation.started_at)
        else:
            raise ValueError(f"不支持的粒度: {granularity}")

        # 查询聚合数据
        stmt = (
            select(
                time_format.label("time_bucket"),
                func.count(ModelInvocation.id).label("total_calls"),
                func.sum(
                    case((ModelInvocation.status == InvocationStatus.SUCCESS, 1), else_=0)
                ).label("success_calls"),
                func.sum(
                    case((ModelInvocation.status == InvocationStatus.ERROR, 1), else_=0)
                ).label("error_calls"),
                func.sum(ModelInvocation.total_tokens).label("total_tokens"),
                func.sum(ModelInvocation.prompt_tokens).label("prompt_tokens"),
                func.sum(ModelInvocation.completion_tokens).label("completion_tokens"),
            )
            .where(ModelInvocation.started_at >= start_time)
            .group_by("time_bucket")
            .order_by("time_bucket")
        )

        result = await session.execute(stmt)
        rows = result.all()

        # 转换为 TimeSeriesDataPoint
        data_points = []
        for row in rows:
            # 解析时间字符串
            time_str = str(row.time_bucket)
            timestamp = None
            
            try:
                # 尝试不同的时间格式
                if granularity == "week":
                    # 周格式: YYYY-Www，需要转换为日期
                    # 简化处理：使用年份和周数，假设是周一
                    if "-W" in time_str:
                        year, week = time_str.split("-W")
                        # 计算该周第一天的日期（简化：使用年初 + 周数*7）
                        year_int = int(year)
                        week_int = int(week)
                        # 1月1日
                        jan1 = datetime(year_int, 1, 1)
                        # 计算到该周周一的偏移（简化处理）
                        days_offset = (week_int - 1) * 7
                        # 调整到周一（1月1日的星期几）
                        jan1_weekday = jan1.weekday()  # 0=Monday, 6=Sunday
                        days_to_monday = (7 - jan1_weekday) % 7
                        timestamp = jan1 + timedelta(days=days_offset + days_to_monday)
                    else:
                        continue
                elif ":" in time_str and time_str.count(":") == 2:
                    timestamp = datetime.strptime(time_str, "%Y-%m-%d %H:%M:%S")
                else:
                    timestamp = datetime.strptime(time_str, "%Y-%m-%d 00:00:00")
            except (ValueError, AttributeError, IndexError):
                # 如果解析失败，跳过这条记录
                continue

            if timestamp:
                data_points.append(
                    TimeSeriesDataPoint(
                        timestamp=timestamp,
                        total_calls=row.total_calls or 0,
                        success_calls=row.success_calls or 0,
                        error_calls=row.error_calls or 0,
                        total_tokens=row.total_tokens or 0,
                        prompt_tokens=row.prompt_tokens or 0,
                        completion_tokens=row.completion_tokens or 0,
                    )
                )

        result = TimeSeriesResponse(
            granularity=granularity,  # type: ignore
            data=data_points,
        )

        # 缓存结果
        if self.cache_service:
            await self.cache_service.set_time_series(granularity, time_range_hours, result)

        return result

    async def get_grouped_time_series(
        self,
        session: AsyncSession,
        group_by: str,  # "model" or "provider"
        granularity: str = "day",
        time_range_hours: int = 168,  # 默认7天
    ) -> GroupedTimeSeriesResponse:
        """获取按模型或provider分组的时间序列数据

        Args:
            group_by: 分组方式，"model" 或 "provider"
            granularity: 聚合粒度，可选值: "hour", "day", "week", "month"
            time_range_hours: 时间范围（小时）
        """
        if group_by not in ["model", "provider"]:
            raise ValueError(f"group_by 必须是 'model' 或 'provider'，当前值: {group_by}")

        # 尝试从缓存获取
        if self.cache_service:
            cached = await self.cache_service.get_grouped_time_series(group_by, granularity, time_range_hours)
            if cached:
                return cached

        now = datetime.utcnow()
        start_time = now - timedelta(hours=time_range_hours)

        # 根据粒度确定时间格式字符串（SQLite strftime 格式）
        if granularity == "hour":
            time_format = func.strftime("%Y-%m-%d %H:00:00", ModelInvocation.started_at)
        elif granularity == "day":
            time_format = func.strftime("%Y-%m-%d 00:00:00", ModelInvocation.started_at)
        elif granularity == "week":
            time_format = func.strftime("%Y-W%W", ModelInvocation.started_at)
        elif granularity == "month":
            time_format = func.strftime("%Y-%m-01 00:00:00", ModelInvocation.started_at)
        else:
            raise ValueError(f"不支持的粒度: {granularity}")

        # 根据分组方式选择字段
        if group_by == "model":
            group_column = Model.name.label("group_name")
            join_table = Model
            join_condition = ModelInvocation.model_id == Model.id
        else:  # provider
            group_column = Provider.name.label("group_name")
            join_table = Provider
            join_condition = ModelInvocation.provider_id == Provider.id

        # 查询聚合数据
        stmt = (
            select(
                time_format.label("time_bucket"),
                group_column,
                func.count(ModelInvocation.id).label("total_calls"),
                func.sum(
                    case((ModelInvocation.status == InvocationStatus.SUCCESS, 1), else_=0)
                ).label("success_calls"),
                func.sum(
                    case((ModelInvocation.status == InvocationStatus.ERROR, 1), else_=0)
                ).label("error_calls"),
                func.sum(ModelInvocation.total_tokens).label("total_tokens"),
                func.sum(ModelInvocation.prompt_tokens).label("prompt_tokens"),
                func.sum(ModelInvocation.completion_tokens).label("completion_tokens"),
            )
            .join(join_table, join_condition)
            .where(ModelInvocation.started_at >= start_time)
            .group_by("time_bucket", "group_name")
            .order_by("time_bucket", "group_name")
        )

        result = await session.execute(stmt)
        rows = result.all()

        # 转换为 GroupedTimeSeriesDataPoint
        data_points = []
        for row in rows:
            time_str = str(row.time_bucket)
            timestamp = None
            
            try:
                if granularity == "week":
                    if "-W" in time_str:
                        year, week = time_str.split("-W")
                        year_int = int(year)
                        week_int = int(week)
                        jan1 = datetime(year_int, 1, 1)
                        days_offset = (week_int - 1) * 7
                        jan1_weekday = jan1.weekday()
                        days_to_monday = (7 - jan1_weekday) % 7
                        timestamp = jan1 + timedelta(days=days_offset + days_to_monday)
                    else:
                        continue
                elif ":" in time_str and time_str.count(":") == 2:
                    timestamp = datetime.strptime(time_str, "%Y-%m-%d %H:%M:%S")
                else:
                    timestamp = datetime.strptime(time_str, "%Y-%m-%d 00:00:00")
            except (ValueError, AttributeError, IndexError):
                continue

            if timestamp:
                data_points.append(
                    GroupedTimeSeriesDataPoint(
                        timestamp=timestamp,
                        group_name=row.group_name,
                        total_calls=row.total_calls or 0,
                        success_calls=row.success_calls or 0,
                        error_calls=row.error_calls or 0,
                        total_tokens=row.total_tokens or 0,
                        prompt_tokens=row.prompt_tokens or 0,
                        completion_tokens=row.completion_tokens or 0,
                    )
                )

        result = GroupedTimeSeriesResponse(
            granularity=granularity,  # type: ignore
            group_by=group_by,  # type: ignore
            data=data_points,
        )

        # 缓存结果
        if self.cache_service:
            await self.cache_service.set_grouped_time_series(group_by, granularity, time_range_hours, result)

        return result


__all__ = ["MonitorService"]

