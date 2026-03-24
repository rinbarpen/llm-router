#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PRICING_DIR_DEFAULT="$PROJECT_ROOT/data/pricing"

API_URL="${LLM_ROUTER_API_URL:-http://127.0.0.1:18000}"
PRICING_DIR="$PRICING_DIR_DEFAULT"
SYNC_ALL=1
PRINT_ENV=0

print_usage() {
  cat <<'EOF'
用法:
  ./scripts/pricing_sync.sh [选项]

选项:
  --api-url <url>        后端地址（默认: http://127.0.0.1:18000）
  --pricing-dir <path>   定价 JSON 目录（默认: data/pricing）
  --dry-run              仅拉取 /pricing/latest，不执行 /pricing/sync-all
  --print-env            仅打印 LLM_ROUTER_PRICING_SOURCE_URLS 并退出
  -h, --help             显示帮助

说明:
  - 自动读取 <pricing-dir> 下以下文件（存在才纳入）：
    openai.json claude.json gemini.json deepseek.json qwen.json kimi.json glm.json groq.json
  - 生成 file:// 形式的 LLM_ROUTER_PRICING_SOURCE_URLS 并注入本次请求环境。
EOF
}

require_command() {
  local cmd=$1
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "错误: 缺少命令 $cmd" >&2
    exit 1
  fi
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --api-url)
      API_URL="$2"
      shift 2
      ;;
    --pricing-dir)
      PRICING_DIR="$2"
      shift 2
      ;;
    --dry-run)
      SYNC_ALL=0
      shift
      ;;
    --print-env)
      PRINT_ENV=1
      shift
      ;;
    -h|--help)
      print_usage
      exit 0
      ;;
    *)
      echo "错误: 未知参数: $1" >&2
      print_usage
      exit 1
      ;;
  esac
done

require_command python3
require_command curl

if [[ ! -d "$PRICING_DIR" ]]; then
  echo "错误: 定价目录不存在: $PRICING_DIR" >&2
  exit 1
fi

SOURCE_JSON="$(python3 - "$PRICING_DIR" <<'PY'
import json
import sys
from pathlib import Path

pricing_dir = Path(sys.argv[1]).resolve()
providers = ["openai", "claude", "gemini", "deepseek", "qwen", "kimi", "glm", "groq"]
result = {}
for provider in providers:
    p = pricing_dir / f"{provider}.json"
    if p.exists():
        result[provider] = f"file://{p}"
print(json.dumps(result, ensure_ascii=False))
PY
)"

if [[ "$SOURCE_JSON" == "{}" ]]; then
  echo "错误: 未发现任何 provider 定价文件，请检查目录: $PRICING_DIR" >&2
  exit 1
fi

if [[ "$PRINT_ENV" -eq 1 ]]; then
  echo "LLM_ROUTER_PRICING_SOURCE_URLS=$SOURCE_JSON"
  exit 0
fi

echo "[1/3] 检查后端健康状态: $API_URL/health"
curl -fsS "$API_URL/health" >/dev/null

echo "[2/3] 拉取最新定价（使用本地多来源配置）"
env LLM_ROUTER_PRICING_SOURCE_URLS="$SOURCE_JSON" \
  curl -fsS "$API_URL/pricing/latest" >/dev/null

echo "[3/3] 同步模型定价"
if [[ "$SYNC_ALL" -eq 1 ]]; then
  env LLM_ROUTER_PRICING_SOURCE_URLS="$SOURCE_JSON" \
    curl -fsS -X POST "$API_URL/pricing/sync-all"
  echo
  echo "完成: 已执行 /pricing/sync-all"
else
  echo "跳过: --dry-run 模式未执行 /pricing/sync-all"
fi
