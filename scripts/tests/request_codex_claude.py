#!/usr/bin/env python3
"""请求 Codex CLI / Claude Code 模型的便捷脚本。"""

from __future__ import annotations

import argparse
import json
import sys
from typing import Any, Callable, Sequence

from curl_cffi import requests

DEFAULT_BASE_URL = "http://localhost:18000"
DEFAULT_TIMEOUT = 120.0
DEFAULT_CODEX_MODEL = "codex_cli/gpt-5.3-codex"
DEFAULT_CLAUDE_MODEL = "claude_code_cli/claude-sonnet-4-5"


def _split_model_reference(model: str) -> tuple[str, str]:
    if "/" not in model:
        raise ValueError(f"模型引用必须是 provider/model 形式，当前值: {model}")
    provider, model_name = model.split("/", 1)
    provider = provider.strip()
    model_name = model_name.strip()
    if not provider or not model_name:
        raise ValueError(f"模型引用必须是 provider/model 形式，当前值: {model}")
    return provider, model_name


def _build_headers(token: str | None, include_claude_header: bool = False) -> dict[str, str]:
    headers: dict[str, str] = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    if include_claude_header:
        headers["anthropic-version"] = "2023-06-01"
    return headers


def _print_error(resp: requests.Response) -> None:
    print(f"请求失败，HTTP {resp.status_code}", file=sys.stderr)
    try:
        data = resp.json()
        print(json.dumps(data, ensure_ascii=False, indent=2), file=sys.stderr)
    except Exception:
        print(resp.text, file=sys.stderr)


def _print_response_text(
    data: dict[str, Any],
    show_json: bool,
    endpoint_type: str,
) -> None:
    output_text = data.get("output_text")
    if isinstance(output_text, str) and output_text.strip():
        print(output_text)
    else:
        content = data.get("content")
        if isinstance(content, list):
            text_parts: list[str] = []
            for block in content:
                if isinstance(block, dict) and block.get("type") == "text":
                    value = block.get("text")
                    if isinstance(value, str):
                        text_parts.append(value)
            text = "\n".join(part for part in text_parts if part)
            if text:
                print(text)
            else:
                print(json.dumps(data, ensure_ascii=False, indent=2))
        else:
            choices = data.get("choices")
            if isinstance(choices, list):
                text_parts: list[str] = []
                for choice in choices:
                    if not isinstance(choice, dict):
                        continue
                    message = choice.get("message")
                    if isinstance(message, dict):
                        content = message.get("content")
                        if isinstance(content, str) and content.strip():
                            text_parts.append(content)
                        elif isinstance(content, list):
                            for block in content:
                                if isinstance(block, dict) and block.get("type") == "text":
                                    value = block.get("text")
                                    if isinstance(value, str):
                                        text_parts.append(value)
                    text = choice.get("text")
                    if isinstance(text, str) and text.strip():
                        text_parts.append(text)
                merged = "\n".join(part for part in text_parts if part)
                if merged:
                    print(merged)
                else:
                    print(json.dumps(data, ensure_ascii=False, indent=2))
            else:
                print(json.dumps(data, ensure_ascii=False, indent=2))

    if show_json:
        print("\n--- RAW JSON ---")
        envelope = {
            "endpoint_type": endpoint_type,
            "response": data,
        }
        print(json.dumps(envelope, ensure_ascii=False, indent=2))


def _post_json(
    url: str,
    headers: dict[str, str],
    payload: dict[str, Any],
    timeout: float,
    *,
    error_prefix: str = "请求失败",
) -> requests.Response:
    try:
        return requests.post(
            url,
            headers=headers,
            json=payload,
            timeout=timeout,
        )
    except Exception as exc:
        raise RuntimeError(f"{error_prefix}: {exc}") from exc


def _invoke_fallback_invoke_endpoint(
    base_url: str,
    provider: str,
    model_name: str,
    prompt: str,
    max_tokens: int,
    temperature: float,
    token: str | None,
    timeout: float,
    use_messages: bool,
) -> requests.Response:
    url = f"{base_url.rstrip('/')}/models/{provider}/{model_name}/invoke"
    payload: dict[str, Any] = {
        "parameters": {
            "max_tokens": max_tokens,
        }
    }
    if temperature >= 0:
        payload["parameters"]["temperature"] = temperature

    if use_messages:
        payload["messages"] = [{"role": "user", "content": prompt}]
    else:
        payload["prompt"] = prompt

    return _post_json(
        url,
        headers=_build_headers(token),
        payload=payload,
        timeout=timeout,
        error_prefix="调用回退端点失败",
    )


def _invoke_fallback_chat_completions(
    base_url: str,
    model: str,
    prompt: str,
    max_tokens: int,
    temperature: float,
    token: str | None,
    timeout: float,
) -> requests.Response:
    url = f"{base_url.rstrip('/')}/v1/chat/completions"
    payload: dict[str, Any] = {
        "model": model,
        "messages": [{"role": "user", "content": prompt}],
        "max_tokens": max_tokens,
    }
    if temperature >= 0:
        payload["temperature"] = temperature

    return _post_json(
        url,
        headers=_build_headers(token),
        payload=payload,
        timeout=timeout,
        error_prefix="调用 /v1/chat/completions 回退端点失败",
    )


