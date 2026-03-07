"""OAuth 服务：支持 OpenRouter、Gemini 等 Provider 的 OAuth 登录与凭证持久化。"""

from __future__ import annotations

import base64
import hashlib
import json
import logging
import os
import secrets
from abc import ABC, abstractmethod
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Any, Optional
from urllib.parse import urlencode

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from ..db.models import Provider, ProviderOAuthCredential
from ..db.redis_client import get_redis

logger = logging.getLogger(__name__)
OAUTH_STATE_PREFIX = "llm_router:oauth:state:"
OAUTH_STATE_TTL = 600


def _get_fernet():
    """获取 Fernet 实例用于加密存储。若未配置 LLM_ROUTER_OAUTH_SECRET 则返回 None（不加密）。"""
    secret = os.getenv("LLM_ROUTER_OAUTH_SECRET")
    if not secret or len(secret) < 32:
        return None
    from cryptography.fernet import Fernet
    from cryptography.hazmat.primitives import hashes
    from cryptography.hazmat.primitives.kdf.pbkdf2 import PBKDF2HMAC

    kdf = PBKDF2HMAC(
        algorithm=hashes.SHA256(),
        length=32,
        salt=b"llm_router_oauth",
        iterations=100000,
    )
    key = base64.urlsafe_b64encode(kdf.derive(secret.encode()))
    return Fernet(key)


def _encrypt(plain: str | None) -> str | None:
    if plain is None:
        return None
    f = _get_fernet()
    if f is None:
        return plain
    return f.encrypt(plain.encode()).decode()


def _decrypt(cipher: str | None) -> str | None:
    if cipher is None:
        return None
    f = _get_fernet()
    if f is None:
        return cipher
    try:
        return f.decrypt(cipher.encode()).decode()
    except Exception:
        return None


def _pkce_code_verifier() -> str:
    return secrets.token_urlsafe(32)


def _pkce_code_challenge(verifier: str) -> str:
    digest = hashlib.sha256(verifier.encode()).digest()
    return base64.urlsafe_b64encode(digest).rstrip(b"=").decode()


@dataclass
class OAuthAuthorizeResult:
    url: str
    state: str
    code_verifier: str | None = None


@dataclass
class OAuthExchangeResult:
    api_key: str | None = None
    access_token: str | None = None
    refresh_token: str | None = None
    expires_at: datetime | None = None


class BaseOAuthHandler(ABC):
    """OAuth 处理器基类"""

    provider_type: str

    @abstractmethod
    def get_authorize_url(
        self,
        callback_url: str,
        state: str,
        provider_name: str,
        code_verifier: str | None = None,
    ) -> OAuthAuthorizeResult:
        """生成 OAuth 授权 URL"""
        ...

    @abstractmethod
    async def exchange_code(
        self,
        code: str,
        state: str,
        code_verifier: str | None = None,
        redirect_uri: str | None = None,
    ) -> OAuthExchangeResult:
        """用授权码换取 token/API key。redirect_uri 为 OAuth 回调完整 URL（Gemini 需要）。"""
        ...

    async def refresh_token(self, refresh_token: str) -> OAuthExchangeResult:
        """刷新 access_token（Gemini 等需要）"""
        return OAuthExchangeResult()


