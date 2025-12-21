from __future__ import annotations

from collections.abc import AsyncIterator
from typing import Callable

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession
from starlette.applications import Starlette
from starlette.middleware import Middleware
from starlette.routing import Route

from ..config import RouterSettings, load_settings
from ..db import create_engine, create_session_factory, init_db
from ..db.models import RateLimit
from ..model_config import apply_model_config, load_model_config
from ..providers import ProviderRegistry
from ..services import (
    APIKeyService,
    ModelDownloader,
    ModelService,
    MonitorService,
    RateLimiterManager,
    RouterEngine,
)
from . import routes
from .auth import APIKeyAuthMiddleware


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


async def lifespan(app: Starlette) -> AsyncIterator[None]:
    settings: RouterSettings = load_settings()
    engine = create_engine(settings.database_url)
    await init_db(engine)

    session_factory = create_session_factory(engine)
    downloader = ModelDownloader(settings)
    rate_limiter = RateLimiterManager()
    model_service = ModelService(downloader, rate_limiter)
    api_key_service = APIKeyService()
    provider_registry = ProviderRegistry(settings)
    monitor_service = MonitorService()
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
    if settings.model_config_file and settings.model_config_file.exists():
        config_data = load_model_config(settings.model_config_file)
        await apply_model_config(config_data, model_service, session_factory)
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
                        import logging
                        logger = logging.getLogger(__name__)
                        logger.warning(
                            f"API Key '{api_key_cfg.name or 'unnamed'}' 无法解析："
                            f"key 和 key_env 都未提供有效值"
                        )
                
                # 重新加载数据库中的 API Key
                api_key_configs = await api_key_service.load_all_active_keys(session)
                settings.api_keys.clear()
                settings.api_keys.extend(api_key_configs)

    app.state.settings = settings
    app.state.engine = engine
    app.state.session_factory = session_factory
    app.state.model_service = model_service
    app.state.api_key_service = api_key_service
    app.state.router_engine = router_engine
    app.state.rate_limiter = rate_limiter
    app.state.provider_registry = provider_registry
    app.state.monitor_service = monitor_service

    try:
        yield
    finally:
        await provider_registry.aclose()
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
            Route("/auth/bind-model", routes.bind_model, methods=["POST"]),
            Route("/auth/logout", routes.logout, methods=["POST"]),
            # OpenAI 兼容 API
            Route("/models", routes.get_models, methods=["GET"]),
            Route("/models/{provider_name:str}", routes.get_provider_models, methods=["GET"]),
            Route(
                "/models/{provider_name:str}/{model_name:path}/v1/chat/completions",
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
            # Monitor routes
            Route("/monitor/invocations", routes.get_invocations, methods=["GET"]),
            Route(
                "/monitor/invocations/{id:int}",
                routes.get_invocation_by_id,
                methods=["GET"],
            ),
            Route("/monitor/statistics", routes.get_statistics, methods=["GET"]),
            Route("/monitor/time-series", routes.get_time_series, methods=["GET"]),
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


