#!/usr/bin/env python3
"""
一键测试 Codex CLI、Claude Code、OpenRouter 免费模型。
输出汇总表格：模型 | 状态 | 响应摘要
"""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path

SCRIPT_DIR = Path(__file__).resolve().parent
PROJECT_ROOT = SCRIPT_DIR.parent.parent


def run_codex_test() -> tuple[str, str, str]:
    """测试 Codex CLI"""
    try:
        result = subprocess.run(
            [
                sys.executable,
                str(SCRIPT_DIR / "request_codex_claude.py"),
                "--timeout",
                "30",
                "codex",
                "--model",
                "codex_cli/gpt-5.3-codex",
                "--prompt",
                "Say hello in one word.",
            ],
            cwd=PROJECT_ROOT,
            capture_output=True,
            text=True,
            timeout=35,
        )
        if result.returncode == 0:
            out = (result.stdout or "").strip()[:80]
            return "codex_cli/gpt-5.3-codex", "OK", out or "(empty)"
        err = (result.stderr or result.stdout or "").strip()
        return "codex_cli/gpt-5.3-codex", "FAIL", err[:80] if err else "timeout/error"
    except subprocess.TimeoutExpired:
        return "codex_cli/gpt-5.3-codex", "FAIL", "timeout"
    except Exception as e:
        return "codex_cli/gpt-5.3-codex", "FAIL", str(e)[:80]


def run_claude_test() -> tuple[str, str, str]:
    """测试 Claude Code"""
    try:
        result = subprocess.run(
            [
                sys.executable,
                str(SCRIPT_DIR / "request_codex_claude.py"),
                "--timeout",
                "30",
                "claude",
                "--model",
                "claude_code_cli/claude-sonnet-4-5",
                "--prompt",
                "Say hello in one word.",
            ],
            cwd=PROJECT_ROOT,
            capture_output=True,
            text=True,
            timeout=35,
        )
        if result.returncode == 0:
            out = (result.stdout or "").strip()[:80]
            return "claude_code_cli/claude-sonnet-4-5", "OK", out or "(empty)"
        err = (result.stderr or result.stdout or "").strip()
        return "claude_code_cli/claude-sonnet-4-5", "FAIL", err[:80] if err else "error"
    except subprocess.TimeoutExpired:
        return "claude_code_cli/claude-sonnet-4-5", "FAIL", "timeout"
    except Exception as e:
        return "claude_code_cli/claude-sonnet-4-5", "FAIL", str(e)[:80]


def run_openrouter_sample() -> tuple[str, str, str]:
    """测试 OpenRouter 免费模型样本（glm-4.5-air 或 nemotron-nano-9b-v2）"""
    try:
        import json

        from curl_cffi import requests

        # 使用 /{provider}/v1/chat/completions，model 只需模型名
        url = "http://localhost:18000/openrouter/v1/chat/completions"
        for model in ["nemotron-nano-9b-v2", "glm-4.5-air"]:
            try:
                resp = requests.post(
                    url,
                    json={
                        "model": model,  # provider 在路径中，model 只需模型名
                        "messages": [{"role": "user", "content": "Say hello"}],
                        "max_tokens": 10,
                    },
                    timeout=15,
                )
                if resp.status_code == 200:
                    data = resp.json()
                    content = (
                        data.get("choices", [{}])[0]
                        .get("message", {})
                        .get("content", "")
                        .strip()[:50]
                    )
                    return f"openrouter/{model}", "OK", content or "(empty)"
            except Exception:
                continue
        return "openrouter/*", "FAIL", "no working free model"
    except ImportError:
        return "openrouter/*", "SKIP", "curl_cffi not installed"
    except Exception as e:
        return "openrouter/*", "FAIL", str(e)[:80]


def main() -> int:
    print("=" * 70)
    print("Codex CLI / Claude Code / OpenRouter 模型测试")
    print("=" * 70)

    rows: list[tuple[str, str, str]] = []
    rows.append(run_codex_test())
    rows.append(run_claude_test())
    rows.append(run_openrouter_sample())

    print()
    print(f"{'模型':<45} {'状态':<8} 响应摘要")
    print("-" * 70)
    for model, status, summary in rows:
        print(f"{model:<45} {status:<8} {summary}")

    print()
    ok = sum(1 for _, s, _ in rows if s == "OK")
    print(f"通过: {ok}/{len(rows)}")
    return 0 if ok > 0 else 1


if __name__ == "__main__":
    sys.exit(main())
