# 工具脚本

本目录提供 Go 后端的运维与开发辅助脚本。

## 核心脚本

- `start.sh`：本地启动后端/监控（`all|backend|monitor`）
- `start-db.sh`：启动/复用本地 PostgreSQL Docker 容器
- `import-db.sh`：触发 SQLite -> PostgreSQL 数据导入
- `check-db.sh`：检查 PostgreSQL 容器与连通性
- `check-service.sh`：检查后端健康状态
- `test_apis.sh`：执行 API smoke（health/models/route/chat）
- `check-free-models.sh`：先健康检查，再执行 API smoke
- `pricing_sync.sh`：从 `data/pricing/*.json` 同步模型定价
- `generate_api_key.sh`：生成 API Key（纯 shell）

## 快速用法

```bash
# 启动后端 + 监控
./scripts/start.sh

# 只启动后端
./scripts/start.sh backend

# 启动本地数据库
./scripts/start-db.sh

# 触发数据库导入（SQLite -> PostgreSQL）
./scripts/import-db.sh --start-db

# 检查后端健康
./scripts/check-service.sh --url http://127.0.0.1:18000

# API smoke
./scripts/test_apis.sh

# 定价同步
./scripts/pricing_sync.sh

# 生成 key
./scripts/generate_api_key.sh --length 40 --count 3
./scripts/generate_api_key.sh --env LLM_ROUTER_ADMIN_KEY --length 32
```

## 开机启动

- Linux: `scripts/linux/`
- macOS: `scripts/macos/`
- Windows: `scripts/windows/`

## 说明

- 脚本默认面向 Go 后端（`go run ./cmd/llm-router`）。
- 脚本不会自动安装依赖；请先准备 `go`、`curl`、`npm`、Docker（按需）。
