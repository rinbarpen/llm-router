#!/usr/bin/env python3
"""
测试 OpenAI 直连 API 与 LLM Router API 调用

1. 直连 OpenAI：调用 api.openai.com/v1/chat/completions
2. 经 LLM Router：调用 localhost:18000/v1/chat/completions、/{provider}/v1/chat/completions 及 /models/{provider}/{model}/invoke
"""

import os
import sys
from pathlib import Path

# 加载项目根目录 .env
root = Path(__file__).resolve().parents[2]
env_path = root / ".env"
if env_path.exists():
    from dotenv import load_dotenv
    load_dotenv(env_path)

from curl_cffi import requests

# 配置
OPENAI_API_KEY = os.getenv("OPENAI_API_KEY")
LLM_ROUTER_BASE = os.getenv("LLM_ROUTER_BASE_URL", "http://localhost:18000")
# LLM Router 测试用模型：OpenAI 直连用 gpt-4o-mini，经 Router 用免费模型减少成本
OPENAI_DIRECT_MODEL = "gpt-4o-mini"
LLMROUTER_OPENAI_MODEL = "openai/gpt-4o-mini"  # 经 Router 调 OpenAI
LLMROUTER_FREE_MODEL = "openrouter/glm-4.5-air"  # 经 Router 调免费模型
INVOKE_PROVIDER = "openrouter"
INVOKE_MODEL = "glm-4.5-air"


def check_llm_router_health():
    """测试 LLM Router 健康检查"""
    print("\n--- 1. LLM Router 健康检查 ---")
    try:
        r = requests.get(f"{LLM_ROUTER_BASE}/health", timeout=5)
        r.raise_for_status()
        print(f"   GET {LLM_ROUTER_BASE}/health -> {r.status_code}")
        print(f"   响应: {r.json()}")
        return True
    except requests.RequestsError as e:
        print(f"   失败: {e}")
        return False


def check_llm_router_v1_chat(model: str, messages: list, max_tokens: int = 80):
    """测试 LLM Router OpenAI 兼容接口 POST /v1/chat/completions 及 /{provider}/v1/chat/completions"""
    print(f"\n--- 2. LLM Router chat completions (model={model}) ---")
    if "/" in model:
        provider, model_name = model.split("/", 1)
        url = f"{LLM_ROUTER_BASE}/{provider}/v1/chat/completions"
        payload = {"model": model_name, "messages": messages, "max_tokens": max_tokens, "temperature": 0.3}
    else:
        url = f"{LLM_ROUTER_BASE}/v1/chat/completions"
        payload = {"model": model, "messages": messages, "max_tokens": max_tokens, "temperature": 0.3}
    headers = {"Content-Type": "application/json"}
    try:
        r = requests.post(url, json=payload, headers=headers, timeout=60)
        r.raise_for_status()
        data = r.json()
        content = data.get("choices", [{}])[0].get("message", {}).get("content", "")
        usage = data.get("usage", {})
        print(f"   POST {url}")
        print(f"   回复: {content[:200]}{'...' if len(content) > 200 else ''}")
        print(f"   usage: {usage}")
        return True
    except requests.RequestsError as e:
        print(f"   失败: {e}")
        if hasattr(e, "response") and e.response is not None:
            print(f"   响应: {e.response.text[:500]}")
        return False


def check_llm_router_invoke():
    """测试 LLM Router 原生 invoke 接口"""
    print(f"\n--- 3. LLM Router /models/{{provider}}/{{model}}/invoke ---")
    url = f"{LLM_ROUTER_BASE}/models/{INVOKE_PROVIDER}/{INVOKE_MODEL}/invoke"
    payload = {
        "messages": [{"role": "user", "content": "Say hello in one short sentence."}],
        "parameters": {"temperature": 0.3, "max_tokens": 50},
    }
    headers = {"Content-Type": "application/json"}
    try:
        r = requests.post(url, json=payload, headers=headers, timeout=60)
        r.raise_for_status()
        data = r.json()
        text = data.get("output_text", "")
        usage = (data.get("raw") or {}).get("usage", {})
        print(f"   POST {url}")
        print(f"   output_text: {text[:200]}{'...' if len(text) > 200 else ''}")
        print(f"   usage: {usage}")
        return True
    except requests.RequestsError as e:
        print(f"   失败: {e}")
        if hasattr(e, "response") and e.response is not None:
            print(f"   响应: {e.response.text[:500]}")
        return False


def check_openai_direct():
    """直连 OpenAI API"""
    print("\n--- 4. 直连 OpenAI API (api.openai.com) ---")
    if not OPENAI_API_KEY or OPENAI_API_KEY.startswith("sk-..."):
        print("   跳过: 未配置有效 OPENAI_API_KEY（.env 中需填写真实 key）")
        return None
    url = "https://api.openai.com/v1/chat/completions"
    payload = {
        "model": OPENAI_DIRECT_MODEL,
        "messages": [{"role": "user", "content": "Reply with exactly: OK"}],
        "max_tokens": 20,
        "temperature": 0,
    }
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {OPENAI_API_KEY}",
    }
    try:
        r = requests.post(url, json=payload, headers=headers, timeout=30)
        r.raise_for_status()
        data = r.json()
        content = data.get("choices", [{}])[0].get("message", {}).get("content", "")
        usage = data.get("usage", {})
        print(f"   POST https://api.openai.com/v1/chat/completions")
        print(f"   回复: {content}")
        print(f"   usage: {usage}")
        return True
    except requests.RequestsError as e:
        print(f"   失败: {e}")
        if hasattr(e, "response") and e.response is not None:
            print(f"   响应: {e.response.text[:500]}")
        return False


def main():
    print("=" * 60)
    print("测试 OpenAI 与 LLM Router API 调用")
    print("=" * 60)

    ok_health = check_llm_router_health()
    if not ok_health:
        print("\nLLM Router 未就绪，请先启动: uv run llm-router")
        sys.exit(1)

    messages = [{"role": "user", "content": "Reply with exactly: OK"}]

    ok_v1_free = check_llm_router_v1_chat(LLMROUTER_FREE_MODEL, messages)
    ok_invoke = check_llm_router_invoke()
    ok_openai = check_openai_direct()

    if OPENAI_API_KEY and OPENAI_API_KEY != "sk-...":
        ok_v1_openai = check_llm_router_v1_chat(LLMROUTER_OPENAI_MODEL, messages)
    else:
        ok_v1_openai = None

    print("\n" + "=" * 60)
    print("结果汇总")
    print("=" * 60)
    print(f"  LLM Router 健康检查: {'通过' if ok_health else '失败'}")
    print(f"  LLM Router /v1/chat (免费模型): {'通过' if ok_v1_free else '失败'}")
    print(f"  LLM Router /models/.../invoke: {'通过' if ok_invoke else '失败'}")
    if ok_v1_openai is not None:
        print(f"  LLM Router /v1/chat (OpenAI 模型): {'通过' if ok_v1_openai else '失败'}")
    print(f"  直连 OpenAI: {'通过' if ok_openai else '跳过' if ok_openai is None else '失败'}")

    failed = [x for x in [ok_health, ok_v1_free, ok_invoke] if not x]
    if ok_v1_openai is False:
        failed.append(False)
    if ok_openai is False:
        failed.append(False)
    sys.exit(1 if failed else 0)


if __name__ == "__main__":
    main()
