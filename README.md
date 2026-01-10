# LLM Router

统一的 LLM 路由服务，支持多厂商 API（OpenAI, Gemini, Claude, GLM, Qwen, Kimi, OpenRouter 等）及本地模型（Ollama, vLLM, Transformers），提供统一的 REST 接口、灵活的标签路由策略、完整的调用监控和细粒度的 API Key 访问控制。

## 功能特性

### 核心功能

- **统一接口**：屏蔽各厂商 API 差异，通过统一的 REST 接口调用所有模型
- **OpenAI 兼容 API**：提供标准的 OpenAI 兼容接口，通过 `/v1/chat/completions` 端点调用模型，model 参数在请求体中，支持无缝替换 OpenAI SDK
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

### 1. 安装依赖

推荐使用 `uv` 进行依赖管理：

```bash
# 克隆或下载项目
cd llm-router

# 初始化并安装依赖
uv sync
```

如果没有安装 `uv`，可以安装：
```bash
curl -LsSf https://astral.sh/uv/install.sh | sh
```

### 2. 配置文件

编辑 `router.toml` 文件，配置 Provider 和模型。

创建 `.env` 文件（如果不存在），填入各 Provider 的 API Key：

```bash
# 如果 .env 文件不存在，可以创建
touch .env
```

编辑 `.env` 文件，填入各 Provider 的 API Key。

### 3. 启动服务

#### 启动后端

```bash
# 使用 uv（推荐）
uv run llm-router

# 或使用 Python
python -m llm_router
```

服务将根据 `router.toml` 中的 `[server]` 配置启动。如果 `router.toml` 存在于项目根目录，系统会自动读取其中的端口配置。

**配置优先级**：环境变量 > router.toml > 默认值（8000）

验证服务运行：
```bash
# 根据 router.toml 中的配置访问（如配置了 18000 端口）
curl http://localhost:18000/health

# 或默认端口
curl http://localhost:8000/health
```

**注意**：健康检查端点无需认证，本机请求也无需认证。

#### 启动前端监控界面（可选）

```bash
cd frontend
npm install  # 首次运行需要
npm run dev
```

访问 `http://localhost:3000`（或配置的端口）查看监控界面。

**详细启动说明请参考 [QUICKSTART.md](QUICKSTART.md)**

### 4. 配置说明

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

#### 手动启动

```bash
# 启动后端服务
uv run llm-router

# 启动前端监控界面（可选，需要先安装前端依赖）
cd frontend
npm install
npm run dev
```

#### 开机自启（可选）

项目提供了跨平台的开机启动脚本，支持 Linux、macOS 和 Windows。详细说明请参考 [scripts/README.md](scripts/README.md)。

**快速开始**：

- **Linux**: `cd scripts/linux && sudo ./install.sh`
- **macOS**: `cd scripts/macos && ./install.sh`
- **Windows**: 以管理员身份运行 PowerShell，执行 `cd scripts\windows && .\install-backend.ps1`

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
base_url = "https://api.openai.com"  # 可选，API 基础地址（如果不配置则使用默认值）
is_active = true             # 是否启用
settings = {}                # Provider 特定设置
```

**base_url 配置说明**：
- `base_url` 字段是可选的，用于自定义 API 服务器地址
- 如果不配置 `base_url`，系统会使用各 Provider 的默认地址：
  - OpenAI: `https://api.openai.com`
  - DeepSeek: `https://api.deepseek.com`
  - Qwen: `https://dashscope.aliyuncs.com`
  - Kimi: `https://api.moonshot.cn`
  - GLM: `https://open.bigmodel.cn/api/paas`
  - OpenRouter: `https://openrouter.ai/api`
- 如果需要使用自定义 API 服务器（如代理服务器），可以配置 `base_url` 覆盖默认值

**支持的 Provider 类型**：
- `openai` - OpenAI API
- `claude` - Anthropic Claude API
- `gemini` - Google Gemini API
- `deepseek` - DeepSeek API
- `qwen` - 阿里云通义千问 API
- `kimi` - 月之暗面 Kimi API
- `glm` - 智谱 GLM API
- `openrouter` - OpenRouter API
- `grok` - xAI Grok API
- `ollama` - 本地 Ollama 服务
- `vllm` - 本地 vLLM 服务
- `transformers` - 本地 Transformers 模型

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

