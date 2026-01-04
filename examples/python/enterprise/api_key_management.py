#!/usr/bin/env python3
"""
API Key 管理示例

演示如何创建、查询、更新和删除 API Key。
注意：需要管理员权限。
"""

import os
from curl_cffi import requests
from dotenv import load_dotenv

# 加载环境变量
load_dotenv()

# 配置
BASE_URL = os.getenv("LLM_ROUTER_BASE_URL", "http://localhost:18000")
API_KEY = os.getenv("LLM_ROUTER_API_KEY")  # 需要管理员 API Key


def create_api_key(key, name, is_active=True, allowed_models=None, 
                   allowed_providers=None, parameter_limits=None):
    """创建 API Key"""
    url = f"{BASE_URL}/api-keys"
    
    headers = {"Content-Type": "application/json"}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    
    payload = {
        "key": key,
        "name": name,
        "is_active": is_active
    }
    
    if allowed_models:
        payload["allowed_models"] = allowed_models
    if allowed_providers:
        payload["allowed_providers"] = allowed_providers
    if parameter_limits:
        payload["parameter_limits"] = parameter_limits
    
    try:
        response = requests.post(url, json=payload, headers=headers, timeout=10)
        response.raise_for_status()
        return response.json()
    except Exception as e:
        print(f"✗ 创建 API Key 失败: {e}")
        if hasattr(e, 'response') and e.response is not None:
            print(f"响应: {e.response.text}")
        return None


def list_api_keys():
    """列出所有 API Key"""
    url = f"{BASE_URL}/api-keys"
    
    headers = {}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    
    try:
        response = requests.get(url, headers=headers, timeout=10)
        response.raise_for_status()
        return response.json()
    except Exception as e:
        print(f"✗ 获取 API Key 列表失败: {e}")
        return []


def get_api_key(key_id):
    """获取特定 API Key"""
    url = f"{BASE_URL}/api-keys/{key_id}"
    
    headers = {}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    
    try:
        response = requests.get(url, headers=headers, timeout=10)
        response.raise_for_status()
        return response.json()
    except Exception as e:
        print(f"✗ 获取 API Key 失败: {e}")
        return None


def update_api_key(key_id, **updates):
    """更新 API Key"""
    url = f"{BASE_URL}/api-keys/{key_id}"
    
    headers = {"Content-Type": "application/json"}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    
    payload = {}
    if "name" in updates:
        payload["name"] = updates["name"]
    if "is_active" in updates:
        payload["is_active"] = updates["is_active"]
    if "allowed_models" in updates:
        payload["allowed_models"] = updates["allowed_models"]
    if "allowed_providers" in updates:
        payload["allowed_providers"] = updates["allowed_providers"]
    if "parameter_limits" in updates:
        payload["parameter_limits"] = updates["parameter_limits"]
    
    try:
        response = requests.patch(url, json=payload, headers=headers, timeout=10)
        response.raise_for_status()
        return response.json()
    except Exception as e:
        print(f"✗ 更新 API Key 失败: {e}")
        if hasattr(e, 'response') and e.response is not None:
            print(f"响应: {e.response.text}")
        return None


def delete_api_key(key_id):
    """删除 API Key"""
    url = f"{BASE_URL}/api-keys/{key_id}"
    
    headers = {}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    
    try:
        response = requests.delete(url, headers=headers, timeout=10)
        response.raise_for_status()
        return response.status_code == 204
    except Exception as e:
        print(f"✗ 删除 API Key 失败: {e}")
        return False


def print_api_key(api_key):
    """打印 API Key 信息"""
    if not api_key:
        return
    
    print(f"ID: {api_key.get('id')}")
    print(f"  Key: {api_key.get('key')}")
    print(f"  名称: {api_key.get('name')}")
    print(f"  状态: {'激活' if api_key.get('is_active') else '未激活'}")
    print(f"  允许的模型: {api_key.get('allowed_models') or '无限制'}")
    print(f"  允许的 Provider: {api_key.get('allowed_providers') or '无限制'}")
    print(f"  参数限制: {api_key.get('parameter_limits') or '无限制'}")
    print(f"  创建时间: {api_key.get('created_at')}")
    print(f"  更新时间: {api_key.get('updated_at')}")


if __name__ == "__main__":
    print("=" * 60)
    print("LLM Router API Key 管理示例")
    print("=" * 60)
    print()
    
    if not API_KEY:
        print("⚠ 警告: 未设置 LLM_ROUTER_API_KEY 环境变量")
        print("API Key 管理需要管理员权限")
        print()
    
    # 1. 列出所有 API Key
    print("1. 列出所有 API Key")
    print("-" * 60)
    api_keys = list_api_keys()
    if api_keys:
        print(f"找到 {len(api_keys)} 个 API Key:")
        for key in api_keys[:5]:  # 只显示前5个
            print_api_key(key)
            print()
    else:
        print("没有找到 API Key 或无法访问")
    print()
    
    # 2. 创建 API Key（示例，实际使用时请使用唯一的 key）
    print("2. 创建 API Key（示例）")
    print("-" * 60)
    print("注意: 实际使用时请使用唯一的 key 值")
    # new_key = create_api_key(
    #     key="example-key-123",
    #     name="示例 API Key",
    #     is_active=True,
    #     allowed_models=["openrouter/openrouter-llama-3.3-70b-instruct"],
    #     allowed_providers=["openrouter"],
    #     parameter_limits={"max_tokens": 1000, "temperature": 0.7}
    # )
    # if new_key:
    #     print("✓ API Key 创建成功:")
    #     print_api_key(new_key)
    print("（示例代码已注释，避免创建重复的 key）")
    print()
    
    # 3. 获取特定 API Key
    if api_keys and len(api_keys) > 0:
        print("3. 获取特定 API Key")
        print("-" * 60)
        first_id = api_keys[0].get("id")
        if first_id:
            key_detail = get_api_key(first_id)
            if key_detail:
                print("✓ API Key 详情:")
                print_api_key(key_detail)
        print()
    
    # 4. 更新 API Key（示例）
    if api_keys and len(api_keys) > 0:
        print("4. 更新 API Key（示例）")
        print("-" * 60)
        print("注意: 实际使用时请确认要更新的 key ID")
        # first_id = api_keys[0].get("id")
        # updated = update_api_key(
        #     first_id,
        #     name="更新后的名称",
        #     is_active=False
        # )
        # if updated:
        #     print("✓ API Key 更新成功:")
        #     print_api_key(updated)
        print("（示例代码已注释，避免修改现有 key）")
        print()
    
    # 5. 删除 API Key（示例）
    print("5. 删除 API Key（示例）")
    print("-" * 60)
    print("注意: 删除操作不可恢复，请谨慎操作")
    # if api_keys and len(api_keys) > 0:
    #     first_id = api_keys[0].get("id")
    #     if delete_api_key(first_id):
    #         print(f"✓ API Key {first_id} 删除成功")
    print("（示例代码已注释，避免删除现有 key）")
    print()
    
    print("API Key 管理最佳实践:")
    print("1. 为不同应用创建不同的 API Key")
    print("2. 使用参数限制控制资源使用")
    print("3. 定期轮换 API Key")
    print("4. 及时停用不再使用的 API Key")
    print("5. 记录 API Key 的使用情况")
    print("6. 使用最小权限原则，只授予必要的模型和 Provider 访问权限")

