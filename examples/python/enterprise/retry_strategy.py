#!/usr/bin/env python3
"""
重试策略示例

演示不同的重试策略，包括指数退避、固定间隔、自定义重试条件等。
"""

import os
import time
import random
from typing import Callable, Optional, Any
from curl_cffi import requests
from dotenv import load_dotenv

# 加载环境变量
load_dotenv()

# 配置
BASE_URL = os.getenv("LLM_ROUTER_BASE_URL", "http://localhost:18000")
API_KEY = os.getenv("LLM_ROUTER_API_KEY")  # 可选，远程请求时需要

# 模型配置
PROVIDER_NAME = "openrouter"
MODEL_NAME = "openrouter-llama-3.3-70b-instruct"


def invoke(prompt: str, **kwargs) -> Optional[dict]:
    """基础调用函数"""
    url = f"{BASE_URL}/models/{PROVIDER_NAME}/{MODEL_NAME}/invoke"
    
    headers = {"Content-Type": "application/json"}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    
    payload = {
        "prompt": prompt,
        "parameters": {
            "temperature": kwargs.get("temperature", 0.7),
            "max_tokens": kwargs.get("max_tokens", 200)
        }
    }
    
    try:
        response = requests.post(url, json=payload, headers=headers, timeout=60)
        response.raise_for_status()
        return response.json()
    except Exception as e:
        raise


def retry_with_exponential_backoff(
    func: Callable,
    max_retries: int = 3,
    initial_delay: float = 1.0,
    max_delay: float = 60.0,
    exponential_base: float = 2.0,
    jitter: bool = True
):
    """指数退避重试策略"""
    delay = initial_delay
    
    for attempt in range(max_retries):
        try:
            return func()
        except Exception as e:
            if attempt == max_retries - 1:
                raise
            
            # 计算等待时间
            wait_time = min(delay, max_delay)
            
            # 添加抖动（jitter）避免雷群效应
            if jitter:
                wait_time = wait_time * (0.5 + random.random())
            
            print(f"⚠ 重试 {attempt + 1}/{max_retries}，等待 {wait_time:.2f} 秒...")
            time.sleep(wait_time)
            
            # 指数增长延迟
            delay *= exponential_base
    
    raise Exception("所有重试都失败")


def retry_with_fixed_interval(
    func: Callable,
    max_retries: int = 3,
    interval: float = 2.0
):
    """固定间隔重试策略"""
    for attempt in range(max_retries):
        try:
            return func()
        except Exception as e:
            if attempt == max_retries - 1:
                raise
            
            print(f"⚠ 重试 {attempt + 1}/{max_retries}，等待 {interval} 秒...")
            time.sleep(interval)
    
    raise Exception("所有重试都失败")


def retry_with_linear_backoff(
    func: Callable,
    max_retries: int = 3,
    initial_delay: float = 1.0,
    increment: float = 1.0
):
    """线性退避重试策略"""
    delay = initial_delay
    
    for attempt in range(max_retries):
        try:
            return func()
        except Exception as e:
            if attempt == max_retries - 1:
                raise
            
            print(f"⚠ 重试 {attempt + 1}/{max_retries}，等待 {delay:.2f} 秒...")
            time.sleep(delay)
            delay += increment
    
    raise Exception("所有重试都失败")


def retry_with_custom_condition(
    func: Callable,
    should_retry: Callable[[Exception], bool],
    max_retries: int = 3,
    delay: float = 2.0
):
    """自定义重试条件策略"""
    for attempt in range(max_retries):
        try:
            return func()
        except Exception as e:
            if not should_retry(e):
                raise
            
            if attempt == max_retries - 1:
                raise
            
            print(f"⚠ 满足重试条件，重试 {attempt + 1}/{max_retries}，等待 {delay} 秒...")
            time.sleep(delay)
    
    raise Exception("所有重试都失败")


def is_retryable_error(error: Exception) -> bool:
    """判断错误是否可重试"""
    # 网络错误、超时错误可以重试
    if isinstance(error, (requests.RequestsError, requests.Timeout)):
        return True
    
    # 5xx 服务器错误可以重试
    if hasattr(error, 'response') and error.response is not None:
        status_code = error.response.status_code
        if 500 <= status_code < 600:
            return True
        # 429 限流错误可以重试
        if status_code == 429:
            return True
    
    # 4xx 客户端错误通常不可重试
    return False


