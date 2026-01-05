#!/usr/bin/env python3
"""
OpenAI 兼容 API 示例

演示如何使用 OpenAI 兼容的 API 接口，可以无缝替换 OpenAI SDK。
"""

import os
from curl_cffi import requests
from dotenv import load_dotenv

# 加载环境变量
load_dotenv()

# 配置
BASE_URL = os.getenv("LLM_ROUTER_BASE_URL", "http://localhost:18000")
API_KEY = os.getenv("LLM_ROUTER_API_KEY")  # 可选，远程请求时需要

# 模型配置（使用标准格式：provider/model）
PROVIDER_NAME = "openrouter"
MODEL_NAME = "glm-4.5-air"  # 数据库中的模型名称
STANDARD_MODEL = f"{PROVIDER_NAME}/{MODEL_NAME}"  # 标准格式：openrouter/glm-4.5-air


def openai_chat_completions(messages, model=None, **kwargs):
    """OpenAI 兼容的聊天补全接口（标准格式）"""
    # 使用标准端点
    url = f"{BASE_URL}/v1/chat/completions"
    
    headers = {"Content-Type": "application/json"}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    
    # 构建 OpenAI 兼容的请求体（model 在请求体中）
    payload = {
        "model": model or STANDARD_MODEL,  # model 参数在请求体中
        "messages": messages,
    }
    
    # 添加其他参数
    if "temperature" in kwargs:
        payload["temperature"] = kwargs["temperature"]
    if "max_tokens" in kwargs:
        payload["max_tokens"] = kwargs["max_tokens"]
    if "top_p" in kwargs:
        payload["top_p"] = kwargs["top_p"]
    if "stream" in kwargs:
        payload["stream"] = kwargs["stream"]
    
    print(f"调用 OpenAI 兼容 API: {url}")
    print(f"模型: {model}")
    print(f"消息数量: {len(messages)}")
    
    try:
        response = requests.post(url, json=payload, headers=headers, timeout=60)
        response.raise_for_status()
        
        data = response.json()
        
        print(f"✓ 调用成功")
        
        # 解析 OpenAI 格式的响应
        if "choices" in data and len(data["choices"]) > 0:
            choice = data["choices"][0]
            message = choice.get("message", {})
            content = message.get("content", "")
            finish_reason = choice.get("finish_reason", "")
            
            print(f"回复: {content}")
            print(f"完成原因: {finish_reason}")
        
        # 显示使用统计
        if "usage" in data:
            usage = data["usage"]
            print(f"Token 使用:")
            print(f"  Prompt: {usage.get('prompt_tokens', 0)}")
            print(f"  Completion: {usage.get('completion_tokens', 0)}")
            print(f"  总计: {usage.get('total_tokens', 0)}")
        
        return data
        
    except requests.RequestsError as e:
        print(f"✗ 请求失败: {e}")
        if hasattr(e, 'response') and e.response is not None:
            print(f"响应: {e.response.text}")
        return None
    except Exception as e:
        print(f"✗ 发生错误: {e}")
        return None


def openai_chat_with_system_prompt(system_prompt, user_prompt, **kwargs):
    """使用系统提示的聊天"""
    messages = [
        {"role": "system", "content": system_prompt},
        {"role": "user", "content": user_prompt}
    ]
    return openai_chat_completions(messages, **kwargs)


def openai_chat_conversation(conversation_history, **kwargs):
    """多轮对话"""
    return openai_chat_completions(conversation_history, **kwargs)


if __name__ == "__main__":
    print("=" * 60)
    print("LLM Router OpenAI 兼容 API 示例")
    print("=" * 60)
    print()
    
    # 示例 1: 简单对话
    print("示例 1: 简单对话")
    print("-" * 60)
    messages = [
        {"role": "user", "content": "Hello! How are you?"}
    ]
    openai_chat_completions(messages, temperature=0.7, max_tokens=100)
    print()
    
    # 示例 2: 带系统提示
    print("示例 2: 带系统提示")
    print("-" * 60)
    openai_chat_with_system_prompt(
        "你是一个专业的 Python 编程助手，擅长编写清晰、高效的代码。",
        "请写一个快速排序算法的 Python 实现",
        temperature=0.3,
        max_tokens=500
    )
    print()
    
    # 示例 3: 多轮对话
    print("示例 3: 多轮对话")
    print("-" * 60)
    conversation = [
        {"role": "user", "content": "What is Python?"},
        {"role": "assistant", "content": "Python is a high-level programming language known for its simplicity and readability."},
        {"role": "user", "content": "Can you give me an example?"}
    ]
    openai_chat_completions(conversation, temperature=0.7, max_tokens=200)
    print()
    
    # 示例 4: 指定模型（覆盖远程标识符）
    print("示例 4: 指定模型覆盖")
    print("-" * 60)
    print("注意: 可以在请求中指定 model 参数来覆盖远程模型标识符")
    messages = [
        {"role": "user", "content": "Say hello"}
    ]
    # 使用 model 参数覆盖
    openai_chat_completions(
        messages,
        model="x-ai/grok-beta",  # 覆盖远程标识符
        temperature=0.7,
        max_tokens=50
    )
    print()
    
    # 示例 5: 使用不同参数
    print("示例 5: 使用不同参数")
    print("-" * 60)
    messages = [
        {"role": "user", "content": "Write a creative haiku about technology"}
    ]
    openai_chat_completions(
        messages,
        temperature=0.9,  # 高温度，更创造性
        top_p=0.95,
        max_tokens=100
    )
    print()
    
    print("提示:")
    print("1. OpenAI 兼容 API 可以无缝替换 OpenAI SDK")
    print("2. 使用 /models/{provider}/{model}/v1/chat/completions 端点")
    print("3. 如果已绑定模型到 Session，可以不指定 model 参数")
    print("4. 支持所有 OpenAI API 的标准参数")

