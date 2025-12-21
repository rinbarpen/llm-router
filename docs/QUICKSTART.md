# 快速启动指南

## 前置准备

1. **安装 uv**（如果未安装）：
   ```bash
   curl -LsSf https://astral.sh/uv/install.sh | sh
   ```

2. **确保有 router.toml 配置文件**：
   - 项目根目录应已有 `router.toml` 配置文件
   - 如果不存在，请创建并配置 Provider 和模型

3. **配置环境变量**（推荐使用 .env 文件）：
   ```bash
   # 如果 .env 文件不存在，可以创建
   touch .env
   # 编辑 .env 文件，填入各 Provider 的 API Key
   # 示例：LLM_ROUTER_ADMIN_KEY=your-admin-api-key
   ```

## 启动后端服务

### 方式1：使用 uv（推荐）

```bash
# 安装依赖（首次运行）
uv sync

# 启动服务（注意：命令是 llm-router，使用连字符，不是下划线）
uv run llm-router
```

**重要**：命令是 `llm-router`（连字符），不是 `llm_router`（下划线）。

服务将根据 `router.toml` 中的 `[server]` 配置或环境变量启动。默认端口为 8000，如果配置文件中设置了其他端口（如 18000），将使用配置文件中的端口。

### 方式2：使用 Python 直接运行

```bash
# 确保在虚拟环境中
source .venv/bin/activate  # 或使用 uv 创建的虚拟环境

# 启动服务
python -m llm_router
```

**注意**：Python 模块名是 `llm_router`（下划线），但命令行工具是 `llm-router`（连字符）。

### 验证服务运行

启动后，访问健康检查端点：

```bash
# 默认端口 8000
curl http://localhost:8000/health

# 或如果 router.toml 中配置了其他端口（如 18000）
curl http://localhost:18000/health
```

应该返回：
```json
{"status": "ok"}
```

**注意**：服务端口会根据 `router.toml` 中的 `[server]` 配置自动设置，无需手动指定环境变量。

### 快速调用路由接口（本机免认证）
```bash
curl -X POST "http://localhost:18000/route/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "query": {"tags": ["chat","general"], "provider_types": ["openai","gemini","claude"]},
    "request": {"messages": [{"role": "user", "content": "Hello, how are you?"}], "stream": false}
  }'
```
> 远程访问或强制认证时，请在请求头加入 `Authorization: Bearer <session-token 或 api_key>`。

## 启动前端监控界面（可选）

如果需要使用监控界面查看调用历史：

```bash
cd frontend

# 安装依赖（首次运行）
npm install

# 启动开发服务器
npm run dev
```

前端将根据 `router.toml` 中的 `[frontend]` 配置启动。默认端口为 3000，如果配置文件中设置了其他端口（如 4022），将使用配置文件中的端口。

访问 `http://localhost:3000`（或 `router.toml` 中配置的端口，如 4022）查看监控界面。

**注意**：前端端口会根据 `router.toml` 中的 `[frontend]` 配置自动设置。

## 启动顺序

1. **先启动后端服务**：
   ```bash
   uv run llm-router
   ```

2. **再启动前端**（如果需要）：
   ```bash
cd frontend && npm run dev
   ```

## 常见问题

### 命令错误：`llm_router` vs `llm-router`

- ✅ **正确**：`uv run llm-router`（连字符）
- ❌ **错误**：`uv run llm_router`（下划线）

命令行工具使用连字符，Python 模块使用下划线。

### 端口被占用

如果遇到端口被占用，可以：

1. **修改 router.toml**：
   ```toml
   [server]
   port = 9000  # 改为其他端口
   ```

2. **或使用环境变量**（优先级更高）：
   ```bash
   export LLM_ROUTER_PORT=9000
   uv run llm-router
   ```

### 数据库初始化

首次启动时，系统会自动创建 SQLite 数据库文件 `llm_router.db`。

### 配置文件未找到

确保 `router.toml` 文件存在于项目根目录，或通过环境变量指定：

```bash
export LLM_ROUTER_MODEL_CONFIG=/path/to/router.toml
uv run llm-router
```

### 启动失败：NameError 或 ImportError

如果遇到导入错误，确保：

1. 已运行 `uv sync` 安装依赖
2. 在项目根目录执行命令
3. 检查是否有语法错误或缺失的导入

### 查看启动日志

服务启动时会显示详细的日志信息，包括：
- 服务器绑定的地址和端口
- 数据库初始化状态
- 配置文件加载情况

## 测试调用

服务启动后，可以测试调用：

```bash
# 如果启用了认证，需要提供 API Key
curl -X POST http://localhost:8000/models/openai/gpt-4o/invoke \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{
    "prompt": "Hello, world!"
  }'
```

## 停止服务

- 后端：按 `Ctrl+C` 停止
- 前端：按 `Ctrl+C` 停止

