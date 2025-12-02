#!/usr/bin/env python3
"""
检查哪些免费模型现在可以调用
"""
import asyncio
import sys
from typing import Dict, List, Optional, Tuple

import httpx


async def check_model_availability(
    client: httpx.AsyncClient,
    api_url: str,
    provider: str,
    model: str,
    timeout: float = 10.0,
) -> Tuple[bool, Optional[str]]:
    """检查模型是否可用"""
    url = f"{api_url}/models/{provider}/{model}/invoke"
    payload = {
        "prompt": "hi",
        "parameters": {
            "max_tokens": 10,
            "temperature": 0.1,
        },
    }
    
    try:
        response = await client.post(
            url,
            json=payload,
            timeout=timeout,
        )
        if response.status_code == 200:
            return True, None
        else:
            return False, f"HTTP {response.status_code}: {response.text[:100]}"
    except httpx.TimeoutException:
        return False, "请求超时"
    except httpx.RequestError as e:
        return False, f"请求错误: {str(e)}"
    except Exception as e:
        return False, f"未知错误: {str(e)}"


async def main():
    api_url = "http://localhost:18000"
    
    # 检查服务是否运行
    try:
        async with httpx.AsyncClient() as client:
            response = await client.get(f"{api_url}/health", timeout=5.0)
            if response.status_code != 200:
                print("❌ 错误: 服务未运行或无法访问")
                sys.exit(1)
    except Exception as e:
        print(f"❌ 错误: 无法连接到服务: {e}")
        print(f"   请确保服务已启动: uv run llm-router")
        sys.exit(1)
    
    # 获取所有带有 "free" 标签的模型
    print("📋 获取免费模型列表...")
    async with httpx.AsyncClient() as client:
        try:
            response = await client.get(
                f"{api_url}/models",
                params={"tags": "free"},
                timeout=10.0,
            )
            if response.status_code != 200:
                print(f"❌ 错误: 获取模型列表失败 (HTTP {response.status_code})")
                sys.exit(1)
            models = response.json()
        except Exception as e:
            print(f"❌ 错误: 获取模型列表失败: {e}")
            sys.exit(1)
    
    if not models:
        print("⚠️  未找到带有 'free' 标签的模型")
        sys.exit(0)
    
    print(f"✅ 找到 {len(models)} 个免费模型\n")
    
    # 测试每个模型
    print("🔍 正在测试模型可用性...\n")
    
    results: List[Dict] = []
    
    async with httpx.AsyncClient() as client:
        # 创建所有测试任务
        tasks = []
        model_info = []
        for model in models:
            provider = model["provider_name"]
            model_name = model["name"]
            display_name = model.get("display_name", model_name)
            tags = model.get("tags", [])
            
            task = check_model_availability(
                client, api_url, provider, model_name
            )
            tasks.append(task)
            model_info.append({
                "provider": provider,
                "model": model_name,
                "display_name": display_name,
                "tags": tags,
            })
        
        # 并发测试所有模型
        test_results = await asyncio.gather(*tasks, return_exceptions=True)
        
        # 处理结果
        for info, result in zip(model_info, test_results):
            if isinstance(result, Exception):
                is_available = False
                error = f"异常: {str(result)}"
            else:
                is_available, error = result
            
            results.append({
                "provider": info["provider"],
                "model": info["model"],
                "display_name": info["display_name"],
                "tags": info["tags"],
                "available": is_available,
                "error": error,
            })
    
    # 显示结果
    available_models = [r for r in results if r["available"]]
    unavailable_models = [r for r in results if not r["available"]]
    
    print("=" * 80)
    print(f"✅ 可用模型 ({len(available_models)}/{len(results)}):")
    print("=" * 80)
    
    if available_models:
        for r in available_models:
            tags_str = ", ".join(r["tags"])
            print(f"  ✓ {r['provider']}/{r['model']}")
            print(f"    显示名称: {r['display_name']}")
            print(f"    标签: {tags_str}")
            print()
    else:
        print("  (无)")
        print()
    
    if unavailable_models:
        print("=" * 80)
        print(f"❌ 不可用模型 ({len(unavailable_models)}):")
        print("=" * 80)
        for r in unavailable_models:
            print(f"  ✗ {r['provider']}/{r['model']}")
            print(f"    显示名称: {r['display_name']}")
            if r["error"]:
                print(f"    错误: {r['error']}")
            print()
    
    # 总结
    print("=" * 80)
    print(f"📊 总结:")
    print(f"   总模型数: {len(results)}")
    print(f"   可用: {len(available_models)}")
    print(f"   不可用: {len(unavailable_models)}")
    print("=" * 80)
    
    # 返回退出码
    if len(available_models) == 0:
        sys.exit(1)
    else:
        sys.exit(0)


if __name__ == "__main__":
    asyncio.run(main())

