from __future__ import annotations

import importlib.util
from pathlib import Path

import pytest


def _load_module(module_path: Path, module_name: str):
    spec = importlib.util.spec_from_file_location(module_name, module_path)
    assert spec is not None and spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def test_check_all_openrouter_free_check_model_connection_error_fail_fast(monkeypatch):
    module = _load_module(
        Path("scripts/tests/check_all_openrouter_free.py"),
        "check_all_openrouter_free_script",
    )

    def _raise_connection_error(*args, **kwargs):
        raise module.requests_exceptions.ConnectionError("connection refused")

    monkeypatch.setattr(module.requests, "post", _raise_connection_error)

    with pytest.raises(module.ServiceUnavailableError):
        module.check_model("demo-model")


def test_check_and_clean_openrouter_free_check_model_connection_error_fail_fast(monkeypatch):
    module = _load_module(
        Path("scripts/check_and_clean_openrouter_free.py"),
        "check_and_clean_openrouter_free_script",
    )

    def _raise_connection_error(*args, **kwargs):
        raise module.requests_exceptions.ConnectionError("connection refused")

    monkeypatch.setattr(module.requests, "post", _raise_connection_error)

    with pytest.raises(module.ServiceUnavailableError):
        module.check_model("demo-model")


def test_check_all_main_stops_immediately_when_router_unavailable(monkeypatch):
    module = _load_module(
        Path("scripts/tests/check_all_openrouter_free.py"),
        "check_all_openrouter_free_main_script",
    )

    def _raise_unavailable():
        raise module.ServiceUnavailableError("router unavailable")

    monkeypatch.setattr(module, "ensure_router_available", _raise_unavailable)

    with pytest.raises(module.ServiceUnavailableError):
        module.main()

