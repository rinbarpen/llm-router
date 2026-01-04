#!/usr/bin/env python3
"""
智能路由示例

演示如何使用智能路由功能，根据标签和 Provider 类型自动选择最佳模型。
"""

import os
from curl_cffi import requests
from dotenv import load_dotenv

# 加载环境变量
load_dotenv()

# 配置
BASE_URL = os.getenv("LLM_ROUTER_BASE_URL", "http://localhost:18000")
API_KEY = os.getenv("LLM_ROUTER_API_KEY")  # 可选，远程请求时需要


def route_by_tags(tags, provider_types=None, prompt=None, messages=None):
    """根据标签路由请求"""
    url = f"{BASE_URL}/route/invoke"
    
    headers = {"Content-Type": "application/json"}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    
    query = {
        "tags": tags if isinstance(tags, list) else [tags]
    }
    
    if provider_types:
        query["provider_types"] = provider_types if isinstance(provider_types, list) else [provider_types]
    
    request_payload = {}
    if prompt:
        request_payload["prompt"] = prompt
    elif messages:
        request_payload["messages"] = messages
    else:
        print("✗ 错误: 需要提供 prompt 或 messages")
        return None
    
    request_payload["parameters"] = {
        "temperature": 0.7,
        "max_tokens": 200
    }
    
    payload = {
        "query": query,
        "request": request_payload
    }
    
    print(f"智能路由请求")
    print(f"  标签: {tags}")
    if provider_types:
        print(f"  Provider 类型: {provider_types}")
    print(f"  提示词: {prompt or messages[0]['content'][:50]}...")
    
    try:
        response = requests.post(url, json=payload, headers=headers, timeout=60)
        response.raise_for_status()
        
        data = response.json()
        
        print(f"✓ 路由成功")
        print(f"输出: {data.get('output_text', 'N/A')}")
        
        # 显示实际使用的模型（从 raw 响应中获取）
        if 'raw' in data:
            raw = data['raw']
            if 'model' in raw:
                print(f"使用的模型: {raw['model']}")
        
        return data
        
    except requests.RequestsError as e:
        print(f"✗ 请求失败: {e}")
        if hasattr(e, 'response') and e.response is not None:
            print(f"响应: {e.response.text}")
        return None
    except Exception as e:
        print(f"✗ 发生错误: {e}")
        return None


def route_by_provider(provider_types, prompt=None, messages=None):
    """根据 Provider 类型路由请求"""
    return route_by_tags([], provider_types=provider_types, prompt=prompt, messages=messages)


def route_free_fast(prompt=None, messages=None):
    """路由到免费快速模型"""
    return route_by_tags(["free", "fast"], prompt=prompt, messages=messages)


def route_chinese(prompt=None, messages=None):
    """路由到中文模型"""
    return route_by_tags(["chinese"], prompt=prompt, messages=messages)


if __name__ == "__main__":
    print("=" * 60)
    print("LLM Router 智能路由示例")
    print("=" * 60)
    print()
    
    # 示例 1: 根据标签路由（免费快速模型）
    print("示例 1: 路由到免费快速模型")
    print("-" * 60)
    route_free_fast(prompt="What is 2+2?")
    print()
    
    # 示例 2: 根据标签路由（中文模型）
    print("示例 2: 路由到中文模型")
    print("-" * 60)
    route_chinese(prompt="请用一句话解释什么是人工智能")
    print()
    
    # 示例 3: 根据 Provider 类型路由
    print("示例 3: 路由到 OpenRouter 模型")
    print("-" * 60)
    route_by_provider(["openrouter"], prompt="Write a haiku about nature")
    print()
    
    # 示例 4: 组合条件路由
    print("示例 4: 组合条件路由（免费 + OpenRouter）")
    print("-" * 60)
    route_by_tags(["free"], provider_types=["openrouter"], 
                  prompt="Explain quantum computing in simple terms")
    print()
    
    # 示例 5: 使用 messages 格式
    print("示例 5: 使用 messages 格式路由")
    print("-" * 60)
    messages = [
        {"role": "user", "content": "Hello, how are you?"}
    ]
    route_by_tags(["chat", "general"], messages=messages)
    print()
    
    # 示例 6: 编程任务路由（低温度）
    print("示例 6: 编程任务路由（低温度）")
    print("-" * 60)
    url = f"{BASE_URL}/route/invoke"
    headers = {"Content-Type": "application/json"}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    
    payload = {
        "query": {
            "tags": ["coding"],
            "provider_types": ["openrouter"]
        },
        "request": {
            "prompt": "Write a Python function to calculate factorial",
            "parameters": {
                "temperature": 0.2,
                "max_tokens": 300
            }
        }
    }
    
    try:
        response = requests.post(url, json=payload, headers=headers, timeout=60)
        response.raise_for_status()
        data = response.json()
        print(f"✓ 路由成功")
        print(f"输出: {data.get('output_text', 'N/A')}")
    except Exception as e:
        print(f"✗ 发生错误: {e}")

