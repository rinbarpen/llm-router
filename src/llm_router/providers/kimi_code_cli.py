from __future__ import annotations

from .code_cli import CodeCLIProviderClient


class KimiCodeCLIProviderClient(CodeCLIProviderClient):
    PROVIDER_NAME = "kimi_code_cli"
    DEFAULT_EXECUTABLE = "kimi"


__all__ = ["KimiCodeCLIProviderClient"]
