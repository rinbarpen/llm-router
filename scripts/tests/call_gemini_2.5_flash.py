#!/usr/bin/env python3
"""
调用 gemini-2.5-flash 的 Python 示例脚本
"""

import asyncio
import json
from typing import Optional

import httpx


async def invoke_gemini_flash(
    prompt: Optional[str] = None,
    messages: Optional[list] = None,
    parameters: Optional[dict] = None,
    api_key: Optional[str] = None,
    base_url: str = "http://localhost:18000",
) -> dict:
    """
    调用 gemini-2.5-flash 模型
    
    Args:
        prompt: 简单文本提示（与 messages 二选一）
        messages: 对话消息列表（与 prompt 二选一）
        parameters: 模型参数（temperature, max_tokens 等）
        api_key: LLM Router API Key（如果启用了认证）
        base_url: API 服务器地址
    
    Returns:
        模型响应字典
    """
    url = f"{base_url}/models/gemini/gemini-2.5-flash/invoke"
    
    payload = {}
    if prompt:
        payload["prompt"] = prompt
    if messages:
        payload["messages"] = messages
    if parameters:
        payload["parameters"] = parameters
    
    headers = {"Content-Type": "application/json"}
    if api_key:
        headers["Authorization"] = f"Bearer {api_key}"
    
    async with httpx.AsyncClient(timeout=60.0) as client:
        response = await client.post(url, json=payload, headers=headers)
        response.raise_for_status()
        return response.json()


async def route_invoke(
    prompt: str,
    tags: Optional[list] = None,
    provider_types: Optional[list] = None,
    parameters: Optional[dict] = None,
    api_key: Optional[str] = None,
    base_url: str = "http://localhost:18000",
) -> dict:
    """
    使用路由功能调用模型（智能选择）
    
    Args:
        prompt: 文本提示
        tags: 模型标签过滤
        provider_types: Provider 类型过滤
        parameters: 模型参数
        api_key: LLM Router API Key
        base_url: API 服务器地址
    
    Returns:
        模型响应字典
    """
    url = f"{base_url}/route/invoke"
    
    query = {}
    if tags:
        query["tags"] = tags
    if provider_types:
        query["provider_types"] = provider_types
    
    payload = {
        "query": query,
        "request": {
            "prompt": prompt,
        }
    }
    if parameters:
        payload["request"]["parameters"] = parameters
    
    headers = {"Content-Type": "application/json"}
    if api_key:
        headers["Authorization"] = f"Bearer {api_key}"
    
    async with httpx.AsyncClient(timeout=60.0) as client:
        response = await client.post(url, json=payload, headers=headers)
        response.raise_for_status()
        return response.json()


async def main():
    """示例调用"""
    
    # 从环境变量读取 API Key（如果启用了认证）
    import os
    api_key = os.getenv("LLM_ROUTER_ADMIN_KEY")
    
    print("=" * 60)
    print("示例1: 使用 prompt 参数")
    print("=" * 60)
    result = await invoke_gemini_flash(
        prompt="What is the capital of France?",
        parameters={"temperature": 0.7, "max_tokens": 150},
        api_key=api_key,
    )
    print(json.dumps(result, indent=2, ensure_ascii=False))
    print()
    
    print("=" * 60)
    print("示例2: 使用 messages 参数（中文对话）")
    print("=" * 60)
    result = await invoke_gemini_flash(
        messages=[
            {"role": "user", "content": "请用中文解释什么是人工智能"}
        ],
        parameters={"temperature": 0.8, "max_tokens": 500},
        api_key=api_key,
    )
    print(json.dumps(result, indent=2, ensure_ascii=False))
    print()
    
    print("=" * 60)
    print("示例3: 多轮对话")
    print("=" * 60)
    result = await invoke_gemini_flash(
        messages=[
            {"role": "user", "content": "Hello, how are you?"},
            {"role": "assistant", "content": "I'm doing well, thank you! How can I help you today?"},
            {"role": "user", "content": "Can you explain quantum computing in simple terms?"},
        ],
        parameters={"temperature": 0.7},
        api_key=api_key,
    )
    print(json.dumps(result, indent=2, ensure_ascii=False))
    print()
    
    print("=" * 60)
    print("示例4: 使用路由功能")
    print("=" * 60)
    result = await route_invoke(
        prompt="Write a short poem about technology",
        tags=["fast", "chat"],
        provider_types=["gemini"],
        parameters={"temperature": 0.9},
        api_key=api_key,
    )
    print(json.dumps(result, indent=2, ensure_ascii=False))
    print()


if __name__ == "__main__":
    asyncio.run(main())