class OpenRouterOAuthHandler(BaseOAuthHandler):
    """OpenRouter OAuth PKCE 流程"""

    provider_type = "openrouter"
    AUTH_URL = "https://openrouter.ai/auth"
    EXCHANGE_URL = "https://openrouter.ai/api/v1/auth/keys"

    def get_authorize_url(
        self,
        callback_url: str,
        state: str,
        provider_name: str,
        code_verifier: str | None = None,
    ) -> OAuthAuthorizeResult:
        verifier = code_verifier or _pkce_code_verifier()
        challenge = _pkce_code_challenge(verifier)
        params = {
            "callback_url": callback_url,
            "code_challenge": challenge,
            "code_challenge_method": "S256",
            "state": state,
        }
        url = f"{self.AUTH_URL}?{urlencode(params)}"
        return OAuthAuthorizeResult(url=url, state=state, code_verifier=verifier)

    async def exchange_code(
        self,
        code: str,
        state: str,
        code_verifier: str | None = None,
        redirect_uri: str | None = None,
    ) -> OAuthExchangeResult:
        import httpx

        payload: dict[str, Any] = {
            "code": code,
            "code_challenge_method": "S256",
        }
        if code_verifier:
            payload["code_verifier"] = code_verifier

        async with httpx.AsyncClient(trust_env=False) as client:
            resp = await client.post(
                self.EXCHANGE_URL,
                json=payload,
                headers={"Content-Type": "application/json"},
                timeout=30.0,
            )
        if resp.status_code >= 400:
            raise ValueError(f"OpenRouter exchange failed: {resp.status_code} {resp.text}")
        data = resp.json()
        key = data.get("key") or data.get("data", {}).get("key")
        if not key:
            raise ValueError("OpenRouter response missing key")
        return OAuthExchangeResult(api_key=key)


class GeminiOAuthHandler(BaseOAuthHandler):
    """Google Gemini OAuth 2.0 流程"""

    provider_type = "gemini"
    AUTH_URL = "https://accounts.google.com/o/oauth2/v2/auth"
    TOKEN_URL = "https://oauth2.googleapis.com/token"
    SCOPES = ["https://www.googleapis.com/auth/generative-language"]

    def __init__(self, client_id: str, client_secret: str):
        self.client_id = client_id
        self.client_secret = client_secret

    def get_authorize_url(
        self,
        callback_url: str,
        state: str,
        provider_name: str,
        code_verifier: str | None = None,
    ) -> OAuthAuthorizeResult:
        params = {
            "client_id": self.client_id,
            "redirect_uri": callback_url,
            "response_type": "code",
            "scope": " ".join(self.SCOPES),
            "state": state,
            "access_type": "offline",
            "prompt": "consent",
        }
        if code_verifier:
            params["code_challenge"] = _pkce_code_challenge(code_verifier)
            params["code_challenge_method"] = "S256"
        url = f"{self.AUTH_URL}?{urlencode(params)}"
        return OAuthAuthorizeResult(url=url, state=state, code_verifier=code_verifier)

    async def exchange_code(
        self,
        code: str,
        state: str,
        code_verifier: str | None = None,
        redirect_uri: str | None = None,
    ) -> OAuthExchangeResult:
        import httpx

        payload = {
            "client_id": self.client_id,
            "client_secret": self.client_secret,
            "code": code,
            "grant_type": "authorization_code",
            "redirect_uri": redirect_uri or "",
        }
        if code_verifier:
            payload["code_verifier"] = code_verifier

        async with httpx.AsyncClient(trust_env=False) as client:
            resp = await client.post(
                self.TOKEN_URL,
                data=payload,
                headers={"Content-Type": "application/x-www-form-urlencoded"},
                timeout=30.0,
            )
        if resp.status_code >= 400:
            raise ValueError(f"Gemini token exchange failed: {resp.status_code} {resp.text}")
        data = resp.json()
        access_token = data.get("access_token")
        refresh_token = data.get("refresh_token")
        expires_in = data.get("expires_in", 3600)
        expires_at = (
            datetime.now(timezone.utc).replace(tzinfo=timezone.utc)
            if expires_in
            else None
        )
        if expires_in:
            from datetime import timedelta

            expires_at = datetime.now(timezone.utc) + timedelta(seconds=expires_in)
        return OAuthExchangeResult(
            access_token=access_token,
            refresh_token=refresh_token,
            expires_at=expires_at,
        )

    async def refresh_token(self, refresh_token: str) -> OAuthExchangeResult:
        import httpx

        payload = {
            "client_id": self.client_id,
            "client_secret": self.client_secret,
            "refresh_token": refresh_token,
            "grant_type": "refresh_token",
        }
        async with httpx.AsyncClient(trust_env=False) as client:
            resp = await client.post(
                self.TOKEN_URL,
                data=payload,
                headers={"Content-Type": "application/x-www-form-urlencoded"},
                timeout=30.0,
            )
        if resp.status_code >= 400:
            raise ValueError(f"Gemini token refresh failed: {resp.status_code} {resp.text}")
        data = resp.json()
        access_token = data.get("access_token")
        expires_in = data.get("expires_in", 3600)
        expires_at = datetime.now(timezone.utc)
        if expires_in:
            from datetime import timedelta

            expires_at = datetime.now(timezone.utc) + timedelta(seconds=expires_in)
        return OAuthExchangeResult(
            access_token=access_token,
            expires_at=expires_at,
        )


