#!/usr/bin/env python3
"""
多模态输入示例

演示如何使用图像、音频、视频等多模态输入。
注意：多模态支持取决于具体的 Provider 和模型。
"""

import os
import base64
from curl_cffi import requests
from dotenv import load_dotenv

# 加载环境变量
load_dotenv()

# 配置
BASE_URL = os.getenv("LLM_ROUTER_BASE_URL", "http://localhost:18000")
API_KEY = os.getenv("LLM_ROUTER_API_KEY")  # 可选，远程请求时需要


def invoke_with_image_url(provider_name, model_name, image_url, text_prompt):
    """使用图像 URL 调用支持视觉的模型（OpenAI 兼容格式）"""
    url = f"{BASE_URL}/models/{provider_name}/{model_name}/invoke"
    
    headers = {"Content-Type": "application/json"}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    
    # OpenAI 兼容格式：content 为数组
    messages = [
        {
            "role": "user",
            "content": [
                {
                    "type": "text",
                    "text": text_prompt
                },
                {
                    "type": "image_url",
                    "image_url": {
                        "url": image_url
                    }
                }
            ]
        }
    ]
    
    payload = {
        "messages": messages,
        "parameters": {
            "max_tokens": 300
        }
    }
    
    print(f"调用模型: {provider_name}/{model_name}")
    print(f"图像 URL: {image_url}")
    print(f"文本提示: {text_prompt}")
    
    try:
        response = requests.post(url, json=payload, headers=headers, timeout=60)
        response.raise_for_status()
        
        data = response.json()
        print(f"✓ 调用成功")
        print(f"输出: {data.get('output_text', 'N/A')}")
        
        return data
        
    except requests.RequestsError as e:
        print(f"✗ 请求失败: {e}")
        if hasattr(e, 'response') and e.response is not None:
            print(f"响应: {e.response.text}")
        return None
    except Exception as e:
        print(f"✗ 发生错误: {e}")
        return None


def invoke_with_image_base64(provider_name, model_name, image_path, text_prompt):
    """使用 Base64 编码的图像调用模型"""
    # 读取图像文件并编码为 Base64
    try:
        with open(image_path, "rb") as f:
            image_data = f.read()
            image_base64 = base64.b64encode(image_data).decode("utf-8")
            
            # 根据文件扩展名确定 MIME 类型
            ext = image_path.lower().split(".")[-1]
            mime_type = {
                "jpg": "image/jpeg",
                "jpeg": "image/jpeg",
                "png": "image/png",
                "gif": "image/gif",
                "webp": "image/webp"
            }.get(ext, "image/jpeg")
            
            data_url = f"data:{mime_type};base64,{image_base64}"
            
    except FileNotFoundError:
        print(f"✗ 错误: 文件不存在: {image_path}")
        return None
    except Exception as e:
        print(f"✗ 错误: 无法读取图像文件: {e}")
        return None
    
    url = f"{BASE_URL}/models/{provider_name}/{model_name}/invoke"
    
    headers = {"Content-Type": "application/json"}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    
    messages = [
        {
            "role": "user",
            "content": [
                {
                    "type": "text",
                    "text": text_prompt
                },
                {
                    "type": "image_url",
                    "image_url": {
                        "url": data_url
                    }
                }
            ]
        }
    ]
    
    payload = {
        "messages": messages,
        "parameters": {
            "max_tokens": 300
        }
    }
    
    print(f"调用模型: {provider_name}/{model_name}")
    print(f"图像文件: {image_path}")
    print(f"文本提示: {text_prompt}")
    
    try:
        response = requests.post(url, json=payload, headers=headers, timeout=60)
        response.raise_for_status()
        
        data = response.json()
        print(f"✓ 调用成功")
        print(f"输出: {data.get('output_text', 'N/A')}")
        
        return data
        
    except requests.RequestsError as e:
        print(f"✗ 请求失败: {e}")
        if hasattr(e, 'response') and e.response is not None:
            print(f"响应: {e.response.text}")
        return None
    except Exception as e:
        print(f"✗ 发生错误: {e}")
        return None