def retry_with_smart_strategy(
    func: Callable,
    max_retries: int = 3,
    initial_delay: float = 1.0
):
    """智能重试策略（根据错误类型选择策略）"""
    delay = initial_delay
    
    for attempt in range(max_retries):
        try:
            return func()
        except Exception as e:
            if not is_retryable_error(e):
                raise
            
            if attempt == max_retries - 1:
                raise
            
            # 根据错误类型调整延迟
            if hasattr(e, 'response') and e.response is not None:
                if e.response.status_code == 429:
                    # 限流错误，等待更长时间
                    delay = 60.0
                elif 500 <= e.response.status_code < 600:
                    # 服务器错误，指数退避
                    delay = min(delay * 2, 60.0)
                else:
                    # 其他错误，固定延迟
                    delay = 2.0
            else:
                # 网络错误，指数退避
                delay = min(delay * 2, 60.0)
            
            print(f"⚠ 重试 {attempt + 1}/{max_retries}，等待 {delay:.2f} 秒...")
            time.sleep(delay)
    
    raise Exception("所有重试都失败")


if __name__ == "__main__":
    print("=" * 60)
    print("LLM Router 重试策略示例")
    print("=" * 60)
    print()
    
    prompt = "What is Python?"
    
    # 示例 1: 指数退避
    print("示例 1: 指数退避重试策略")
    print("-" * 60)
    try:
        result = retry_with_exponential_backoff(
            lambda: invoke(prompt, max_tokens=100),
            max_retries=3,
            initial_delay=1.0,
            max_delay=10.0
        )
        print(f"✓ 调用成功: {result.get('output_text', 'N/A')[:50]}...")
    except Exception as e:
        print(f"✗ 调用失败: {e}")
    print()
    
    # 示例 2: 固定间隔
    print("示例 2: 固定间隔重试策略")
    print("-" * 60)
    try:
        result = retry_with_fixed_interval(
            lambda: invoke(prompt, max_tokens=100),
            max_retries=3,
            interval=2.0
        )
        print(f"✓ 调用成功: {result.get('output_text', 'N/A')[:50]}...")
    except Exception as e:
        print(f"✗ 调用失败: {e}")
    print()
    
    # 示例 3: 线性退避
    print("示例 3: 线性退避重试策略")
    print("-" * 60)
    try:
        result = retry_with_linear_backoff(
            lambda: invoke(prompt, max_tokens=100),
            max_retries=3,
            initial_delay=1.0,
            increment=1.0
        )
        print(f"✓ 调用成功: {result.get('output_text', 'N/A')[:50]}...")
    except Exception as e:
        print(f"✗ 调用失败: {e}")
    print()
    
    # 示例 4: 自定义重试条件
    print("示例 4: 自定义重试条件策略")
    print("-" * 60)
    try:
        result = retry_with_custom_condition(
            lambda: invoke(prompt, max_tokens=100),
            should_retry=is_retryable_error,
            max_retries=3,
            delay=2.0
        )
        print(f"✓ 调用成功: {result.get('output_text', 'N/A')[:50]}...")
    except Exception as e:
        print(f"✗ 调用失败: {e}")
    print()
    
    # 示例 5: 智能重试策略
    print("示例 5: 智能重试策略（推荐）")
    print("-" * 60)
    try:
        result = retry_with_smart_strategy(
            lambda: invoke(prompt, max_tokens=100),
            max_retries=3,
            initial_delay=1.0
        )
        print(f"✓ 调用成功: {result.get('output_text', 'N/A')[:50]}...")
    except Exception as e:
        print(f"✗ 调用失败: {e}")
    print()
    
    print("重试策略选择建议:")
    print("1. 指数退避: 适合网络不稳定场景，避免对服务器造成压力")
    print("2. 固定间隔: 适合简单的重试场景，实现简单")
    print("3. 线性退避: 介于固定间隔和指数退避之间")
    print("4. 自定义条件: 适合需要精确控制重试逻辑的场景")
    print("5. 智能策略: 推荐使用，根据错误类型自动调整策略")
    print()
    print("最佳实践:")
    print("- 区分可重试错误（网络错误、5xx）和不可重试错误（4xx）")
    print("- 使用指数退避避免雷群效应")
    print("- 添加抖动（jitter）避免同时重试")
    print("- 设置最大重试次数和最大延迟时间")
    print("- 记录重试日志，便于问题排查")

