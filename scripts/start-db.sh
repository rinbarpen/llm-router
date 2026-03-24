#!/usr/bin/env bash

set -euo pipefail

CONTAINER_NAME="${LLM_ROUTER_DB_CONTAINER:-llm-router-pg}"
IMAGE="${LLM_ROUTER_DB_IMAGE:-postgres:16}"
HOST_PORT="${LLM_ROUTER_DB_PORT:-5432}"
DB_NAME="${LLM_ROUTER_DB_NAME:-llm_router}"
DB_USER="${LLM_ROUTER_DB_USER:-rczx}"
DB_PASSWORD="${LLM_ROUTER_DB_PASSWORD:-rczx}"
STARTUP_TIMEOUT="${LLM_ROUTER_DB_STARTUP_TIMEOUT:-30}"
DATA_VOLUME="${LLM_ROUTER_DB_VOLUME:-}"

print_usage() {
    cat <<'USAGE'
用法:
  ./scripts/start-db.sh

说明:
  - 启动（或复用）本地 PostgreSQL Docker 容器
  - 默认容器名: llm-router-pg
  - 默认数据库: llm_router
  - 默认账号/密码: rczx/rczx

可覆盖环境变量:
  LLM_ROUTER_DB_CONTAINER       容器名 (默认 llm-router-pg)
  LLM_ROUTER_DB_IMAGE           镜像 (默认 postgres:16)
  LLM_ROUTER_DB_PORT            主机映射端口 (默认 5432)
  LLM_ROUTER_DB_NAME            数据库名 (默认 llm_router)
  LLM_ROUTER_DB_USER            用户名 (默认 rczx)
  LLM_ROUTER_DB_PASSWORD        密码 (默认 rczx)
  LLM_ROUTER_DB_STARTUP_TIMEOUT 等待就绪超时秒数 (默认 30)
  LLM_ROUTER_DB_VOLUME          可选数据卷 (例如 llm-router-pg-data)
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

container_exists() {
    docker container inspect "${CONTAINER_NAME}" >/dev/null 2>&1
}

container_running() {
    local running
    running="$(docker inspect -f '{{.State.Running}}' "${CONTAINER_NAME}" 2>/dev/null || echo false)"
    [[ "${running}" == "true" ]]
}

wait_for_ready() {
    local waited=0
    local timeout="${STARTUP_TIMEOUT}"

    if ! [[ "${timeout}" =~ ^[0-9]+$ ]]; then
        timeout=30
    fi

    while (( waited < timeout )); do
        if docker exec "${CONTAINER_NAME}" pg_isready -U "${DB_USER}" -d "${DB_NAME}" >/dev/null 2>&1; then
            echo "数据库就绪: ${CONTAINER_NAME} (127.0.0.1:${HOST_PORT})"
            return 0
        fi
        waited=$((waited + 1))
        echo "等待数据库就绪... (${waited}s/${timeout}s)"
        sleep 1
    done

    echo "错误: 数据库在 ${timeout}s 内未就绪。" >&2
    echo "最近日志:" >&2
    docker logs --tail 50 "${CONTAINER_NAME}" >&2 || true
    return 1
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
    print_usage
    exit 0
fi

require_command "docker" "Docker"

echo "数据库容器: ${CONTAINER_NAME}"
echo "镜像: ${IMAGE}"
echo "数据库: ${DB_NAME}"
echo "用户: ${DB_USER}"
echo "端口: 127.0.0.1:${HOST_PORT}"

if container_exists; then
    if container_running; then
        echo "检测到容器已运行，跳过创建。"
    else
        echo "检测到容器已存在但未运行，正在启动..."
        docker start "${CONTAINER_NAME}" >/dev/null
    fi
else
    echo "创建并启动数据库容器..."
    docker_run_args=(
        run -d
        --name "${CONTAINER_NAME}"
        -e "POSTGRES_DB=${DB_NAME}"
        -e "POSTGRES_USER=${DB_USER}"
        -e "POSTGRES_PASSWORD=${DB_PASSWORD}"
        -p "${HOST_PORT}:5432"
    )

    if [[ -n "${DATA_VOLUME}" ]]; then
        docker_run_args+=( -v "${DATA_VOLUME}:/var/lib/postgresql/data" )
    fi

    docker_run_args+=( "${IMAGE}" )
    docker "${docker_run_args[@]}" >/dev/null
fi

wait_for_ready

echo "可用连接串:"
echo "  export LLM_ROUTER_PG_DSN='postgres://${DB_USER}:${DB_PASSWORD}@localhost:${HOST_PORT}/${DB_NAME}?sslmode=disable'"
