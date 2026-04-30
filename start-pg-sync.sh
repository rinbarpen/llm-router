#!/bin/bash
set -e

echo "=== 启动 PostgreSQL Docker 容器 ==="
sudo docker rm -f llm-router-pg 2>/dev/null || true
sudo docker run -d --name llm-router-pg -p 5432:5432 \
  -e POSTGRES_PASSWORD=password \
  -e POSTGRES_DB=llm_router \
  postgres:15

echo "=== 等待 PostgreSQL 就绪 ==="
for i in $(seq 1 30); do
  if sudo docker exec llm-router-pg pg_isready -U postgres 2>/dev/null; then
    echo "PostgreSQL 已就绪！"
    break
  fi
  echo "等待中... ($i/30)"
  sleep 2
done

echo "=== 同步 router.toml 到数据库 ==="
cd /home/rczx/workspace/rinbarpen/llm-router
LLM_ROUTER_PG_DSN="postgres://postgres:password@localhost:5432/llm_router?sslmode=disable" \
go run ./cmd/llm-router 2>&1 | head -30

echo ""
echo "=== 完成 ==="
echo "PostgreSQL 容器在后台运行，停止请用：sudo docker stop llm-router-pg"
