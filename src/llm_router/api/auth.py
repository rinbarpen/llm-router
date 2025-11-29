from __future__ import annotations

from typing import Callable

from starlette.exceptions import HTTPException
from starlette.requests import Request
from starlette.responses import Response
from starlette.status import HTTP_401_UNAUTHORIZED, HTTP_403_FORBIDDEN

from ..api_key_config import APIKeyConfig
from ..config import RouterSettings


# 不需要认证的端点
PUBLIC_ENDPOINTS = {
    "/health",
}


def extract_api_key(request: Request) -> str | None:
    """从请求中提取 API Key，支持多种方式：
    1. Authorization: Bearer <key>
    2. X-API-Key 头
    3. api_key 查询参数
    """
    # 方式1: Authorization Bearer
    auth_header = request.headers.get("Authorization", "")
    if auth_header.startswith("Bearer "):
        return auth_header[7:].strip()

    # 方式2: X-API-Key 头
    api_key_header = request.headers.get("X-API-Key")
    if api_key_header:
        return api_key_header.strip()

    # 方式3: 查询参数
    api_key_param = request.query_params.get("api_key")
    if api_key_param:
        return api_key_param.strip()

    return None


class APIKeyAuthMiddleware:
    """API Key 认证中间件，支持模型和参数限制"""

    def __init__(self, app: Callable, settings: RouterSettings) -> None:
        self.app = app
        self.settings = settings

    async def __call__(self, scope, receive, send) -> None:
        if scope["type"] != "http":
            await self.app(scope, receive, send)
            return

        request = Request(scope, receive)

        # 检查是否为公开端点
        if request.url.path in PUBLIC_ENDPOINTS:
            await self.app(scope, receive, send)
            return

        # 如果未启用认证，直接通过
        if not self.settings.require_auth or not self.settings.has_api_keys():
            await self.app(scope, receive, send)
            return

        # 提取并验证 API Key
        api_key = extract_api_key(request)
        if api_key is None:
            response = Response(
                content='{"detail":"缺少 API Key。请通过 Authorization: Bearer <key>、X-API-Key 头或 api_key 查询参数提供。"}',
                status_code=HTTP_401_UNAUTHORIZED,
                media_type="application/json",
            )
            await response(scope, receive, send)
            return

        api_key_config = self.settings.get_api_key_config(api_key)
        if api_key_config is None:
            response = Response(
                content='{"detail":"无效的 API Key"}',
                status_code=HTTP_403_FORBIDDEN,
                media_type="application/json",
            )
            await response(scope, receive, send)
            return

        # 将 API Key 配置存储到 request.state 中，供路由使用
        request.state.api_key_config = api_key_config

        # 继续处理请求
        await self.app(scope, receive, send)


__all__ = ["APIKeyAuthMiddleware", "extract_api_key"]

