#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

IMPORT_PORT="${LLM_ROUTER_IMPORT_PORT:-18080}"
STARTUP_TIMEOUT="${LLM_ROUTER_IMPORT_TIMEOUT:-30}"
LOG_FILE="${LLM_ROUTER_IMPORT_LOG:-/tmp/llm-router-import.log}"
FORCE_REIMPORT=false
START_DB=false
BACKEND_PID=""

print_usage() {
    cat <<'EOF'
用法:
  ./scripts/import-db.sh [--start-db] [--force]

说明:
  - 通过临时启动 Go 后端触发 SQLite -> PostgreSQL 导入
  - 导入成功后自动停止临时后端进程
  - 导入依赖环境变量:
      LLM_ROUTER_PG_DSN / LLM_ROUTER_POSTGRES_DSN（PostgreSQL DSN）
      LLM_ROUTER_MIGRATE_FROM_SQLITE（建议 true）
      LLM_ROUTER_SQLITE_MAIN_PATH / LLM_ROUTER_SQLITE_MONITOR_PATH（可选，默认 data/*.db）

选项:
  --start-db  先执行 ./scripts/start-db.sh 启动本地 PostgreSQL
  --force     重置导入标记 sqlite_bootstrap_v1 后再导入（需 psql）
  -h, --help  显示帮助

可覆盖环境变量:
  LLM_ROUTER_IMPORT_PORT     临时后端端口 (默认 18080)
  LLM_ROUTER_IMPORT_TIMEOUT  等待健康检查秒数 (默认 30)
  LLM_ROUTER_IMPORT_LOG      临时后端日志路径 (默认 /tmp/llm-router-import.log)
EOF
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

resolve_pg_dsn() {
    if [[ -n "${LLM_ROUTER_PG_DSN:-}" ]]; then
        echo "${LLM_ROUTER_PG_DSN}"
        return
    fi
    if [[ -n "${LLM_ROUTER_POSTGRES_DSN:-}" ]]; then
        echo "${LLM_ROUTER_POSTGRES_DSN}"
        return
    fi
    if [[ "${LLM_ROUTER_DATABASE_URL:-}" == postgres* ]]; then
        echo "${LLM_ROUTER_DATABASE_URL}"
        return
    fi
    echo "postgres://localhost:5432/llm_router?sslmode=disable"
}

cleanup() {
    local exit_code=$?
    trap - EXIT INT TERM

    if [[ -n "${BACKEND_PID}" ]] && kill -0 "${BACKEND_PID}" 2>/dev/null; then
        kill "${BACKEND_PID}" 2>/dev/null || true
        wait "${BACKEND_PID}" 2>/dev/null || true
    fi

    exit "${exit_code}"
}

reset_migration_marker() {
    local dsn="$1"
    psql "${dsn}" -v ON_ERROR_STOP=1 <<'SQL'
DO $$
BEGIN
    IF to_regclass('public.go_bootstrap_migrations') IS NOT NULL THEN
        DELETE FROM go_bootstrap_migrations WHERE name = 'sqlite_bootstrap_v1';
    END IF;
END$$;
SQL
}

wait_for_backend_ready() {
    local waited=0

    while (( waited < STARTUP_TIMEOUT )); do
        if [[ -n "${BACKEND_PID}" ]] && ! kill -0 "${BACKEND_PID}" 2>/dev/null; then
            wait "${BACKEND_PID}" || true
            echo "错误: 导入过程中的临时后端提前退出。" >&2
            echo "日志: ${LOG_FILE}" >&2
            tail -n 80 "${LOG_FILE}" >&2 || true
            return 1
        fi

        if curl -fsS "http://127.0.0.1:${IMPORT_PORT}/health" >/dev/null 2>&1; then
            return 0
        fi

        waited=$((waited + 1))
        sleep 1
    done

    echo "错误: ${STARTUP_TIMEOUT}s 内未通过健康检查，导入可能失败。" >&2
    echo "日志: ${LOG_FILE}" >&2
    tail -n 80 "${LOG_FILE}" >&2 || true
    return 1
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --start-db)
            START_DB=true
            shift
            ;;
        --force)
            FORCE_REIMPORT=true
            shift
            ;;
        -h|--help)
            print_usage
            exit 0
            ;;
        *)
            echo "错误: 未知参数: $1" >&2
            print_usage >&2
            exit 1
            ;;
    esac
done

require_command "go" "Go"
require_command "curl" "curl"

PG_DSN="$(resolve_pg_dsn)"
export LLM_ROUTER_PG_DSN="${PG_DSN}"
export LLM_ROUTER_MIGRATE_FROM_SQLITE="${LLM_ROUTER_MIGRATE_FROM_SQLITE:-true}"

if [[ "${START_DB}" == "true" ]]; then
    "${PROJECT_ROOT}/scripts/start-db.sh"
fi

if [[ "${FORCE_REIMPORT}" == "true" ]]; then
    require_command "psql" "PostgreSQL Client (psql)"
    echo "重置导入标记: sqlite_bootstrap_v1"
    reset_migration_marker "${PG_DSN}"
fi

echo "开始导入（通过临时后端触发）"
echo "PostgreSQL DSN: ${PG_DSN}"
echo "导入模式: LLM_ROUTER_MIGRATE_FROM_SQLITE=${LLM_ROUTER_MIGRATE_FROM_SQLITE}"
echo "临时端口: ${IMPORT_PORT}"
echo "日志文件: ${LOG_FILE}"

trap cleanup EXIT INT TERM

(
    cd "${PROJECT_ROOT}"
    LLM_ROUTER_PORT="${IMPORT_PORT}" go run ./cmd/llm-router >"${LOG_FILE}" 2>&1
) &
BACKEND_PID=$!

if ! wait_for_backend_ready; then
    exit 1
fi

echo "导入触发成功，临时后端已完成启动。"
echo "即将停止临时后端进程。"

kill "${BACKEND_PID}" 2>/dev/null || true
wait "${BACKEND_PID}" 2>/dev/null || true
BACKEND_PID=""

echo "完成。后续可执行:"
echo "  ./scripts/start.sh backend"
