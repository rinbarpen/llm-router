from __future__ import annotations

import os
from functools import lru_cache
from pathlib import Path
from typing import Dict, List, Optional, Set, Tuple

from dotenv import load_dotenv
from pydantic import BaseModel, Field, field_validator

from sqlalchemy.engine import make_url

from .api_key_config import APIKeyConfig
from .db import DEFAULT_DB_FILENAME, build_sqlite_url

# 加载 .env 文件（如果存在）
# 在项目根目录查找 .env 文件，只在首次导入时加载一次
_env_file = Path.cwd() / ".env"
if _env_file.exists():
    load_dotenv(_env_file, override=False)  # override=False 表示不覆盖已存在的环境变量


def _sqlite_path_from_url(url: str) -> Optional[Path]:
    """从 SQLite 连接 URL 解析出数据库文件路径；非 SQLite 或无法解析时返回 None。"""
    if not url.startswith("sqlite"):
        return None
    if "///" in url:
        path_str = url.split("///", 1)[1]
    else:
        return None
    return Path(path_str).expanduser().resolve()


def _default_data_dir() -> Path:
    """默认数据目录为项目根下的 data/，数据库文件为 data/llm_router.db、data/llm_datas.db。
    注意：所有数据库文件和模型存储都必须位于此目录下。
    """
    return Path.cwd() / "data"


def _default_database_url() -> str:
    return build_sqlite_url(_default_data_dir() / DEFAULT_DB_FILENAME)


def _default_monitor_database_url() -> str:
    return build_sqlite_url(_default_data_dir() / "llm_datas.db")


def _default_model_store() -> Path:
    """默认模型存储目录。"""
    return _default_data_dir() / "models"


def _default_download_cache() -> Path:
    """默认下载缓存目录。"""
    return _default_data_dir() / "download_cache"


