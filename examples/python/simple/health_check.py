#!/usr/bin/env python3
"""
健康检查示例

演示如何检查 LLM Router 服务的健康状态。
本机请求（localhost）无需认证，远程请求需要认证。
"""

import os
from curl_cffi import requests
from dotenv import load_dotenv

# 加载环境变量
load_dotenv()

# 配置
BASE_URL = os.getenv("LLM_ROUTER_BASE_URL", "http://localhost:18000")
API_KEY = os.getenv("LLM_ROUTER_API_KEY")  # 可选，远程请求时需要


def health_check():
    """检查服务健康状态"""
    url = f"{BASE_URL}/health"
    
    print(f"检查服务健康状态: {url}")
    
    try:
        # 本机请求无需认证，远程请求需要添加认证头
        headers = {}
        if API_KEY:
            headers["Authorization"] = f"Bearer {API_KEY}"
        
        response = requests.get(url, headers=headers, timeout=10)
        
        if response.status_code == 200:
            data = response.json()
            print(f"✓ 服务运行正常: {data}")
            return True
        else:
            print(f"✗ 服务异常: {response.status_code} - {response.text}")
            return False
            
    except requests.RequestsError as e:
        print(f"✗ 请求失败: {e}")
        return False
    except Exception as e:
        print(f"✗ 发生错误: {e}")
        return False


if __name__ == "__main__":
    print("=" * 60)
    print("LLM Router 健康检查示例")
    print("=" * 60)
    print()
    
    health_check()

