# LLM Router（Go Backend）

## 项目概览

LLM Router 是一个统一的 LLM 路由服务，后端基于 Go，提供 OpenAI 兼容接口、Provider/Model 管理、鉴权、监控与定价同步能力。

## 核心目录

- `cmd/llm-router/`：服务入口
- `src/api/`：HTTP 路由与鉴权中间件
- `src/services/`：业务服务（路由、模型、定价、监控）
- `src/providers/`：各 Provider 适配层
- `src/config/`：配置加载与解析
- `src/db/`：数据库访问与模型定义
- `src/migrate/`：启动迁移与 SQLite 导入
- `examples/monitor/`：监控前端（React + Vite）
- `scripts/`：运维与开发脚本

## 开发命令

```bash
# 安装 Go 依赖
go mod download

# 启动后端
go run ./cmd/llm-router

# 启动后端 + 监控前端
./scripts/start.sh

# 运行 Go 测试
go test ./...
```

## 运行依赖

- Go 1.24+
- PostgreSQL（主存储）
- Node.js/npm（仅监控前端）

## 配置说明

- 主配置文件：`router.toml`
- 环境变量样例：`.env.example`
- 关键变量：
  - `LLM_ROUTER_PG_DSN` / `LLM_ROUTER_POSTGRES_DSN`
  - `LLM_ROUTER_HOST` / `LLM_ROUTER_PORT`
  - `LLM_ROUTER_REQUIRE_AUTH`
  - `LLM_ROUTER_ALLOW_LOCAL_WITHOUT_AUTH`

## 迁移说明

后端已完全切换到 Go。SQLite 仅用于迁移输入（可由 `src/migrate` 在启动阶段导入到 PostgreSQL）。
