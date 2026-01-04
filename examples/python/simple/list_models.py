#!/usr/bin/env python3
"""
获取模型列表示例

演示如何获取可用的模型列表，支持按标签、Provider 类型等过滤。
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


def list_all_models():
    """获取所有模型"""
    url = f"{BASE_URL}/models"
    
    headers = {}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    
    print(f"获取所有模型: {url}")
    
    try:
        response = requests.get(url, headers=headers, timeout=10)
        response.raise_for_status()
        
        models = response.json()
        print(f"✓ 找到 {len(models)} 个模型")
        return models
        
    except requests.RequestsError as e:
        print(f"✗ 请求失败: {e}")
        return []
    except Exception as e:
        print(f"✗ 发生错误: {e}")
        return []


def list_models_by_tags(tags):
    """按标签过滤模型"""
    url = f"{BASE_URL}/models"
    
    # 支持多个标签，用逗号分隔
    if isinstance(tags, list):
        tags = ",".join(tags)
    
    params = {"tags": tags}
    
    headers = {}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    
    print(f"按标签过滤模型 (tags={tags}): {url}")
    
    try:
        response = requests.get(url, params=params, headers=headers, timeout=10)
        response.raise_for_status()
        
        models = response.json()
        print(f"✓ 找到 {len(models)} 个匹配的模型")
        return models
        
    except requests.RequestsError as e:
        print(f"✗ 请求失败: {e}")
        return []
    except Exception as e:
        print(f"✗ 发生错误: {e}")
        return []


def list_models_by_provider(provider_types):
    """按 Provider 类型过滤模型"""
    url = f"{BASE_URL}/models"
    
    if isinstance(provider_types, list):
        provider_types = ",".join(provider_types)
    
    params = {"provider_types": provider_types}
    
    headers = {}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    
    print(f"按 Provider 类型过滤模型 (provider_types={provider_types}): {url}")
    
    try:
        response = requests.get(url, params=params, headers=headers, timeout=10)
        response.raise_for_status()
        
        models = response.json()
        print(f"✓ 找到 {len(models)} 个匹配的模型")
        return models
        
    except requests.RequestsError as e:
        print(f"✗ 请求失败: {e}")
        return []
    except Exception as e:
        print(f"✗ 发生错误: {e}")
        return []


def print_model_info(models):
    """打印模型信息"""
    if not models:
        print("没有找到模型")
        return
    
    print("\n模型列表:")
    print("-" * 80)
    for model in models:
        print(f"名称: {model.get('provider_name')}/{model.get('name')}")
        print(f"显示名: {model.get('display_name', 'N/A')}")
        print(f"标签: {', '.join(model.get('tags', []))}")
        print(f"状态: {'激活' if model.get('is_active') else '未激活'}")
        print("-" * 80)


if __name__ == "__main__":
    print("=" * 60)
    print("LLM Router 获取模型列表示例")
    print("=" * 60)
    print()
    
    # 1. 获取所有模型
    all_models = list_all_models()
    print()
    
    # 2. 按标签过滤（免费模型）
    free_models = list_models_by_tags("free")
    print_model_info(free_models[:3])  # 只显示前3个
    print()
    
    # 3. 按标签过滤（中文模型）
    chinese_models = list_models_by_tags("chinese")
    print_model_info(chinese_models[:3])
    print()
    
    # 4. 按 Provider 类型过滤
    openrouter_models = list_models_by_provider("openrouter")
    print_model_info(openrouter_models[:3])
    print()
    
    # 5. 组合过滤（标签 + Provider 类型）
    print("组合过滤: tags=free, provider_types=openrouter")
    url = f"{BASE_URL}/models"
    params = {"tags": "free", "provider_types": "openrouter"}
    headers = {}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    
    try:
        response = requests.get(url, params=params, headers=headers, timeout=10)
        response.raise_for_status()
        models = response.json()
        print(f"✓ 找到 {len(models)} 个匹配的模型")
        print_model_info(models[:3])
    except Exception as e:
        print(f"✗ 发生错误: {e}")

