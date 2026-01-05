from __future__ import annotations

import asyncio
import logging
from collections.abc import AsyncIterator
from pathlib import Path
from typing import Any, Callable

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession
from starlette.applications import Starlette
from starlette.middleware import Middleware
from starlette.routing import Route
from watchfiles import awatch

from ..config import RouterSettings, load_settings
from ..db import create_engine, create_session_factory, init_db
from ..db.models import RateLimit
from ..model_config import apply_model_config, load_model_config
from ..providers import ProviderRegistry
from ..services import (
    APIKeyService,
    CacheService,
    ModelDownloader,
    ModelService,
    MonitorService,
    RateLimiterManager,
    RouterEngine,
)
from . import routes
from .auth import APIKeyAuthMiddleware

logger = logging.getLogger(__name__)


class DBSessionMiddleware:
    def __init__(self, app: Callable) -> None:
        self.app = app

    async def __call__(self, scope, receive, send) -> None:  # type: ignore[override]
        if scope["type"] not in ("http", "websocket"):
            await self.app(scope, receive, send)
            return

        app_instance = scope.get("app")
        if app_instance is None:
            await self.app(scope, receive, send)
            return

        session_factory = getattr(app_instance.state, "session_factory", None)
        if session_factory is None:
            await self.app(scope, receive, send)
            return

        session: AsyncSession = session_factory()
        scope.setdefault("state", {})
        scope["state"]["session"] = session

        try:
            await self.app(scope, receive, send)
            await session.commit()
        except Exception:  # pragma: no cover - propagate error
            await session.rollback()
            raise
        finally:
            await session.close()


async def reload_config_from_file(
    config_file: Path,
    model_service: ModelService,
    api_key_service: APIKeyService,
    session_factory: Callable,
    rate_limiter: RateLimiterManager,
    settings: RouterSettings,
) -> None:
    """从配置文件重新加载配置并应用到数据库"""
    try:
        logger.info(f"检测到配置文件变化，重新加载: {config_file}")
        config_data = load_model_config(config_file)
        
        # 应用模型和 Provider 配置
        await apply_model_config(config_data, model_service, session_factory)
        
        # 刷新速率限制
        async def refresh_rate_limits() -> None:
            async with session_factory() as session:
                result = await session.scalars(select(RateLimit))
                rate_limiter.load_from_records(result.all())
        
        await refresh_rate_limits()
        
        # 从配置文件同步 API Key 到数据库
        if config_data.api_keys:
            async with session_factory() as session:
                for api_key_cfg in config_data.api_keys:
                    # 解析环境变量，支持多个 key（逗号分隔）
                    resolved_keys = api_key_cfg.resolved_keys()
                    if resolved_keys:
                        for idx, key in enumerate(resolved_keys):
                            # 检查是否已存在
                            existing = await api_key_service.get_api_key_by_key(session, key)
                            if existing:
                                # 更新现有记录
                                await api_key_service.update_api_key(
                                    session,
                                    existing,
                                    name=api_key_cfg.name if idx == 0 else f"{api_key_cfg.name or 'API Key'} #{idx + 1}",
                                    is_active=api_key_cfg.is_active,
                                    allowed_models=api_key_cfg.allowed_models,
                                    allowed_providers=api_key_cfg.allowed_providers,
                                    parameter_limits=api_key_cfg.parameter_limits,
                                )
                            else:
                                # 创建新记录
                                name = api_key_cfg.name
                                if len(resolved_keys) > 1:
                                    name = f"{api_key_cfg.name or 'API Key'} #{idx + 1}"
                                await api_key_service.create_api_key(
                                    session,
                                    key=key,
                                    name=name,
                                    is_active=api_key_cfg.is_active,
                                    allowed_models=api_key_cfg.allowed_models,
                                    allowed_providers=api_key_cfg.allowed_providers,
                                    parameter_limits=api_key_cfg.parameter_limits,
                                )
                        await session.commit()
                    else:
                        # 如果无法解析 key，记录警告
                        logger.warning(
                            f"API Key '{api_key_cfg.name or 'unnamed'}' 无法解析："
                            f"key 和 key_env 都未提供有效值"
                        )
                
                # 重新加载数据库中的 API Key
                api_key_configs = await api_key_service.load_all_active_keys(session)
                settings.api_keys.clear()
                settings.api_keys.extend(api_key_configs)
        
        logger.info("配置重新加载完成")
    except Exception as e:
        logger.error(f"重新加载配置失败: {e}", exc_info=True)


