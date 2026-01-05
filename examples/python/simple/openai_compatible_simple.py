#!/usr/bin/env python3
"""
OpenAI 兼容 API 简单示例

演示如何使用 OpenAI 兼容的 API 接口，可以无缝替换 OpenAI SDK。
这是最简单的使用方式，适合快速上手。
"""

import os
import requests  # 使用标准 requests 库替代 curl_cffi
from dotenv import load_dotenv

# 加载环境变量
load_dotenv()

# 配置
BASE_URL = os.getenv("LLM_ROUTER_BASE_URL", "http://localhost:18000")
API_KEY = os.getenv("LLM_ROUTER_API_KEY")  # 可选，远程请求时需要

# 模型配置（使用标准格式）
PROVIDER_NAME = "openrouter"
MODEL_NAME = "glm-4.5-air"  # 数据库中的模型名称（对应 ollama 中 gpt-oss:20b）
# 标准格式: provider/model
STANDARD_MODEL = f"{PROVIDER_NAME}/{MODEL_NAME}"


def openai_chat(messages, **kwargs):
    """
    最简单的 OpenAI 兼容 API 调用
    
    参数:
        messages: 消息列表，格式: [{"role": "user", "content": "..."}]
        **kwargs: 其他 OpenAI API 参数（temperature, max_tokens 等）
    
    返回:
        OpenAI 格式的响应
    """
    # 标准 OpenAI API 端点（model 在请求体中）
    url = f"{BASE_URL}/v1/chat/completions"
    
    headers = {"Content-Type": "application/json"}
    # if API_KEY:
    #     headers["Authorization"] = f"Bearer {API_KEY}"
    
    # 构建 OpenAI 兼容的请求体（标准格式）
    payload = {
        "model": STANDARD_MODEL,  # model 在请求体中
        "messages": messages,
        **kwargs  # 直接传递其他参数（temperature, max_tokens 等）
    }
    
    print(f"调用 OpenAI 兼容 API: {url}")
    print(f"消息: {messages[0]['content'][:50]}...")
    
    try:
        response = requests.post(url, json=payload, headers=headers, timeout=60)
        response.raise_for_status()
        
        data = response.json()
        
        # 调试：打印完整响应
        print(f"调试 - 完整响应: {data}")
        
        # OpenAI 格式的响应
        if "choices" in data and len(data["choices"]) > 0:
            content = data["choices"][0]["message"]["content"]
            print(f"✓ 回复: {content}")
            
            # 显示 Token 使用
            if "usage" in data:
                usage = data["usage"]
                print(f"Token 使用: {usage.get('total_tokens', 0)}")
        
        return data
        
    except requests.HTTPError as e:
        print(f"✗ 请求失败: {e}")
        if hasattr(e, 'response') and e.response is not None:
            print(f"响应: {e.response.text}")
        return None
    except Exception as e:
        print(f"✗ 发生错误: {e}")
        return None


if __name__ == "__main__":
    print("=" * 60)
    print("LLM Router OpenAI 兼容 API 简单示例")
    print("=" * 60)
    print()
    print("这个示例演示如何使用 OpenAI 兼容的 API 接口")
    print("可以无缝替换 OpenAI SDK，只需修改 base_url 即可")
    print()
    
    # 示例 1: 最简单的调用
    print("示例 1: 最简单的调用")
    print("-" * 60)
    messages = [
        {"role": "user", "content": "Hello! How are you?"}
    ]
    openai_chat(messages, temperature=0.7, max_tokens=100)
    print()
    
    # 示例 2: 带系统提示
    print("示例 2: 带系统提示")
    print("-" * 60)
    messages = [
        {"role": "system", "content": "你是一个专业的 Python 编程助手。"},
        {"role": "user", "content": "请写一个快速排序算法"}
    ]
    openai_chat(messages, temperature=0.3, max_tokens=500)
    print()
    
    # 示例 3: 多轮对话
    print("示例 3: 多轮对话")
    print("-" * 60)
    messages = [
        {"role": "user", "content": "What is Python?"},
        {"role": "assistant", "content": "Python is a high-level programming language."},
        {"role": "user", "content": "Can you give me an example?"}
    ]
    openai_chat(messages, temperature=0.7, max_tokens=200)
    print()
    
    print("=" * 60)
    print("使用说明")
    print("=" * 60)
    print()
    print("1. 本机请求（localhost）可以免认证")
    print("2. 远程请求需要设置 LLM_ROUTER_API_KEY 环境变量")
    print("3. 可以无缝替换 OpenAI SDK，只需修改 base_url:")
    print()
    print("   # 原 OpenAI SDK 代码")
    print("   # client = OpenAI(api_key='...', base_url='https://api.openai.com/v1')")
    print()
    print("   # 替换为 LLM Router（标准格式）")
    print("   # client = OpenAI(api_key='...', base_url='http://localhost:18000/v1')")
    print()
    print("   # 使用时指定 model 参数")
    print("   # client.chat.completions.create(model='openrouter/glm-4.5-air', ...)")
    print()
    print("4. 支持所有 OpenAI API 的标准参数：")
    print("   - temperature, max_tokens, top_p, frequency_penalty 等")
    print("   - stream (流式响应)")
    print("   - n (生成多个回复)")

