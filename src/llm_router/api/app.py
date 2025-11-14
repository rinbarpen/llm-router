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
from ..services import ModelDownloader, ModelService, RateLimiterManager, RouterEngine
from . import routes


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
    provider_registry = ProviderRegistry(settings)
    router_engine = RouterEngine(model_service, provider_registry, rate_limiter)

    async def refresh_rate_limits() -> None:
        async with session_factory() as session:
            result = await session.scalars(select(RateLimit))
            rate_limiter.load_from_records(result.all())

    await refresh_rate_limits()

    if settings.model_config_file and settings.model_config_file.exists():
        config_data = load_model_config(settings.model_config_file)
        await apply_model_config(config_data, model_service, session_factory)
        await refresh_rate_limits()

    app.state.settings = settings
    app.state.engine = engine
    app.state.session_factory = session_factory
    app.state.model_service = model_service
    app.state.router_engine = router_engine
    app.state.rate_limiter = rate_limiter
    app.state.provider_registry = provider_registry

    try:
        yield
    finally:
        await engine.dispose()


def create_app() -> Starlette:
    middleware = [Middleware(DBSessionMiddleware)]

    app = Starlette(
        routes=[
            Route("/health", routes.health, methods=["GET"]),
            Route("/providers", routes.create_provider, methods=["POST"]),
            Route("/providers", routes.list_providers, methods=["GET"]),
            Route("/models", routes.create_model, methods=["POST"]),
            Route("/models", routes.get_models, methods=["GET"]),
            Route(
                "/models/{provider_name:str}/{model_name:str}",
                routes.update_model,
                methods=["PATCH"],
            ),
            Route(
                "/models/{provider_name:str}/{model_name:str}/invoke",
                routes.invoke_model,
                methods=["POST"],
            ),
            Route("/route/invoke", routes.route_model, methods=["POST"]),
        ],
        middleware=middleware,
        lifespan=lifespan,
    )
    return app


