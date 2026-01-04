#!/usr/bin/env python3
"""
简单对话示例

演示如何使用 messages 格式进行多轮对话。

注意：这是使用标准接口（/invoke），如果要使用 OpenAI 兼容接口，
请参考 openai_compatible_simple.py 示例。
"""

import os
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


def simple_chat(messages, temperature=0.7, max_tokens=300):
    """使用 messages 格式进行对话"""
    url = f"{BASE_URL}/models/{PROVIDER_NAME}/{MODEL_NAME}/invoke"
    
    headers = {"Content-Type": "application/json"}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    
    payload = {
        "messages": messages,
        "parameters": {
            "temperature": temperature,
            "max_tokens": max_tokens
        }
    }
    
    print(f"调用模型: {PROVIDER_NAME}/{MODEL_NAME}")
    print("对话历史:")
    for msg in messages:
        print(f"  {msg['role']}: {msg['content'][:50]}...")
    print()
    
    try:
        response = requests.post(url, json=payload, headers=headers, timeout=60)
        response.raise_for_status()
        
        data = response.json()
        
        print("✓ 调用成功")
        print(f"回复: {data.get('output_text', 'N/A')}")
        
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
    print("LLM Router 简单对话示例")
    print("=" * 60)
    print()
    
    # 示例 1: 单轮对话
    print("示例 1: 单轮对话")
    print("-" * 60)
    messages = [
        {"role": "user", "content": "Explain quantum computing in simple terms"}
    ]
    response = simple_chat(messages)
    print()
    
    # 示例 2: 多轮对话
    print("示例 2: 多轮对话")
    print("-" * 60)
    messages = [
        {"role": "user", "content": "Hello, how are you?"},
        {"role": "assistant", "content": "I'm doing well, thank you! How can I help you today?"},
        {"role": "user", "content": "Can you explain machine learning?"}
    ]
    response = simple_chat(messages)
    print()
    
    # 示例 3: 带系统提示的对话
    print("示例 3: 带系统提示的对话")
    print("-" * 60)
    messages = [
        {"role": "system", "content": "你是一个专业的AI助手，擅长用中文回答问题，回答要简洁明了。"},
        {"role": "user", "content": "请用Python写一个快速排序算法"}
    ]
    response = simple_chat(messages, temperature=0.3, max_tokens=500)
    print()
    
    # 示例 4: 持续对话（模拟对话流程）
    print("示例 4: 持续对话")
    print("-" * 60)
    conversation = [
        {"role": "user", "content": "What is Python?"}
    ]
    
    # 第一轮
    response = simple_chat(conversation, max_tokens=200)
    if response and response.get('output_text'):
        conversation.append({"role": "assistant", "content": response['output_text']})
        conversation.append({"role": "user", "content": "Can you give me an example?"})
        
        # 第二轮
        print("\n继续对话...")
        response = simple_chat(conversation, max_tokens=200)

