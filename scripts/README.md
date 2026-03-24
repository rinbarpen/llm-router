# 工具脚本

本目录包含 LLM Router 的各种工具脚本，包括开机启动、模型有效性检查和 API Key 生成工具。

## 目录结构

```
scripts/
├── generate_api_key.py      # 生成 API Key 的 Python 脚本
├── generate_api_key.sh      # 生成 API Key 的 Shell 包装脚本
├── pricing_sync.sh          # 从多来源价格文件同步模型定价
├── start-db.sh              # 启动/复用本地 PostgreSQL Docker 容器
├── check-db.sh              # 检查本地 PostgreSQL Docker 容器与连通性
├── check-service.sh         # 检查 LLM Router 服务是否已启动
├── check-free-models.sh     # 检查免费模型是否可调用
├── check_providers_validity.py         # 批量检查 router.toml 中模型可用性
├── check_and_clean_openrouter_free.py  # 检查并清理无效 OpenRouter 免费模型
├── tests/request_codex_claude.py       # 请求 Codex CLI / Claude Code 模型
├── linux/                   # Linux systemd 服务文件
├── macos/                   # macOS launchd 服务文件
├── windows/                 # Windows 任务计划脚本
└── tests/                   # 手工检查脚本（非 pytest 用例）
```

## 模型检查脚本

```bash
# 检查后端服务健康状态
./scripts/check-service.sh

# 检查免费模型可用性（依赖后端服务已启动）
./scripts/check-free-models.sh

# 检查 router.toml 中全部模型可用性并生成 test_report.json
python scripts/check_providers_validity.py

# 检查并清理无效的 OpenRouter 免费模型
python scripts/check_and_clean_openrouter_free.py

# scripts/tests/ 下的快速检查脚本
python scripts/tests/check_openrouter_free.py
python scripts/tests/check_all_openrouter_free.py

# 请求 Code CLI / Claude Code 模型
python scripts/tests/request_codex_claude.py codex --prompt "解释一下 RAG"
python scripts/tests/request_codex_claude.py opencode --prompt "写一个快速排序"
python scripts/tests/request_codex_claude.py kimi-code --prompt "给我一个 Rust 单元测试样例"
python scripts/tests/request_codex_claude.py qwen-code --prompt "解释这段 Python 代码"
python scripts/tests/request_codex_claude.py claude --model "claude_code_cli/claude-sonnet-4-5" --prompt "总结这段代码"
python scripts/tests/request_codex_claude.py all --prompt "给我一个 3 行 Python 示例"
```

## 模型定价同步脚本

```bash
# 仅生成环境变量（查看将使用哪些本地来源）
./scripts/pricing_sync.sh --print-env

# 仅验证价格来源并请求 /pricing/latest（不写回模型配置）
./scripts/pricing_sync.sh --dry-run

# 完整执行：/pricing/latest + /pricing/sync-all
./scripts/pricing_sync.sh

# 指定后端地址
./scripts/pricing_sync.sh --api-url http://127.0.0.1:18000
```

说明：
- 默认读取 `data/pricing/*.json`（openai/claude/gemini/deepseek/qwen/kimi/glm/groq）。
- 自动构造 `LLM_ROUTER_PRICING_SOURCE_URLS`（`file://` 形式）并仅用于本次请求。
- 价格源文件格式见 `data/pricing/README.md`。

`request_codex_claude.py` 会优先请求新版端点（`/v1/responses`、`/v1/messages`），当返回 `404` 时会依次回退到 `/v1/chat/completions`、`/{provider}/v1/chat/completions`、`/models/{provider}/{model}/invoke`、`/route/invoke`，用于兼容旧版后端实例。

说明：以上脚本为手工运维/验证工具，不属于 `pytest` 自动回归测试；项目核心自动化测试位于 `tests/`。

## API Key 生成工具

### 快速使用

```bash
# 生成一个默认长度的 API Key（32 字符）
python scripts/generate_api_key.py

# 或使用 shell 脚本
./scripts/generate_api_key.sh
```

### 高级用法

```bash
# 生成指定长度的 key
python scripts/generate_api_key.py --length 64

# 生成多个 key
python scripts/generate_api_key.py --count 5

# 生成并输出为环境变量格式（方便添加到 .env 文件）
python scripts/generate_api_key.py --env LLM_ROUTER_ADMIN_KEY

# 生成多个 key 并输出为环境变量格式（逗号分隔）
python scripts/generate_api_key.py --count 3 --env LLM_ROUTER_ADMIN_KEY

# 添加前缀（如 sk-）
python scripts/generate_api_key.py --prefix sk- --length 40
```

### 使用示例