class RouterSettings(BaseModel):
    """Runtime configuration loaded from environment variables."""

    database_url: str = Field(default_factory=_default_database_url)
    monitor_database_url: str = Field(default_factory=_default_monitor_database_url)
    model_store_dir: Path = Field(default_factory=_default_model_store)
    download_cache_dir: Path = Field(default_factory=_default_download_cache)
    download_concurrency: int = Field(default=2, ge=1)
    default_timeout: float = Field(default=60.0, gt=0)
    log_level: str = Field(default="INFO")
    model_config_file: Optional[Path] = None
    api_keys: List[APIKeyConfig] = Field(default_factory=list)
    require_auth: bool = Field(default=True)  # 默认开启认证
    allow_local_without_auth: bool = Field(
        default=True,
        description="本机请求是否免认证（True=免认证，False=本机也需 API Key）",
    )
    host: str = Field(default="0.0.0.0", description="服务绑定的主机地址")
    port: int = Field(default=8000, ge=1, le=65535, description="服务绑定的端口")
    redis_url: str = Field(
        default="redis://localhost:6379/0",
        description="Redis 连接 URL（用于存储登录记录等数据）",
    )
    routing_analyzer_model: Optional[str] = Field(default=None)
    routing_default_strong_model: Optional[str] = Field(default=None)
    routing_default_weak_model: Optional[str] = Field(default=None)
    routing_default_pair: Optional[str] = Field(default=None)
    routing_pairs: Dict[str, Tuple[str, str]] = Field(default_factory=dict)
    routing_analyzer_timeout_ms: int = Field(default=1500, ge=100, le=10000)
    routing_auto_fallback_mode: str = Field(default="weak")

    @field_validator("model_store_dir", mode="before")
    @classmethod
    def _validate_model_store(cls, value: str | Path) -> Path:
        return Path(value).expanduser().resolve()

    @field_validator("download_cache_dir", mode="before")
    @classmethod
    def _validate_cache_dir(cls, value: str | Path | None) -> Path:
        if value is None:
            return _default_download_cache()
        return Path(value).expanduser().resolve()

    @field_validator("model_config_file", mode="before")
    @classmethod
    def _validate_model_config(cls, value: str | Path | None) -> Optional[Path]:
        if value is None:
            return None
        return Path(value).expanduser().resolve()

    @field_validator("api_keys", mode="before")
    @classmethod
    def _validate_api_keys(cls, value: str | List[str] | List[APIKeyConfig] | None) -> List[APIKeyConfig]:
        if value is None:
            return []
        if isinstance(value, str):
            # 支持逗号分隔的简单 API Key（向后兼容）
            keys = [k.strip() for k in value.split(",") if k.strip()]
            return [APIKeyConfig(key=k) for k in keys]
        if isinstance(value, list):
            result = []
            for item in value:
                if isinstance(item, APIKeyConfig):
                    # 如果已经是 APIKeyConfig，解析环境变量（支持多个 key）
                    resolved_keys = item.resolved_keys()
                    for idx, key in enumerate(resolved_keys):
                        name = item.name
                        if len(resolved_keys) > 1:
                            name = f"{item.name or 'API Key'} #{idx + 1}"
                        result.append(APIKeyConfig(
                            key=key,
                            name=name,
                            allowed_models=item.allowed_models,
                            allowed_providers=item.allowed_providers,
                            parameter_limits=item.parameter_limits,
                            is_active=item.is_active,
                        ))
                elif isinstance(item, str):
                    result.append(APIKeyConfig(key=item.strip()))
                elif isinstance(item, dict):
                    # 从字典创建，支持 key_env（支持多个 key）
                    api_key_cfg = APIKeyConfig(**item)
                    resolved_keys = api_key_cfg.resolved_keys()
                    for idx, key in enumerate(resolved_keys):
                        name = api_key_cfg.name
                        if len(resolved_keys) > 1:
                            name = f"{api_key_cfg.name or 'API Key'} #{idx + 1}"
                        result.append(APIKeyConfig(
                            key=key,
                            name=name,
                            allowed_models=api_key_cfg.allowed_models,
                            allowed_providers=api_key_cfg.allowed_providers,
                            parameter_limits=api_key_cfg.parameter_limits,
                            is_active=api_key_cfg.is_active,
                        ))
            return result
        return []

    def get_api_key_config(self, key: str) -> Optional[APIKeyConfig]:
        """根据 key 值获取 API Key 配置"""
        for api_key_config in self.api_keys:
            if api_key_config.key == key:
                return api_key_config
        return None

    def has_api_keys(self) -> bool:
        """检查是否配置了 API Key"""
        return len(self.api_keys) > 0

    def ensure_directories(self) -> None:
        # 确保模型存储目录存在
        if self.model_store_dir:
            self.model_store_dir.mkdir(parents=True, exist_ok=True)
        # 确保下载缓存目录存在
        if self.download_cache_dir:
            self.download_cache_dir.mkdir(parents=True, exist_ok=True)
        # 确保 SQLite 数据库所在目录存在，避免 "readonly database" 等权限类错误
        for url in (self.database_url, self.monitor_database_url):
            db_path = _sqlite_path_from_url(url)
            if db_path is not None:
                db_path.parent.mkdir(parents=True, exist_ok=True)


