#!/usr/bin/env python3
"""对已启动的 LLM Router（Python Starlette）做 API 冒烟测试。

默认：只读/轻量 POST，不调用上游模型。
可选 --with-llm：调用 chat completions 与 invoke（产生费用/耗时）。

环境变量（可被 CLI 覆盖）：
  LLM_ROUTER_BASE_URL   默认 http://localhost:18000
  LLM_ROUTER_API_KEY    可选 Bearer
  LLM_ROUTER_TEST_MODEL 可选，格式 provider/model；未设时从仓库 router.toml 取第一个 [[models]]
"""
from __future__ import annotations

import argparse
import asyncio
import json
import os
import sys
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Literal

import tomli
from curl_cffi.requests import AsyncSession
from dotenv import load_dotenv

REPO_ROOT = Path(__file__).resolve().parent.parent
ROUTER_TOML = REPO_ROOT / "router.toml"

ResultStatus = Literal["ok", "fail", "skip"]


@dataclass
class CaseResult:
    name: str
    method: str
    path: str
    status: ResultStatus
    detail: str = ""


@dataclass
class RunState:
    first_provider: str | None = None
    results: list[CaseResult] = field(default_factory=list)

    def add(
        self,
        name: str,
        method: str,
        path: str,
        status: ResultStatus,
        detail: str = "",
    ) -> None:
        self.results.append(CaseResult(name, method, path, status, detail))


def load_dotenv_and_env() -> None:
    load_dotenv(REPO_ROOT / ".env")
    load_dotenv()


def auth_headers(api_key: str | None) -> dict[str, str]:
    h: dict[str, str] = {}
    if api_key:
        h["Authorization"] = f"Bearer {api_key}"
    return h


def first_model_from_toml() -> tuple[str, str] | None:
    if not ROUTER_TOML.exists():
        return None
    with ROUTER_TOML.open("rb") as f:
        cfg = tomli.load(f)
    for m in cfg.get("models", []) or []:
        prov = m.get("provider")
        name = m.get("name")
        if prov and name:
            return str(prov), str(name)
    return None


def resolve_test_model(explicit: str | None) -> tuple[str, str] | None:
    if explicit:
        if "/" not in explicit:
            return None
        p, n = explicit.split("/", 1)
        if p and n:
            return p, n
        return None
    env_m = os.getenv("LLM_ROUTER_TEST_MODEL", "").strip()
    if env_m and "/" in env_m:
        p, n = env_m.split("/", 1)
        if p and n:
            return p, n
    return first_model_from_toml()


def _snippet(text: str, max_len: int = 200) -> str:
    t = text.strip().replace("\n", " ")
    if len(t) <= max_len:
        return t
    return t[: max_len - 3] + "..."


async def _check_json_response(
    client: AsyncSession,
    state: RunState,
    name: str,
    method: str,
    url: str,
    *,
    headers: dict[str, str],
    timeout: float,
    json_body: Any | None = None,
    expect_2xx: bool = True,
) -> None:
    try:
        if method.upper() == "GET":
            resp = await client.get(url, headers=headers, timeout=timeout)
        else:
            resp = await client.post(url, headers=headers, json=json_body, timeout=timeout)
    except Exception as exc:
        state.add(name, method, url, "fail", f"请求异常: {exc}")
        return

    rel = url
    if expect_2xx and not (200 <= resp.status_code < 300):
        body = _snippet(resp.text or "")
        try:
            err = resp.json()
            if isinstance(err, dict) and err.get("detail") is not None:
                body = str(err.get("detail"))
        except Exception:
            pass
        state.add(name, method, rel, "fail", f"HTTP {resp.status_code}: {body}")
        return

    if not expect_2xx:
        state.add(name, method, rel, "ok", f"HTTP {resp.status_code}")
        return

    try:
        resp.json()
    except Exception:
        state.add(name, method, rel, "fail", "响应不是合法 JSON")
        return

    state.add(name, method, rel, "ok", f"HTTP {resp.status_code}")


def _path_only(base: str, path: str) -> str:
    return f"{base.rstrip('/')}{path}"