# 模型能力及定价配置
[models.config]
context_window = "128k"     # 上下文窗口
supports_vision = true      # 支持视觉
supports_tools = true       # 支持工具调用
languages = ["en", "zh"]    # 支持的语言
cost_per_1k_tokens = 0.0005          # 每 1k 输入 token 的费用 (USD)
cost_per_1k_completion_tokens = 0.003 # 每 1k 输出 token 的费用 (USD)
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

### 认证策略

LLM Router 实现了基于来源的认证策略：

1. **本机请求（localhost/127.0.0.1）**：
   - ✅ **不需要认证** - 可以直接访问所有端点
   - 如果提供了认证信息，仍然会应用相应的权限限制（如模型限制、参数限制等）
   - 适用于本地开发和测试

2. **远程请求（其他来源）**：
   - ❌ **必须认证**（如果启用了认证）
   - 需要先登录获取 Session Token，或直接使用 API Key
   - 适用于生产环境和远程访问

### 推荐方式：先登录后请求（Session Token）

**步骤 1: 登录获取 Session Token**

```bash
curl -X POST http://localhost:18000/auth/login \
  -H "Content-Type: application/json" \
  -d '{"api_key": "your-api-key"}'
```

**响应：**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_in": 86400,
  "message": "登录成功，请使用此 token 进行后续请求。使用 /auth/bind-model 绑定模型。"
}
```

**步骤 2: 绑定模型到 Session（推荐，用于 OpenAI 兼容 API）**

```bash
curl -X POST http://localhost:18000/auth/bind-model \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_SESSION_TOKEN" \
  -d '{"provider_name": "openai", "model_name": "gpt-4o"}'
```

**响应：**
```json
{
  "message": "模型 openai/gpt-4o 已绑定到 session",
  "provider_name": "openai",
  "model_name": "gpt-4o"
}
```

**步骤 3: 使用 Session Token 进行请求**

**方式 1: 使用标准接口**
```bash
curl -X POST http://localhost:18000/models/openai/gpt-4o/invoke \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_SESSION_TOKEN" \
  -d '{"prompt": "Hello", "parameters": {"max_tokens": 100}}'
```

**方式 2: 使用 OpenAI 兼容 API（推荐）**
```bash
# 使用模型特定的端点
curl -X POST http://localhost:18000/models/openai/gpt-4o/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_SESSION_TOKEN" \
  -d '{
    "messages": [{"role": "user", "content": "Hello!"}],
    "temperature": 0.7
  }'
```

如果已绑定模型，可以不指定 `model` 字段；也可以在请求中指定模型，系统会自动绑定到 session。

**步骤 4: 登出（可选）**

```bash
curl -X POST http://localhost:18000/auth/logout \
  -H "Authorization: Bearer YOUR_SESSION_TOKEN"
```

### 兼容方式：直接使用 API Key（向后兼容）

如果不想使用登录流程，仍然可以直接使用 API Key（不推荐，安全性较低）：

1. **Authorization Bearer**：
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

### 认证方式对比

| 方式 | 安全性 | 推荐度 | 说明 |
|------|--------|--------|------|
| Session Token（登录后） | 高 | ⭐⭐⭐⭐⭐ | 推荐方式，API Key 不会在每次请求中传输 |
| 直接使用 API Key | 中 | ⭐⭐⭐ | 向后兼容，但 API Key 会在每次请求中传输 |
| 本机请求（免认证） | - | - | 仅限 localhost，自动跳过认证 |

### 访问限制

每个 API Key 可以配置以下限制：

- **模型限制**：`allowed_models` - 只能调用指定的模型列表
- **Provider 限制**：`allowed_providers` - 只能调用指定 Provider 的模型
- **参数限制**：`parameter_limits` - 自动限制调用参数（如 max_tokens、temperature 等）

如果限制为 `null` 或未设置，表示无限制。

### 管理 API Key

通过 REST API 管理 API Key：

```bash
# 登录获取 Token（推荐）
TOKEN=$(curl -s -X POST http://localhost:18000/auth/login \
  -H "Content-Type: application/json" \
  -d '{"api_key": "admin-api-key"}' | jq -r '.token')

# 创建 API Key
curl -X POST http://localhost:18000/api-keys \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "key": "my-api-key",
    "name": "我的密钥",
    "allowed_models": ["openai/gpt-4o"]
  }'

