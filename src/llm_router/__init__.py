from __future__ import annotations

import os

import uvicorn

from .api.app import create_app

app = create_app()

__all__ = ["app", "create_app", "main"]


def main() -> None:
    host = os.getenv("LLM_ROUTER_HOST", "0.0.0.0")
    port = int(os.getenv("LLM_ROUTER_PORT", "8000"))
    uvicorn.run("llm_router:app", host=host, port=port, reload=False)

