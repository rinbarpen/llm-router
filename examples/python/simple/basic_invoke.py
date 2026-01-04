#!/usr/bin/env python3
"""
基础调用示例

演示如何使用简单的 prompt 调用模型。
"""

import os
import json
from curl_cffi import requests
from dotenv import load_dotenv

# 加载环境变量
load_dotenv()

# 配置
BASE_URL = os.getenv("LLM_ROUTER_BASE_URL", "http://localhost:18000")
API_KEY = os.getenv("LLM_ROUTER_API_KEY")  # 可选，远程请求时需要

# 使用免费模型作为示例
PROVIDER_NAME = "openrouter"
MODEL_NAME = "openrouter-llama-3.3-70b-instruct"


def basic_invoke(prompt, temperature=0.7, max_tokens=200):
    """基础文本调用"""
    url = f"{BASE_URL}/models/{PROVIDER_NAME}/{MODEL_NAME}/invoke"
    
    headers = {"Content-Type": "application/json"}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    
    payload = {
        "prompt": prompt,
        "parameters": {
            "temperature": temperature,
            "max_tokens": max_tokens
        }
    }
    
    print(f"调用模型: {PROVIDER_NAME}/{MODEL_NAME}")
    print(f"提示词: {prompt}")
    print(f"参数: temperature={temperature}, max_tokens={max_tokens}")
    print()
    
    try:
        response = requests.post(url, json=payload, headers=headers, timeout=60)
        response.raise_for_status()
        
        data = response.json()
        
        print("✓ 调用成功")
        print(f"输出: {data.get('output_text', 'N/A')}")
        
        # 显示使用统计
        if 'raw' in data and 'usage' in data['raw']:
            usage = data['raw']['usage']
            print(f"Token 使用: {usage.get('total_tokens', 0)} "
                  f"(prompt: {usage.get('prompt_tokens', 0)}, "
                  f"completion: {usage.get('completion_tokens', 0)})")
        
        return data
        
    except requests.RequestsError as e:
        print(f"✗ 请求失败: {e}")
        if hasattr(e, 'response') and e.response is not None:
            print(f"响应: {e.response.text}")
        return None
    except Exception as e:
        print(f"✗ 发生错误: {e}")
        return None


if __name__ == "__main__":
    print("=" * 60)
    print("LLM Router 基础调用示例")
    print("=" * 60)
    print()
    
    # 示例 1: 简单问题
    print("示例 1: 简单问题")
    print("-" * 60)
    basic_invoke("What is the capital of France?")
    print()
    
    # 示例 2: 中文问题
    print("示例 2: 中文问题")
    print("-" * 60)
    basic_invoke("请用一句话解释什么是人工智能", temperature=0.5, max_tokens=100)
    print()
    
    # 示例 3: 创意任务（高温度）
    print("示例 3: 创意任务（高温度）")
    print("-" * 60)
    basic_invoke("Write a short haiku about technology", temperature=0.9, max_tokens=50)
    print()
    
    # 示例 4: 编程任务（低温度）
    print("示例 4: 编程任务（低温度）")
    print("-" * 60)
    basic_invoke("Write a Python function to calculate factorial", temperature=0.2, max_tokens=200)

