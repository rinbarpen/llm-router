"""Tests for request_utils."""

from __future__ import annotations

import pytest

from llm_router.api.request_utils import normalize_claude_provider_name


def test_normalize_claude_provider_name_claude_code() -> None:
    assert normalize_claude_provider_name("claude_code") == "claude_code_cli"
    assert normalize_claude_provider_name("CLAUDE_CODE") == "claude_code_cli"
    assert normalize_claude_provider_name("  claude_code  ") == "claude_code_cli"


def test_normalize_claude_provider_name_unchanged() -> None:
    assert normalize_claude_provider_name("claude_code_cli") == "claude_code_cli"
    assert normalize_claude_provider_name("claude") == "claude"
    assert normalize_claude_provider_name("openai") == "openai"


def test_normalize_claude_provider_name_empty() -> None:
    assert normalize_claude_provider_name("") == ""
    assert normalize_claude_provider_name(None) == ""
