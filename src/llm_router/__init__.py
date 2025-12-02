from __future__ import annotations

import uvicorn

from .api.app import create_app
from .config import load_settings  # 导入时会自动加载 .env 文件

app = create_app()

__all__ = ["app", "create_app", "main"]


def main() -> None:
    """启动 LLM Router 服务"""
    # .env 文件已在 config 模块导入时自动加载
    settings = load_settings()
    uvicorn.run("llm_router:app", host=settings.host, port=settings.port, reload=False)
