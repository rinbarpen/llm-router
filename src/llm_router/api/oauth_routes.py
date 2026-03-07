"""OAuth 路由：Provider OAuth 授权与回调"""

from __future__ import annotations

import logging

from starlette.requests import Request
from starlette.responses import JSONResponse, RedirectResponse
from starlette.status import HTTP_400_BAD_REQUEST, HTTP_404_NOT_FOUND

from ..services.oauth_service import OAUTH_SUPPORTED_PROVIDERS, OAuthService

logger = logging.getLogger(__name__)


def _get_oauth_service(request: Request) -> OAuthService:
    service = getattr(request.app.state, "oauth_service", None)
    if service is None:
        raise RuntimeError("OAuthService 尚未初始化")
    return service


async def oauth_authorize(request: Request):
    """
    GET /auth/oauth/{provider}/authorize
    查询参数: provider_name, callback_url (前端回调地址，如 https://app.example.com/)
    """
    provider = request.path_params.get("provider", "").lower()
    if provider not in OAUTH_SUPPORTED_PROVIDERS:
        return JSONResponse(
            {"detail": f"OAuth 不支持该 Provider: {provider}"},
            status_code=HTTP_400_BAD_REQUEST,
        )
    provider_name = request.query_params.get("provider_name") or provider
    callback_url = request.query_params.get("callback_url", "/")
    if not callback_url or callback_url == "/":
        base = str(request.base_url).rstrip("/")
        callback_url = base

    oauth_service = _get_oauth_service(request)
    backend_base = str(request.base_url).rstrip("/")
    backend_callback_url = f"{backend_base}/auth/oauth/{provider}/callback"

    result = oauth_service.get_authorize_url(
        provider_type=provider,
        provider_name=provider_name,
        frontend_callback_url=callback_url,
        backend_callback_url=backend_callback_url,
    )
    await oauth_service.store_oauth_state(
        state=result.state,
        provider_name=provider_name,
        frontend_callback_url=callback_url,
        code_verifier=result.code_verifier,
        backend_callback_url=backend_callback_url,
    )
    return JSONResponse({"url": result.url})


async def oauth_callback(request: Request):
    """
    GET /auth/oauth/{provider}/callback
    查询参数: code, state (由 OAuth 提供商重定向带回)
    """
    provider = request.path_params.get("provider", "").lower()
    if provider not in OAUTH_SUPPORTED_PROVIDERS:
        return JSONResponse(
            {"detail": f"OAuth 不支持该 Provider: {provider}"},
            status_code=HTTP_400_BAD_REQUEST,
        )
    code = request.query_params.get("code")
    state = request.query_params.get("state")
    if not code or not state:
        return JSONResponse(
            {"detail": "缺少 code 或 state 参数"},
            status_code=HTTP_400_BAD_REQUEST,
        )

    oauth_service = _get_oauth_service(request)
    try:
        provider_name, redirect_url = await oauth_service.handle_callback(
            provider_type=provider,
            code=code,
            state=state,
        )
        return RedirectResponse(url=redirect_url, status_code=302)
    except ValueError as e:
        logger.warning("OAuth callback failed: %s", e)
        return JSONResponse(
            {"detail": str(e)},
            status_code=HTTP_400_BAD_REQUEST,
        )


async def oauth_status(request: Request):
    """
    GET /auth/oauth/{provider}/status?provider_name=xxx
    查询指定 Provider 是否已绑定 OAuth 凭证
    """
    provider = request.path_params.get("provider", "").lower()
    if provider not in OAUTH_SUPPORTED_PROVIDERS:
        return JSONResponse(
            {"detail": f"OAuth 不支持该 Provider: {provider}"},
            status_code=HTTP_400_BAD_REQUEST,
        )
    provider_name = request.query_params.get("provider_name") or provider

    oauth_service = _get_oauth_service(request)
    session_factory = getattr(request.app.state, "session_factory", None)
    if not session_factory:
        return JSONResponse(
            {"detail": "Session 未初始化"},
            status_code=HTTP_400_BAD_REQUEST,
        )

    from sqlalchemy.ext.asyncio import AsyncSession

    async with session_factory() as session:
        has_cred = await oauth_service.has_oauth_credential(session, provider_name)
    return JSONResponse({"provider_name": provider_name, "has_oauth": has_cred})


async def oauth_revoke(request: Request):
    """
    POST /auth/oauth/{provider}/revoke
    请求体: {"provider_name": "xxx"}
    解除 OAuth 绑定
    """
    provider = request.path_params.get("provider", "").lower()
    if provider not in OAUTH_SUPPORTED_PROVIDERS:
        return JSONResponse(
            {"detail": f"OAuth 不支持该 Provider: {provider}"},
            status_code=HTTP_400_BAD_REQUEST,
        )

    try:
        body = await request.json() if request.headers.get("content-type", "").startswith("application/json") else {}
    except Exception:
        body = {}
    provider_name = body.get("provider_name") or provider

    oauth_service = _get_oauth_service(request)
    session_factory = getattr(request.app.state, "session_factory", None)
    if not session_factory:
        return JSONResponse(
            {"detail": "Session 未初始化"},
            status_code=HTTP_400_BAD_REQUEST,
        )

    async with session_factory() as session:
        revoked = await oauth_service.revoke_oauth_credential(session, provider_name)
        if revoked:
            await session.commit()
    return JSONResponse({"provider_name": provider_name, "revoked": revoked})
