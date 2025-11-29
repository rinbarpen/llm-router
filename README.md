# LLM Router

统一的 LLM 路由服务，支持多厂商 API（OpenAI, Gemini, Claude, GLM, Qwen, Kimi, OpenRouter 等）及本地模型（Ollama, vLLM, Transformers），提供统一的 REST 接口、灵活的标签路由策略、完整的调用监控和细粒度的 API Key 访问控制。

## 功能特性

### 核心功能

- **统一接口**：屏蔽各厂商 API 差异，通过统一的 REST 接口调用所有模型
- **智能路由**：按任务类型（如 `coding`, `reasoning`, `image`, `chinese`）自动选择最佳模型
- **灵活配置**：通过 TOML 文件管理所有 Provider、模型及标签，支持热加载
- **多源支持**：
  - **远程 API**：OpenAI, Gemini, Claude, Grok, DeepSeek, Qwen, Kimi, GLM, OpenRouter 等
  - **本地运行**：Ollama, vLLM, Transformers (HuggingFace)

### 高级功能

- **访问控制**：细粒度的 API Key 认证系统，支持模型限制、Provider 限制和参数限制
- **调用监控**：完整的调用历史记录、统计分析和实时监控界面
- **限流控制**：支持按模型配置限流策略
- **多模态支持**：标记模型的多模态能力（视觉、音频等）

## 快速开始

### 1. 安装

推荐使用 `uv` 进行依赖管理：

```bash
# 克隆或下载项目
cd llm-router

# 初始化并安装依赖
uv sync
```

### 2. 配置文件

复制示例配置文件：

```bash
cp router.example.toml router.toml
cp .env.example .env
```

### 3. 配置说明

#### 服务器配置

在 `router.toml` 中配置服务器端口（也可通过环境变量）：

```toml
[server]
host = "0.0.0.0"  # 服务绑定的主机地址
port = 8000       # 服务绑定的端口
```

#### 前端配置（可选）

如果使用监控界面，配置前端端口：

```toml
[frontend]
port = 3000                    # 前端开发服务器端口
api_url = "http://localhost:8000"  # 后端API服务器地址
api_base_url = "/api"          # 生产环境API基础路径
```

#### Provider 配置

在 `router.toml` 中配置 Provider：

```toml
[[providers]]
name = "openai"
type = "openai"
api_key_env = "OPENAI_API_KEY"  # 从环境变量读取 API Key
base_url = "https://api.openai.com"
```

#### 模型配置

配置模型及其标签：

```toml
[[models]]
name = "gpt-4o"
provider = "openai"
display_name = "GPT-4o"
tags = ["chat", "general", "image", "reasoning"]
[models.config]
context_window = "128k"
supports_vision = true
supports_tools = true
```

### 4. 环境变量

在 `.env` 文件中配置 API Keys：

```bash
# Provider API Keys
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
GEMINI_API_KEY=AIza...
DASHSCOPE_API_KEY=sk-...  # Qwen
GLM_API_KEY=sk-...        # GLM
KIMI_API_KEY=sk-...       # Moonshot

# LLM Router API Keys（在 router.toml 中通过 key_env 引用）
LLM_ROUTER_ADMIN_KEY=admin-key-12345
LLM_ROUTER_LIMITED_KEY=limited-key-67890

# LLM Router 配置
LLM_ROUTER_MODEL_CONFIG=./router.toml
```

### 5. 启动服务

```bash
# 启动后端服务
uv run llm-router

# 启动前端监控界面（可选，需要先安装前端依赖）
cd frontend
npm install
npm run dev
```

## 配置文件详解

### router.toml 结构

配置文件采用 TOML 格式，包含以下部分：

#### 服务器配置

```toml
[server]
host = "0.0.0.0"  # 可选，默认 0.0.0.0
port = 8000       # 可选，默认 8000
```

#### 前端配置

```toml
[frontend]
port = 3000                    # 前端开发服务器端口
api_url = "http://localhost:8000"  # 后端API地址（开发环境代理用）
api_base_url = "/api"          # 生产环境API基础路径
```

#### Provider 配置

```toml
[[providers]]
name = "openai"              # Provider 名称（唯一标识）
type = "openai"              # Provider 类型
api_key_env = "OPENAI_API_KEY"  # 环境变量名（推荐）
# 或直接指定（不推荐）
# api_key = "sk-..."
base_url = "https://api.openai.com"  # 可选，API 基础地址
is_active = true             # 是否启用
settings = {}                # Provider 特定设置
```

#### 模型配置

```toml
[[models]]
name = "gpt-4o"              # 模型名称
provider = "openai"         # 所属 Provider
display_name = "GPT-4o"     # 显示名称
tags = ["chat", "general"]  # 标签列表
is_active = true            # 是否启用

# 限流配置（可选）
[models.rate_limit]
max_requests = 50           # 最大请求数
per_seconds = 60            # 时间窗口（秒）
burst_size = 100            # 突发大小（可选）

# 模型能力配置
[models.config]
context_window = "128k"     # 上下文窗口
supports_vision = true      # 支持视觉
supports_tools = true       # 支持工具调用
languages = ["en", "zh"]    # 支持的语言
```