# 列出所有 API Key
curl http://localhost:18000/api-keys \
  -H "Authorization: Bearer $TOKEN"

# 更新 API Key
curl -X PATCH http://localhost:18000/api-keys/1 \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"is_active": false}'

# 删除 API Key
curl -X DELETE http://localhost:18000/api-keys/1 \
  -H "Authorization: Bearer $TOKEN"
```

**注意**：本机请求（localhost）可以省略认证头。

## API 使用示例

### OpenAI 兼容 API 示例（推荐）

**使用标准的 `/v1/chat/completions` 端点（完全兼容 OpenAI SDK）：**

```bash
# 标准调用：model 参数在请求体中
curl -X POST http://localhost:18000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "openrouter/glm-4.5-air",
    "messages": [{"role": "user", "content": "Hello!"}],
    "temperature": 0.7,
    "max_tokens": 150
  }'
```

**使用 OpenAI SDK：**

```python
from openai import OpenAI

# 创建客户端，指向 LLM Router 的标准端点
client = OpenAI(
    base_url="http://localhost:18000/v1",
    api_key="dummy"  # 本机请求可用任意值
)

# 标准 OpenAI API 调用
response = client.chat.completions.create(
    model="openrouter/glm-4.5-air",
    messages=[
        {"role": "user", "content": "Hello!"}
    ],
    temperature=0.7,
    max_tokens=150
)

print(response.choices[0].message.content)
```

**关于 model 参数：**
- `model` 参数（必填）：格式为 `provider_name/model_name`，例如 `"openrouter/glm-4.5-air"`
- 如果使用 session 绑定模型，`model` 参数可以省略（见下方 Session 绑定示例）
- 也可以使用完整的远程模型标识符来调用未配置在数据库中的模型


### 标准 API 示例

#### 本机请求示例（免认证）

**注意**：以下示例仅在本机（localhost）访问时有效，远程访问仍需要认证。

```bash
# 本机请求可以直接调用，无需认证
curl -X POST http://localhost:18000/models/openrouter/openrouter-glm-4.5-air/invoke \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "你好",
    "parameters": {"max_tokens": 100}
  }'
```

#### 远程请求示例（需要认证）

**方式 1: 使用 Session Token（推荐）**

```bash
# 1. 登录获取 Token
TOKEN=$(curl -s -X POST http://your-server:18000/auth/login \
  -H "Content-Type: application/json" \
  -d '{"api_key": "your-api-key"}' | jq -r '.token')

# 2. 使用 Token 调用模型
curl -X POST http://your-server:18000/models/openai/gpt-4o/invoke \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "prompt": "解释一下量子纠缠",
    "parameters": {
      "temperature": 0.7,
      "max_tokens": 500
    }
  }'
```

#### 方式 2: 直接使用 API Key（向后兼容）

```bash
curl -X POST http://your-server:18000/models/openai/gpt-4o/invoke \
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
curl -X POST http://localhost:18000/models/claude/claude-3.5-sonnet/invoke \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_SESSION_TOKEN" \
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
curl -X POST http://localhost:18000/route/invoke \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_SESSION_TOKEN" \
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
# 列出所有模型（本机请求免认证）
curl http://localhost:18000/models

# 按标签筛选
curl "http://localhost:18000/models?tag=coding&tag=fast"

# 按 Provider 类型筛选
curl "http://localhost:18000/models?provider_type=openai"
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

- **活动概览面板**：显示总体统计（花费、令牌数、请求数）、时间序列图表、调用历史列表
  - 摘要卡片：显示花费、令牌数、请求数及其日均值和过去一月数据
  - 时间序列可视化：支持按小时/天/周/月粒度查看调用趋势，可按模型或提供商分组查看 Token 消耗
  - 调用历史：查看所有模型调用记录，支持日期范围筛选和分页
- **调用历史列表**：独立的调用历史查看页面，支持详细筛选和分页
- **调用详情**：查看单次调用的详细信息，包括请求、响应、原始数据
- **实时更新**：自动刷新统计数据（每10秒）

### 监控 API

#### 获取调用历史

```bash
# 本机请求（免认证）
curl "http://localhost:18000/monitor/invocations?limit=50&offset=0"

# 远程请求（需要认证）
curl "http://your-server:18000/monitor/invocations?limit=50&offset=0" \
  -H "Authorization: Bearer YOUR_SESSION_TOKEN"
```

