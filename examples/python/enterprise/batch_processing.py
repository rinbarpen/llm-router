#!/usr/bin/env python3
"""
批量处理示例

演示如何高效地批量处理多个请求，包括并发处理和结果收集。
"""

import os
import asyncio
from typing import List, Dict, Any, Optional
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


def single_invoke(prompt: str, **kwargs) -> Optional[Dict[str, Any]]:
    """单个请求调用"""
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
        print(f"✗ 请求失败: {prompt[:50]}... - {e}")
        return None


def batch_sequential(prompts: List[str], **kwargs) -> List[Optional[Dict[str, Any]]]:
    """顺序批量处理（简单但慢）"""
    print(f"顺序处理 {len(prompts)} 个请求...")
    results = []
    
    for i, prompt in enumerate(prompts, 1):
        print(f"处理 {i}/{len(prompts)}: {prompt[:50]}...")
        result = single_invoke(prompt, **kwargs)
        results.append(result)
    
    return results


def batch_concurrent(prompts: List[str], max_workers: int = 5, **kwargs) -> List[Optional[Dict[str, Any]]]:
    """并发批量处理（使用线程池）"""
    from concurrent.futures import ThreadPoolExecutor, as_completed
    
    print(f"并发处理 {len(prompts)} 个请求 (max_workers={max_workers})...")
    results = [None] * len(prompts)
    
    with ThreadPoolExecutor(max_workers=max_workers) as executor:
        # 提交所有任务
        future_to_index = {
            executor.submit(single_invoke, prompt, **kwargs): i
            for i, prompt in enumerate(prompts)
        }
        
        # 收集结果
        completed = 0
        for future in as_completed(future_to_index):
            index = future_to_index[future]
            completed += 1
            try:
                result = future.result()
                results[index] = result
                print(f"完成 {completed}/{len(prompts)}")
            except Exception as e:
                print(f"✗ 任务 {index} 失败: {e}")
    
    return results


async def async_single_invoke(prompt: str, **kwargs) -> Optional[Dict[str, Any]]:
    """异步单个请求调用"""
    from curl_cffi.requests import AsyncSession
    
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
        async with AsyncSession() as session:
            response = await session.post(url, json=payload, headers=headers, timeout=60)
            response.raise_for_status()
            return response.json()
    except Exception as e:
        print(f"✗ 请求失败: {prompt[:50]}... - {e}")
        return None


async def batch_async(prompts: List[str], max_concurrent: int = 5, **kwargs) -> List[Optional[Dict[str, Any]]]:
    """异步批量处理（最高效）"""
    import asyncio
    
    print(f"异步处理 {len(prompts)} 个请求 (max_concurrent={max_concurrent})...")
    
    # 创建信号量限制并发数
    semaphore = asyncio.Semaphore(max_concurrent)
    
    async def bounded_invoke(prompt: str, index: int) -> tuple[int, Optional[Dict[str, Any]]]:
        async with semaphore:
            result = await async_single_invoke(prompt, **kwargs)
            print(f"完成 {index + 1}/{len(prompts)}")
            return index, result
    
    # 创建所有任务
    tasks = [bounded_invoke(prompt, i) for i, prompt in enumerate(prompts)]
    
    # 等待所有任务完成
    results = [None] * len(prompts)
    for index, result in await asyncio.gather(*tasks):
        results[index] = result
    
    return results


def process_results(results: List[Optional[Dict[str, Any]]]) -> Dict[str, Any]:
    """处理结果统计"""
    total = len(results)
    success = sum(1 for r in results if r is not None)
    failed = total - success
    
    total_tokens = 0
    for result in results:
        if result and 'raw' in result and 'usage' in result['raw']:
            total_tokens += result['raw']['usage'].get('total_tokens', 0)
    
    return {
        "total": total,
        "success": success,
        "failed": failed,
        "success_rate": success / total if total > 0 else 0,
        "total_tokens": total_tokens
    }


if __name__ == "__main__":
    print("=" * 60)
    print("LLM Router 批量处理示例")
    print("=" * 60)
    print()
    
    # 准备测试数据
    prompts = [
        "What is Python?",
        "What is JavaScript?",
        "What is Rust?",
        "What is Go?",
        "What is TypeScript?",
    ]
    
    # 示例 1: 顺序处理
    print("示例 1: 顺序批量处理")
    print("-" * 60)
    import time
    start_time = time.time()
    sequential_results = batch_sequential(prompts, max_tokens=100)
    sequential_time = time.time() - start_time
    stats = process_results(sequential_results)
    print(f"\n统计:")
    print(f"  总请求数: {stats['total']}")
    print(f"  成功: {stats['success']}")
    print(f"  失败: {stats['failed']}")
    print(f"  成功率: {stats['success_rate']:.2%}")
    print(f"  总 Token: {stats['total_tokens']}")
    print(f"  耗时: {sequential_time:.2f} 秒")
    print()
    
    # 示例 2: 并发处理
    print("示例 2: 并发批量处理")
    print("-" * 60)
    start_time = time.time()
    concurrent_results = batch_concurrent(prompts, max_workers=3, max_tokens=100)
    concurrent_time = time.time() - start_time
    stats = process_results(concurrent_results)
    print(f"\n统计:")
    print(f"  总请求数: {stats['total']}")
    print(f"  成功: {stats['success']}")
    print(f"  失败: {stats['failed']}")
    print(f"  成功率: {stats['success_rate']:.2%}")
    print(f"  总 Token: {stats['total_tokens']}")
    print(f"  耗时: {concurrent_time:.2f} 秒")
    print(f"  速度提升: {sequential_time / concurrent_time:.2f}x")
    print()
    
    # 示例 3: 异步处理
    print("示例 3: 异步批量处理")
    print("-" * 60)
    start_time = time.time()
    async_results = asyncio.run(batch_async(prompts, max_concurrent=3, max_tokens=100))
    async_time = time.time() - start_time
    stats = process_results(async_results)
    print(f"\n统计:")
    print(f"  总请求数: {stats['total']}")
    print(f"  成功: {stats['success']}")
    print(f"  失败: {stats['failed']}")
    print(f"  成功率: {stats['success_rate']:.2%}")
    print(f"  总 Token: {stats['total_tokens']}")
    print(f"  耗时: {async_time:.2f} 秒")
    print(f"  速度提升: {sequential_time / async_time:.2f}x")
    print()
    
    print("提示:")
    print("1. 顺序处理简单但慢，适合小批量")
    print("2. 并发处理使用线程池，适合中等批量")
    print("3. 异步处理最高效，适合大批量")
    print("4. 注意控制并发数，避免超过 API 限流")