def _get_oauth_handler(provider_type: str) -> BaseOAuthHandler | None:
    if provider_type == "openrouter":
        return OpenRouterOAuthHandler()
    if provider_type == "gemini":
        client_id = os.getenv("GEMINI_OAUTH_CLIENT_ID")
        client_secret = os.getenv("GEMINI_OAUTH_CLIENT_SECRET")
        if client_id and client_secret:
            return GeminiOAuthHandler(client_id, client_secret)
    return None


OAUTH_SUPPORTED_PROVIDERS = frozenset({"openrouter", "gemini"})


class OAuthService:
    """OAuth 服务：授权 URL、code 交换、凭证存储与查询"""

    def __init__(self, session_factory: Any):
        self._session_factory = session_factory

    def get_authorize_url(
        self,
        provider_type: str,
        provider_name: str,
        frontend_callback_url: str,
        backend_callback_url: str,
    ) -> OAuthAuthorizeResult:
        """生成 OAuth 授权 URL。backend_callback_url 为 OAuth 提供商回调的地址；frontend_callback_url 为处理完成后重定向的前端地址。"""
        handler = _get_oauth_handler(provider_type)
        if not handler:
            raise ValueError(f"OAuth not supported for provider type: {provider_type}")
        state = secrets.token_urlsafe(24)
        code_verifier = _pkce_code_verifier() if provider_type == "openrouter" else None
        result = handler.get_authorize_url(
            callback_url=backend_callback_url,
            state=state,
            provider_name=provider_name,
            code_verifier=code_verifier,
        )
        result.state = state
        result.code_verifier = code_verifier
        return result

    async def store_oauth_state(
        self,
        state: str,
        provider_name: str,
        frontend_callback_url: str,
        code_verifier: str | None,
        backend_callback_url: str,
    ) -> None:
        """将 OAuth state 存入 Redis，供 callback 时使用"""
        try:
            redis = await get_redis()
            payload = {
                "provider_name": provider_name,
                "frontend_callback_url": frontend_callback_url,
                "code_verifier": code_verifier,
                "backend_callback_url": backend_callback_url,
            }
            await redis.setex(
                f"{OAUTH_STATE_PREFIX}{state}",
                OAUTH_STATE_TTL,
                json.dumps(payload),
            )
        except Exception as e:
            logger.warning("Failed to store OAuth state in Redis: %s", e)

    async def consume_oauth_state(self, state: str) -> dict | None:
        """从 Redis 获取并删除 OAuth state"""
        try:
            redis = await get_redis()
            key = f"{OAUTH_STATE_PREFIX}{state}"
            raw = await redis.get(key)
            if raw:
                await redis.delete(key)
                return json.loads(raw)
        except Exception as e:
            logger.warning("Failed to consume OAuth state from Redis: %s", e)
        return None

    async def handle_callback(
        self,
        provider_type: str,
        code: str,
        state: str,
    ) -> tuple[str, str]:
        """
        处理 OAuth 回调：从 Redis 取 state 数据，交换 code，存储凭证，返回 (provider_name, frontend_redirect_url)。
        """
        state_data = await self.consume_oauth_state(state)
        if not state_data:
            raise ValueError("Invalid or expired OAuth state")
        provider_name = state_data.get("provider_name", "")
        frontend_callback_url = state_data.get("frontend_callback_url", "/")
        code_verifier = state_data.get("code_verifier")
        backend_callback_url = state_data.get("backend_callback_url", "")

        handler = _get_oauth_handler(provider_type)
        if not handler:
            raise ValueError(f"OAuth not supported for provider type: {provider_type}")

        result = await handler.exchange_code(
            code, state, code_verifier=code_verifier, redirect_uri=backend_callback_url
        )

        async with self._session_factory() as session:
            stmt = select(Provider).where(Provider.name == provider_name)
            provider = await session.scalar(stmt)
            if not provider:
                raise ValueError(f"Provider '{provider_name}' not found")

            cred = await self._get_or_create_credential(session, provider, provider_type)
            cred.provider_type = provider_type
            cred.api_key = _encrypt(result.api_key) if result.api_key else None
            cred.access_token = _encrypt(result.access_token) if result.access_token else None
            if result.refresh_token:
                cred.refresh_token = _encrypt(result.refresh_token)
            cred.expires_at = result.expires_at
            session.add(cred)
            if result.api_key:
                provider.api_key = result.api_key
                session.add(provider)
            elif result.access_token and provider_type == "gemini":
                provider.api_key = result.access_token
                session.add(provider)
            await session.commit()

        base = frontend_callback_url.rstrip("/")
        sep = "&" if "?" in base else "?"
        redirect = f"{base}{sep}oauth=success&provider={provider_name}"
        return provider_name, redirect

    async def _get_or_create_credential(
        self,
        session: AsyncSession,
        provider: Provider,
        provider_type: str,
    ) -> ProviderOAuthCredential:
        stmt = select(ProviderOAuthCredential).where(
            ProviderOAuthCredential.provider_id == provider.id
        )
        cred = await session.scalar(stmt)
        if cred:
            return cred
        cred = ProviderOAuthCredential(
            provider_id=provider.id,
            provider_type=provider_type,
        )
        session.add(cred)
        await session.flush()
        return cred

    async def get_credential_for_provider(
        self, session: AsyncSession, provider: Provider
    ) -> ProviderOAuthCredential | None:
        stmt = select(ProviderOAuthCredential).where(
            ProviderOAuthCredential.provider_id == provider.id
        )
        return await session.scalar(stmt)

    async def get_effective_api_key(
        self, session: AsyncSession, provider: Provider
    ) -> str | None:
        """优先返回 OAuth 凭证中的 api_key/access_token，否则返回 Provider.api_key"""
        cred = await self.get_credential_for_provider(session, provider)
        if cred:
            if cred.api_key:
                dec = _decrypt(cred.api_key)
                if dec:
                    return dec
            if cred.access_token and cred.expires_at:
                if datetime.now(timezone.utc) < cred.expires_at.replace(tzinfo=timezone.utc):
                    dec = _decrypt(cred.access_token)
                    if dec:
                        return dec
        return provider.api_key

    async def has_oauth_credential(
        self, session: AsyncSession, provider_name: str
    ) -> bool:
        stmt = (
            select(ProviderOAuthCredential)
            .join(Provider)
            .where(Provider.name == provider_name)
        )
        cred = await session.scalar(stmt)
        if not cred:
            return False
        if cred.api_key and _decrypt(cred.api_key):
            return True
        if cred.refresh_token and _decrypt(cred.refresh_token):
            return True
        return False

    async def revoke_oauth_credential(
        self, session: AsyncSession, provider_name: str
    ) -> bool:
        stmt = select(Provider).where(Provider.name == provider_name)
        provider = await session.scalar(stmt)
        if not provider:
            return False
        cred_stmt = select(ProviderOAuthCredential).where(
            ProviderOAuthCredential.provider_id == provider.id
        )
        cred = await session.scalar(cred_stmt)
        if cred:
            await session.delete(cred)
            await session.flush()
            return True
