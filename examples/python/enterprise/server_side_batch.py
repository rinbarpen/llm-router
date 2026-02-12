#!/usr/bin/env python3
"""
LLM Router 批量并发处理示例 (Server-side Batch)

演示如何使用 LLM Router 的服务端批量处理功能，通过单个请求发送多个子请求并并发执行。
"""

import os
import json
from typing import List, Dict, Any, Optional
from curl_cffi import requests
from dotenv import load_dotenv

# 加载环境变量
load_dotenv()

# 配置
BASE_URL = os.getenv("LLM_ROUTER_BASE_URL", "http://localhost:18000")
API_KEY = os.getenv("LLM_ROUTER_API_KEY")

# 模型配置 (请确保这些模型在您的 router.toml 中已配置)
PROVIDER_NAME = "openai"
MODEL_NAME = "gpt-4o-mini"

def server_side_batch_invoke(prompts: List[str]):
    """服务端批量调用示例"""
    url = f"{BASE_URL}/models/{PROVIDER_NAME}/{MODEL_NAME}/invoke"
    
    headers = {"Content-Type": "application/json"}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    
    # 构建批量请求体
    batch_requests = [
        {
            "prompt": prompt,
            "parameters": {"temperature": 0.7, "max_tokens": 100}
        }
        for prompt in prompts
    ]
    
    payload = {
        "batch": batch_requests
    }
    
    print(f"发送服务端批量请求，包含 {len(prompts)} 个子请求...")
    
    try:
        response = requests.post(url, json=payload, headers=headers, timeout=60)
        response.raise_for_status()
        result = response.json()
        
        if "batch" in result:
            print(f"批量处理完成，收到 {len(result['batch'])} 个响应内容。")
            for i, res in enumerate(result["batch"]):
                output = res.get("output_text", "N/A")
                print(f"\n响应 #{i+1}:")
                print(f"Prompt: {prompts[i]}")
                print(f"Output: {output[:100]}...")
        else:
            print("响应中未发现 batch 字段，可能服务端未正确处理批量请求。")
            print(json.dumps(result, indent=2))
            
    except Exception as e:
        print(f"✗ 批量请求失败: {e}")

def server_side_route_batch_invoke(prompts: List[str]):
    """服务端路由批量调用示例"""
    url = f"{BASE_URL}/route/invoke"
    
    headers = {"Content-Type": "application/json"}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    
    # 构建批量请求体
    batch_requests = [
        {
            "prompt": prompt,
            "parameters": {"temperature": 0.7, "max_tokens": 100}
        }
        for prompt in prompts
    ]
    
    payload = {
        "query": {
            "tags": ["chat", "general"]
        },
        "request": {
            "batch": batch_requests
        }
    }
    
    print(f"\n发送服务端路由批量请求，包含 {len(prompts)} 个子请求...")
    
    try:
        response = requests.post(url, json=payload, headers=headers, timeout=60)
        response.raise_for_status()
        result = response.json()
        
        if "batch" in result:
            print(f"路由批量处理完成，收到 {len(result['batch'])} 个响应内容。")
            for i, res in enumerate(result["batch"]):
                output = res.get("output_text", "N/A")
                print(f"\n响应 #{i+1}:")
                print(f"Prompt: {prompts[i]}")
                print(f"Output: {output[:100]}...")
        else:
            print("响应中未发现 batch 字段。")
            
    except Exception as e:
        print(f"✗ 路由批量请求失败: {e}")

if __name__ == "__main__":
    test_prompts = [
        "Hello, how are you?",
        "What is the capital of France?",
        "Tell me a short joke.",
        "Translate 'Hello' to Spanish."
    ]
    
    print("=" * 60)
    print("LLM Router 服务端批量并发处理测试")
    print("=" * 60)
    
    # 测试指定模型的批量调用
    server_side_batch_invoke(test_prompts)
    
    # 测试路由模式的批量调用
    server_side_route_batch_invoke(test_prompts)
