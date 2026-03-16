from __future__ import annotations

from .code_cli import CodeCLIProviderClient


class QwenCodeCLIProviderClient(CodeCLIProviderClient):
    PROVIDER_NAME = "qwen_code_cli"
    DEFAULT_EXECUTABLE = "qwen"


__all__ = ["QwenCodeCLIProviderClient"]
