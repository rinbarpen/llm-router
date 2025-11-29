# LLM Router

统一的 LLM 路由服务，支持多厂商 API（OpenAI, Gemini, Claude, GLM, Qwen, Kimi, OpenRouter 等）及本地模型（Ollama, vLLM, Transformers），提供统一的 REST 接口与灵活的标签路由策略。

## 功能特性

- **统一接口**：屏蔽各厂商 API 差异，通过 `/invoke` 统一调用。
- **标签路由**：按任务类型（如 `coding`, `reasoning`, `image`, `chinese`）自动选择最佳模型。
- **灵活配置**：通过 TOML 文件管理所有 Provider、模型及标签，支持热加载。
- **多源支持**：
  - **远程 API**：OpenAI, Gemini, Claude, Grok, DeepSeek, Qwen, Kimi, GLM, OpenRouter 等。
  - **本地运行**：Ollama, vLLM, Transformers (HuggingFace)。
- **高级控制**：支持限流（Rate Limit）、上下文长度与多模态能力标记。

## 快速开始

### 1. 安装与启动

推荐使用 `uv` 进行依赖管理与启动：

```bash
# 初始化并安装依赖
uv sync

# 启动服务 (默认 8000 端口)
uv run llm-router
```

### 2. 配置文件 (router.toml)

复制示例配置并修改：

```bash
cp router.example.toml router.toml
```

在 `router.toml` 中定义 Provider 与模型（支持最新模型如 GPT-4o, Claude 3.5/3.7, Gemini 1.5/2.5 等）：

```toml
[[providers]]
name = "openai"
type = "openai"
api_key_env = "OPENAI_API_KEY"

[[models]]
name = "gpt-4o"
provider = "openai"
display_name = "GPT-4o"
tags = ["chat", "general", "image", "reasoning"]
[models.config]
context_window = "128k"
supports_vision = true

[[models]]
name = "claude-3.5-sonnet"
provider = "claude"
tags = ["coding", "analysis", "latest"]
```

### 3. 设置环境变量

**方式1：使用 .env 文件（推荐）**

复制示例文件并填写实际值：

```bash
cp .env.example .env
# 编辑 .env 文件，填写各 Provider 的 API Key 和 LLM Router 的 API Key
```

`.env` 文件示例：

```bash
# Provider API Keys
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
GEMINI_API_KEY=AIza...

# LLM Router API Keys（在 router.toml 中通过 key_env 引用）
LLM_ROUTER_ADMIN_KEY=admin-key-12345
LLM_ROUTER_LIMITED_KEY=limited-key-67890

# LLM Router 配置
LLM_ROUTER_MODEL_CONFIG=./router.toml
```

**方式2：直接设置环境变量**

```bash
# 指定配置文件路径
export LLM_ROUTER_MODEL_CONFIG=$(pwd)/router.toml

# 设置各 Provider 的 API Keys
export OPENAI_API_KEY="sk-..."
export ANTHROPIC_API_KEY="sk-ant-..."
export GEMINI_API_KEY="AIza..."
export DASHSCOPE_API_KEY="sk-..."  # Qwen
export GLM_API_KEY="sk-..."        # GLM
export KIMI_API_KEY="sk-..."       # Moonshot

# LLM Router 的 API Key（简单配置，向后兼容）
export LLM_ROUTER_API_KEYS="llm-router-key-1,llm-router-key-2"
# 禁用认证（不推荐）
export LLM_ROUTER_REQUIRE_AUTH=false
```

**注意**：`.env` 文件不会被提交到版本控制系统，确保敏感信息的安全。

### 4. 调用示例

**指定模型调用：**

```bash
# 如果启用了认证，需要提供 API Key（三种方式任选其一）
# 方式1: Authorization Bearer
curl -X POST http://127.0.0.1:8000/models/openai/gpt-4o/invoke \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer llm-router-key-1" \
  -d '{
    "prompt": "解释一下量子纠缠",
    "parameters": { "temperature": 0.7 }
  }'

# 方式2: X-API-Key 头
curl -X POST http://127.0.0.1:8000/models/openai/gpt-4o/invoke \
  -H "Content-Type: application/json" \
  -H "X-API-Key: llm-router-key-1" \
  -d '{
    "prompt": "解释一下量子纠缠",
    "parameters": { "temperature": 0.7 }
  }'

# 方式3: 查询参数
curl -X POST "http://127.0.0.1:8000/models/openai/gpt-4o/invoke?api_key=llm-router-key-1" \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "解释一下量子纠缠",
    "parameters": { "temperature": 0.7 }
  }'
```

**按标签自动路由：**

```bash
# 寻找支持 coding 且适合分析的模型
curl -X POST http://127.0.0.1:8000/route/invoke \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer llm-router-key-1" \
  -d '{
    "query": { "tags": ["coding", "analysis"] },
    "request": { "prompt": "写一个 Python 快速排序" }
  }'
```

## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `LLM_ROUTER_HOST` | 服务绑定的主机地址 | `0.0.0.0`（也可在 router.toml 的 `[server]` 中配置） |
| `LLM_ROUTER_PORT` | 服务绑定的端口 | `8000`（也可在 router.toml 的 `[server]` 中配置） |
| `LLM_ROUTER_DATABASE_URL` | 数据库连接字符串 | `sqlite+aiosqlite:///llm_router.db` |
| `LLM_ROUTER_MODEL_STORE` | 模型文件存储目录 | `./model_store` |
| `LLM_ROUTER_DOWNLOAD_CACHE` | 下载缓存目录 | None |
| `LLM_ROUTER_DOWNLOAD_CONCURRENCY` | 并发下载数 | `2` |
| `LLM_ROUTER_DEFAULT_TIMEOUT` | 请求超时时间（秒） | `60.0` |
| `LLM_ROUTER_LOG_LEVEL` | 日志级别 | `INFO` |
| `LLM_ROUTER_MODEL_CONFIG` | 模型配置文件路径 | None |
| `LLM_ROUTER_API_KEYS` | LLM Router API Key（多个用逗号分隔，简单配置） | None |
| `LLM_ROUTER_REQUIRE_AUTH` | 是否启用 API Key 认证 | `true`（默认开启） |

## API Key 配置

LLM Router 支持通过配置文件管理多个 API Key，每个 API Key 可以设置：

- **模型限制**：限制可以调用的模型列表
- **Provider 限制**：限制可以调用的 Provider 列表
- **参数限制**：限制调用参数（如 max_tokens、temperature 等）

### 在 router.toml 中配置 API Key（推荐）

**重要**：API Key 的具体值应该定义在 `.env` 文件中，而不是直接写在配置文件中，这样可以避免将敏感信息提交到版本控制系统。

**1. 在 `.env` 文件中定义 API Key（支持多个，逗号分隔）：**

```bash
# .env 文件
# 单个 key
LLM_ROUTER_ADMIN_KEY=admin-key-12345

# 多个 key（逗号分隔）- 会为每个 key 创建独立的配置
LLM_ROUTER_LIMITED_KEY=limited-key-1,limited-key-2,limited-key-3
LLM_ROUTER_RESTRICTED_KEY=restricted-key-abcde
```

**2. 在 `router.toml` 中引用环境变量：**

```toml
# 无限制的管理员密钥（从环境变量读取，支持多个 key）
[[api_keys]]
key_env = "LLM_ROUTER_ADMIN_KEY"  # 从 .env 文件读取，支持多个 key（逗号分隔）
name = "管理员密钥"
is_active = true

# 限制只能调用特定模型的密钥
[[api_keys]]
key_env = "LLM_ROUTER_LIMITED_KEY"
name = "受限密钥 - 仅 GPT 模型"
is_active = true
allowed_models = [
    "openai/gpt-5.1",
    "openai/gpt-5-pro",
]

# 限制参数的密钥
[[api_keys]]
key_env = "LLM_ROUTER_RESTRICTED_KEY"
name = "受限参数密钥"
is_active = true
allowed_models = ["openai/gpt-5.1", "claude/claude-4.5-sonnet"]
[api_keys.parameter_limits]
max_tokens = 2000
temperature = 0.7
top_p = 0.9
```

**注意**：也可以直接使用 `key` 字段指定 API Key（不推荐用于生产环境，仅用于测试）：
```toml
[[api_keys]]
key = "test-key-direct"  # 直接指定（不推荐）
name = "测试密钥"
```

### API Key 使用方式

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

### 模型和参数限制

- 如果 API Key 配置了 `allowed_models`，只能调用列表中的模型
- 如果 API Key 配置了 `allowed_providers`，只能调用指定 Provider 的模型
- 如果 API Key 配置了 `parameter_limits`，参数会被自动限制在允许范围内
- 如果限制为 `null` 或未设置，表示无限制

## 数据库与模型存储

默认使用 SQLite (`llm_router.db`) 和本地目录 (`model_store/`)。可自定义：

```bash
export LLM_ROUTER_DATABASE_URL="sqlite+aiosqlite:////path/to/db.sqlite"
export LLM_ROUTER_MODEL_STORE="/path/to/models"
```

## 支持的 Provider 类型

| Type | 说明 |
| :--- | :--- |
| `openai` | OpenAI 及兼容接口 (DeepSeek, Grok, Moonshot 等) |
| `anthropic` / `claude` | Anthropic Claude 系列 |
| `gemini` | Google Gemini 系列 |
| `glm` | 智谱 GLM |
| `qwen` | 阿里通义千问 Qwen |
| `kimi` | Moonshot Kimi |
| `openrouter` | OpenRouter 聚合接口 |
| `ollama` | 本地 Ollama 服务 |
| `vllm` | 本地 vLLM 服务 |
| `transformers` | 本地 HuggingFace Transformers |

## API 文档

详细的 API 文档请参考 [API.md](API.md)。

## 开发与测试

```bash
# 运行测试
uv run pytest
```