async def lifespan(app: Starlette) -> AsyncIterator[None]:
    settings: RouterSettings = load_settings()
    engine = create_engine(settings.database_url)
    await init_db(engine)

    session_factory = create_session_factory(engine)
    
    # 创建独立的监控数据库引擎
    monitor_engine = create_engine(settings.monitor_database_url)
    from ..db.monitor_models import Base as MonitorBase
    async with monitor_engine.begin() as conn:
        await conn.run_sync(MonitorBase.metadata.create_all)
    monitor_session_factory = create_session_factory(monitor_engine)
    downloader = ModelDownloader(settings)
    rate_limiter = RateLimiterManager()
    model_service = ModelService(downloader, rate_limiter)
    api_key_service = APIKeyService()
    provider_registry = ProviderRegistry(settings)
    # 创建缓存服务，用于优化数据API查询性能
    cache_service = CacheService(
        default_ttl=30,  # invocations缓存30秒
        stats_ttl=60,  # 统计数据缓存60秒
        time_series_ttl=60,  # 时间序列数据缓存60秒
    )
    # 启动缓存清理任务
    cache_cleanup_task = cache_service.start_cleanup_task(interval_seconds=60)
    # MonitorService 使用独立的监控数据库和缓存服务
    # 确保所有模型调用都会被记录到独立的监控数据库
    monitor_service = MonitorService(
        monitor_session_factory=monitor_session_factory,
        cache_service=cache_service
    )
    router_engine = RouterEngine(
        model_service, provider_registry, rate_limiter, monitor_service
    )

    async def refresh_rate_limits() -> None:
        async with session_factory() as session:
            result = await session.scalars(select(RateLimit))
            rate_limiter.load_from_records(result.all())

    await refresh_rate_limits()

    # 从数据库加载所有激活的 API Key
    async with session_factory() as session:
        api_key_configs = await api_key_service.load_all_active_keys(session)
        settings.api_keys.extend(api_key_configs)

    # 从配置文件加载并同步到数据库（如果配置了）
    config_file: Path | None = None
    if settings.model_config_file and settings.model_config_file.exists():
        config_file = settings.model_config_file
        await reload_config_from_file(
            config_file,
            model_service,
            api_key_service,
            session_factory,
            rate_limiter,
            settings,
        )
    elif not settings.model_config_file:
        # 尝试使用默认路径
        default_config_file = Path.cwd() / "router.toml"
        if default_config_file.exists():
            config_file = default_config_file
            await reload_config_from_file(
                config_file,
                model_service,
                api_key_service,
                session_factory,
                rate_limiter,
                settings,
            )

    app.state.settings = settings
    app.state.engine = engine
    app.state.session_factory = session_factory
    app.state.monitor_engine = monitor_engine
    app.state.monitor_session_factory = monitor_session_factory
    app.state.model_service = model_service
    app.state.api_key_service = api_key_service
    app.state.router_engine = router_engine
    app.state.rate_limiter = rate_limiter
    app.state.provider_registry = provider_registry
    app.state.monitor_service = monitor_service
    app.state.cache_service = cache_service

    # 启动配置文件热加载任务
    config_watch_task: asyncio.Task | None = None
    if config_file and config_file.exists():
        config_file_abs = config_file.resolve()
        config_file_str = str(config_file_abs)
        
        def watch_filter(change: Any, path: str) -> bool:
            """过滤函数，只监听目标配置文件"""
            return str(Path(path).resolve()) == config_file_str
        
        async def watch_config_file() -> None:
            """监听配置文件变化并自动重新加载"""
            try:
                async for changes in awatch(config_file.parent, watch_filter=watch_filter):
                    # changes 是一个集合，包含 (Change, path) 元组
                    # 由于使用了 watch_filter，这里只会包含目标文件的变化
                    if changes:
                        # 防抖：等待一小段时间，避免频繁触发
                        await asyncio.sleep(0.5)
                        logger.info(f"检测到配置文件变化: {config_file}")
                        await reload_config_from_file(
                            config_file,
                            model_service,
                            api_key_service,
                            session_factory,
                            rate_limiter,
                            settings,
                        )
            except asyncio.CancelledError:
                logger.info("配置文件监听任务已取消")
            except Exception as e:
                logger.error(f"配置文件监听任务出错: {e}", exc_info=True)
        
        config_watch_task = asyncio.create_task(watch_config_file())
        logger.info(f"已启动配置文件热加载监听: {config_file}")

    try:
        yield
    finally:
        # 取消配置文件监听任务
        if config_watch_task:
            config_watch_task.cancel()
            try:
                await config_watch_task
            except asyncio.CancelledError:
                pass
        # 取消缓存清理任务
        if cache_cleanup_task:
            cache_cleanup_task.cancel()
            try:
                await cache_cleanup_task
            except asyncio.CancelledError:
                pass
        await provider_registry.aclose()
        await monitor_engine.dispose()
        await engine.dispose()