@lru_cache(1)
def load_settings() -> RouterSettings:
    """Load settings from environment variables and config file, caching the result."""
    from .model_config import load_model_config

    # 检查环境变量是否明确设置（用于确定优先级）
    host_env_set = os.getenv("LLM_ROUTER_HOST") is not None
    port_env_set = os.getenv("LLM_ROUTER_PORT") is not None
    allow_local_without_auth_env_set = os.getenv("LLM_ROUTER_ALLOW_LOCAL_WITHOUT_AUTH") is not None

    # 确定配置文件路径：环境变量 > 默认路径（当前目录的 router.toml）
    model_config_file_env = os.getenv("LLM_ROUTER_MODEL_CONFIG")
    if model_config_file_env:
        default_config_file = Path(model_config_file_env).expanduser().resolve()
    else:
        # 默认查找当前目录的 router.toml
        default_config_file = Path.cwd() / "router.toml"

    env_mapping = {
        "database_url": os.getenv("LLM_ROUTER_DATABASE_URL"),
        "monitor_database_url": os.getenv("LLM_ROUTER_MONITOR_DATABASE_URL"),
        "model_store_dir": os.getenv("LLM_ROUTER_MODEL_STORE"),
        "download_cache_dir": os.getenv("LLM_ROUTER_DOWNLOAD_CACHE"),
        "download_concurrency": os.getenv("LLM_ROUTER_DOWNLOAD_CONCURRENCY"),
        "default_timeout": os.getenv("LLM_ROUTER_DEFAULT_TIMEOUT"),
        "log_level": os.getenv("LLM_ROUTER_LOG_LEVEL"),
        "model_config_file": model_config_file_env,  # 只有明确设置时才使用
        "api_keys": os.getenv("LLM_ROUTER_API_KEYS"),  # 向后兼容：支持简单字符串
        "require_auth": os.getenv("LLM_ROUTER_REQUIRE_AUTH", "true").lower() in ("true", "1", "yes"),
        "allow_local_without_auth": os.getenv("LLM_ROUTER_ALLOW_LOCAL_WITHOUT_AUTH", "true").lower()
        in ("true", "1", "yes"),
        "host": os.getenv("LLM_ROUTER_HOST", "0.0.0.0"),
        "port": int(os.getenv("LLM_ROUTER_PORT", "8000")),
        "redis_url": os.getenv("LLM_ROUTER_REDIS_URL", "redis://localhost:6379/0"),
    }

    data = {key: value for key, value in env_mapping.items() if value is not None}
    
    # --- 路径规范化校验 ---
    # 默认强制数据库和模型目录位于 data/ 下，避免误写到非预期路径；
    # 但测试环境需要允许临时目录，避免测试互相污染。
    is_test_env = os.getenv("PYTEST_CURRENT_TEST") is not None
    if not is_test_env:
        data_dir = _default_data_dir()

        if "database_url" in data:
            db_path = _sqlite_path_from_url(data["database_url"])
            if db_path and not str(db_path).startswith(str(data_dir)):
                import logging
                logging.getLogger(__name__).warning(
                    f"环境变量 LLM_ROUTER_DATABASE_URL 指向非 data 目录 ({db_path})，已回退到默认路径"
                )
                data["database_url"] = _default_database_url()

        if "monitor_database_url" in data:
            db_path = _sqlite_path_from_url(data["monitor_database_url"])
            if db_path and not str(db_path).startswith(str(data_dir)):
                import logging
                logging.getLogger(__name__).warning(
                    f"环境变量 LLM_ROUTER_MONITOR_DATABASE_URL 指向非 data 目录 ({db_path})，已回退到默认路径"
                )
                data["monitor_database_url"] = _default_monitor_database_url()

        if "model_store_dir" in data:
            store_path = Path(data["model_store_dir"]).expanduser().resolve()
            expected_store = _default_model_store()
            if store_path != expected_store:
                import logging
                logging.getLogger(__name__).warning(
                    f"环境变量 LLM_ROUTER_MODEL_STORE 指向非规范目录 ({store_path})，已强制设为 {expected_store}"
                )
                data["model_store_dir"] = expected_store
    # ----------------------

    settings = RouterSettings(**data)
    
    # 尝试加载配置文件（从环境变量指定的路径或默认路径）
    config_file_to_load = settings.model_config_file if settings.model_config_file else default_config_file
    
    if config_file_to_load and config_file_to_load.exists():
        try:
            config_data = load_model_config(config_file_to_load)
            if config_data.server:
                # 只有在环境变量未明确设置时才使用配置文件的值
                if not host_env_set and config_data.server.host is not None:
                    settings.host = config_data.server.host
                if not port_env_set and config_data.server.port is not None:
                    settings.port = config_data.server.port
                if (
                    not allow_local_without_auth_env_set
                    and config_data.server.allow_local_without_auth is not None
                ):
                    settings.allow_local_without_auth = config_data.server.allow_local_without_auth
            if config_data.routing:
                settings.routing_analyzer_model = config_data.routing.analyzer_model
                settings.routing_default_strong_model = config_data.routing.default_strong_model
                settings.routing_default_weak_model = config_data.routing.default_weak_model
                settings.routing_default_pair = config_data.routing.default_pair
                settings.routing_analyzer_timeout_ms = config_data.routing.analyzer_timeout_ms
                settings.routing_auto_fallback_mode = config_data.routing.auto_fallback_mode
                pairs_dict: Dict[str, Tuple[str, str]] = {}
                for p in config_data.routing.pairs:
                    pairs_dict[p.name] = (p.strong_model, p.weak_model)
                settings.routing_pairs = pairs_dict
        except Exception as e:
            # 如果加载配置文件失败，记录警告但继续使用环境变量或默认值
            import logging
            logger = logging.getLogger(__name__)
            logger.warning(f"无法加载配置文件 {config_file_to_load}: {e}")
    
    settings.ensure_directories()
    return settings


__all__ = ["RouterSettings", "load_settings"]
