# 环境变量说明（Go 后端）

## 使用方式

1. 在项目根目录创建 `.env`：
   ```bash
   cp .env.example .env
   ```
2. 填入各 Provider API Key；SQLite 默认写入 `data/llm_router.db`。
3. 启动服务：
   ```bash
   go run ./cmd/llm-router
   ```

## 关键变量

- `LLM_ROUTER_HOST`：监听地址，默认 `0.0.0.0`
- `LLM_ROUTER_PORT`：监听端口，默认 `8000`
- `LLM_ROUTER_SQLITE_PATH`：运行时 SQLite 数据库路径，默认 `data/llm_router.db`
- `LLM_ROUTER_MIGRATE_FROM_SQLITE`：是否从旧 SQLite 文件补齐导入（默认 `true`）
- `LLM_ROUTER_SQLITE_MAIN_PATH`：SQLite 主库路径（迁移输入）
- `LLM_ROUTER_SQLITE_MONITOR_PATH`：SQLite 监控库路径（迁移输入）
- `LLM_ROUTER_REQUIRE_AUTH`：是否启用认证（默认 `false`）
- `LLM_ROUTER_ALLOW_LOCAL_WITHOUT_AUTH`：本机是否免认证（默认 `true`）

## 注意事项

- `.env` 不应提交到版本控制系统。
- 生产环境建议显式设置 `LLM_ROUTER_SQLITE_PATH`，并挂载持久化目录。
- 切换期如需导入旧 SQLite 数据，请确保输入文件可读、运行时数据库目录可写。
