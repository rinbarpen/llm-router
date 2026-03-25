# 快速启动指南

## 前置准备

1. 安装 Go（建议 1.24+）与 Node.js/npm。
2. 准备 PostgreSQL（本地可用 `./scripts/start-db.sh`）。
3. 准备配置文件：
   ```bash
   cp .env.example .env
   ```

## 启动后端

### 方式 1：项目启动脚本（推荐）

```bash
./scripts/start.sh backend
```

### 方式 2：直接运行 Go 服务

```bash
go mod download
go run ./cmd/llm-router
```

## 启动监控界面（可选）

```bash
cd examples/monitor
npm install
npm run dev
```

## 一键本地开发（后端+监控）

```bash
./scripts/start.sh
```

## 验证服务

```bash
curl http://localhost:18000/health
```

## 运行回归测试

```bash
go test ./...
```

## 常见问题

1. PostgreSQL 未就绪：先执行 `./scripts/start-db.sh`，并确认 `LLM_ROUTER_PG_DSN` 配置可连通。
2. 端口冲突：修改 `router.toml` 中 `[server].port` 或设置 `LLM_ROUTER_PORT`。
3. monitor 依赖缺失：执行 `cd examples/monitor && npm install`。
