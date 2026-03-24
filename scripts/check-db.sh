#!/usr/bin/env bash

set -euo pipefail

CONTAINER_NAME="${LLM_ROUTER_DB_CONTAINER:-llm-router-pg}"
HOST_PORT="${LLM_ROUTER_DB_PORT:-5432}"
DB_NAME="${LLM_ROUTER_DB_NAME:-llm_router}"
DB_USER="${LLM_ROUTER_DB_USER:-rczx}"

print_usage() {
    cat <<'USAGE'
用法:
  ./scripts/check-db.sh

说明:
  - 检查 PostgreSQL Docker 容器是否存在/运行
  - 检查数据库健康状态（pg_isready）
  - 执行一条 SQL 验证连接

可覆盖环境变量:
  LLM_ROUTER_DB_CONTAINER  容器名 (默认 llm-router-pg)
  LLM_ROUTER_DB_PORT       主机映射端口 (默认 5432)
  LLM_ROUTER_DB_NAME       数据库名 (默认 llm_router)
  LLM_ROUTER_DB_USER       用户名 (默认 rczx)
USAGE
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

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
    print_usage
    exit 0
fi

require_command "docker" "Docker"

if ! docker container inspect "${CONTAINER_NAME}" >/dev/null 2>&1; then
    echo "错误: 未找到数据库容器 ${CONTAINER_NAME}。" >&2
    echo "请先执行: ./scripts/start-db.sh" >&2
    exit 1
fi

running="$(docker inspect -f '{{.State.Running}}' "${CONTAINER_NAME}" 2>/dev/null || echo false)"
if [[ "${running}" != "true" ]]; then
    echo "错误: 数据库容器 ${CONTAINER_NAME} 未运行。" >&2
    echo "请先执行: docker start ${CONTAINER_NAME} 或 ./scripts/start-db.sh" >&2
    exit 1
fi

echo "容器状态: 运行中 (${CONTAINER_NAME})"
echo "预期地址: 127.0.0.1:${HOST_PORT}"

if docker exec "${CONTAINER_NAME}" pg_isready -U "${DB_USER}" -d "${DB_NAME}" >/dev/null 2>&1; then
    echo "健康检查: pg_isready 通过"
else
    echo "错误: pg_isready 检查失败。" >&2
    docker logs --tail 30 "${CONTAINER_NAME}" >&2 || true
    exit 1
fi

if query_result="$(docker exec "${CONTAINER_NAME}" psql -U "${DB_USER}" -d "${DB_NAME}" -Atqc "select current_database() || '|' || current_user || '|' || version();" 2>/dev/null)"; then
    db_name="${query_result%%|*}"
    rest="${query_result#*|}"
    db_user="${rest%%|*}"
    db_version="${rest#*|}"

    echo "SQL 检查: 通过"
    echo "database: ${db_name}"
    echo "user: ${db_user}"
    echo "version: ${db_version}"
else
    echo "错误: SQL 检查失败，无法连接数据库 ${DB_NAME}。" >&2
    exit 1
fi

if [[ -n "${LLM_ROUTER_PG_DSN:-}" ]]; then
    echo "LLM_ROUTER_PG_DSN: 已设置"
else
    echo "LLM_ROUTER_PG_DSN: 未设置"
fi
