#!/usr/bin/env python3
"""
错误处理示例

演示如何处理各种错误情况，包括网络错误、API 错误、限流等。
"""

import os
import time
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


class LLMRouterError(Exception):
    """LLM Router 基础错误类"""
    pass


class AuthenticationError(LLMRouterError):
    """认证错误"""
    pass


class RateLimitError(LLMRouterError):
    """限流错误"""
    pass


class ModelNotFoundError(LLMRouterError):
    """模型未找到错误"""
    pass


class APIError(LLMRouterError):
    """API 错误"""
    def __init__(self, message, status_code=None, response=None):
        super().__init__(message)
        self.status_code = status_code
        self.response = response


def invoke_with_error_handling(prompt: str, max_retries: int = 3, **kwargs):
    """带错误处理的调用"""
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
    
    last_exception = None
    
    for attempt in range(max_retries):
        try:
            response = requests.post(url, json=payload, headers=headers, timeout=60)
            
            # 处理不同的 HTTP 状态码
            if response.status_code == 200:
                return response.json()
            
            elif response.status_code == 401:
                raise AuthenticationError("认证失败: 无效的 API Key 或 Session Token")
            
            elif response.status_code == 403:
                raise AuthenticationError("权限不足: API Key 没有访问该模型的权限")
            
            elif response.status_code == 404:
                raise ModelNotFoundError(f"模型未找到: {PROVIDER_NAME}/{MODEL_NAME}")
            
            elif response.status_code == 429:
                # 限流错误，需要等待
                retry_after = int(response.headers.get("Retry-After", 60))
                raise RateLimitError(f"请求过于频繁，请在 {retry_after} 秒后重试")
            
            elif response.status_code >= 500:
                # 服务器错误，可以重试
                error_msg = f"服务器错误 ({response.status_code}): {response.text}"
                if attempt < max_retries - 1:
                    print(f"⚠ 服务器错误，重试中 ({attempt + 1}/{max_retries})...")
                    time.sleep(2 ** attempt)  # 指数退避
                    continue
                else:
                    raise APIError(error_msg, response.status_code, response)
            
            else:
                # 其他错误
                error_msg = f"请求失败 ({response.status_code}): {response.text}"
                raise APIError(error_msg, response.status_code, response)
        
        except RateLimitError as e:
            # 限流错误，等待后重试
            retry_after = 60  # 默认等待 60 秒
            if attempt < max_retries - 1:
                print(f"⚠ {e}，等待 {retry_after} 秒后重试...")
                time.sleep(retry_after)
                continue
            else:
                raise
        
        except (requests.RequestsError, requests.Timeout) as e:
            # 网络错误，可以重试
            last_exception = e
            if attempt < max_retries - 1:
                wait_time = 2 ** attempt  # 指数退避
                print(f"⚠ 网络错误: {e}，{wait_time} 秒后重试 ({attempt + 1}/{max_retries})...")
                time.sleep(wait_time)
                continue
            else:
                raise APIError(f"网络错误，已重试 {max_retries} 次: {e}") from e
        
        except (AuthenticationError, ModelNotFoundError) as e:
            # 这些错误不应该重试
            raise
        
        except Exception as e:
            # 其他未知错误
            last_exception = e
            if attempt < max_retries - 1:
                wait_time = 2 ** attempt
                print(f"⚠ 未知错误: {e}，{wait_time} 秒后重试 ({attempt + 1}/{max_retries})...")
                time.sleep(wait_time)
                continue
            else:
                raise APIError(f"请求失败，已重试 {max_retries} 次: {e}") from e
    
    # 如果所有重试都失败
    if last_exception:
        raise APIError(f"请求失败，已重试 {max_retries} 次") from last_exception


def safe_invoke(prompt: str, **kwargs):
    """安全调用，捕获所有错误并返回结果或错误信息"""
    try:
        result = invoke_with_error_handling(prompt, **kwargs)
        return {"success": True, "data": result}
    
    except AuthenticationError as e:
        return {"success": False, "error": "认证错误", "message": str(e)}
    
    except RateLimitError as e:
        return {"success": False, "error": "限流错误", "message": str(e)}
    
    except ModelNotFoundError as e:
        return {"success": False, "error": "模型未找到", "message": str(e)}
    
    except APIError as e:
        return {"success": False, "error": "API 错误", "message": str(e), "status_code": e.status_code}
    
    except Exception as e:
        return {"success": False, "error": "未知错误", "message": str(e)}


def check_service_health():
    """检查服务健康状态"""
    url = f"{BASE_URL}/health"
    
    try:
        response = requests.get(url, timeout=5)
        if response.status_code == 200:
            return True, "服务正常"
        else:
            return False, f"服务异常: {response.status_code}"
    except Exception as e:
        return False, f"无法连接到服务: {e}"


if __name__ == "__main__":
    print("=" * 60)
    print("LLM Router 错误处理示例")
    print("=" * 60)
    print()
    
    # 1. 检查服务健康
    print("1. 检查服务健康状态")
    print("-" * 60)
    is_healthy, message = check_service_health()
    print(f"{'✓' if is_healthy else '✗'} {message}")
    print()
    
    if not is_healthy:
        print("⚠ 服务不可用，无法继续演示")
        exit(1)
    
    # 2. 正常调用
    print("2. 正常调用（带错误处理）")
    print("-" * 60)
    result = safe_invoke("What is Python?", max_tokens=100)
    if result["success"]:
        print(f"✓ 调用成功")
        print(f"输出: {result['data'].get('output_text', 'N/A')}")
    else:
        print(f"✗ 调用失败: {result['error']} - {result['message']}")
    print()
    
    # 3. 处理认证错误（使用无效的 API Key）
    print("3. 处理认证错误")
    print("-" * 60)
    print("（演示代码，实际需要无效的 API Key 才能触发）")
    # 这里可以测试无效 API Key 的情况
    print()
    
    # 4. 处理网络超时
    print("4. 处理网络超时（带重试）")
    print("-" * 60)
    print("（演示代码，实际需要网络问题才能触发）")
    # 这里可以测试网络超时的情况
    print()
    
    # 5. 批量调用（带错误处理）
    print("5. 批量调用（带错误处理）")
    print("-" * 60)
    prompts = [
        "What is Python?",
        "What is JavaScript?",
        "Invalid prompt that might fail",
    ]
    
    results = []
    for prompt in prompts:
        result = safe_invoke(prompt, max_tokens=50)
        results.append(result)
        if result["success"]:
            print(f"✓ {prompt[:30]}... - 成功")
        else:
            print(f"✗ {prompt[:30]}... - 失败: {result['error']}")
    
    success_count = sum(1 for r in results if r["success"])
    print(f"\n统计: {success_count}/{len(results)} 成功")
    print()
    
    print("错误处理最佳实践:")
    print("1. 总是检查 HTTP 状态码")
    print("2. 区分可重试错误（网络错误、5xx）和不可重试错误（4xx）")
    print("3. 实现指数退避重试策略")
    print("4. 处理限流错误（429），等待 Retry-After 时间")
    print("5. 记录错误日志，便于调试和监控")
    print("6. 提供用户友好的错误消息")

