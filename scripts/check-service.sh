#!/usr/bin/env bash

set -euo pipefail

DEFAULT_PORT="${LLM_ROUTER_PORT:-18000}"
SERVICE_URL="${LLM_ROUTER_API_URL:-http://127.0.0.1:${DEFAULT_PORT}}"
HEALTH_PATH="${LLM_ROUTER_HEALTH_PATH:-/health}"
TIMEOUT="${LLM_ROUTER_SERVICE_TIMEOUT:-3}"

print_usage() {
    cat <<'USAGE'
用法:
  ./scripts/check-service.sh [--url <api_base_url>] [--timeout <seconds>]

说明:
  - 检查 LLM Router 服务是否已启动
  - 默认检查: http://127.0.0.1:${LLM_ROUTER_PORT:-18000}/health

可覆盖环境变量:
  LLM_ROUTER_PORT            默认端口（默认 18000）
  LLM_ROUTER_API_URL         API 基础地址（例如 http://127.0.0.1:18000）
  LLM_ROUTER_HEALTH_PATH     健康检查路径（默认 /health）
  LLM_ROUTER_SERVICE_TIMEOUT 请求超时秒数（默认 3）
USAGE
}

error_tips() {
    echo "排查建议:"
    echo "1) 查看后端是否正在运行: ss -ltnp | rg ':${DEFAULT_PORT}'"
    echo "2) 启动后端: ./scripts/start.sh backend"
    echo "3) 若使用 Go 后端，确认数据库可达并检查启动日志"
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --url)
            SERVICE_URL="${2:-}"
            shift 2
            ;;
        --timeout)
            TIMEOUT="${2:-}"
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

HEALTH_URL="${SERVICE_URL%/}${HEALTH_PATH}"

if ! [[ "${TIMEOUT}" =~ ^[0-9]+$ ]]; then
    echo "错误: --timeout 必须为整数秒。" >&2
    exit 1
fi

echo "检查服务地址: ${HEALTH_URL}"

status_code=""
if command -v curl >/dev/null 2>&1; then
    status_code="$(curl -sS -m "${TIMEOUT}" -o /tmp/llm-router-health.out -w "%{http_code}" "${HEALTH_URL}" || true)"
elif command -v python3 >/dev/null 2>&1; then
    status_code="$(python3 - "${HEALTH_URL}" "${TIMEOUT}" <<'PY'
import sys
from urllib import request

url = sys.argv[1]
timeout = int(sys.argv[2])

try:
    with request.urlopen(url, timeout=timeout) as resp:
        print(resp.getcode())
except Exception:
    print("")
PY
)"
else
    echo "错误: 未找到 curl 或 python3，无法执行健康检查。" >&2
    exit 1
fi

if [[ -n "${status_code}" && "${status_code}" =~ ^2[0-9][0-9]$ ]]; then
    echo "服务状态: 可用 (HTTP ${status_code})"
    exit 0
fi

if [[ -n "${status_code}" && "${status_code}" =~ ^5[0-9][0-9]$ ]]; then
    echo "服务状态: 已启动但不健康 (HTTP ${status_code})"
    if [[ -f /tmp/llm-router-health.out ]]; then
        body="$(cat /tmp/llm-router-health.out)"
        if [[ -n "${body}" ]]; then
            echo "健康检查响应:"
            echo "${body}"
        fi
    fi
else
    echo "服务状态: 不可用"
fi

if [[ -n "${status_code}" ]]; then
    echo "HTTP 状态码: ${status_code}"
fi
error_tips
exit 1
