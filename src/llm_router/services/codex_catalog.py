from __future__ import annotations

import json
import logging
import time
from pathlib import Path
from typing import Iterable, List, Optional

import tomli

logger = logging.getLogger(__name__)


class CodexModelCatalog:
    """Caches supported Codex CLI models + default selection."""

    def __init__(
        self,
        codex_home: Path | None = None,
        ttl_seconds: float = 60.0,
    ) -> None:
        self.codex_home = codex_home or Path.home() / ".codex"
        self._models_cache_path = self.codex_home / "models_cache.json"
        self._config_path = self.codex_home / "config.toml"
        self._ttl_seconds = ttl_seconds
        self._last_load = 0.0
        self._models_cache_mtime = 0.0
        self._config_mtime = 0.0
        self._supported_models: List[str] = []
        self._default_model: Optional[str] = None

    def supported_models(self) -> List[str]:
        self._maybe_reload()
        return list(self._supported_models)

    def default_model(self) -> Optional[str]:
        self._maybe_reload()
        return self._default_model

    def priority_candidates(self) -> List[str]:
        self._maybe_reload()
        result: List[str] = []
        default = self._default_model
        if default:
            result.append(default)
        for slug in self._supported_models:
            if slug not in result:
                result.append(slug)
        return result

    def _maybe_reload(self) -> None:
        now = time.time()
        if now - self._last_load < self._ttl_seconds:
            models_mtime = self._get_mtime(self._models_cache_path)
            config_mtime = self._get_mtime(self._config_path)
            if (
                models_mtime == self._models_cache_mtime
                and config_mtime == self._config_mtime
            ):
                return
        self._reload(now)

    def _reload(self, now: float) -> None:
        self._last_load = now
        supported = self._load_supported_models()
        default = self._load_default_model()
        if supported:
            self._supported_models = supported
        else:
            logger.debug("Codex catalog has no supported models")
            self._supported_models = []
        self._default_model = default

    def _load_supported_models(self) -> List[str]:
        try:
            models_mtime = self._get_mtime(self._models_cache_path)
            self._models_cache_mtime = models_mtime
            if not self._models_cache_path.exists():
                return []
            data = json.loads(self._models_cache_path.read_text(encoding="utf-8"))
            models = data.get("models") or []
            result: List[str] = []
            for entry in models:
                if not isinstance(entry, dict):
                    continue
                if not entry.get("supported_in_api"):
                    continue
                slug = entry.get("slug")
                if slug and isinstance(slug, str):
                    result.append(slug)
            return result
        except Exception as exc:
            logger.warning("Failed to read Codex models cache: %s", exc)
            return []

    def _load_default_model(self) -> Optional[str]:
        try:
            config_mtime = self._get_mtime(self._config_path)
            self._config_mtime = config_mtime
            if not self._config_path.exists():
                return None
            data = tomli.loads(self._config_path.read_text(encoding="utf-8"))
            model = data.get("model")
            if isinstance(model, str) and model.strip():
                return model.strip()
        except Exception as exc:
            logger.warning("Failed to read Codex config: %s", exc)
        return None

    def _get_mtime(self, path: Path) -> float:
        try:
            return path.stat().st_mtime
        except OSError:
            return 0.0
