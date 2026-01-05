#!/usr/bin/env python3
"""
流式响应示例

演示如何处理流式响应（Server-Sent Events）。
注意：当前实现可能不支持所有 Provider 的流式响应。
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

# 使用免费模型作为示例
PROVIDER_NAME = "openrouter"
MODEL_NAME = "openrouter-llama-3.3-70b-instruct"


def stream_invoke(messages, temperature=0.7, max_tokens=200):
    """流式调用模型（使用标准 OpenAI API 端点）"""
    # 使用标准 OpenAI API 端点
    url = f"{BASE_URL}/v1/chat/completions"
    
    headers = {"Content-Type": "application/json"}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    
    payload = {
        "model": "openrouter/glm-4.5-air",  # 使用标准格式
        "messages": messages,
        "stream": True,
        "temperature": temperature,
        "max_tokens": max_tokens
    }
    
    print(f"流式调用模型: {PROVIDER_NAME}/{MODEL_NAME}")
    if messages:
        print(f"提示词: {messages[0].get('content', '')[:50]}...")
    print("\n流式输出:")
    print("-" * 60)
    
    try:
        # 使用流式请求
        response = requests.post(
            url, 
            json=payload, 
            headers=headers, 
            timeout=60,
            stream=True
        )
        response.raise_for_status()
        
        # 处理流式响应（JSONL 格式）
        full_text = ""
        for line in response.iter_lines():
            if not line:
                continue
            
            # 解析 JSONL
            try:
                data = json.loads(line)
                
                # 提取文本片段
                if 'delta' in data:
                    text_piece = data['delta'].get('content', '')
                elif 'text' in data:
                    text_piece = data['text']
                else:
                    text_piece = data.get('output_text', '')
                
                if text_piece:
                    print(text_piece, end='', flush=True)
                    full_text += text_piece
                
                # 检查是否完成
                if data.get('is_final') or data.get('finish_reason'):
                    break
                    
            except json.JSONDecodeError:
                continue
        
        print("\n" + "-" * 60)
        print(f"\n✓ 流式调用完成")
        print(f"完整输出: {full_text}")
        
        return full_text
        
    except requests.RequestsError as e:
        print(f"\n✗ 请求失败: {e}")
        if hasattr(e, 'response') and e.response is not None:
            print(f"响应: {e.response.text}")
        return None
    except Exception as e:
        print(f"\n✗ 发生错误: {e}")
        return None


def stream_openai_compatible(messages, temperature=0.7, max_tokens=200):
    """流式调用 OpenAI 兼容 API"""
    url = f"{BASE_URL}/models/{PROVIDER_NAME}/{MODEL_NAME}/v1/chat/completions"
    
    headers = {"Content-Type": "application/json"}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    
    payload = {
        "messages": messages,
        "temperature": temperature,
        "max_tokens": max_tokens,
        "stream": True
    }
    
    print(f"流式调用 OpenAI 兼容 API: {PROVIDER_NAME}/{MODEL_NAME}")
    print("\n流式输出:")
    print("-" * 60)
    
    try:
        response = requests.post(
            url,
            json=payload,
            headers=headers,
            timeout=60,
            stream=True
        )
        response.raise_for_status()
        
        # OpenAI 兼容格式使用 Server-Sent Events (SSE)
        full_text = ""
        for line in response.iter_lines():
            if not line:
                continue
            
            # SSE 格式: "data: {...}\n\n"
            line_str = line.decode('utf-8')
            if line_str.startswith("data: "):
                data_str = line_str[6:]  # 移除 "data: " 前缀
                
                if data_str.strip() == "[DONE]":
                    break
                
                try:
                    data = json.loads(data_str)
                    
                    # 提取 delta content
                    if 'choices' in data and len(data['choices']) > 0:
                        delta = data['choices'][0].get('delta', {})
                        content = delta.get('content', '')
                        
                        if content:
                            print(content, end='', flush=True)
                            full_text += content
                        
                        # 检查完成原因
                        finish_reason = data['choices'][0].get('finish_reason')
                        if finish_reason:
                            break
                            
                except json.JSONDecodeError:
                    continue
        
        print("\n" + "-" * 60)
        print(f"\n✓ 流式调用完成")
        print(f"完整输出: {full_text}")
        
        return full_text
        
    except requests.RequestsError as e:
        print(f"\n✗ 请求失败: {e}")
        if hasattr(e, 'response') and e.response is not None:
            print(f"响应: {e.response.text}")
        return None
    except Exception as e:
        print(f"\n✗ 发生错误: {e}")
        return None


def stream_route(tags, prompt, temperature=0.7, max_tokens=200):
    """流式路由调用"""
    url = f"{BASE_URL}/route/invoke"
    
    headers = {"Content-Type": "application/json"}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    
    payload = {
        "query": {
            "tags": tags if isinstance(tags, list) else [tags]
        },
        "request": {
            "prompt": prompt,
            "parameters": {
                "temperature": temperature,
                "max_tokens": max_tokens
            },
            "stream": True
        }
    }
    
    print(f"流式路由调用 (tags: {tags})")
    print(f"提示词: {prompt}")
    print("\n流式输出:")
    print("-" * 60)
    
    try:
        response = requests.post(
            url,
            json=payload,
            headers=headers,
            timeout=60,
            stream=True
        )
        response.raise_for_status()
        
        full_text = ""
        for line in response.iter_lines():
            if not line:
                continue
            
            try:
                data = json.loads(line)
                
                text_piece = data.get('text', '') or data.get('delta', {}).get('content', '')
                if text_piece:
                    print(text_piece, end='', flush=True)
                    full_text += text_piece
                
                if data.get('is_final') or data.get('finish_reason'):
                    break
                    
            except json.JSONDecodeError:
                continue
        
        print("\n" + "-" * 60)
        print(f"\n✓ 流式路由完成")
        print(f"完整输出: {full_text}")
        
        return full_text
        
    except requests.RequestsError as e:
        print(f"\n✗ 请求失败: {e}")
        return None
    except Exception as e:
        print(f"\n✗ 发生错误: {e}")
        return None


if __name__ == "__main__":
    print("=" * 60)
    print("LLM Router 流式响应示例")
    print("=" * 60)
    print()
    
    # 注意：流式响应可能不被所有 Provider 支持
    # 如果遇到错误，可能是该 Provider 不支持流式输出
    
    # 示例 1: 标准接口流式调用
    print("示例 1: 标准接口流式调用")
    print("-" * 60)
    messages = [
        {"role": "user", "content": "Write a short story about a robot learning to paint"}
    ]
    stream_invoke(messages, max_tokens=300)
    print()
    
    # 示例 2: OpenAI 兼容 API 流式调用
    print("示例 2: OpenAI 兼容 API 流式调用")
    print("-" * 60)
    messages = [
        {"role": "user", "content": "Explain quantum computing in simple terms"}
    ]
    stream_openai_compatible(messages, max_tokens=300)
    print()
    
    # 示例 3: 流式路由
    print("示例 3: 流式路由")
    print("-" * 60)
    stream_route(["free", "fast"], "What is artificial intelligence?", max_tokens=200)
    print()
    
    print("提示:")
    print("1. 流式响应适用于需要实时显示输出的场景")
    print("2. 某些 Provider 可能不支持流式输出，会返回错误")
    print("3. 流式响应使用 JSONL 或 SSE 格式")
    print("4. 注意处理网络中断和超时情况")

