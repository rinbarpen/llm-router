#!/usr/bin/env python3
"""
监控示例

演示如何查询调用历史、统计信息和监控数据。
"""

import os
from datetime import datetime, timedelta
from curl_cffi import requests
from dotenv import load_dotenv

# 加载环境变量
load_dotenv()

# 配置
BASE_URL = os.getenv("LLM_ROUTER_BASE_URL", "http://localhost:18000")
API_KEY = os.getenv("LLM_ROUTER_API_KEY")  # 可选，远程请求时需要


def get_invocations(limit=100, offset=0, **filters):
    """获取调用历史"""
    url = f"{BASE_URL}/monitor/invocations"
    
    headers = {}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    
    params = {
        "limit": limit,
        "offset": offset
    }
    
    # 添加过滤条件
    if "model_name" in filters:
        params["model_name"] = filters["model_name"]
    if "provider_name" in filters:
        params["provider_name"] = filters["provider_name"]
    if "status" in filters:
        params["status"] = filters["status"]
    if "start_time" in filters:
        params["start_time"] = filters["start_time"]
    if "end_time" in filters:
        params["end_time"] = filters["end_time"]
    
    try:
        response = requests.get(url, params=params, headers=headers, timeout=10)
        response.raise_for_status()
        return response.json()
    except Exception as e:
        print(f"✗ 获取调用历史失败: {e}")
        return []


def get_invocation_by_id(invocation_id):
    """获取特定调用详情"""
    url = f"{BASE_URL}/monitor/invocations/{invocation_id}"
    
    headers = {}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    
    try:
        response = requests.get(url, headers=headers, timeout=10)
        response.raise_for_status()
        return response.json()
    except Exception as e:
        print(f"✗ 获取调用详情失败: {e}")
        return None


def get_statistics(time_range="24h"):
    """获取使用统计"""
    url = f"{BASE_URL}/monitor/statistics"
    
    headers = {}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    
    params = {"time_range": time_range}
    
    try:
        response = requests.get(url, params=params, headers=headers, timeout=10)
        response.raise_for_status()
        return response.json()
    except Exception as e:
        print(f"✗ 获取统计信息失败: {e}")
        return None


def get_time_series(granularity="hour", start_time=None, end_time=None):
    """获取时间序列数据"""
    url = f"{BASE_URL}/monitor/time-series"
    
    headers = {}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    
    params = {"granularity": granularity}
    if start_time:
        params["start_time"] = start_time
    if end_time:
        params["end_time"] = end_time
    
    try:
        response = requests.get(url, params=params, headers=headers, timeout=10)
        response.raise_for_status()
        return response.json()
    except Exception as e:
        print(f"✗ 获取时间序列数据失败: {e}")
        return None


def print_invocations(invocations):
    """打印调用历史"""
    if not invocations:
        print("没有调用记录")
        return
    
    print(f"\n调用历史 (共 {len(invocations)} 条):")
    print("-" * 100)
    for inv in invocations[:10]:  # 只显示前10条
        print(f"ID: {inv.get('id')}")
        print(f"  模型: {inv.get('provider_name')}/{inv.get('model_name')}")
        print(f"  状态: {inv.get('status')}")
        print(f"  时间: {inv.get('started_at')}")
        print(f"  耗时: {inv.get('duration_ms', 0):.2f} ms")
        print(f"  Tokens: {inv.get('total_tokens', 0)}")
        if inv.get('error_message'):
            print(f"  错误: {inv.get('error_message')}")
        print("-" * 100)


def print_statistics(stats):
    """打印统计信息"""
    if not stats:
        print("无法获取统计信息")
        return
    
    overall = stats.get("overall", {})
    print(f"\n总体统计 ({overall.get('time_range', 'N/A')}):")
    print("-" * 60)
    print(f"总调用数: {overall.get('total_calls', 0)}")
    print(f"成功调用: {overall.get('success_calls', 0)}")
    print(f"失败调用: {overall.get('error_calls', 0)}")
    print(f"成功率: {overall.get('success_rate', 0):.2%}")
    print(f"总 Token: {overall.get('total_tokens', 0)}")
    print(f"平均耗时: {overall.get('avg_duration_ms', 0):.2f} ms")
    
    by_model = stats.get("by_model", [])
    if by_model:
        print(f"\n按模型统计:")
        print("-" * 60)
        for model_stat in by_model[:5]:  # 只显示前5个模型
            print(f"模型: {model_stat.get('provider_name')}/{model_stat.get('model_name')}")
            print(f"  调用数: {model_stat.get('total_calls', 0)}")
            print(f"  成功率: {model_stat.get('success_rate', 0):.2%}")
            print(f"  总 Token: {model_stat.get('total_tokens', 0)}")
            print(f"  平均耗时: {model_stat.get('avg_duration_ms', 0):.2f} ms")
            print("-" * 60)


if __name__ == "__main__":
    print("=" * 60)
    print("LLM Router 监控示例")
    print("=" * 60)
    print()
    
    # 1. 获取最近的调用历史
    print("1. 获取最近的调用历史")
    print("-" * 60)
    invocations = get_invocations(limit=10)
    print_invocations(invocations)
    print()
    
    # 2. 按状态过滤
    print("2. 获取成功的调用")
    print("-" * 60)
    success_invocations = get_invocations(limit=10, status="success")
    print_invocations(success_invocations)
    print()
    
    # 3. 按模型过滤
    print("3. 获取特定模型的调用")
    print("-" * 60)
    model_invocations = get_invocations(
        limit=10,
        provider_name="openrouter",
        model_name="openrouter-llama-3.3-70b-instruct"
    )
    print_invocations(model_invocations)
    print()
    
    # 4. 获取统计信息
    print("4. 获取使用统计（24小时）")
    print("-" * 60)
    stats = get_statistics(time_range="24h")
    print_statistics(stats)
    print()
    
    # 5. 获取时间序列数据
    print("5. 获取时间序列数据（按小时）")
    print("-" * 60)
    time_series = get_time_series(granularity="hour")
    if time_series:
        data = time_series.get("data", [])
        print(f"数据点数量: {len(data)}")
        if data:
            print("\n前5个数据点:")
            for point in data[:5]:
                print(f"  时间: {point.get('timestamp')}")
                print(f"  总调用: {point.get('total_calls', 0)}")
                print(f"  成功: {point.get('success_calls', 0)}")
                print(f"  失败: {point.get('error_calls', 0)}")
                print(f"  Tokens: {point.get('total_tokens', 0)}")
                print()
    print()
    
    # 6. 获取特定调用详情
    if invocations and len(invocations) > 0:
        print("6. 获取特定调用详情")
        print("-" * 60)
        first_id = invocations[0].get("id")
        if first_id:
            detail = get_invocation_by_id(first_id)
            if detail:
                print(f"调用 ID: {detail.get('id')}")
                print(f"模型: {detail.get('provider_name')}/{detail.get('model_name')}")
                print(f"状态: {detail.get('status')}")
                print(f"请求: {detail.get('request_prompt', 'N/A')[:100]}...")
                print(f"响应: {detail.get('response_text', 'N/A')[:100]}...")
                print(f"耗时: {detail.get('duration_ms', 0):.2f} ms")
                print(f"Tokens: {detail.get('total_tokens', 0)}")
    print()
    
    print("监控功能说明:")
    print("1. 调用历史: 查看所有 API 调用记录")
    print("2. 统计信息: 查看总体和按模型的统计")
    print("3. 时间序列: 查看时间维度的使用趋势")
    print("4. 调用详情: 查看单次调用的完整信息")
    print("5. 支持多种过滤条件: 模型、Provider、状态、时间范围等")

