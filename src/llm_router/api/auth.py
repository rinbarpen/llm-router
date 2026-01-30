from __future__ import annotations

import asyncio
import ipaddress
from typing import Callable

from starlette.exceptions import HTTPException
from starlette.requests import Request
from starlette.responses import Response
from starlette.status import HTTP_401_UNAUTHORIZED, HTTP_403_FORBIDDEN

from ..api_key_config import APIKeyConfig
from ..config import RouterSettings
from ..db.login_models import LoginRecord
from ..services.login_record_service import get_login_record_service
from .session_store import get_session_store


# 不需要认证的端点
PUBLIC_ENDPOINTS = {
    "/health",
    "/auth/login",  # 登录端点不需要认证
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


def extract_session_token(request: Request) -> str | None:
    """从请求中提取 Session Token，支持多种方式：
    1. Authorization: Bearer <token>（优先）
    2. X-Session-Token 头
    3. session_token 查询参数
    """
    # 方式1: Authorization Bearer（优先，因为这是标准方式）
    auth_header = request.headers.get("Authorization", "")
    if auth_header.startswith("Bearer "):
        return auth_header[7:].strip()

    # 方式2: X-Session-Token 头
    session_header = request.headers.get("X-Session-Token")
    if session_header:
        return session_header.strip()

    # 方式3: 查询参数
    session_param = request.query_params.get("session_token")
    if session_param:
        return session_param.strip()

    return None


def is_local_request(request: Request) -> bool:
    """检查请求是否来自本机
    
    支持以下情况：
    - 127.0.0.1 (IPv4 loopback)
    - ::1 (IPv6 loopback)
    - localhost
    - 如果客户端 IP 为空（某些情况下可能发生）
    
    Args:
        request: Starlette 请求对象
        
    Returns:
        如果是本机请求返回 True，否则返回 False
    """
    client_host = request.client.host if request.client else None
    
    if not client_host:
        # 如果无法获取客户端 IP，默认认为是本地请求（更宽松的策略）
        return True
    
    # 检查是否为 localhost
    if client_host.lower() in ("localhost", "127.0.0.1", "::1"):
        return True
    
    # 检查是否为 IPv4/IPv6 loopback
    try:
        ip = ipaddress.ip_address(client_host)
        if ip.is_loopback:
            return True
    except ValueError:
        # 如果不是有效的 IP 地址，可能是域名，继续检查
        pass
    
    # 检查是否为私有网络地址（可选，如果需要允许内网访问）
    # 这里只允许真正的本机，如果需要允许内网，可以取消注释以下代码
    # try:
    #     ip = ipaddress.ip_address(client_host)
    #     if ip.is_private:
    #         return True
    # except ValueError:
    #     pass
    
    return False


class APIKeyAuthMiddleware:
    """API Key 认证中间件，支持模型和参数限制"""

    def __init__(self, app: Callable, settings: RouterSettings) -> None:
        self.app = app
        self.settings = settings

    async def _record_login(
        self,
        ip_address: str,
        is_local: bool,
        auth_type: str,
        is_success: bool,
        api_key_id: int | None = None,
    ) -> None:
        """记录登录事件到 Redis（不阻塞请求）"""
        try:
            from datetime import datetime

            record = LoginRecord(
                timestamp=datetime.utcnow(),
                ip_address=ip_address,
                auth_type=auth_type,
                is_success=is_success,
                api_key_id=api_key_id,
                is_local=is_local,
            )
            service = get_login_record_service()
            await service.create_login_record(record)
        except Exception:
            pass

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

        # 检查是否为本地请求：仅当 allow_local_without_auth 为 True 时本机免认证
        if is_local_request(request) and self.settings.allow_local_without_auth:
            # 本地请求跳过认证，但如果有提供认证信息，仍然可以设置 api_key_config
            # 这样本地请求也可以使用 API Key 的限制功能（如果需要）
            session_token = extract_session_token(request)
            api_key_config = None
            session_data = None
            
            if session_token:
                session_store = get_session_store()
                session_data = session_store.get_session(session_token)
                if session_data:
                    api_key_config = session_data.api_key_config
                    request.state.session_data = session_data
            
            if api_key_config is None:
                api_key = extract_api_key(request)
                if api_key:
                    api_key_config = self.settings.get_api_key_config(api_key)
            
            # 如果有有效的认证信息，设置 api_key_config（用于权限限制）
            if api_key_config:
                request.state.api_key_config = api_key_config

            # 记录本地免认证通过
            ip_address = request.client.host if request.client else "unknown"
            asyncio.create_task(
                self._record_login(ip_address, True, "none", True)
            )
            await self.app(scope, receive, send)
            return

        # 远程请求需要认证
        # 优先尝试使用 Session Token（登录后请求）
        session_token = extract_session_token(request)
        api_key_config = None
        session_data = None
        
        if session_token:
            session_store = get_session_store()
            session_data = session_store.get_session(session_token)
            if session_data:
                api_key_config = session_data.api_key_config
                request.state.session_data = session_data
        
        # 如果 Session Token 无效或不存在，回退到直接使用 API Key（向后兼容）
        ip_address = request.client.host if request.client else "unknown"
        is_local = is_local_request(request)

        if api_key_config is None:
            api_key = extract_api_key(request)
            if api_key is None:
                asyncio.create_task(
                    self._record_login(ip_address, is_local, "none", False)
                )
                response = Response(
                    content='{"detail":"未认证。请先通过 /auth/login 登录获取 Session Token，或使用 API Key 进行认证。"}',
                    status_code=HTTP_401_UNAUTHORIZED,
                    media_type="application/json",
                )
                await response(scope, receive, send)
                return

            api_key_config = self.settings.get_api_key_config(api_key)
            if api_key_config is None:
                asyncio.create_task(
                    self._record_login(ip_address, is_local, "api_key", False)
                )
                response = Response(
                    content='{"detail":"无效的 API Key 或 Session Token"}',
                    status_code=HTTP_403_FORBIDDEN,
                    media_type="application/json",
                )
                await response(scope, receive, send)
                return

        # 将 API Key 配置存储到 request.state 中，供路由使用
        request.state.api_key_config = api_key_config

        # 记录认证成功（api_key 或 session_token）
        auth_type = "session_token" if session_data else "api_key"
        asyncio.create_task(
            self._record_login(ip_address, is_local, auth_type, True)
        )
        await self.app(scope, receive, send)


__all__ = ["APIKeyAuthMiddleware", "extract_api_key", "extract_session_token", "is_local_request"]

