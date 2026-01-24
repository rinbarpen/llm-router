#!/usr/bin/env python3
"""
从 OpenRouter API 获取最新的免费模型并更新 router.toml
"""

import json
import os
import re
import sys
from pathlib import Path
from typing import Dict, List, Optional

import requests
import tomli

# 配置文件路径
ROUTER_TOML = Path(__file__).parent.parent / "router.toml"
BACKUP_FILE = Path(__file__).parent.parent / "router.toml.backup"
OPENROUTER_API_URL = "https://openrouter.ai/api/v1/models"
TIMEOUT = 30

# 中文模型提供者关键词
CHINESE_PROVIDERS = ["qwen", "glm", "z-ai", "kimi", "moonshotai", "deepseek", "alibaba", "tongyi", "xiaomi"]


def fetch_openrouter_models(api_key: Optional[str] = None) -> List[Dict]:
    """从 OpenRouter API 获取所有模型"""
    print(f"正在从 OpenRouter API 获取模型列表...")
    print(f"API 地址: {OPENROUTER_API_URL}")

    headers = {}
    if api_key:
        headers["Authorization"] = f"Bearer {api_key}"

    try:
        response = requests.get(OPENROUTER_API_URL, headers=headers, timeout=TIMEOUT)
        response.raise_for_status()
        data = response.json()
        models = data.get("data", [])
        print(f"✓ 成功获取 {len(models)} 个模型")
        return models
    except requests.exceptions.RequestException as e:
        print(f"✗ API 请求失败: {e}")
        sys.exit(1)


def is_free_model(model: Dict) -> bool:
    """判断模型是否为免费模型"""
    model_id = model.get("id", "")
    
    # 检查模型 ID 是否以 :free 结尾
    if model_id.endswith(":free"):
        return True
    
    # 检查定价是否为 0
    pricing = model.get("pricing", {})
    prompt_price = pricing.get("prompt", 0)
    completion_price = pricing.get("completion", 0)
    
    return prompt_price == 0 and completion_price == 0


def filter_free_models(models: List[Dict]) -> List[Dict]:
    """筛选出免费模型"""
    free_models = [m for m in models if is_free_model(m)]
    print(f"✓ 筛选出 {len(free_models)} 个免费模型")
    return free_models


def format_context_window(context_length: Optional[int]) -> str:
    """将上下文长度转换为可读格式"""
    if not context_length:
        return "128k"  # 默认值
    
    if context_length >= 1048576:
        return f"{context_length // 1048576}M"
    elif context_length >= 1024:
        return f"{context_length // 1024}k"
    else:
        return str(context_length)


def infer_tags(model: Dict) -> List[str]:
    """根据模型信息推断标签"""
    tags = ["free", "openrouter"]
    model_id = model.get("id", "").lower()
    name = model.get("name", "").lower()
    description = model.get("description", "").lower()
    
    # 基础标签
    tags.append("chat")
    tags.append("general")
    
    # 根据提供者推断
    if "meta-llama" in model_id or "llama" in model_id:
        tags.append("open-source")
    if "google" in model_id:
        tags.append("google")
    if "qwen" in model_id:
        tags.append("qwen")
        tags.append("chinese")
    if "glm" in model_id or "z-ai" in model_id:
        tags.append("glm")
        tags.append("chinese")
    if "kimi" in model_id or "moonshotai" in model_id:
        tags.append("kimi")
        tags.append("chinese")
    if "deepseek" in model_id:
        tags.append("chinese")
    if "mistral" in model_id:
        tags.append("mistral")
        tags.append("open-source")
    if "nvidia" in model_id:
        tags.append("nvidia")
    if "openai" in model_id:
        tags.append("openai")
    
    # 根据名称推断
    if "flash" in name or "fast" in name:
        tags.append("fast")
    if "pro" in name or "plus" in name:
        tags.append("high-quality")
    if "reasoning" in name or "think" in name or "r1" in name:
        tags.append("reasoning")
    if "long" in name or "context" in name:
        tags.append("long-context")
    if "instruct" in name:
        tags.append("instruction-tuned")
    if "coder" in name or "code" in name:
        tags.append("coding")
    
    # 根据功能推断
    architecture = model.get("architecture", {})
    if architecture.get("vision"):
        tags.append("image")
    if architecture.get("function_calling"):
        tags.append("function-call")
    
    # 去重并保持顺序
    seen = set()
    unique_tags = []
    for tag in tags:
        if tag not in seen:
            seen.add(tag)
            unique_tags.append(tag)
    
    return unique_tags