async def run_quick(
    client: AsyncSession,
    base_url: str,
    headers: dict[str, str],
    timeout: float,
    state: RunState,
) -> None:
    b = base_url.rstrip("/")

    await _check_json_response(
        client,
        state,
        "health",
        "GET",
        _path_only(b, "/health"),
        headers=headers,
        timeout=timeout,
    )

    await _check_json_response(
        client,
        state,
        "openai_list_models",
        "GET",
        _path_only(b, "/v1/models"),
        headers=headers,
        timeout=timeout,
    )

    url_providers = _path_only(b, "/providers")
    try:
        resp = await client.get(url_providers, headers=headers, timeout=timeout)
    except Exception as exc:
        state.add("list_providers", "GET", url_providers, "fail", f"请求异常: {exc}")
    else:
        if not (200 <= resp.status_code < 300):
            state.add(
                "list_providers",
                "GET",
                url_providers,
                "fail",
                f"HTTP {resp.status_code}: {_snippet(resp.text)}",
            )
        else:
            try:
                data = resp.json()
                if isinstance(data, list) and data:
                    first = data[0]
                    if isinstance(first, dict) and first.get("name"):
                        state.first_provider = str(first["name"])
                state.add("list_providers", "GET", url_providers, "ok", "HTTP 200")
            except Exception:
                state.add("list_providers", "GET", url_providers, "fail", "响应不是合法 JSON")

    if state.first_provider:
        sup_url = _path_only(b, f"/providers/{state.first_provider}/supported-models")
        await _check_json_response(
            client,
            state,
            "provider_supported_models",
            "GET",
            sup_url,
            headers=headers,
            timeout=timeout,
        )
        models_p_url = _path_only(b, f"/models/{state.first_provider}")
        await _check_json_response(
            client,
            state,
            "list_models_by_provider",
            "GET",
            models_p_url,
            headers=headers,
            timeout=timeout,
        )
    else:
        state.add(
            "provider_supported_models",
            "GET",
            "/providers/{name}/supported-models",
            "skip",
            "无 provider 可测",
        )
        state.add(
            "list_models_by_provider",
            "GET",
            "/models/{provider_name}",
            "skip",
            "无 provider 可测",
        )

    await _check_json_response(
        client,
        state,
        "list_models",
        "GET",
        _path_only(b, "/models"),
        headers=headers,
        timeout=timeout,
    )

    await _check_json_response(
        client,
        state,
        "route_pairs",
        "GET",
        _path_only(b, "/route/pairs"),
        headers=headers,
        timeout=timeout,
    )

    await _check_json_response(
        client,
        state,
        "route_decision",
        "POST",
        _path_only(b, "/route"),
        headers=headers,
        timeout=timeout,
        json_body={},
    )

    await _check_json_response(
        client,
        state,
        "pricing_latest",
        "GET",
        _path_only(b, "/pricing/latest"),
        headers=headers,
        timeout=timeout,
    )
    await _check_json_response(
        client,
        state,
        "pricing_suggestions",
        "GET",
        _path_only(b, "/pricing/suggestions"),
        headers=headers,
        timeout=timeout,
    )

    await _check_json_response(
        client,
        state,
        "monitor_statistics",
        "GET",
        _path_only(b, "/monitor/statistics?time_range_hours=24&limit=10"),
        headers=headers,
        timeout=timeout,
    )
    await _check_json_response(
        client,
        state,
        "monitor_invocations",
        "GET",
        _path_only(b, "/monitor/invocations?limit=10&offset=0"),
        headers=headers,
        timeout=timeout,
    )
    await _check_json_response(
        client,
        state,
        "monitor_time_series",
        "GET",
        _path_only(b, "/monitor/time-series?granularity=day&time_range_hours=24"),
        headers=headers,
        timeout=timeout,
    )


async def run_auth_login(
    client: AsyncSession,
    base_url: str,
    api_key: str,
    headers: dict[str, str],
    timeout: float,
    state: RunState,
) -> None:
    b = base_url.rstrip("/")
    url = _path_only(b, "/auth/login")
    try:
        resp = await client.post(
            url,
            headers=headers,
            json={"api_key": api_key},
            timeout=timeout,
        )
    except Exception as exc:
        state.add("auth_login", "POST", url, "fail", f"请求异常: {exc}")
        return
    if not (200 <= resp.status_code < 300):
        state.add(
            "auth_login",
            "POST",
            url,
            "fail",
            f"HTTP {resp.status_code}: {_snippet(resp.text)}",
        )
        return
    try:
        data = resp.json()
        if not isinstance(data, dict) or "token" not in data:
            state.add("auth_login", "POST", url, "fail", "响应缺少 token 字段")
            return
    except Exception:
        state.add("auth_login", "POST", url, "fail", "响应不是合法 JSON")
        return
    state.add("auth_login", "POST", url, "ok", "HTTP 200，已返回 token")


