#!/usr/bin/env bash
# 生成 LLM Router API Key 的便捷脚本

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# 使用 Python 脚本生成 API Key
cd "$PROJECT_ROOT" || exit 1
python "$SCRIPT_DIR/generate_api_key.py" "$@"

