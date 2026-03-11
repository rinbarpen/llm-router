#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
FRONTEND_DIR="$PROJECT_ROOT/frontend"

BACKEND_PID=""
FRONTEND_PID=""
BACKEND_PORT=""

print_usage() {
    cat <<'EOF'
用法:
  ./scripts/start.sh [all|backend|frontend]

模式:
  all        启动后端和前端（默认）
  backend    仅启动后端
  frontend   仅启动前端

选项:
  -h, --help 显示帮助

说明:
  - 后端使用 `uv run llm-router`
  - 前端使用 `npm run dev`
  - 该脚本只检查依赖，不自动安装
EOF
}

cleanup() {
    local exit_code=$?

    trap - EXIT INT TERM

    if [[ -n "${FRONTEND_PID}" ]] && kill -0 "${FRONTEND_PID}" 2>/dev/null; then
        kill "${FRONTEND_PID}" 2>/dev/null || true
        wait "${FRONTEND_PID}" 2>/dev/null || true
    fi

    if [[ -n "${BACKEND_PID}" ]] && kill -0 "${BACKEND_PID}" 2>/dev/null; then
        kill "${BACKEND_PID}" 2>/dev/null || true
        wait "${BACKEND_PID}" 2>/dev/null || true
    fi

    exit "${exit_code}"
}

require_command() {
    local command_name=$1
    local install_hint=$2

    if ! command -v "${command_name}" >/dev/null 2>&1; then
        echo "错误: 未找到 ${command_name} 命令。" >&2
        echo "请先安装 ${install_hint}。" >&2
        exit 1
    fi
}

check_backend_requirements() {
    if [[ ! -f "${PROJECT_ROOT}/pyproject.toml" ]]; then
        echo "错误: 未找到 ${PROJECT_ROOT}/pyproject.toml，无法启动后端。" >&2
        exit 1
    fi

    require_command "uv" "uv"
}

check_frontend_requirements() {
    if [[ ! -f "${FRONTEND_DIR}/package.json" ]]; then
        echo "错误: 未找到 ${FRONTEND_DIR}/package.json，无法启动前端。" >&2
        exit 1
    fi

    require_command "npm" "Node.js 和 npm"

    if [[ ! -d "${FRONTEND_DIR}/node_modules" ]]; then
        echo "错误: 前端依赖未安装: ${FRONTEND_DIR}/node_modules" >&2
        echo "请先执行: cd ${FRONTEND_DIR} && npm install" >&2
        exit 1
    fi
}

get_backend_port() {
    if [[ -n "${LLM_ROUTER_PORT:-}" ]]; then
        echo "${LLM_ROUTER_PORT}"
        return
    fi

    if [[ -f "${PROJECT_ROOT}/router.toml" ]]; then
        local parsed_port
        parsed_port="$(awk '
            BEGIN { in_server = 0 }
            /^\[/ { in_server = ($0 == "[server]") }
            in_server && /^[[:space:]]*port[[:space:]]*=/ {
                line = $0
                sub(/#.*/, "", line)
                gsub(/[[:space:]]/, "", line)
                split(line, parts, "=")
                if (parts[2] ~ /^[0-9]+$/) {
                    print parts[2]
                    exit
                }
            }
        ' "${PROJECT_ROOT}/router.toml")"

        if [[ -n "${parsed_port}" ]]; then
            echo "${parsed_port}"
            return
        fi
    fi

    # Keep in sync with backend default in llm_router.config (when no env/config override)
    echo "8000"
}

is_port_listening() {
    local port=$1

    if ! command -v python3 >/dev/null 2>&1; then
        return 1
    fi

    # Exit 0 when bind fails (port already in use), exit 1 otherwise.
    python3 - "${port}" >/dev/null 2>&1 <<'PY'
import socket
import sys

port = int(sys.argv[1])
try:
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
except OSError:
    sys.exit(1)

sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)

try:
    sock.bind(("127.0.0.1", port))
except OSError:
    sys.exit(0)
else:
    sys.exit(1)
finally:
    sock.close()
PY
}

start_backend() {
    echo "启动后端..."
    (
        cd "${PROJECT_ROOT}"
        exec uv run llm-router
    ) &
    BACKEND_PID=$!
}

start_frontend() {
    echo "启动前端..."
    (
        cd "${FRONTEND_DIR}"
        exec npm run dev
    ) &
    FRONTEND_PID=$!
}

wait_for_processes() {
    local target_count=0
    [[ -n "${BACKEND_PID}" ]] && target_count=$((target_count + 1))
    [[ -n "${FRONTEND_PID}" ]] && target_count=$((target_count + 1))

    while true; do
        local finished=0

        if [[ -n "${BACKEND_PID}" ]] && ! kill -0 "${BACKEND_PID}" 2>/dev/null; then
            wait "${BACKEND_PID}" || true
            echo "后端进程已退出。" >&2
            finished=1
        fi

        if [[ -n "${FRONTEND_PID}" ]] && ! kill -0 "${FRONTEND_PID}" 2>/dev/null; then
            wait "${FRONTEND_PID}" || true
            echo "前端进程已退出。" >&2
            finished=1
        fi

        if [[ "${finished}" -eq 1 ]]; then
            return 1
        fi

        if [[ "${target_count}" -eq 1 ]]; then
            if [[ -n "${BACKEND_PID}" ]]; then
                wait "${BACKEND_PID}"
            else
                wait "${FRONTEND_PID}"
            fi
            return $?
        fi

        sleep 1
    done
}

MODE="${1:-all}"

case "${MODE}" in
    all)
        check_backend_requirements
        check_frontend_requirements
        ;;
    backend)
        check_backend_requirements
        ;;
    frontend)
        check_frontend_requirements
        ;;
    -h|--help)
        print_usage
        exit 0
        ;;
    *)
        echo "错误: 不支持的模式: ${MODE}" >&2
        print_usage >&2
        exit 1
        ;;
esac

trap cleanup EXIT INT TERM
BACKEND_PORT="$(get_backend_port)"

case "${MODE}" in
    all)
        if is_port_listening "${BACKEND_PORT}"; then
            echo "后端端口 ${BACKEND_PORT} 已被占用，跳过后端启动（假定已有服务在运行）。"
        else
            start_backend
            sleep 1
        fi
        start_frontend
        ;;
    backend)
        if is_port_listening "${BACKEND_PORT}"; then
            echo "后端端口 ${BACKEND_PORT} 已被占用，后端可能已在运行。"
            echo "如需重启后端，请先停止占用该端口的进程后再执行。"
            exit 0
        fi
        start_backend
        ;;
    frontend)
        start_frontend
        ;;
esac

echo "按 Ctrl+C 停止已启动的进程。"
wait_for_processes