async def run_llm(
    client: AsyncSession,
    base_url: str,
    headers: dict[str, str],
    timeout: float,
    provider: str,
    model: str,
    state: RunState,
) -> None:
    b = base_url.rstrip("/")
    model_id = f"{provider}/{model}"

    chat_url = _path_only(b, "/v1/chat/completions")
    payload = {
        "model": model_id,
        "messages": [{"role": "user", "content": "Reply with OK only."}],
        "stream": False,
        "max_tokens": 16,
        "temperature": 0,
    }
    try:
        resp = await client.post(
            chat_url, headers=headers, json=payload, timeout=timeout
        )
    except Exception as exc:
        state.add("v1_chat_completions", "POST", chat_url, "fail", f"请求异常: {exc}")
    else:
        if not (200 <= resp.status_code < 300):
            state.add(
                "v1_chat_completions",
                "POST",
                chat_url,
                "fail",
                f"HTTP {resp.status_code}: {_snippet(resp.text)}",
            )
        else:
            try:
                data = resp.json()
                choices = data.get("choices") if isinstance(data, dict) else None
                if not choices:
                    state.add(
                        "v1_chat_completions",
                        "POST",
                        chat_url,
                        "fail",
                        "响应缺少 choices",
                    )
                else:
                    state.add("v1_chat_completions", "POST", chat_url, "ok", "HTTP 200")
            except Exception:
                state.add(
                    "v1_chat_completions",
                    "POST",
                    chat_url,
                    "fail",
                    "响应不是合法 JSON",
                )

    inv_url = _path_only(b, f"/models/{provider}/{model}/invoke")
    inv_payload = {
        "prompt": "Hi, please respond with 'OK' only.",
        "parameters": {"max_tokens": 16, "temperature": 0.0},
    }
    try:
        resp = await client.post(
            inv_url, headers=headers, json=inv_payload, timeout=timeout
        )
    except Exception as exc:
        state.add("invoke_model", "POST", inv_url, "fail", f"请求异常: {exc}")
    else:
        if not (200 <= resp.status_code < 300):
            state.add(
                "invoke_model",
                "POST",
                inv_url,
                "fail",
                f"HTTP {resp.status_code}: {_snippet(resp.text)}",
            )
        else:
            try:
                data = resp.json()
                if not isinstance(data, dict) or "output_text" not in data:
                    state.add(
                        "invoke_model",
                        "POST",
                        inv_url,
                        "fail",
                        "响应缺少 output_text",
                    )
                else:
                    state.add("invoke_model", "POST", inv_url, "ok", "HTTP 200")
            except Exception:
                state.add(
                    "invoke_model",
                    "POST",
                    inv_url,
                    "fail",
                    "响应不是合法 JSON",
                )

    prov_chat = _path_only(b, f"/{provider}/v1/chat/completions")
    prov_payload = {
        "model": model,
        "messages": [{"role": "user", "content": "Reply with OK only."}],
        "stream": False,
        "max_tokens": 16,
        "temperature": 0,
    }
    try:
        resp = await client.post(
            prov_chat, headers=headers, json=prov_payload, timeout=timeout
        )
    except Exception as exc:
        state.add(
            "provider_path_chat",
            "POST",
            prov_chat,
            "fail",
            f"请求异常: {exc}",
        )
    else:
        if not (200 <= resp.status_code < 300):
            state.add(
                "provider_path_chat",
                "POST",
                prov_chat,
                "fail",
                f"HTTP {resp.status_code}: {_snippet(resp.text)}",
            )
        else:
            try:
                data = resp.json()
                if not isinstance(data, dict) or not data.get("choices"):
                    state.add(
                        "provider_path_chat",
                        "POST",
                        prov_chat,
                        "fail",
                        "响应缺少 choices",
                    )
                else:
                    state.add(
                        "provider_path_chat",
                        "POST",
                        prov_chat,
                        "ok",
                        "HTTP 200",
                    )
            except Exception:
                state.add(
                    "provider_path_chat",
                    "POST",
                    prov_chat,
                    "fail",
                    "响应不是合法 JSON",
                )


def _display_path(base_url: str, url: str) -> str:
    prefix = base_url.rstrip("/") + "/"
    if url.startswith(prefix):
        return "/" + url[len(prefix) :]
    return url


