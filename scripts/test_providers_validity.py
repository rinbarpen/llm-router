#!/usr/bin/env python3
import json
import sys
import asyncio
import httpx
from pathlib import Path
import tomli

# 配置文件路径
ROUTER_TOML = Path(__file__).parent.parent / "router.toml"
BASE_URL = "http://localhost:18000"
TIMEOUT = 60.0

def load_models_from_config():
    """从 router.toml 中提取所有模型"""
    if not ROUTER_TOML.exists():
        print(f"错误: 找不到配置文件 {ROUTER_TOML}")
        sys.exit(1)

    with ROUTER_TOML.open("rb") as f:
        config = tomli.load(f)

    models = []
    for model in config.get("models", []):
        models.append({
            "name": model.get("name"),
            "provider": model.get("provider"),
            "display_name": model.get("display_name", model.get("name")),
        })

    return models

async def test_model(client, model):
    """测试一个模型是否有效"""
    provider_name = model["provider"]
    model_name = model["name"]
    display_name = model["display_name"]
    
    print(f"正在测试: {provider_name}/{model_name} ({display_name})...", end="", flush=True)
    
    # 使用 invoke 接口进行测试，因为它是最直接的
    url = f"{BASE_URL}/models/{provider_name}/{model_name}/invoke"
    
    payload = {
        "prompt": "Hi, please respond with 'OK' only.",
        "parameters": {"max_tokens": 10, "temperature": 0.0}
    }
    
    try:
        response = await client.post(url, json=payload, timeout=TIMEOUT)
        
        if response.status_code == 200:
            data = response.json()
            content = data.get("output_text", "").strip()
            print(f" ✓ 成功 (回复: {content[:20]})")
            return {
                "model": model_name,
                "provider": provider_name,
                "status": "success",
                "response": content,
                "error": None
            }
        else:
            error_msg = f"HTTP {response.status_code}"
            try:
                detail = response.json().get("detail", "")
                if detail:
                    error_msg += f": {detail}"
            except:
                error_msg += f": {response.text[:100]}"
            print(f" ✗ 失败 ({error_msg})")
            return {
                "model": model_name,
                "provider": provider_name,
                "status": "failed",
                "response": None,
                "error": error_msg
            }
            
    except Exception as e:
        error_msg = str(e)
        print(f" ✗ 异常 ({error_msg})")
        import traceback
        traceback.print_exc()
        return {
            "model": model_name,
            "provider": provider_name,
            "status": "error",
            "response": None,
            "error": error_msg
        }

async def main():
    print("=" * 60)
    print("LLM Router Provider 有效性测试")
    print("=" * 60)
    
    models = load_models_from_config()
    print(f"找到 {len(models)} 个模型需要测试\n")
    
    results = []
    async with httpx.AsyncClient(trust_env=True) as client:
        # 为了不给后端太大压力，我们顺序测试，或者小并发
        # 这里选择顺序测试以获得清晰的输出
        for model in models:
            result = await test_model(client, model)
            results.append(result)
            
    # 输出总结报告
    print("\n" + "=" * 60)
    print("测试总结报告")
    print("=" * 60)
    
    success_count = sum(1 for r in results if r["status"] == "success")
    failed_count = len(results) - success_count
    
    print(f"总计: {len(results)}")
    print(f"成功: {success_count}")
    print(f"失败: {failed_count}")
    
    if failed_count > 0:
        print("\n失败模型详情:")
        for r in results:
            if r["status"] != "success":
                print(f"- {r['provider']}/{r['model']}: {r['error']}")
    
    # 保存结果
    output_path = Path("test_report.json")
    with open(output_path, "w", encoding="utf-8") as f:
        json.dump({
            "summary": {
                "total": len(results),
                "success": success_count,
                "failed": failed_count
            },
            "details": results
        }, f, indent=2, ensure_ascii=False)
    print(f"\n详细报告已保存至: {output_path}")

if __name__ == "__main__":
    asyncio.run(main())