#### API Key 配置

```toml
[[api_keys]]
key_env = "LLM_ROUTER_ADMIN_KEY"  # 从环境变量读取（推荐）
name = "管理员密钥"
is_active = true

# 模型限制（可选）
allowed_models = [
    "openai/gpt-4o",
    "claude/claude-3.5-sonnet",
]

# Provider 限制（可选）
allowed_providers = ["openai", "claude"]

# 参数限制（可选）
[api_keys.parameter_limits]
max_tokens = 2000
temperature = 0.7
top_p = 0.9
```

## API Key 认证系统

### 认证方式

API Key 可以通过以下三种方式提供：

1. **Authorization Bearer**（推荐）：
   ```bash
   curl -H "Authorization: Bearer your-api-key" ...
   ```

2. **X-API-Key 头**：
   ```bash
   curl -H "X-API-Key: your-api-key" ...
   ```

3. **查询参数**：
   ```bash
   curl "http://...?api_key=your-api-key" ...
   ```

### 访问限制

每个 API Key 可以配置以下限制：

- **模型限制**：`allowed_models` - 只能调用指定的模型列表
- **Provider 限制**：`allowed_providers` - 只能调用指定 Provider 的模型
- **参数限制**：`parameter_limits` - 自动限制调用参数（如 max_tokens、temperature 等）

如果限制为 `null` 或未设置，表示无限制。

### 管理 API Key

通过 REST API 管理 API Key：

```bash
# 创建 API Key
curl -X POST http://localhost:8000/api-keys \
  -H "Content-Type: application/json" \
  -d '{
    "key": "my-api-key",
    "name": "我的密钥",
    "allowed_models": ["openai/gpt-4o"]
  }'

# 列出所有 API Key
curl http://localhost:8000/api-keys

# 更新 API Key
curl -X PATCH http://localhost:8000/api-keys/1 \
  -H "Content-Type: application/json" \
  -d '{"is_active": false}'

# 删除 API Key
curl -X DELETE http://localhost:8000/api-keys/1
```

## API 使用示例

### 指定模型调用

```bash
curl -X POST http://localhost:8000/models/openai/gpt-4o/invoke \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{
    "prompt": "解释一下量子纠缠",
    "parameters": {
      "temperature": 0.7,
      "max_tokens": 500
    }
  }'
```

### 使用消息格式

```bash
curl -X POST http://localhost:8000/models/claude/claude-3.5-sonnet/invoke \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{
    "messages": [
      {"role": "system", "content": "你是一个有用的助手"},
      {"role": "user", "content": "写一个 Python 快速排序算法"}
    ],
    "parameters": {
      "temperature": 0.7
    }
  }'
```

### 智能路由

根据标签自动选择模型：

```bash
curl -X POST http://localhost:8000/route/invoke \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{
    "query": {
      "tags": ["coding", "analysis"]
    },
    "request": {
      "prompt": "分析这段代码的性能问题",
      "parameters": {"temperature": 0.5}
    }
  }'
```

### 查询可用模型

```bash
# 列出所有模型
curl http://localhost:8000/models

# 按标签筛选
curl "http://localhost:8000/models?tag=coding&tag=fast"

# 按 Provider 类型筛选
curl "http://localhost:8000/models?provider_type=openai"
```

## 监控系统

### 前端监控界面

项目包含完整的前端监控界面，可以实时查看模型调用情况。

#### 启动前端

```bash
cd frontend
npm install
npm run dev
```

访问 `http://localhost:3000`（或配置的端口）查看监控界面。

#### 功能特性

- **统计信息面板**：显示总体统计、按模型统计、最近错误
- **调用历史列表**：查看所有模型调用记录，支持筛选和分页
- **调用详情**：查看单次调用的详细信息，包括请求、响应、原始数据
- **实时更新**：自动刷新统计数据（每5秒）

### 监控 API

#### 获取调用历史

```bash
curl "http://localhost:8000/monitor/invocations?limit=50&offset=0" \
  -H "Authorization: Bearer your-api-key"
```

#### 获取统计信息

```bash
curl "http://localhost:8000/monitor/statistics?time_range_hours=24&limit=10" \
  -H "Authorization: Bearer your-api-key"
```

#### 获取单次调用详情

```bash
curl "http://localhost:8000/monitor/invocations/123" \
  -H "Authorization: Bearer your-api-key"
```

## 环境变量