```bash
# 1. 生成管理员 key
python scripts/generate_api_key.py --env LLM_ROUTER_ADMIN_KEY --length 32
# 输出: LLM_ROUTER_ADMIN_KEY=xxx...

# 2. 将输出添加到 .env 文件
python scripts/generate_api_key.py --env LLM_ROUTER_ADMIN_KEY >> .env

# 3. 生成多个受限 key
python scripts/generate_api_key.py --count 3 --length 40
```

### 安全说明

- 生成的 API Key 使用 Python `secrets` 模块，确保加密安全
- 默认长度 32 字符，建议至少 16 字符
- 自动排除容易混淆的字符（0, O, I, l）
- 生成的 key 包含字母、数字和部分特殊字符（-、_）

## 本地开发启动脚本

### 快速使用

```bash
./scripts/start.sh
```

默认会同时启动后端和监控界面。

### 可用模式

```bash
./scripts/start.sh all
./scripts/start.sh backend
./scripts/start.sh monitor
./scripts/start.sh --help
```

### 行为说明

- 后端默认使用 Go（`go run ./backend/cmd/llm-router`）
- 可通过 `LLM_ROUTER_BACKEND_IMPL=python` 切换回 Python 后端（`uv run llm-router`）
- Go 后端模式会在启动前检查 PostgreSQL 可达性（默认 `localhost:5432`，或从 `LLM_ROUTER_PG_DSN` / `LLM_ROUTER_POSTGRES_DSN` / `LLM_ROUTER_DATABASE_URL` 解析）
- 监控界面使用 `npm run dev`
- 脚本会检查 `go` 或 `uv`、`npm` 和 `examples/monitor/node_modules`
- 脚本不会自动安装依赖；若缺失，请先执行 `uv sync` 或 `cd examples/monitor && npm install`
- 在 `all` 模式下，任一子进程退出时，脚本会终止另一进程并退出

## 本地数据库脚本

### 启动数据库

```bash
./scripts/start-db.sh
```

默认会启动（或复用）`llm-router-pg` 容器，并创建数据库 `llm_router`，账号密码默认为 `rczx/rczx`。

### 检查数据库

```bash
./scripts/check-db.sh
```

会检查容器状态、`pg_isready` 健康状态，并执行一条 SQL 验证连通性。

### 常用环境变量

```bash
LLM_ROUTER_DB_CONTAINER=llm-router-pg
LLM_ROUTER_DB_IMAGE=postgres:16
LLM_ROUTER_DB_PORT=5432
LLM_ROUTER_DB_NAME=llm_router
LLM_ROUTER_DB_USER=rczx
LLM_ROUTER_DB_PASSWORD=rczx
```

## 开机启动脚本

## 目录结构

```
scripts/
├── linux/          # Linux systemd 服务文件
│   ├── install.sh              # 安装脚本
│   ├── uninstall.sh            # 卸载脚本
│   ├── llm-router-backend.service
│   ├── llm-router-monitor.service
│   ├── llm-router-pricing-sync.service
│   ├── llm-router-pricing-sync.timer
│   ├── install-pricing-sync.sh
│   └── uninstall-pricing-sync.sh
├── macos/          # macOS launchd 服务文件
│   ├── install.sh              # 安装脚本
│   ├── uninstall.sh            # 卸载脚本
│   ├── com.llmrouter.backend.plist
│   └── com.llmrouter.monitor.plist
└── windows/        # Windows 任务计划脚本
    ├── install-backend.ps1     # 后端安装脚本
    ├── install-monitor.ps1     # 监控界面安装脚本
    └── uninstall.ps1            # 卸载脚本
```

## Linux (systemd)

### 安装

```bash
cd scripts/linux
sudo ./install.sh
```

脚本会：
1. 检测项目路径和用户信息
2. 询问要安装的服务（后端/监控界面/两者）
3. 创建 systemd 服务文件
4. 启用开机自启

### 服务管理

```bash
# 启动服务
sudo systemctl start llm-router-backend
sudo systemctl start llm-router-monitor

# 停止服务
sudo systemctl stop llm-router-backend
sudo systemctl stop llm-router-monitor

# 查看状态
sudo systemctl status llm-router-backend
sudo systemctl status llm-router-monitor

# 查看日志
sudo journalctl -u llm-router-backend -f
sudo journalctl -u llm-router-monitor -f

# 禁用开机自启
sudo systemctl disable llm-router-backend
sudo systemctl disable llm-router-monitor
```

### 卸载

```bash
cd scripts/linux
sudo ./uninstall.sh
```

### 定时同步模型价格（systemd timer）

```bash
cd scripts/linux
sudo ./install-pricing-sync.sh
```

默认每 6 小时执行一次 `scripts/pricing_sync.sh`，并使用 `data/pricing/*.json` 作为来源。

