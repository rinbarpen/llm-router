from __future__ import annotations

import importlib.util
from pathlib import Path
from typing import Any


SCRIPT_PATH = Path(__file__).resolve().parent.parent / "scripts" / "tests" / "request_codex_claude.py"
SPEC = importlib.util.spec_from_file_location("request_codex_claude_script", SCRIPT_PATH)
assert SPEC and SPEC.loader
SCRIPT = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(SCRIPT)


class FakeResponse:
    def __init__(self, status_code: int, payload: dict[str, Any] | None = None, text: str = "") -> None:
        self.status_code = status_code
        self._payload = payload if payload is not None else {}
        self.text = text

    def json(self) -> dict[str, Any]:
        return self._payload


def _install_post_stub(monkeypatch, responses: list[FakeResponse]) -> list[str]:
    called_urls: list[str] = []

    def _fake_post(url: str, **kwargs: Any) -> FakeResponse:
        del kwargs
        called_urls.append(url)
        if not responses:
            raise AssertionError("stub responses exhausted")
        return responses.pop(0)

    monkeypatch.setattr(SCRIPT.requests, "post", _fake_post)
    return called_urls


def test_invoke_codex_fallback_to_chat_completions(monkeypatch, capsys) -> None:
    called_urls = _install_post_stub(
        monkeypatch,
        [
            FakeResponse(404, {"detail": "Not Found"}),
            FakeResponse(200, {"choices": [{"message": {"content": "hello from chat"}}]}),
        ],
    )

    rc = SCRIPT.invoke_codex(
        base_url="http://localhost:18000",
        model="codex_cli/gpt-5.3-codex",
        prompt="say hi",
        max_tokens=64,
        temperature=0.2,
        token=None,
        timeout=30.0,
        show_json=True,
    )

    captured = capsys.readouterr()
    assert rc == 0
    assert "hello from chat" in captured.out
    assert "responses-fallback-chat-completions" in captured.out
    assert called_urls == [
        "http://localhost:18000/v1/responses",
        "http://localhost:18000/v1/chat/completions",
    ]


def test_invoke_codex_fallback_until_route_invoke(monkeypatch, capsys) -> None:
    called_urls = _install_post_stub(
        monkeypatch,
        [
            FakeResponse(404, {"detail": "Not Found"}),
            FakeResponse(404, {"detail": "Not Found"}),
            FakeResponse(404, {"detail": "Not Found"}),
            FakeResponse(200, {"output_text": "hello from route invoke"}),
        ],
    )

    rc = SCRIPT.invoke_codex(
        base_url="http://localhost:18000",
        model="codex_cli/gpt-5.3-codex",
        prompt="say hi",
        max_tokens=64,
        temperature=0.2,
        token=None,
        timeout=30.0,
        show_json=True,
    )

    captured = capsys.readouterr()
    assert rc == 0
    assert "hello from route invoke" in captured.out
    assert "responses-fallback-route-invoke" in captured.out
    assert called_urls == [
        "http://localhost:18000/v1/responses",
        "http://localhost:18000/v1/chat/completions",
        "http://localhost:18000/models/codex_cli/gpt-5.3-codex/invoke",
        "http://localhost:18000/route/invoke",
    ]


def test_invoke_codex_does_not_fallback_for_non_404(monkeypatch, capsys) -> None:
    called_urls = _install_post_stub(
        monkeypatch,
        [
            FakeResponse(401, {"detail": "Unauthorized"}),
        ],
    )

    rc = SCRIPT.invoke_codex(
        base_url="http://localhost:18000",
        model="codex_cli/gpt-5.3-codex",
        prompt="say hi",
        max_tokens=64,
        temperature=0.2,
        token=None,
        timeout=30.0,
        show_json=False,
    )

    captured = capsys.readouterr()
    assert rc == 1
    assert "HTTP 401" in captured.err
    assert called_urls == ["http://localhost:18000/v1/responses"]


def test_invoke_claude_fallback_to_invoke(monkeypatch, capsys) -> None:
    called_urls = _install_post_stub(
        monkeypatch,
        [
            FakeResponse(404, {"detail": "Not Found"}),
            FakeResponse(404, {"detail": "Not Found"}),
            FakeResponse(200, {"output_text": "hello from invoke"}),
        ],
    )

    rc = SCRIPT.invoke_claude(
        base_url="http://localhost:18000",
        model="claude_code_cli/claude-sonnet-4-5",
        prompt="say hi",
        max_tokens=64,
        temperature=0.2,
        token=None,
        timeout=30.0,
        show_json=True,
    )

    captured = capsys.readouterr()
    assert rc == 0
    assert "hello from invoke" in captured.out
    assert "messages-fallback-invoke" in captured.out
    assert called_urls == [
        "http://localhost:18000/v1/messages",
        "http://localhost:18000/v1/chat/completions",
        "http://localhost:18000/models/claude_code_cli/claude-sonnet-4-5/invoke",
    ]


def test_invoke_claude_all_404_returns_error(monkeypatch, capsys) -> None:
    called_urls = _install_post_stub(
        monkeypatch,
        [
            FakeResponse(404, {"detail": "Not Found"}),
            FakeResponse(404, {"detail": "Not Found"}),
            FakeResponse(404, {"detail": "Not Found"}),
            FakeResponse(404, {"detail": "Not Found"}),
        ],
    )

    rc = SCRIPT.invoke_claude(
        base_url="http://localhost:18000",
        model="claude_code_cli/claude-sonnet-4-5",
        prompt="say hi",
        max_tokens=64,
        temperature=0.2,
        token=None,
        timeout=30.0,
        show_json=False,
    )

    captured = capsys.readouterr()
    assert rc == 1
    assert "HTTP 404" in captured.err
    assert called_urls == [
        "http://localhost:18000/v1/messages",
        "http://localhost:18000/v1/chat/completions",
        "http://localhost:18000/models/claude_code_cli/claude-sonnet-4-5/invoke",
        "http://localhost:18000/route/invoke",
    ]


def test_parser_supports_new_code_cli_commands() -> None:
    parser = SCRIPT.build_parser()
    args = parser.parse_args(["opencode", "--prompt", "hello"])
    assert args.command == "opencode"
    assert args.model == SCRIPT.DEFAULT_OPENCODE_MODEL

    args = parser.parse_args(["kimi-code", "--prompt", "hello"])
    assert args.command == "kimi-code"
    assert args.model == SCRIPT.DEFAULT_KIMI_CODE_MODEL

    args = parser.parse_args(["qwen-code", "--prompt", "hello"])
    assert args.command == "qwen-code"
    assert args.model == SCRIPT.DEFAULT_QWEN_CODE_MODEL