#### 获取统计信息

```bash
# 本机请求（免认证）
curl "http://localhost:18000/monitor/statistics?time_range_hours=24&limit=10"

# 远程请求（需要认证）
curl "http://your-server:18000/monitor/statistics?time_range_hours=24&limit=10" \
  -H "Authorization: Bearer YOUR_SESSION_TOKEN"
```

#### 获取单次调用详情

```bash
# 本机请求（免认证）
curl "http://localhost:18000/monitor/invocations/123"

# 远程请求（需要认证）
curl "http://your-server:18000/monitor/invocations/123" \
  -H "Authorization: Bearer YOUR_SESSION_TOKEN"
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

| Type | 说明 | 示例 | 默认 base_url |
|------|------|------|---------------|
| `openai` | OpenAI 及兼容接口 | GPT-4o, GPT-3.5-turbo | `https://api.openai.com` |
| `claude` / `anthropic` | Anthropic Claude 系列 | Claude 3.5 Sonnet, Claude 3 Opus | - |
| `gemini` | Google Gemini 系列 | Gemini 2.0 Flash, Gemini 1.5 Pro | - |
| `deepseek` | DeepSeek API | DeepSeek Chat, DeepSeek Coder | `https://api.deepseek.com` |
| `glm` | 智谱 GLM | GLM-4 Plus, GLM-4 Flash | `https://open.bigmodel.cn/api/paas` |
| `qwen` | 阿里通义千问 | Qwen2.5 72B, Qwen Plus | `https://dashscope.aliyuncs.com` |
| `kimi` | 月之暗面 Kimi | Kimi K2, Kimi Flash | `https://api.moonshot.cn` |
| `openrouter` | OpenRouter 聚合接口 | 支持多种模型 | `https://openrouter.ai/api` |
| `grok` | xAI Grok | Grok-2 | `https://api.x.ai` |
| `ollama` | 本地 Ollama 服务 | 本地部署的模型 | - |
| `vllm` | 本地 vLLM 服务 | 本地 vLLM 服务 | - |
| `transformers` | 本地 HuggingFace Transformers | 本地 Transformers 模型 | - |

**注意**：所有 Provider 都支持通过 `base_url` 配置自定义 API 地址。如果不配置，将使用上表中的默认值。

## API 端点

### 认证

- `POST /auth/login` - 登录获取 Session Token
- `POST /auth/logout` - 登出使 Session Token 失效

### 健康检查

- `GET /health` - 检查服务状态（无需认证）

### Provider 管理

- `GET /providers` - 列出所有 Provider
- `POST /providers` - 创建或更新 Provider

### 模型管理

- `GET /models` - 列出所有模型（支持筛选）
- `POST /models` - 创建模型
- `PATCH /models/{provider_name}/{model_name}` - 更新模型

### 模型调用

- `POST /models/{provider_name}/{model_name}/invoke` - 直接调用指定模型
- `POST /v1/chat/completions` - **标准 OpenAI 兼容端点**，model 在请求体中（推荐）
- `POST /route/invoke` - 智能路由调用

### 监控

- `GET /monitor/invocations` - 获取调用历史列表
- `GET /monitor/invocations/{id}` - 获取单次调用详情
- `GET /monitor/statistics` - 获取统计信息
- `GET /monitor/time-series` - 获取时间序列数据

### API Key 管理

- `GET /api-keys` - 列出所有 API Key
- `POST /api-keys` - 创建 API Key
- `GET /api-keys/{id}` - 获取 API Key 详情
- `PATCH /api-keys/{id}` - 更新 API Key
- `DELETE /api-keys/{id}` - 删除 API Key

**注意**：
- 本机请求（localhost）可以省略认证
- 远程请求需要提供 Session Token 或 API Key
- 详细的 API 文档请参考 [API.md](docs/API.md)

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
# 本机请求（免认证）
curl "http://localhost:18000/monitor/invocations"

# 远程请求（需要认证）
TOKEN=$(curl -s -X POST http://your-server:18000/auth/login \
  -H "Content-Type: application/json" \
  -d '{"api_key": "your-api-key"}' | jq -r '.token')
curl "http://your-server:18000/monitor/invocations" \
  -H "Authorization: Bearer $TOKEN"
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
