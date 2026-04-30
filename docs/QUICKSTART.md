# 快速启动指南

## 前置准备

1. 安装 Go（建议 1.24+）与 Node.js/npm。
2. 准备可写数据目录（默认 `data/`，启动脚本会自动创建）。
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

## 历史 SQLite 数据（可选）

默认运行库是 `data/llm_router.db`。如果存在旧的 `data/llm_datas.db`，启动时会按配置补齐导入监控记录。

## 验证服务

```bash
curl http://localhost:18000/health
```

## 运行回归测试

```bash
go test ./...
```

## 多机部署与高可用（参考）

- 参考文档：`docs/DEPLOYMENT_HA.md`
- Compose 多实例模板：`deploy/compose/docker-compose.ha.yml`
- K8s 基础模板：`deploy/k8s/*.yaml`

## 常见问题

1. SQLite 路径不可写：确认 `data/` 存在且当前用户有写入权限，或设置 `LLM_ROUTER_SQLITE_PATH`。
2. 端口冲突：修改 `router.toml` 中 `[server].port` 或设置 `LLM_ROUTER_PORT`。
3. monitor 依赖缺失：执行 `cd examples/monitor && npm install`。