def _invoke_fallback_route_invoke(
    base_url: str,
    model_name: str,
    prompt: str,
    max_tokens: int,
    temperature: float,
    token: str | None,
    timeout: float,
    use_messages: bool,
) -> requests.Response:
    url = f"{base_url.rstrip('/')}/route/invoke"
    request_payload: dict[str, Any] = {
        "parameters": {
            "max_tokens": max_tokens,
        }
    }
    if temperature >= 0:
        request_payload["parameters"]["temperature"] = temperature
    if use_messages:
        request_payload["messages"] = [{"role": "user", "content": prompt}]
    else:
        request_payload["prompt"] = prompt

    payload: dict[str, Any] = {
        "query": {
            "name": model_name,
        },
        "request": request_payload,
    }
    return _post_json(
        url,
        headers=_build_headers(token),
        payload=payload,
        timeout=timeout,
        error_prefix="调用 /route/invoke 回退端点失败",
    )


def _request_with_404_fallback(
    attempts: Sequence[tuple[str, Callable[[], requests.Response]]],
) -> tuple[requests.Response, str]:
    if not attempts:
        raise RuntimeError("请求配置错误：缺少可用端点")

    last_resp: requests.Response | None = None
    last_name = ""
    for endpoint_name, request_fn in attempts:
        last_name = endpoint_name
        resp = request_fn()
        last_resp = resp
        if resp.status_code != 404:
            return resp, endpoint_name
    assert last_resp is not None
    return last_resp, last_name


def invoke_codex(
    base_url: str,
    model: str,
    prompt: str,
    max_tokens: int,
    temperature: float,
    token: str | None,
    timeout: float,
    show_json: bool,
) -> int:
    try:
        provider, model_name = _split_model_reference(model)
    except ValueError as exc:
        print(str(exc), file=sys.stderr)
        return 1

    responses_payload: dict[str, Any] = {
        "model": model,
        "input": prompt,
        "max_output_tokens": max_tokens,
    }
    if temperature >= 0:
        responses_payload["temperature"] = temperature

    attempts: list[tuple[str, Callable[[], requests.Response]]] = [
        (
            "responses",
            lambda: _post_json(
                f"{base_url.rstrip('/')}/v1/responses",
                headers=_build_headers(token),
                payload=responses_payload,
                timeout=timeout,
            ),
        ),
        (
            "responses-fallback-chat-completions",
            lambda: _invoke_fallback_chat_completions(
                base_url=base_url,
                model=model,
                prompt=prompt,
                max_tokens=max_tokens,
                temperature=temperature,
                token=token,
                timeout=timeout,
            ),
        ),
        (
            "responses-fallback-invoke",
            lambda: _invoke_fallback_invoke_endpoint(
                base_url=base_url,
                provider=provider,
                model_name=model_name,
                prompt=prompt,
                max_tokens=max_tokens,
                temperature=temperature,
                token=token,
                timeout=timeout,
                use_messages=False,
            ),
        ),
        (
            "responses-fallback-route-invoke",
            lambda: _invoke_fallback_route_invoke(
                base_url=base_url,
                model_name=model_name,
                prompt=prompt,
                max_tokens=max_tokens,
                temperature=temperature,
                token=token,
                timeout=timeout,
                use_messages=False,
            ),
        ),
    ]

    try:
        resp, endpoint_type = _request_with_404_fallback(attempts)
    except RuntimeError as exc:
        print(str(exc), file=sys.stderr)
        return 1

    if resp.status_code != 200:
        _print_error(resp)
        return 1

    data = resp.json()
    _print_response_text(data, show_json, endpoint_type)
    return 0