def infer_languages(model: Dict) -> List[str]:
    """根据模型信息推断支持的语言"""
    model_id = model.get("id", "").lower()
    
    # 检查是否为中文模型
    for provider in CHINESE_PROVIDERS:
        if provider in model_id:
            return ["zh", "en"]
    
    return ["en"]


def generate_model_name(model_id: str, existing_names: set) -> str:
    """从模型 ID 生成本地模型名称"""
    # 移除 :free 后缀
    if model_id.endswith(":free"):
        model_id = model_id[:-5]
    
    # 提取模型名称部分（provider/model-name -> model-name）
    if "/" in model_id:
        model_id = model_id.split("/", 1)[1]
    
    # 转换为小写，替换特殊字符为连字符
    name = re.sub(r'[^a-z0-9-]', '-', model_id.lower())
    name = re.sub(r'-+', '-', name)  # 合并多个连字符
    name = name.strip('-')  # 移除首尾连字符
    
    # 确保唯一性
    original_name = name
    counter = 1
    while name in existing_names:
        name = f"{original_name}-{counter}"
        counter += 1
    
    return name


def load_existing_models() -> Dict[str, Dict]:
    """加载现有配置中的 OpenRouter 免费模型"""
    if not ROUTER_TOML.exists():
        print(f"错误: 找不到配置文件 {ROUTER_TOML}")
        sys.exit(1)
    
    with ROUTER_TOML.open("rb") as f:
        config = tomli.load(f)
    
    existing = {}
    for model in config.get("models", []):
        if model.get("provider") == "openrouter" and "free" in model.get("tags", []):
            remote_id = model.get("remote_identifier", "")
            existing[remote_id] = {
                "name": model.get("name"),
                "display_name": model.get("display_name"),
                "remote_identifier": remote_id,
            }
    
    print(f"✓ 找到 {len(existing)} 个已配置的 OpenRouter 免费模型")
    return existing


def generate_model_config(model: Dict, existing_names: set) -> Dict:
    """为新模型生成配置"""
    model_id = model.get("id", "")
    name = model.get("name", model_id)
    
    # 生成本地模型名称
    local_name = generate_model_name(model_id, existing_names)
    existing_names.add(local_name)
    
    # 生成显示名称
    display_name = name
    if not display_name.endswith("(免费)"):
        display_name = f"{display_name} (免费)"
    
    # 推断配置
    context_length = model.get("context_length")
    architecture = model.get("architecture", {})
    
    config = {
        "name": local_name,
        "provider": "openrouter",
        "remote_identifier": model_id,
        "display_name": display_name,
        "tags": infer_tags(model),
        "config": {
            "context_window": format_context_window(context_length),
            "supports_vision": architecture.get("vision", False),
            "supports_tools": architecture.get("function_calling", False),
            "languages": infer_languages(model),
        }
    }
    
    return config


def format_model_toml(config: Dict) -> str:
    """将模型配置格式化为 TOML 格式"""
    lines = [
        "[[models]]",
        f'name = "{config["name"]}"',
        f'provider = "{config["provider"]}"',
        f'remote_identifier = "{config["remote_identifier"]}"',
        f'display_name = "{config["display_name"]}"',
        f'tags = {json.dumps(config["tags"], ensure_ascii=False)}',
        "[models.config]",
        f'context_window = "{config["config"]["context_window"]}"',
        f'supports_vision = {str(config["config"]["supports_vision"]).lower()}',
        f'supports_tools = {str(config["config"]["supports_tools"]).lower()}',
        f'languages = {json.dumps(config["config"]["languages"], ensure_ascii=False)}',
        "",
    ]
    return "\n".join(lines)


def find_openrouter_section(content: str) -> int:
    """找到 OpenRouter Models 部分的位置"""
    lines = content.split('\n')
    for i, line in enumerate(lines):
        if line.strip() == "######################" and i + 1 < len(lines):
            if "# OpenRouter Models" in lines[i + 1]:
                return i
    return -1