常用命令：

```bash
sudo systemctl status llm-router-pricing-sync.timer
sudo systemctl list-timers llm-router-pricing-sync.timer
sudo systemctl start llm-router-pricing-sync.service
sudo journalctl -u llm-router-pricing-sync.service -f
```

卸载：

```bash
cd scripts/linux
sudo ./uninstall-pricing-sync.sh
```

## macOS (launchd)

### 安装

```bash
cd scripts/macos
./install.sh
```

脚本会：
1. 检测项目路径和用户信息
2. 询问要安装的服务（后端/监控界面/两者）
3. 创建 launchd plist 文件到 `~/Library/LaunchAgents/`
4. 加载并启动服务

### 服务管理

```bash
# 启动服务
launchctl start com.llmrouter.backend
launchctl start com.llmrouter.monitor

# 停止服务
launchctl stop com.llmrouter.backend
launchctl stop com.llmrouter.monitor

# 查看状态
launchctl list | grep llmrouter

# 查看日志
tail -f ~/workspace/sxy/gym/llm-router/logs/backend.log
tail -f ~/workspace/sxy/gym/llm-router/logs/monitor.log

# 卸载服务（停止并删除）
launchctl unload ~/Library/LaunchAgents/com.llmrouter.backend.plist
launchctl unload ~/Library/LaunchAgents/com.llmrouter.monitor.plist
```

### 卸载

```bash
cd scripts/macos
./uninstall.sh
```

## Windows (任务计划程序)

### 安装后端

1. 以**管理员身份**打开 PowerShell
2. 运行安装脚本：

```powershell
cd scripts\windows
.\install-backend.ps1
```

### 安装监控界面

```powershell
.\install-monitor.ps1
```

### 服务管理

```powershell
# 启动服务
Start-ScheduledTask -TaskName "LLMRouter-Backend"
Start-ScheduledTask -TaskName "LLMRouter-Monitor"

# 停止服务
Stop-ScheduledTask -TaskName "LLMRouter-Backend"
Stop-ScheduledTask -TaskName "LLMRouter-Monitor"

# 查看状态
Get-ScheduledTask -TaskName "LLMRouter-Backend"
Get-ScheduledTask -TaskName "LLMRouter-Monitor"

# 查看任务计划（图形界面）
taskschd.msc
```

### 卸载

```powershell
cd scripts\windows
.\uninstall.ps1
```

## 注意事项

### 前置要求

1. **已安装依赖**：
   - 后端：默认需要 `go`（若切换 Python 后端则需要 `uv`）
   - 监控界面：`npm` 已安装并在 PATH 中

2. **配置文件**：
   - 确保 `router.toml` 已正确配置
   - 确保 `.env` 文件包含必要的 API Keys

3. **项目路径**：
   - 脚本会自动检测项目路径
   - 如果项目路径不是默认路径，可能需要修改脚本中的路径

### 路径配置

如果项目不在默认路径，需要修改：

- **Linux**: 编辑 `install.sh` 中的 `PROJECT_ROOT` 变量
- **macOS**: 编辑 `install.sh` 中的 `PROJECT_ROOT` 变量
- **Windows**: 运行脚本时指定路径：
  ```powershell
  .\install-backend.ps1 -ProjectPath "C:\path\to\llm-router"
  ```

### 日志位置

- **Linux**: 使用 `journalctl` 查看日志
- **macOS**: `~/workspace/sxy/gym/llm-router/logs/`
- **Windows**: 任务计划程序的执行历史

### 故障排查

1. **服务无法启动**：
   - 检查 `uv` 和 `npm` 是否在 PATH 中
   - 检查项目路径是否正确
   - 查看日志文件

2. **端口冲突**：
   - 检查 `router.toml` 中的端口配置
   - 确保端口未被其他程序占用

3. **权限问题**：
   - Linux/macOS: 确保脚本有执行权限
   - Windows: 确保以管理员身份运行

## 手动配置（高级）

如果自动安装脚本不满足需求，可以手动配置：

### Linux systemd

复制服务文件到 `/etc/systemd/system/`，修改路径和用户，然后：

```bash
sudo systemctl daemon-reload
sudo systemctl enable llm-router-backend
sudo systemctl start llm-router-backend
```

### macOS launchd

复制 plist 文件到 `~/Library/LaunchAgents/`，修改路径，然后：

```bash
launchctl load ~/Library/LaunchAgents/com.llmrouter.backend.plist
```

### Windows 任务计划

1. 打开"任务计划程序" (`taskschd.msc`)
2. 创建基本任务
3. 设置触发器为"计算机启动时"
4. 设置操作为运行脚本
