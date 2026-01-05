#!/usr/bin/env python3
"""
认证流程示例

演示完整的认证流程：
1. 登录获取 Session Token
2. 绑定模型到 Session（可选）
3. 使用 Session Token 调用 API
4. 登出（可选）
"""

import os
from curl_cffi import requests
from dotenv import load_dotenv

# 加载环境变量
load_dotenv()

# 配置
BASE_URL = os.getenv("LLM_ROUTER_BASE_URL", "http://localhost:18000")
API_KEY = os.getenv("LLM_ROUTER_API_KEY")  # 必需，用于登录

# 模型配置（使用标准格式：provider/model）
PROVIDER_NAME = "openrouter"
MODEL_NAME = "glm-4.5-air"  # 数据库中的模型名称
STANDARD_MODEL = f"{PROVIDER_NAME}/{MODEL_NAME}"  # 标准格式：openrouter/glm-4.5-air


def login(api_key=None):
    """登录获取 Session Token"""
    if not api_key:
        api_key = API_KEY
    
    if not api_key:
        print("✗ 错误: 需要提供 API Key")
        return None
    
    url = f"{BASE_URL}/auth/login"
    
    # 方式 1: 使用请求体
    payload = {"api_key": api_key}
    headers = {"Content-Type": "application/json"}
    
    # 方式 2: 使用 Authorization header（也可以这样）
    # headers = {"Authorization": f"Bearer {api_key}"}
    
    print(f"登录: {url}")
    
    try:
        response = requests.post(url, json=payload, headers=headers, timeout=10)
        response.raise_for_status()
        
        data = response.json()
        token = data.get("token")
        expires_in = data.get("expires_in", 0)
        
        print(f"✓ 登录成功")
        print(f"  Token: {token[:20]}...")
        print(f"  过期时间: {expires_in} 秒 ({expires_in // 3600} 小时)")
        
        return token
        
    except requests.RequestsError as e:
        print(f"✗ 登录失败: {e}")
        if hasattr(e, 'response') and e.response is not None:
            print(f"响应: {e.response.text}")
        return None
    except Exception as e:
        print(f"✗ 发生错误: {e}")
        return None


def bind_model(token, provider_name, model_name):
    """
    绑定模型到 Session（通过调用 OpenAI 兼容 API 自动完成）
    
    注意：模型绑定是在首次调用 OpenAI 兼容 API 时自动完成的。
    这里演示如何通过调用 API 来触发绑定。
    """
    # 模型绑定通过调用 OpenAI 兼容 API 自动完成
    # 首次调用时，如果 session 中没有绑定模型，会自动绑定
    url = f"{BASE_URL}/v1/chat/completions"
    
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {token}"
    }
    
    payload = {
        "model": f"{provider_name}/{model_name}",
        "messages": [
            {"role": "user", "content": "test"}
        ]
    }
    
    print(f"绑定模型: {provider_name}/{model_name}")
    print("（通过调用 OpenAI 兼容 API 自动绑定）")
    
    try:
        response = requests.post(url, json=payload, headers=headers, timeout=10)
        response.raise_for_status()
        
        print(f"✓ 模型绑定成功（已自动绑定到 session）")
        
        return True
        
    except requests.RequestsError as e:
        print(f"✗ 绑定失败: {e}")
        if hasattr(e, 'response') and e.response is not None:
            print(f"响应: {e.response.text}")
        return False
    except Exception as e:
        print(f"✗ 发生错误: {e}")
        return False


