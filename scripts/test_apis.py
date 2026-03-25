#!/usr/bin/env python3
"""LLM Router API smoke checks; optional JSON report (stdlib only)."""

from __future__ import annotations

import argparse
import json
import os
import ssl
import sys
import urllib.error
import urllib.request
from typing import Any


def _full_url(base_url: str, path: str) -> str:
    path = path.strip()
    if path.startswith("http://") or path.startswith("https://"):
        return path
    return base_url.rstrip("/") + "/" + path.lstrip("/")


def _one_request(
    method: str,
    url: str,
    *,
    data: bytes | None,
    headers: dict[str, str],
    timeout: float,
) -> tuple[int, str]:
    ctx = ssl.create_default_context()
    req = urllib.request.Request(url, data=data, method=method.upper(), headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=timeout, context=ctx) as resp:
            return resp.status, f"HTTP {resp.status}"
    except urllib.error.HTTPError as e:
        body = e.read().decode("utf-8", errors="replace")[:500]
        detail = f"HTTP {e.code}"
        if body:
            detail += f": {body}"
        return e.code, detail
    except urllib.error.URLError as e:
        return -1, str(e.reason if hasattr(e, "reason") else e)


def main() -> int:
    p = argparse.ArgumentParser(description="Smoke-test LLM Router HTTP API.")
    p.add_argument("--json-report", metavar="FILE", help="Write results as JSON to this path.")
    p.add_argument("--timeout", type=float, default=60.0, help="Per-request timeout seconds (default: 60).")
    p.add_argument(
        "--base-url",
        default=os.environ.get("LLM_ROUTER_BASE_URL", "http://127.0.0.1:18000"),
        help="Router base URL (default: env LLM_ROUTER_BASE_URL or http://127.0.0.1:18000).",
    )
    args = p.parse_args()

    base = args.base_url.rstrip("/")
    timeout = args.timeout
    api_key = (os.environ.get("LLM_ROUTER_API_KEY") or "").strip()
    smoke_model = (os.environ.get("LLM_ROUTER_SMOKE_MODEL") or "openai/gpt-4o-mini").strip()

    def auth_headers(extra: dict[str, str] | None = None) -> dict[str, str]:
        h = {"User-Agent": "llm-router-test_apis/1"}
        if extra:
            h.update(extra)
        if api_key:
            h["Authorization"] = f"Bearer {api_key}"
        return h

    route_body = json.dumps(
        {"model_hint": smoke_model, "messages": [{"role": "user", "content": "hi"}]}
    ).encode("utf-8")

    cases: list[dict[str, Any]] = [
        {"name": "health", "method": "GET", "path": "/health", "headers": auth_headers()},
        {"name": "openai_list_models", "method": "GET", "path": "/v1/models", "headers": auth_headers()},
        {"name": "list_providers", "method": "GET", "path": "/providers", "headers": auth_headers()},
        {
            "name": "provider_supported_models",
            "method": "GET",
            "path": "/providers/openai/supported-models",
            "headers": auth_headers(),
        },
        {"name": "list_models_by_provider", "method": "GET", "path": "/models/openai", "headers": auth_headers()},
        {"name": "list_models", "method": "GET", "path": "/models", "headers": auth_headers()},
        {"name": "route_pairs", "method": "GET", "path": "/route/pairs", "headers": auth_headers()},
        {
            "name": "route_decision",
            "method": "POST",
            "path": "/route",
            "headers": auth_headers({"Content-Type": "application/json"}),
            "body": route_body,
        },
        {"name": "pricing_latest", "method": "GET", "path": "/pricing/latest", "headers": auth_headers()},
        {"name": "pricing_suggestions", "method": "GET", "path": "/pricing/suggestions", "headers": auth_headers()},
        {
            "name": "monitor_statistics",
            "method": "GET",
            "path": "/monitor/statistics?time_range_hours=24&limit=10",
            "headers": auth_headers(),
        },
        {
            "name": "monitor_invocations",
            "method": "GET",
            "path": "/monitor/invocations?limit=10&offset=0",
            "headers": auth_headers(),
        },
        {
            "name": "monitor_time_series",
            "method": "GET",
            "path": "/monitor/time-series?granularity=day&time_range_hours=24",
            "headers": auth_headers(),
        },
    ]

    results: list[dict[str, Any]] = []
    for c in cases:
        url = _full_url(base, c["path"])
        method = c["method"]
        headers = dict(c["headers"])
        body: bytes | None = c.get("body")
        code, detail = _one_request(method, url, data=body, headers=headers, timeout=timeout)

        status: str
        if c["name"] == "monitor_time_series" and code == 501:
            status = "skip"
            detail = "HTTP 501: endpoint not implemented yet in Go backend"
        elif 200 <= code < 300:
            status = "ok"
        else:
            status = "fail"

        results.append(
            {
                "name": c["name"],
                "method": method,
                "path": url,
                "status": status,
                "detail": detail,
            }
        )

    if api_key:
        login_url = _full_url(base, "/auth/login")
        login_body = json.dumps({"api_key": api_key}).encode("utf-8")
        h = auth_headers({"Content-Type": "application/json"})
        code, detail = _one_request("POST", login_url, data=login_body, headers=h, timeout=timeout)
        results.append(
            {
                "name": "auth_login",
                "method": "POST",
                "path": login_url,
                "status": "ok" if 200 <= code < 300 else "fail",
                "detail": detail,
            }
        )
    else:
        results.append(
            {
                "name": "auth_login",
                "method": "POST",
                "path": "/auth/login",
                "status": "skip",
                "detail": "未设置 API Key",
            }
        )

    summary = {
        "ok": sum(1 for r in results if r["status"] == "ok"),
        "fail": sum(1 for r in results if r["status"] == "fail"),
        "skip": sum(1 for r in results if r["status"] == "skip"),
    }
    out: dict[str, Any] = {"base_url": base, "summary": summary, "results": results}

    if args.json_report:
        with open(args.json_report, "w", encoding="utf-8") as f:
            json.dump(out, f, ensure_ascii=False, indent=2)
            f.write("\n")

    for r in results:
        print(f"[{r['status']}] {r['name']}: {r['detail']}")
    print(f"summary: ok={summary['ok']} fail={summary['fail']} skip={summary['skip']}")

    return 1 if summary["fail"] else 0


if __name__ == "__main__":
    sys.exit(main())
