#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

DEFAULT_PORT="${LLM_ROUTER_PORT:-18000}"
BASE_URL="${LLM_ROUTER_BASE_URL:-http://localhost:${DEFAULT_PORT}}"

print_usage() {
    cat <<'USAGE'
用法:
  ./scripts/check-free-models.sh [--base-url <url>]

说明:
  - 先检查 LLM Router 服务健康状态
  - 再执行通用 API smoke 检查（Go 脚本）

可覆盖环境变量:
  LLM_ROUTER_PORT      默认端口（默认 18000）
  LLM_ROUTER_BASE_URL  服务基础地址（默认 http://localhost:${LLM_ROUTER_PORT:-18000}）
USAGE
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --base-url)
            BASE_URL="${2:-}"
            shift 2
            ;;
        -h|--help)
            print_usage
            exit 0
            ;;
        *)
            echo "错误: 不支持的参数: $1" >&2
            print_usage >&2
            exit 1
            ;;
    esac
done

if [[ ! -x "${PROJECT_ROOT}/scripts/test_apis.sh" ]]; then
    echo "错误: 未找到可执行脚本 ${PROJECT_ROOT}/scripts/test_apis.sh" >&2
    exit 1
fi

echo "步骤 1/2: 检查服务健康状态"
"${SCRIPT_DIR}/check-service.sh" --url "${BASE_URL}"

echo "步骤 2/2: 执行 API smoke 检查"
echo "服务地址: ${BASE_URL}"
LLM_ROUTER_BASE_URL="${BASE_URL}" "${PROJECT_ROOT}/scripts/test_apis.sh"
