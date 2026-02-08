from __future__ import annotations

import asyncio
import shutil
import subprocess
from pathlib import Path
from typing import Optional

from ..config import RouterSettings
from ..db.models import Model, Provider, ProviderType


class DownloadError(RuntimeError):
    pass


class ModelDownloader:
    def __init__(self, settings: RouterSettings) -> None:
        self.settings = settings

    async def ensure_available(self, provider: Provider, model: Model) -> Optional[Path]:
        """Download or prepare local resources for models that require them."""

        if provider.type not in {
            ProviderType.TRANSFORMERS,
            ProviderType.OLLAMA,
            ProviderType.VLLM,
        }:
            return None

        if provider.type == ProviderType.OLLAMA:
            await self._download_ollama(model)
            return None

        if provider.type == ProviderType.VLLM:
            # vLLM 不再下载模型，由外部服务管理
            return None

        # Transformers 处理
        if model.local_path:
            target_dir = Path(model.local_path).expanduser().resolve()
            target_dir.mkdir(parents=True, exist_ok=True)
            return target_dir

        # 无 local_path 时，仅下载到缓存，不落盘到 model_store
        await self._download_transformers(model, None)
        return None

    async def _download_transformers(self, model: Model, target_dir: Optional[Path]) -> None:
        identifier = (
            model.download_uri
            or model.remote_identifier
            or model.config.get("repo_id")
            or model.name
        )
        if not identifier:
            raise DownloadError("未指定Transformers模型的下载标识")

        try:
            from huggingface_hub import snapshot_download
        except ImportError as exc:
            raise DownloadError("缺少 huggingface_hub 依赖，无法下载Transformers模型") from exc

        kwargs = {
            "repo_id": identifier,
        }
        if target_dir:
            kwargs["local_dir"] = str(target_dir)
            kwargs["local_dir_use_symlinks"] = False

        if self.settings.download_cache_dir:
            kwargs["cache_dir"] = str(self.settings.download_cache_dir)

        await asyncio.to_thread(snapshot_download, **kwargs)

    async def _download_ollama(self, model: Model) -> None:
        model_name = model.remote_identifier or model.download_uri or model.name
        if not shutil.which("ollama"):
            raise DownloadError("未找到 ollama CLI，请先安装")

        process = await asyncio.create_subprocess_exec(
            "ollama",
            "pull",
            model_name,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
        )
        stdout, stderr = await process.communicate()
        if process.returncode != 0:
            raise DownloadError(
                f"ollama pull 失败: {stderr.decode().strip() or stdout.decode().strip()}"
            )


