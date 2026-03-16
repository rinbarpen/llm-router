from __future__ import annotations

from .code_cli import CodeCLIProviderClient


class OpenCodeCLIProviderClient(CodeCLIProviderClient):
    PROVIDER_NAME = "opencode_cli"
    DEFAULT_EXECUTABLE = "opencode"


__all__ = ["OpenCodeCLIProviderClient"]