def find_insertion_point(content: str, section_start: int) -> int:
    """找到插入新模型的位置（OpenRouter 部分末尾，下一个部分之前）"""
    lines = content.split('\n')
    
    # 从 OpenRouter 部分开始查找（跳过标题行）
    i = section_start + 3  # 跳过 ######################, # OpenRouter Models #, ######################
    last_model_end = i
    
    while i < len(lines):
        line = lines[i]
        
        # 如果遇到下一个主要部分（以 ###################### 开头），停止
        if line.strip() == "######################" and i > section_start + 3:
            break
        
        # 记录最后一个模型块的结束位置
        if line.strip() == "[[models]]":
            # 查找这个模型块的结束位置
            j = i + 1
            while j < len(lines):
                next_line = lines[j]
                # 如果遇到下一个模型或下一个主要部分，停止
                if next_line.strip() == "[[models]]" or (
                    next_line.strip() == "######################" and j > section_start + 3
                ):
                    last_model_end = j
                    break
                j += 1
            if j >= len(lines):
                last_model_end = len(lines)
                break
        
        i += 1
    
    # 如果没找到任何模型，在 OpenRouter 部分标题后插入
    if last_model_end == section_start + 3:
        # 找到第一个空行或直接插入
        for j in range(section_start + 3, min(section_start + 10, len(lines))):
            if lines[j].strip() == "":
                last_model_end = j + 1
                break
    
    return last_model_end if last_model_end > section_start + 3 else section_start + 3


def add_models_to_config(new_models: List[Dict]) -> None:
    """将新模型添加到 router.toml"""
    if not new_models:
        print("\n没有新模型需要添加")
        return
    
    # 读取现有文件
    with ROUTER_TOML.open("r", encoding="utf-8") as f:
        content = f.read()
    
    # 创建备份
    if not BACKUP_FILE.exists():
        with BACKUP_FILE.open("w", encoding="utf-8") as f:
            f.write(content)
        print(f"\n备份已创建: {BACKUP_FILE}")
    
    # 找到 OpenRouter 部分
    section_start = find_openrouter_section(content)
    if section_start == -1:
        print("警告: 找不到 OpenRouter Models 部分，将在文件末尾添加")
        insertion_point = len(content.split('\n'))
        lines = content.split('\n')
    else:
        # 找到插入点
        insertion_point = find_insertion_point(content, section_start)
        lines = content.split('\n')
    
    # 生成新模型配置
    existing_names = set()
    for model in new_models:
        config = generate_model_config(model, existing_names)
        toml_block = format_model_toml(config)
        
        # 插入到指定位置
        lines.insert(insertion_point, toml_block)
        insertion_point += len(toml_block.split('\n'))
        print(f"  ✓ 添加模型: {config['name']} ({config['display_name']})")
    
    # 写入文件
    new_content = '\n'.join(lines)
    with ROUTER_TOML.open("w", encoding="utf-8") as f:
        f.write(new_content)
    
    print(f"\n✓ 已添加 {len(new_models)} 个新模型到 router.toml")


def main():
    print("=" * 60)
    print("更新 OpenRouter 免费模型")
    print("=" * 60)
    
    # 获取 API Key（可选）
    api_key = os.getenv("OPENROUTER_API_KEY")
    if api_key:
        print("使用环境变量中的 OPENROUTER_API_KEY")
    else:
        print("未设置 OPENROUTER_API_KEY，使用公开 API（可能有限制）")
    
    # 1. 从 API 获取模型
    all_models = fetch_openrouter_models(api_key)
    
    # 2. 筛选免费模型
    free_models = filter_free_models(all_models)
    
    # 3. 加载现有配置
    existing_models = load_existing_models()
    
    # 4. 找出新模型
    new_models = []
    for model in free_models:
        model_id = model.get("id", "")
        if model_id not in existing_models:
            new_models.append(model)
    
    print(f"\n✓ 发现 {len(new_models)} 个新模型")
    
    if new_models:
        print("\n新模型列表:")
        for model in new_models:
            print(f"  - {model.get('name', model.get('id'))} ({model.get('id')})")
        
        # 5. 添加到配置文件
        add_models_to_config(new_models)
    else:
        print("\n所有免费模型都已配置，无需更新")
    
    print("\n" + "=" * 60)
    print("完成！")
    print("=" * 60)


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
