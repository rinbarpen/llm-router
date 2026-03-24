#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MONITOR_DIR="$PROJECT_ROOT/examples/monitor"

BACKEND_PID=""
MONITOR_PID=""
BACKEND_PORT=""
BACKEND_IMPL="${LLM_ROUTER_BACKEND_IMPL:-go}"
STARTUP_TIMEOUT="${LLM_ROUTER_STARTUP_TIMEOUT:-25}"

print_start_summary() {
    local mode="$1"
    echo "启动模式: ${mode}"
    echo "后端实现: ${BACKEND_IMPL}"
    echo "后端端口: ${BACKEND_PORT}"
    if [[ -n "${LLM_ROUTER_PG_DSN:-}" ]]; then
        echo "数据库连接: 已设置 LLM_ROUTER_PG_DSN"
    else
        echo "数据库连接: 未设置 LLM_ROUTER_PG_DSN (将使用默认配置)"
    fi
}

print_backend_tips() {
    echo "排查建议:"
    echo "1) 检查端口占用: ss -ltnp | rg \":${BACKEND_PORT}\""
    echo "2) 检查后端日志输出是否有配置或依赖报错"
    if [[ "${BACKEND_IMPL}" == "go" ]]; then
        echo "3) 检查 Go 是否可用: go version"
        echo "4) 检查 PostgreSQL 是否可达: ss -ltnp | rg ':5432'"
        echo "5) 若使用 PostgreSQL，确认数据库可连接并且 DSN 正确"
    else
        echo "3) 检查 Python 依赖: uv sync"
    fi
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
  - 后端默认使用 Go: `go run ./backend/cmd/llm-router`
  - 设置 `LLM_ROUTER_BACKEND_IMPL=python` 时，后端使用 `uv run llm-router`
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

resolve_pg_host_port() {
    local pg_dsn=""
    if [[ -n "${LLM_ROUTER_PG_DSN:-}" ]]; then
        pg_dsn="${LLM_ROUTER_PG_DSN}"
    elif [[ -n "${LLM_ROUTER_POSTGRES_DSN:-}" ]]; then
        pg_dsn="${LLM_ROUTER_POSTGRES_DSN}"
    elif [[ "${LLM_ROUTER_DATABASE_URL:-}" == postgres* ]]; then
        pg_dsn="${LLM_ROUTER_DATABASE_URL}"
    fi

    if [[ -z "${pg_dsn}" ]]; then
        echo "localhost:5432"
        return
    fi

    if command -v python3 >/dev/null 2>&1; then
        python3 - "${pg_dsn}" <<'PY'
import sys
from urllib.parse import urlparse

dsn = sys.argv[1]
parsed = urlparse(dsn)
host = parsed.hostname or "localhost"
port = parsed.port or 5432
print(f"{host}:{port}")
PY
        return
    fi

    echo "localhost:5432"
}

is_tcp_reachable() {
    local host="$1"
    local port="$2"

    if command -v python3 >/dev/null 2>&1; then
        python3 - "${host}" "${port}" >/dev/null 2>&1 <<'PY'
import socket
import sys

host = sys.argv[1]
port = int(sys.argv[2])
sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
sock.settimeout(1)
try:
    sock.connect((host, port))
except OSError:
    sys.exit(1)
else:
    sys.exit(0)
finally:
    sock.close()
PY
        return $?
    fi

    return 1
}

check_go_postgres_ready() {
    local pg_addr
    local pg_host
    local pg_port

    pg_addr="$(resolve_pg_host_port)"
    pg_host="${pg_addr%:*}"
    pg_port="${pg_addr##*:}"

    if is_tcp_reachable "${pg_host}" "${pg_port}"; then
        echo "PostgreSQL 可达: ${pg_host}:${pg_port}"
        return 0
    fi

    echo "错误: Go 后端依赖 PostgreSQL，但当前不可达: ${pg_host}:${pg_port}" >&2
    echo "请先启动 PostgreSQL 并确认 DSN 配置后重试。" >&2
    echo "提示: 可执行 ./scripts/check-db.sh（Docker）或检查本机 PostgreSQL 服务状态。" >&2
    return 1
}

check_backend_requirements() {
    if [[ ! -f "${PROJECT_ROOT}/pyproject.toml" ]]; then
        echo "错误: 未找到 ${PROJECT_ROOT}/pyproject.toml，无法启动后端。" >&2
        exit 1
    fi

    if [[ "${BACKEND_IMPL}" == "go" ]]; then
        if [[ ! -f "${PROJECT_ROOT}/backend/go.mod" ]]; then
            echo "错误: 未找到 ${PROJECT_ROOT}/backend/go.mod，无法启动 Go 后端。" >&2
            exit 1
        fi
        require_command "go" "Go"
        check_go_postgres_ready
    else
        require_command "uv" "uv"
    fi
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

is_backend_healthy() {
    local port=$1

    if command -v curl >/dev/null 2>&1; then
        curl -fsS --max-time 1 "http://127.0.0.1:${port}/health" >/dev/null 2>&1
        return $?
    fi

    if command -v python3 >/dev/null 2>&1; then
        python3 - "${port}" >/dev/null 2>&1 <<'PY'
import sys
from urllib import request

port = int(sys.argv[1])
try:
    with request.urlopen(f"http://127.0.0.1:{port}/health", timeout=1) as resp:
        code = resp.getcode()
except Exception:
    sys.exit(1)

sys.exit(0 if 200 <= code < 500 else 1)
PY
        return $?
    fi

    return 1
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
    if [[ "${BACKEND_IMPL}" == "go" ]]; then
        local go_port_env="${LLM_ROUTER_PORT:-${BACKEND_PORT}}"
        echo "执行命令: (cd ${PROJECT_ROOT}/backend && LLM_ROUTER_PORT=${go_port_env} go run ./cmd/llm-router)"
        (
            cd "${PROJECT_ROOT}/backend"
            export LLM_ROUTER_PORT="${go_port_env}"
            exec go run ./cmd/llm-router
        ) &
    else
        echo "执行命令: (cd ${PROJECT_ROOT} && uv run llm-router)"
        (
            cd "${PROJECT_ROOT}"
            exec uv run llm-router
        ) &
    fi
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
