from __future__ import annotations

import os
import subprocess
import sys
from pathlib import Path

import uvicorn

from .api.app import create_app
from .config import load_settings  # 导入时会自动加载 .env 文件

app = create_app()

__all__ = ["app", "create_app", "main", "monitor", "web"]


def main() -> None:
    """启动 LLM Router 服务"""
    # .env 文件已在 config 模块导入时自动加载
    settings = load_settings()
    uvicorn.run("llm_router:app", host=settings.host, port=settings.port, reload=False)


def monitor() -> None:
    """启动监控前端界面"""
    frontend_dir = Path(__file__).parent.parent.parent / "frontend"
    if not frontend_dir.exists():
        print(f"错误: 前端目录不存在: {frontend_dir}", file=sys.stderr)
        sys.exit(1)
    
    os.chdir(frontend_dir)
    try:
        subprocess.run(["npm", "run", "dev"], check=True)
    except subprocess.CalledProcessError as e:
        print(f"错误: 启动前端失败: {e}", file=sys.stderr)
        sys.exit(1)
    except FileNotFoundError:
        print("错误: 未找到 npm 命令，请先安装 Node.js", file=sys.stderr)
        sys.exit(1)


def web() -> None:
    """启动 Web 前端界面（monitor 的别名）"""
    monitor()