def invoke_claude(
    base_url: str,
    model: str,
    prompt: str,
    max_tokens: int,
    temperature: float,
    token: str | None,
    timeout: float,
    show_json: bool,
) -> int:
    try:
        provider, model_name = _split_model_reference(model)
    except ValueError as exc:
        print(str(exc), file=sys.stderr)
        return 1

    messages_payload: dict[str, Any] = {
        "model": model,
        "max_tokens": max_tokens,
        "messages": [
            {
                "role": "user",
                "content": prompt,
            }
        ],
    }
    if temperature >= 0:
        messages_payload["temperature"] = temperature

    attempts: list[tuple[str, Callable[[], requests.Response]]] = [
        (
            "messages",
            lambda: _post_json(
                f"{base_url.rstrip('/')}/v1/messages",
                headers=_build_headers(token, include_claude_header=True),
                payload=messages_payload,
                timeout=timeout,
            ),
        ),
        (
            "messages-fallback-chat-completions",
            lambda: _invoke_fallback_chat_completions(
                base_url=base_url,
                model=model,
                prompt=prompt,
                max_tokens=max_tokens,
                temperature=temperature,
                token=token,
                timeout=timeout,
            ),
        ),
        (
            "messages-fallback-invoke",
            lambda: _invoke_fallback_invoke_endpoint(
                base_url=base_url,
                provider=provider,
                model_name=model_name,
                prompt=prompt,
                max_tokens=max_tokens,
                temperature=temperature,
                token=token,
                timeout=timeout,
                use_messages=True,
            ),
        ),
        (
            "messages-fallback-route-invoke",
            lambda: _invoke_fallback_route_invoke(
                base_url=base_url,
                model_name=model_name,
                prompt=prompt,
                max_tokens=max_tokens,
                temperature=temperature,
                token=token,
                timeout=timeout,
                use_messages=True,
            ),
        ),
    ]
    try:
        resp, endpoint_type = _request_with_404_fallback(attempts)
    except RuntimeError as exc:
        print(str(exc), file=sys.stderr)
        return 1

    if resp.status_code != 200:
        _print_error(resp)
        return 1

    data = resp.json()
    _print_response_text(data, show_json, endpoint_type)
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="向 LLM Router 发起 Codex CLI / Claude Code 模型请求",
    )
    parser.add_argument(
        "--base-url",
        default=DEFAULT_BASE_URL,
        help=f"LLM Router 地址，默认: {DEFAULT_BASE_URL}",
    )
    parser.add_argument(
        "--token",
        default=None,
        help="可选。远程或启用认证时传入 API Key / Session Token",
    )
    parser.add_argument(
        "--timeout",
        type=float,
        default=DEFAULT_TIMEOUT,
        help=f"请求超时秒数，默认: {DEFAULT_TIMEOUT}",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        help="输出完整 JSON 响应（包含 endpoint_type）",
    )

    subparsers = parser.add_subparsers(dest="command", required=True)

    codex_parser = subparsers.add_parser("codex", help="请求 /v1/responses（Codex CLI 风格）")
    codex_parser.add_argument("--model", default=DEFAULT_CODEX_MODEL, help=f"模型引用，默认: {DEFAULT_CODEX_MODEL}")
    codex_parser.add_argument("--prompt", required=True, help="提示词")
    codex_parser.add_argument("--max-tokens", type=int, default=1024, help="最大输出 token")
    codex_parser.add_argument("--temperature", type=float, default=0.2, help="温度")

    claude_parser = subparsers.add_parser("claude", help="请求 /v1/messages（Claude Code 风格）")
    claude_parser.add_argument("--model", default=DEFAULT_CLAUDE_MODEL, help=f"模型引用，默认: {DEFAULT_CLAUDE_MODEL}")
    claude_parser.add_argument("--prompt", required=True, help="提示词")
    claude_parser.add_argument("--max-tokens", type=int, default=1024, help="最大输出 token")
    claude_parser.add_argument("--temperature", type=float, default=0.2, help="温度，传负数可跳过")

    all_parser = subparsers.add_parser("all", help="顺序请求 Codex + Claude")
    all_parser.add_argument("--codex-model", default=DEFAULT_CODEX_MODEL, help=f"Codex 模型，默认: {DEFAULT_CODEX_MODEL}")
    all_parser.add_argument("--claude-model", default=DEFAULT_CLAUDE_MODEL, help=f"Claude 模型，默认: {DEFAULT_CLAUDE_MODEL}")
    all_parser.add_argument("--prompt", required=True, help="两端共用的提示词")
    all_parser.add_argument("--max-tokens", type=int, default=1024, help="最大输出 token")
    all_parser.add_argument("--temperature", type=float, default=0.2, help="温度")

    return parser


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()

    if args.command == "codex":
        return invoke_codex(
            base_url=args.base_url,
            model=args.model,
            prompt=args.prompt,
            max_tokens=args.max_tokens,
            temperature=args.temperature,
            token=args.token,
            timeout=args.timeout,
            show_json=args.json,
        )

    if args.command == "claude":
        return invoke_claude(
            base_url=args.base_url,
            model=args.model,
            prompt=args.prompt,
            max_tokens=args.max_tokens,
            temperature=args.temperature,
            token=args.token,
            timeout=args.timeout,
            show_json=args.json,
        )

    if args.command == "all":
        print("=== Codex ===")
        codex_rc = invoke_codex(
            base_url=args.base_url,
            model=args.codex_model,
            prompt=args.prompt,
            max_tokens=args.max_tokens,
            temperature=args.temperature,
            token=args.token,
            timeout=args.timeout,
            show_json=args.json,
        )
        print("\n=== Claude ===")
        claude_rc = invoke_claude(
            base_url=args.base_url,
            model=args.claude_model,
            prompt=args.prompt,
            max_tokens=args.max_tokens,
            temperature=args.temperature,
            token=args.token,
            timeout=args.timeout,
            show_json=args.json,
        )
        return 0 if (codex_rc == 0 and claude_rc == 0) else 1

    parser.print_help()
    return 2


if __name__ == "__main__":
    raise SystemExit(main())
