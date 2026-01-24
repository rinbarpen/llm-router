#!/usr/bin/env python3
"""
测试所有 OpenRouter 免费模型的有效性
从 router.toml 中提取所有标记为 free 的 openrouter 模型并测试
"""

import json
import sys
from pathlib import Path

import requests
import tomli

# 配置文件路径
ROUTER_TOML = Path(__file__).parent / "router.toml"
BASE_URL = "http://localhost:18000/v1/chat/completions"
TIMEOUT = 30


def load_models_from_config():
    """从 router.toml 中提取所有 free 的 openrouter 模型"""
    if not ROUTER_TOML.exists():
        print(f"错误: 找不到配置文件 {ROUTER_TOML}")
        sys.exit(1)

    with ROUTER_TOML.open("rb") as f:
        config = tomli.load(f)

    free_models = []
    for model in config.get("models", []):
        if (
            model.get("provider") == "openrouter"
            and "free" in model.get("tags", [])
        ):
            free_models.append({
                "name": model.get("name"),
                "display_name": model.get("display_name", model.get("name")),
                "remote_identifier": model.get("remote_identifier", ""),
            })

    return free_models


def test_model(model_name):
    """测试一个模型是否有效"""
    print(f"\n测试模型: {model_name}")
    print("-" * 60)

    # 使用最简单的请求（避免参数兼容性问题）
    payload = {
        "model": f"openrouter/{model_name}",
        "messages": [
            {"role": "user", "content": "Say hello in one word."}
        ],
        "max_tokens": 10
    }

    try:
        response = requests.post(BASE_URL, json=payload, timeout=TIMEOUT)

        if response.status_code == 200:
            data = response.json()
            content = data.get("choices", [{}])[0].get("message", {}).get("content", "")
            print(f"✓ 成功！回复: {content.strip()}")
            return True
        else:
            print(f"✗ 失败！状态码: {response.status_code}")
            try:
                error_data = response.json()
                error_msg = error_data.get("detail", error_data.get("error", {}).get("message", str(error_data)))
                print(f"  错误: {error_msg}")
            except:
                print(f"  响应: {response.text[:200]}")
            return False

    except requests.exceptions.Timeout:
        print(f"✗ 超时（{TIMEOUT}秒）")
        return False
    except requests.exceptions.ConnectionError:
        print(f"✗ 连接错误：无法连接到 {BASE_URL}")
        print("  请确保服务正在运行")
        return False
    except Exception as e:
        print(f"✗ 异常: {e}")
        return False


def main():
    print("=" * 60)
    print("测试 OpenRouter 免费模型")
    print("=" * 60)
    print(f"配置文件: {ROUTER_TOML}")
    print(f"API 地址: {BASE_URL}")
    print()

    # 加载模型列表
    models = load_models_from_config()
    print(f"找到 {len(models)} 个免费模型需要测试\n")

    if not models:
        print("没有找到需要测试的模型")
        return

    # 测试所有模型
    working_models = []
    invalid_models = []

    for i, model in enumerate(models, 1):
        print(f"[{i}/{len(models)}] {model['display_name']} ({model['name']})")
        if test_model(model["name"]):
            working_models.append(model)
        else:
            invalid_models.append(model)

    # 输出总结
    print("\n" + "=" * 60)
    print("测试总结")
    print("=" * 60)
    print(f"总模型数: {len(models)}")
    print(f"有效模型: {len(working_models)}")
    print(f"无效模型: {len(invalid_models)}")

    if working_models:
        print(f"\n✓ 有效模型列表 ({len(working_models)}):")
        for model in working_models:
            print(f"  - {model['name']} ({model['display_name']})")

    if invalid_models:
        print(f"\n✗ 无效模型列表 ({len(invalid_models)}):")
        for model in invalid_models:
            print(f"  - {model['name']} ({model['display_name']})")
        print("\n这些模型将从 router.toml 中移除")

    # 保存结果到文件
    results_file = Path(__file__).parent / "test_results.json"
    with results_file.open("w", encoding="utf-8") as f:
        json.dump({
            "working_models": [m["name"] for m in working_models],
            "invalid_models": [m["name"] for m in invalid_models],
        }, f, indent=2, ensure_ascii=False)
    print(f"\n测试结果已保存到: {results_file}")

    return invalid_models


if __name__ == "__main__":
    try:
        invalid_models = main()
        sys.exit(0 if invalid_models else 0)
    except KeyboardInterrupt:
        print("\n\n测试被用户中断")
        sys.exit(1)
    except Exception as e:
        print(f"\n\n发生错误: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)