def invoke_with_token(token, prompt, provider_name=None, model_name=None):
    """使用 Session Token 调用模型"""
    # 如果已绑定模型，可以不指定 provider 和 model（使用 OpenAI 兼容 API）
    # 这里演示指定模型的方式
    
    if not provider_name:
        provider_name = PROVIDER_NAME
    if not model_name:
        model_name = MODEL_NAME
    
    url = f"{BASE_URL}/models/{provider_name}/{model_name}/invoke"
    
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {token}"
    }
    
    payload = {
        "prompt": prompt,
        "parameters": {
            "temperature": 0.7,
            "max_tokens": 200
        }
    }
    
    print(f"使用 Token 调用模型: {provider_name}/{model_name}")
    print(f"提示词: {prompt}")
    
    try:
        response = requests.post(url, json=payload, headers=headers, timeout=60)
        response.raise_for_status()
        
        data = response.json()
        print(f"✓ 调用成功")
        print(f"输出: {data.get('output_text', 'N/A')}")
        
        return data
        
    except requests.RequestsError as e:
        print(f"✗ 调用失败: {e}")
        if hasattr(e, 'response') and e.response is not None:
            print(f"响应: {e.response.text}")
        return None
    except Exception as e:
        print(f"✗ 发生错误: {e}")
        return None


def invoke_openai_compatible(token, messages, provider_name=None, model_name=None):
    """使用 OpenAI 兼容 API（标准端点，model 在请求体中）"""
    if not provider_name:
        provider_name = PROVIDER_NAME
    if not model_name:
        model_name = MODEL_NAME
    
    # 使用标准 OpenAI 兼容端点
    url = f"{BASE_URL}/v1/chat/completions"
    
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {token}"
    }
    
    payload = {
        "model": f"{provider_name}/{model_name}",  # model 在请求体中
        "messages": messages,
        "temperature": 0.7,
        "max_tokens": 200
    }
    
    print(f"使用 OpenAI 兼容 API 调用: /v1/chat/completions")
    print(f"模型: {provider_name}/{model_name}")
    
    try:
        response = requests.post(url, json=payload, headers=headers, timeout=60)
        response.raise_for_status()
        
        data = response.json()
        print(f"✓ 调用成功")
        
        if data.get("choices"):
            content = data["choices"][0]["message"]["content"]
            print(f"回复: {content}")
        
        if data.get("usage"):
            usage = data["usage"]
            print(f"Token 使用: {usage.get('total_tokens', 0)}")
        
        return data
        
    except requests.RequestsError as e:
        print(f"✗ 调用失败: {e}")
        if hasattr(e, 'response') and e.response is not None:
            print(f"响应: {e.response.text}")
        return None
    except Exception as e:
        print(f"✗ 发生错误: {e}")
        return None


def logout(token):
    """登出，使 Session Token 失效"""
    url = f"{BASE_URL}/auth/logout"
    
    headers = {
        "Authorization": f"Bearer {token}"
    }
    
    print("登出...")
    
    try:
        response = requests.post(url, headers=headers, timeout=10)
        response.raise_for_status()
        
        data = response.json()
        print(f"✓ 登出成功: {data.get('message', 'N/A')}")
        
        return True
        
    except requests.RequestsError as e:
        print(f"✗ 登出失败: {e}")
        return False
    except Exception as e:
        print(f"✗ 发生错误: {e}")
        return False


if __name__ == "__main__":
    print("=" * 60)
    print("LLM Router 认证流程示例")
    print("=" * 60)
    print()
    
    if not API_KEY:
        print("⚠ 警告: 未设置 LLM_ROUTER_API_KEY 环境变量")
        print("本机请求（localhost）可以免认证，但无法演示完整认证流程")
        print()
    
    # 1. 登录
    token = login()
    if not token:
        print("\n无法继续，请检查 API Key 配置")
        exit(1)
    
    print()
    
    # 2. 绑定模型（可选，但推荐）
    bind_model(token, PROVIDER_NAME, MODEL_NAME)
    print()
    
    # 3. 使用 Token 调用模型（标准接口）
    print("示例 1: 使用标准接口调用")
    print("-" * 60)
    invoke_with_token(token, "What is Python?")
    print()
    
    # 4. 使用 OpenAI 兼容 API
    print("示例 2: 使用 OpenAI 兼容 API")
    print("-" * 60)
    messages = [
        {"role": "user", "content": "Hello! How are you?"}
    ]
    invoke_openai_compatible(token, messages)
    print()
    
    # 5. 登出（可选）
    logout(token)

