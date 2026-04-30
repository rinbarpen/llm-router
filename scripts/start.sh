#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MONITOR_DIR="$PROJECT_ROOT/examples/monitor"

BACKEND_PID=""
MONITOR_PID=""
BACKEND_PORT=""
STARTUP_TIMEOUT="${LLM_ROUTER_STARTUP_TIMEOUT:-25}"

print_start_summary() {
    local mode="$1"
    echo "启动模式: ${mode}"
    echo "后端实现: go"
    echo "后端端口: ${BACKEND_PORT}"
    if [[ -n "${LLM_ROUTER_SQLITE_PATH:-}" ]]; then
        echo "SQLite 数据库: ${LLM_ROUTER_SQLITE_PATH}"
    else
        echo "SQLite 数据库: data/llm_router.db (默认)"
    fi
}

print_backend_tips() {
    echo "排查建议:"
    echo "1) 检查端口占用: ss -ltnp | rg \":${BACKEND_PORT}\""
    echo "2) 检查后端日志输出是否有配置或依赖报错"
    echo "3) 检查 Go 是否可用: go version"
    echo "4) 检查 SQLite 数据目录是否可写: data/"
}

print_usage() {
    cat <<'EOF'
用法:
  ./scripts/start.sh [all|backend|monitor]

模式:
  all        启动后端和监控界面（默认）
  backend    仅启动后端
  monitor    仅启动监控界面

选项:
  -h, --help 显示帮助

说明:
  - 后端使用 Go: `go run ./cmd/llm-router`
  - 监控界面使用 `npm run dev`
  - 该脚本只检查依赖，不自动安装
EOF
}

cleanup() {
    local exit_code=$?

    trap - EXIT INT TERM

    if [[ -n "${MONITOR_PID}" ]] && kill -0 "${MONITOR_PID}" 2>/dev/null; then
        kill "${MONITOR_PID}" 2>/dev/null || true
        wait "${MONITOR_PID}" 2>/dev/null || true
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
    if [[ ! -f "${PROJECT_ROOT}/go.mod" ]]; then
        echo "错误: 未找到 ${PROJECT_ROOT}/go.mod，无法启动 Go 后端。" >&2
        exit 1
    fi
    require_command "go" "Go"
    require_command "curl" "curl"
    mkdir -p "${PROJECT_ROOT}/data"
}

check_monitor_requirements() {
    if [[ ! -f "${MONITOR_DIR}/package.json" ]]; then
        echo "错误: 未找到 ${MONITOR_DIR}/package.json，无法启动监控界面。" >&2
        exit 1
    fi

    require_command "npm" "Node.js 和 npm"

    if [[ ! -d "${MONITOR_DIR}/node_modules" ]]; then
        echo "错误: 监控界面依赖未安装: ${MONITOR_DIR}/node_modules" >&2
        echo "请先执行: cd ${MONITOR_DIR} && npm install" >&2
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
    ss -ltn "( sport = :${port} )" | awk 'NR>1 {found=1} END {exit found?0:1}'
}

is_backend_healthy() {
    local port=$1

    curl -fsS --max-time 1 "http://127.0.0.1:${port}/health" >/dev/null 2>&1
}

wait_for_backend_ready() {
    local waited=0
    local timeout="${STARTUP_TIMEOUT}"

    if ! [[ "${timeout}" =~ ^[0-9]+$ ]]; then
        timeout=25
    fi

    while (( waited < timeout )); do
        if [[ -n "${BACKEND_PID}" ]] && ! kill -0 "${BACKEND_PID}" 2>/dev/null; then
            wait "${BACKEND_PID}" || true
            echo "后端进程在启动阶段退出。" >&2
            print_backend_tips >&2
            return 1
        fi

        if is_backend_healthy "${BACKEND_PORT}"; then
            echo "后端就绪: http://127.0.0.1:${BACKEND_PORT}/health"
            return 0
        fi

        waited=$((waited + 1))
        echo "等待后端就绪... (${waited}s/${timeout}s)"
        sleep 1
    done

    echo "后端在 ${timeout}s 内未通过健康检查: http://127.0.0.1:${BACKEND_PORT}/health" >&2
    print_backend_tips >&2
    return 1
}

start_backend() {
    echo "启动后端..."
    local go_port_env="${LLM_ROUTER_PORT:-${BACKEND_PORT}}"
    echo "执行命令: (cd ${PROJECT_ROOT} && LLM_ROUTER_PORT=${go_port_env} go run ./cmd/llm-router)"
    (
        cd "${PROJECT_ROOT}"
        export LLM_ROUTER_PORT="${go_port_env}"
        exec go run ./cmd/llm-router
    ) &
    BACKEND_PID=$!
    echo "后端进程 PID: ${BACKEND_PID}"
}

start_monitor() {
    echo "启动监控界面..."
    (
        cd "${MONITOR_DIR}"
        exec npm run dev
    ) &
    MONITOR_PID=$!
}

wait_for_processes() {
    local target_count=0
    [[ -n "${BACKEND_PID}" ]] && target_count=$((target_count + 1))
    [[ -n "${MONITOR_PID}" ]] && target_count=$((target_count + 1))

    while true; do
        local finished=0

        if [[ -n "${BACKEND_PID}" ]] && ! kill -0 "${BACKEND_PID}" 2>/dev/null; then
            wait "${BACKEND_PID}" || true
            echo "后端进程已退出。" >&2
            finished=1
        fi

        if [[ -n "${MONITOR_PID}" ]] && ! kill -0 "${MONITOR_PID}" 2>/dev/null; then
            wait "${MONITOR_PID}" || true
            echo "监控界面进程已退出。" >&2
            finished=1
        fi

        if [[ "${finished}" -eq 1 ]]; then
            return 1
        fi

        if [[ "${target_count}" -eq 1 ]]; then
            if [[ -n "${BACKEND_PID}" ]]; then
                wait "${BACKEND_PID}"
            else
                wait "${MONITOR_PID}"
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
        check_monitor_requirements
        ;;
    backend)
        check_backend_requirements
        ;;
    monitor)
        check_monitor_requirements
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
print_start_summary "${MODE}"

case "${MODE}" in
    all)
        if is_port_listening "${BACKEND_PORT}"; then
            echo "后端端口 ${BACKEND_PORT} 已被占用，跳过后端启动（假定已有服务在运行）。"
        else
            start_backend
            wait_for_backend_ready
        fi
        start_monitor
        ;;
    backend)
        if is_port_listening "${BACKEND_PORT}"; then
            echo "后端端口 ${BACKEND_PORT} 已被占用，后端可能已在运行。"
            echo "如需重启后端，请先停止占用该端口的进程后再执行。"
            exit 0
        fi
        start_backend
        wait_for_backend_ready
        ;;
    monitor)
        start_monitor
        ;;
esac

echo "按 Ctrl+C 停止已启动的进程。"
wait_for_processes