def print_table(state: RunState, base_url: str) -> None:
    disp = [_display_path(base_url, r.path) for r in state.results]
    name_w = max(len(r.name) for r in state.results) if state.results else 10
    path_w = max(len(p) for p in disp) if disp else 20
    name_w = min(max(name_w, 12), 40)
    path_w = min(max(path_w, 24), 72)
    print()
    hdr = f"{'名称':<{name_w}}  {'方法':<6}  {'路径':<{path_w}}  {'结果':<6}  说明"
    print(hdr)
    print("-" * len(hdr))
    for r, p in zip(state.results, disp, strict=True):
        print(
            f"{r.name:<{name_w}}  {r.method:<6}  {p:<{path_w}}  {r.status.upper():<6}  {r.detail}"
        )


def print_footer() -> None:
    print()
    print(
        "未自动测试（避免副作用或需特殊资源）: "
        "变更类 /providers、/models、/api-keys、/config/sync、/pricing/sync*；"
        "WebSocket /v1/realtime；OAuth /auth/oauth/*；"
        "multipart 音频等。"
    )


async def amain() -> int:
    parser = argparse.ArgumentParser(description="LLM Router API 冒烟测试")
    parser.add_argument(
        "--base-url",
        default=os.getenv("LLM_ROUTER_BASE_URL", "http://localhost:18000"),
        help="服务根 URL",
    )
    parser.add_argument(
        "--api-key",
        default=os.getenv("LLM_ROUTER_API_KEY", "") or None,
        help="Bearer API Key（默认读环境变量 LLM_ROUTER_API_KEY）",
    )
    parser.add_argument(
        "--with-llm",
        action="store_true",
        help="调用上游：/v1/chat/completions、invoke、/{provider}/v1/chat/completions",
    )
    parser.add_argument(
        "--test-model",
        default=None,
        help="覆盖测试用模型，格式 provider/model（默认 LLM_ROUTER_TEST_MODEL 或 router.toml 首条）",
    )
    parser.add_argument(
        "--json-report",
        default=None,
        metavar="PATH",
        help="将结果写入 JSON 文件",
    )
    parser.add_argument(
        "--timeout",
        type=float,
        default=120.0,
        help="单次请求超时（秒）",
    )
    args = parser.parse_args()

    load_dotenv_and_env()
    base_url = args.base_url.rstrip("/")
    api_key = args.api_key
    headers = auth_headers(api_key)

    state = RunState()

    async with AsyncSession(trust_env=True) as client:
        await run_quick(client, base_url, headers, args.timeout, state)

        if api_key:
            await run_auth_login(
                client, base_url, api_key, headers, args.timeout, state
            )
        else:
            state.add(
                "auth_login",
                "POST",
                "/auth/login",
                "skip",
                "未设置 API Key",
            )

        if args.with_llm:
            resolved = resolve_test_model(args.test_model)
            if not resolved:
                state.add(
                    "with_llm",
                    "-",
                    "-",
                    "fail",
                    "无法解析测试模型：请设置 --test-model provider/model 或 LLM_ROUTER_TEST_MODEL，"
                    "并确保 router.toml 存在且含 [[models]]",
                )
            else:
                prov, mname = resolved
                await run_llm(
                    client, base_url, headers, args.timeout, prov, mname, state
                )

    print("=" * 60)
    print("LLM Router API 冒烟测试")
    print("=" * 60)
    print(f"BASE_URL: {base_url}")
    print_table(state, base_url)
    print_footer()

    ok = sum(1 for r in state.results if r.status == "ok")
    fail = sum(1 for r in state.results if r.status == "fail")
    skip = sum(1 for r in state.results if r.status == "skip")
    print()
    print(f"汇总: 通过 {ok}  失败 {fail}  跳过 {skip}")

    if args.json_report:
        out_path = Path(args.json_report)
        out_path.write_text(
            json.dumps(
                {
                    "base_url": base_url,
                    "summary": {"ok": ok, "fail": fail, "skip": skip},
                    "results": [
                        {
                            "name": r.name,
                            "method": r.method,
                            "path": r.path,
                            "status": r.status,
                            "detail": r.detail,
                        }
                        for r in state.results
                    ],
                },
                ensure_ascii=False,
                indent=2,
            ),
            encoding="utf-8",
        )
        print(f"JSON 报告: {out_path}")

    return 1 if fail else 0


def main() -> None:
    raise SystemExit(asyncio.run(amain()))


if __name__ == "__main__":
    main()