### 后端配置

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `LLM_ROUTER_HOST` | 服务绑定的主机地址 | `0.0.0.0` |
| `LLM_ROUTER_PORT` | 服务绑定的端口 | `8000` |
| `LLM_ROUTER_DATABASE_URL` | 数据库连接字符串 | `sqlite+aiosqlite:///llm_router.db` |
| `LLM_ROUTER_MODEL_STORE` | 模型文件存储目录 | `./model_store` |
| `LLM_ROUTER_DOWNLOAD_CACHE` | 下载缓存目录 | None |
| `LLM_ROUTER_DOWNLOAD_CONCURRENCY` | 并发下载数 | `2` |
| `LLM_ROUTER_DEFAULT_TIMEOUT` | 请求超时时间（秒） | `60.0` |
| `LLM_ROUTER_LOG_LEVEL` | 日志级别 | `INFO` |
| `LLM_ROUTER_MODEL_CONFIG` | 模型配置文件路径 | None |
| `LLM_ROUTER_API_KEYS` | LLM Router API Key（多个用逗号分隔，简单配置） | None |
| `LLM_ROUTER_REQUIRE_AUTH` | 是否启用 API Key 认证 | `true` |

### 前端配置

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `VITE_PORT` | 前端开发服务器端口 | `3000` |
| `VITE_API_URL` | 后端API服务器地址（开发环境代理用） | `http://localhost:8000` |
| `VITE_API_BASE_URL` | 生产环境API基础路径 | `/api` |

**注意**：所有配置都可以在 `router.toml` 中设置，环境变量会覆盖配置文件中的值。

## 支持的 Provider 类型

| Type | 说明 | 示例 |
|------|------|------|
| `openai` | OpenAI 及兼容接口 | GPT-4o, GPT-3.5-turbo |
| `claude` / `anthropic` | Anthropic Claude 系列 | Claude 3.5 Sonnet, Claude 3 Opus |
| `gemini` | Google Gemini 系列 | Gemini 2.0 Flash, Gemini 1.5 Pro |
| `glm` | 智谱 GLM | GLM-4 Plus, GLM-4 Flash |
| `qwen` | 阿里通义千问 | Qwen2.5 72B, Qwen Plus |
| `kimi` | Moonshot Kimi | Moonshot v1 128K |
| `openrouter` | OpenRouter 聚合接口 | 支持多种模型 |
| `ollama` | 本地 Ollama 服务 | 本地部署的模型 |
| `vllm` | 本地 vLLM 服务 | 本地 vLLM 服务 |
| `transformers` | 本地 HuggingFace Transformers | 本地 Transformers 模型 |

## API 端点

### 健康检查

- `GET /health` - 检查服务状态

### Provider 管理

- `GET /providers` - 列出所有 Provider
- `POST /providers` - 创建或更新 Provider

### 模型管理

- `GET /models` - 列出所有模型（支持筛选）
- `POST /models` - 创建模型
- `PATCH /models/{provider_name}/{model_name}` - 更新模型

### 模型调用

- `POST /models/{provider_name}/{model_name}/invoke` - 直接调用指定模型
- `POST /route/invoke` - 智能路由调用

### 监控

- `GET /monitor/invocations` - 获取调用历史列表
- `GET /monitor/invocations/{id}` - 获取单次调用详情
- `GET /monitor/statistics` - 获取统计信息

### API Key 管理

- `GET /api-keys` - 列出所有 API Key
- `POST /api-keys` - 创建 API Key
- `GET /api-keys/{id}` - 获取 API Key 详情
- `PATCH /api-keys/{id}` - 更新 API Key
- `DELETE /api-keys/{id}` - 删除 API Key

详细的 API 文档请参考 [API.md](API.md)。

## 开发与测试

### 运行测试

```bash
uv run pytest
```

### 开发模式

```bash
# 后端开发（需要手动重启）
uv run llm-router

# 前端开发（支持热重载）
cd frontend
npm run dev
```

### 构建生产版本

```bash
# 构建前端
cd frontend
npm run build
```

## 常见问题

### 1. 如何禁用认证？

设置环境变量：

```bash
export LLM_ROUTER_REQUIRE_AUTH=false
```

或在 `router.toml` 中不配置任何 API Key，并设置 `LLM_ROUTER_REQUIRE_AUTH=false`。

### 2. 如何配置多个 API Key？

在 `.env` 文件中定义多个 key（逗号分隔），或在 `router.toml` 中配置多个 `[[api_keys]]` 条目。

### 3. 如何查看调用历史？

使用前端监控界面（推荐）或通过 API：

```bash
curl "http://localhost:8000/monitor/invocations" \
  -H "Authorization: Bearer your-api-key"
```

### 4. 如何限制 API Key 的调用参数？

在 `router.toml` 中配置 `parameter_limits`：

```toml
[[api_keys]]
key_env = "LLM_ROUTER_LIMITED_KEY"
[api_keys.parameter_limits]
max_tokens = 1000
temperature = 0.7
```

### 5. 配置文件优先级是什么？

优先级顺序：环境变量 > router.toml > 默认值

## 许可证

[根据项目实际情况填写]

## 贡献

欢迎提交 Issue 和 Pull Request！
