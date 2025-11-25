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

```bash
# 指定配置文件路径
export LLM_ROUTER_MODEL_CONFIG=$(pwd)/router.toml

# 设置 API Keys
export OPENAI_API_KEY="sk-..."
export ANTHROPIC_API_KEY="sk-ant-..."
export GEMINI_API_KEY="AIza..."
export DASHSCOPE_API_KEY="sk-..."  # Qwen
export GLM_API_KEY="sk-..."        # GLM
export KIMI_API_KEY="sk-..."       # Moonshot
```

### 4. 调用示例

**指定模型调用：**

```bash
curl -X POST http://127.0.0.1:8000/models/openai/gpt-4o/invoke \
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
  -d '{
    "query": { "tags": ["coding", "analysis"] },
    "request": { "prompt": "写一个 Python 快速排序" }
  }'
```

## 环境变量

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