def create_app() -> Starlette:
    # 预加载 settings 以便在中间件中使用
    settings = load_settings()
    
    middleware = [Middleware(DBSessionMiddleware)]
    
    # 如果启用了认证，添加 API Key 认证中间件
    if settings.require_auth and settings.has_api_keys():
        # 创建一个包装中间件，在运行时从 app.state 获取 settings
        class AuthMiddlewareWrapper:
            def __init__(self, app: Callable) -> None:
                self.app = app
                self.settings = settings
            
            async def __call__(self, scope, receive, send) -> None:
                auth_middleware = APIKeyAuthMiddleware(self.app, self.settings)
                await auth_middleware(scope, receive, send)
        
        middleware.insert(0, Middleware(AuthMiddlewareWrapper))

    app = Starlette(
        routes=[
            Route("/health", routes.health, methods=["GET"]),
            # 认证端点
            Route("/auth/login", routes.login, methods=["POST"]),
            Route("/auth/logout", routes.logout, methods=["POST"]),
            # OpenAI 兼容 API
            Route("/models", routes.get_models, methods=["GET"]),
            Route("/models/{provider_name:str}", routes.get_provider_models, methods=["GET"]),
            # 标准 OpenAI 兼容 API (model 在请求体中)
            Route(
                "/v1/chat/completions",
                routes.openai_chat_completions,
                methods=["POST"],
            ),
            # Provider 和 Model 管理
            Route("/providers", routes.create_provider, methods=["POST"]),
            Route("/providers", routes.list_providers, methods=["GET"]),
            Route("/models", routes.create_model, methods=["POST"]),
            Route(
                "/models/{provider_name:str}/{model_name:path}/invoke",
                routes.invoke_model,
                methods=["POST"],
            ),
            Route(
                "/models/{provider_name:str}/{model_name:path}",
                routes.update_model,
                methods=["PATCH"],
            ),
            Route(
                "/models/{provider_name:str}/{model_name:path}",
                routes.get_model,
                methods=["GET"],
            ),
            Route("/route/invoke", routes.route_model, methods=["POST"]),
            # Monitor export routes
            Route("/monitor/export/json", routes.export_data_json, methods=["GET"]),
            Route("/monitor/export/excel", routes.export_data_excel, methods=["GET"]),
            Route("/monitor/database", routes.download_database, methods=["GET"]),
            # API Key 管理端点
            Route("/api-keys", routes.create_api_key, methods=["POST"]),
            Route("/api-keys", routes.list_api_keys, methods=["GET"]),
            Route("/api-keys/{id:int}", routes.get_api_key, methods=["GET"]),
            Route("/api-keys/{id:int}", routes.update_api_key, methods=["PATCH"]),
            Route("/api-keys/{id:int}", routes.delete_api_key, methods=["DELETE"]),
        ],
        middleware=middleware,
        lifespan=lifespan,
    )
    return app


