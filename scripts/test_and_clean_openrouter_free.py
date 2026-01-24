#!/usr/bin/env python3
"""
测试并清理 OpenRouter 免费模型
自动测试所有标记为 free 的 openrouter 模型，并移除无效的模型配置
"""

import json
import re
import sys
from pathlib import Path

import requests
import tomli

# 配置文件路径
ROUTER_TOML = Path(__file__).parent.parent / "router.toml"
BASE_URL = "http://localhost:18000/v1/chat/completions"
TIMEOUT = 30
BACKUP_FILE = Path(__file__).parent.parent / "router.toml.backup"
TEST_RESULTS = Path(__file__).parent.parent / "test_results.json"


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
            return True, content.strip()
        else:
            try:
                error_data = response.json()
                error_msg = error_data.get("detail", error_data.get("error", {}).get("message", str(error_data)))
            except:
                error_msg = response.text[:200]
            return False, error_msg
    except requests.exceptions.Timeout:
        return False, f"超时（{TIMEOUT}秒）"
    except requests.exceptions.ConnectionError:
        return False, f"连接错误：无法连接到 {BASE_URL}"
    except Exception as e:
        return False, f"异常: {e}"


def remove_invalid_models(invalid_model_names):
    """从 router.toml 中移除无效模型"""
    if not invalid_model_names:
        print("\n没有无效模型需要移除")
        return

    # 读取原始文件
    with ROUTER_TOML.open("r", encoding="utf-8") as f:
        original_content = f.read()

    # 创建备份
    if not BACKUP_FILE.exists():
        with BACKUP_FILE.open("w", encoding="utf-8") as f:
            f.write(original_content)
        print(f"\n备份已创建: {BACKUP_FILE}")

    # 移除无效模型块
    lines = original_content.split('\n')
    result_lines = []
    i = 0
    removed_count = 0

    while i < len(lines):
        line = lines[i]
        if line.strip() == "[[models]]":
            model_block_lines = [line]
            j = i + 1
            model_name = None
            should_remove = False

            while j < len(lines):
                next_line = lines[j]
                if next_line.strip() == "[[models]]":
                    break
                name_match = re.match(r'^\s*name\s*=\s*"([^"]+)"', next_line)
                if name_match:
                    model_name = name_match.group(1)
                    if model_name in invalid_model_names:
                        should_remove = True
                model_block_lines.append(next_line)
                j += 1

            if should_remove:
                print(f"  移除模型: {model_name}")
                removed_count += 1
                i = j
            else:
                result_lines.extend(model_block_lines)
                i = j
        else:
            result_lines.append(line)
            i += 1

    # 写入更新后的文件
    new_content = '\n'.join(result_lines)
    with ROUTER_TOML.open("w", encoding="utf-8") as f:
        f.write(new_content)

    print(f"\n✓ 已移除 {removed_count} 个无效模型")


def main():
    print("=" * 60)
    print("测试并清理 OpenRouter 免费模型")
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
        is_valid, result = test_model(model["name"])
        if is_valid:
            print(f"  ✓ 成功")
            working_models.append(model)
        else:
            print(f"  ✗ 失败: {result[:100]}")
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

    # 保存结果到文件
    results_data = {
        "working_models": [m["name"] for m in working_models],
        "invalid_models": [m["name"] for m in invalid_models],
    }
    with TEST_RESULTS.open("w", encoding="utf-8") as f:
        json.dump(results_data, f, indent=2, ensure_ascii=False)
    print(f"\n测试结果已保存到: {TEST_RESULTS}")

    # 询问是否移除无效模型
    if invalid_models:
        print("\n" + "=" * 60)
        print("清理无效模型")
        print("=" * 60)
        remove_invalid_models(set(m["name"] for m in invalid_models))
    else:
        print("\n所有模型都有效，无需清理")


if __name__ == "__main__":
    try:
        main()
        sys.exit(0)
    except KeyboardInterrupt:
        print("\n\n操作被用户中断")
        sys.exit(1)
    except Exception as e:
        print(f"\n\n发生错误: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)
