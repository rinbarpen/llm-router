#!/usr/bin/env python3
"""测试 OpenAI 兼容 API"""

import os
import requests
from dotenv import load_dotenv
load_dotenv()

BASE_URL = "http://localhost:18000"
# 从环境变量获取管理 API Key
ADMIN_API_KEY = os.getenv("LLM_ROUTER_ADMIN_KEY")
# 优先使用免费模型（OpenRouter）与 Gemini 2.5 Flash
FREE_MODEL = "openrouter/openrouter-llama-3.3-70b-instruct"
GEMINI_MODEL = "gemini/gemini-2.5-flash"

def test_health():
    """测试健康检查"""
    print("1. 测试健康检查...")
    response = requests.get(f"{BASE_URL}/health")
    print(f"   状态码: {response.status_code}")
    print(f"   响应: {response.json()}")
    assert response.status_code == 200
    print("   ✓ 健康检查通过\n")

def test_login_with_model():
    """测试登录并选择模型"""
    print("2. 测试登录（带模型选择）...")
    if not ADMIN_API_KEY:
        print("   未设置 LLM_ROUTER_ADMIN_KEY，跳过登录，直接匿名调用\n")
        return None

    response = requests.post(
        f"{BASE_URL}/auth/login",
        headers={"Authorization": f"Bearer {ADMIN_API_KEY}"},
        json={
            "provider_name": "openrouter",
            "model_name": "openrouter-llama-3.3-70b-instruct"
        }
    )
    print(f"   状态码: {response.status_code}")
    print(f"   响应: {response.text}")
    if response.status_code == 200:
        data = response.json()
        print(f"   Token: {data.get('token', 'N/A')[:20]}...")
        print(f"   模型: {data.get('provider_name')}/{data.get('model_name')}")
        return data.get('token')
    return None

def test_openai_compatible_api(token=None):
    """测试 OpenAI 兼容的 API"""
    print("3. 测试 OpenAI 兼容 API...")
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    
    payload = {
        # 默认使用 OpenRouter 免费模型，减少付费依赖
        "model": FREE_MODEL,
        "messages": [
            {"role": "user", "content": "Say 'Hello, OpenAI compatible API!'"}
        ],
        "temperature": 0.7,
        "max_tokens": 50
    }
    
    print(f"   请求: POST {BASE_URL}/v1/chat/completions")
    print(f"   模型: {payload['model']}")
    print(f"   消息: {payload['messages'][0]['content']}")
    
    try:
        response = requests.post(
            f"{BASE_URL}/v1/chat/completions",
            json=payload,
            headers=headers,
            timeout=30
        )
        print(f"   状态码: {response.status_code}")
        if response.status_code == 200:
            data = response.json()
            print(f"   响应 ID: {data.get('id', 'N/A')}")
            print(f"   模型: {data.get('model', 'N/A')}")
            if data.get('choices'):
                content = data['choices'][0].get('message', {}).get('content', 'N/A')
                print(f"   回复: {content}")
            if data.get('usage'):
                usage = data['usage']
                print(f"   Token 使用: {usage.get('total_tokens', 0)} (prompt: {usage.get('prompt_tokens', 0)}, completion: {usage.get('completion_tokens', 0)})")
            print("   ✓ OpenAI 兼容 API 测试通过\n")
        else:
            print(f"   错误: {response.text}")
            print("   ✗ 测试失败\n")
    except requests.exceptions.Timeout:
        print("   ⚠ 请求超时（可能是实际调用了 OpenAI API，需要更长时间）\n")
    except Exception as e:
        print(f"   ✗ 错误: {e}\n")

def test_openai_without_model_in_request(token=None):
    """测试不指定模型（从 session 获取）"""
    print("4. 测试从 session 获取模型...")
    if not token:
        print("   跳过（需要先登录并选择模型）\n")
        return
    
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    
    payload = {
        "messages": [
            {"role": "user", "content": "Say 'Hi from session model!'"}
        ],
        "max_tokens": 30
    }
    
    print(f"   请求: POST {BASE_URL}/v1/chat/completions（不指定 model）")
    
    try:
        response = requests.post(
            f"{BASE_URL}/v1/chat/completions",
            json=payload,
            headers=headers,
            timeout=30
        )
        print(f"   状态码: {response.status_code}")
        if response.status_code == 200:
            data = response.json()
            print(f"   使用的模型: {data.get('model', 'N/A')}")
            if data.get('choices'):
                content = data['choices'][0].get('message', {}).get('content', 'N/A')
                print(f"   回复: {content}")
            print("   ✓ 从 session 获取模型测试通过\n")
        else:
            print(f"   错误: {response.text}\n")
    except requests.exceptions.Timeout:
        print("   ⚠ 请求超时\n")
    except Exception as e:
        print(f"   ✗ 错误: {e}\n")

if __name__ == "__main__":
    print("=" * 60)
    print("OpenAI 兼容 API 测试")
    print("=" * 60 + "\n")
    
    try:
        test_health()
        token = test_login_with_model()
        test_openai_compatible_api(token)
        test_openai_without_model_in_request(token)
        
        print("=" * 60)
        print("测试完成！")
        print("=" * 60)
    except Exception as e:
        print(f"\n测试出错: {e}")
        import traceback
        traceback.print_exc()