def invoke_gemini_with_image_url(provider_name, model_name, image_url, text_prompt):
    """使用 Gemini 格式调用（图像 URL 在 content 字符串中）"""
    url = f"{BASE_URL}/models/{provider_name}/{model_name}/invoke"
    
    headers = {"Content-Type": "application/json"}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    
    # Gemini 格式：content 为字符串，包含 URL
    messages = [
        {
            "role": "user",
            "content": f"{text_prompt}\n\n图像: {image_url}"
        }
    ]
    
    payload = {
        "messages": messages,
        "parameters": {
            "max_tokens": 300
        }
    }
    
    print(f"调用模型 (Gemini 格式): {provider_name}/{model_name}")
    print(f"图像 URL: {image_url}")
    print(f"文本提示: {text_prompt}")
    
    try:
        response = requests.post(url, json=payload, headers=headers, timeout=60)
        response.raise_for_status()
        
        data = response.json()
        print(f"✓ 调用成功")
        print(f"输出: {data.get('output_text', 'N/A')}")
        
        return data
        
    except requests.RequestsError as e:
        print(f"✗ 请求失败: {e}")
        if hasattr(e, 'response') and e.response is not None:
            print(f"响应: {e.response.text}")
        return None
    except Exception as e:
        print(f"✗ 发生错误: {e}")
        return None


def invoke_with_multiple_images(provider_name, model_name, image_urls, text_prompt):
    """使用多张图像调用模型"""
    url = f"{BASE_URL}/models/{provider_name}/{model_name}/invoke"
    
    headers = {"Content-Type": "application/json"}
    if API_KEY:
        headers["Authorization"] = f"Bearer {API_KEY}"
    
    # 构建包含多张图像的内容
    content = [{"type": "text", "text": text_prompt}]
    for image_url in image_urls:
        content.append({
            "type": "image_url",
            "image_url": {"url": image_url}
        })
    
    messages = [
        {
            "role": "user",
            "content": content
        }
    ]
    
    payload = {
        "messages": messages,
        "parameters": {
            "max_tokens": 500
        }
    }
    
    print(f"调用模型: {provider_name}/{model_name}")
    print(f"图像数量: {len(image_urls)}")
    print(f"文本提示: {text_prompt}")
    
    try:
        response = requests.post(url, json=payload, headers=headers, timeout=60)
        response.raise_for_status()
        
        data = response.json()
        print(f"✓ 调用成功")
        print(f"输出: {data.get('output_text', 'N/A')}")
        
        return data
        
    except requests.RequestsError as e:
        print(f"✗ 请求失败: {e}")
        if hasattr(e, 'response') and e.response is not None:
            print(f"响应: {e.response.text}")
        return None
    except Exception as e:
        print(f"✗ 发生错误: {e}")
        return None


if __name__ == "__main__":
    print("=" * 60)
    print("LLM Router 多模态输入示例")
    print("=" * 60)
    print()
    
    # 注意：以下示例需要实际可访问的图像 URL 或本地图像文件
    # 这里使用示例 URL，实际使用时请替换为真实的图像 URL
    
    # 示例 1: OpenAI 兼容格式 - 图像 URL
    print("示例 1: OpenAI 兼容格式 - 图像 URL")
    print("-" * 60)
    print("注意: 需要替换为实际可访问的图像 URL")
    # invoke_with_image_url(
    #     "openai", "gpt-4o",
    #     "https://example.com/image.jpg",
    #     "请描述这张图片"
    # )
    print("（示例代码已注释，需要实际图像 URL 才能运行）")
    print()
    
    # 示例 2: Base64 编码图像
    print("示例 2: Base64 编码图像")
    print("-" * 60)
    print("注意: 需要替换为实际的本地图像文件路径")
    # invoke_with_image_base64(
    #     "openai", "gpt-4o",
    #     "/path/to/image.jpg",
    #     "这张图片里有什么？"
    # )
    print("（示例代码已注释，需要实际图像文件才能运行）")
    print()
    
    # 示例 3: Gemini 格式
    print("示例 3: Gemini 格式 - 图像 URL")
    print("-" * 60)
    print("注意: 需要替换为实际可访问的图像 URL")
    # invoke_gemini_with_image_url(
    #     "gemini", "gemini-2.5-pro",
    #     "https://example.com/image.jpg",
    #     "请分析这张图片"
    # )
    print("（示例代码已注释，需要实际图像 URL 才能运行）")
    print()
    
    # 示例 4: 多张图像
    print("示例 4: 多张图像")
    print("-" * 60)
    print("注意: 需要替换为实际可访问的图像 URL")
    # invoke_with_multiple_images(
    #     "openai", "gpt-4o",
    #     [
    #         "https://example.com/image1.jpg",
    #         "https://example.com/image2.jpg"
    #     ],
    #     "比较这两张图片的差异"
    # )
    print("（示例代码已注释，需要实际图像 URL 才能运行）")
    print()
    
    print("提示:")
    print("1. 确保使用的模型支持视觉功能（检查 config.supports_vision）")
    print("2. OpenAI 兼容格式适用于 GPT-4 Vision、Claude 等模型")
    print("3. Gemini 格式适用于 Gemini 系列模型")
    print("4. 图像格式支持: JPEG, PNG, GIF, WebP")
    print("5. 图像大小建议: 小于 20MB")

