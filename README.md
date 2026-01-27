# LLM Router

统一的 LLM 路由服务，支持多厂商 API（OpenAI, Gemini, Claude, GLM, Qwen, Kimi, OpenRouter 等）及本地模型（Ollama, vLLM, Transformers），提供统一的 REST 接口、灵活的标签路由策略、完整的调用监控和细粒度的 API Key 访问控制。

## 功能特性

### 核心功能

- **多供应商支持 (Multi-Provider Support)**：连接到各种 LLM 供应商，包括 OpenAI, Gemini, Claude, GLM, Qwen, Kimi, OpenRouter, xAI (Grok), DeepSeek 等。
- **模型管理 (Model Management)**：使用标签、速率限制和自定义设置注册、配置和管理模型（详见 [TAGS.md](TAGS.md)）。
- **智能路由 (Intelligent Routing)**：根据任务类型（标签）、供应商类型和其他标准自动选择最佳模型。
- **OpenAI API 兼容 (OpenAI API Compatibility)**：提供标准的 OpenAI 兼容端点（`/v1/chat/completions`, `/v1/models`），支持无缝替换 OpenAI SDK。
- **统一接口**：屏蔽各厂商 API 差异，通过统一的 REST 接口调用所有模型。
- **灵活配置**：通过 TOML 文件管理所有 Provider、模型及标签，支持热加载。
- **多源支持**：
  - **远程 API**：OpenAI, Gemini, Claude, Grok, DeepSeek, Qwen, Kimi, GLM, OpenRouter 等。
  - **本地运行**：Ollama, vLLM, Transformers (HuggingFace)。

### 高级功能

- **访问控制**：细粒度的 API Key 认证系统，支持模型限制、Provider 限制和参数限制。
- **调用监控**：完整的调用历史记录、统计分析和实时监控界面。
- **限流控制**：支持按模型配置限流策略。
- **多模态支持**：支持视觉、音频、视频等模型能力标记。
- **SQLite 后端**：使用 SQLite 持久化存储 Provider、模型配置及调用记录。

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

### 4. 运行测试

```bash
uv run pytest
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
```

**支持的 Provider 类型**：
- `openai`: OpenAI API
- `claude`: Anthropic Claude API
- `gemini`: Google Gemini API
- `deepseek`: DeepSeek API（OpenAI 兼容）
- `glm`: 智谱 AI GLM（国内版，质谱轻言）
- `glm-z`: 智谱 AI GLM（国际版，z.ai）- 使用 `type = "glm"` 但配置不同的 name 和 API Key
- `qwen`: 阿里云通义千问
- `kimi`: 月之暗面 Kimi
- `openrouter`: OpenRouter
- `ollama`: 本地 Ollama
- `vllm`: 本地 vLLM
- `transformers`: 本地 Transformers

**DeepSeek 配置示例**：
```toml
[[providers]]
name = "deepseek"
type = "deepseek"
api_key_env = "DEEPSEEK_API_KEY"
base_url = "https://api.deepseek.com"
```

**GLM 双版本配置示例**：
```toml
# 国内版（质谱轻言）
[[providers]]
name = "glm"
type = "glm"
api_key_env = "GLM_API_KEY"
base_url = "https://open.bigmodel.cn/api/paas/v4"
[providers.settings]
endpoint = "/v4/chat/completions"

# 国际版（z.ai）
[[providers]]
name = "glm-z"
type = "glm"
api_key_env = "GLM_Z_API_KEY"
base_url = "https://open.bigmodel.cn/api/paas/v4"
[providers.settings]
endpoint = "/v4/chat/completions"
```

#### 模型配置

```toml
[[models]]
name = "gpt-4o"              # 模型名称
provider = "openai"         # 所属 Provider
display_name = "GPT-4o"     # 显示名称
tags = ["chat", "general", "high-quality", "vision"]  # 标签列表
is_active = true            # 是否启用

# 限流配置（可选）
[models.rate_limit]
max_requests = 50           # 最大请求数
per_seconds = 60            # 时间窗口（秒）

# 模型能力配置
[models.config]
context_window = "128k"     # 上下文窗口
supports_vision = true      # 支持视觉
supports_tools = true       # 支持工具调用
```

**当前配置的模型列表**（根据 `router.toml` 自动生成）：

- **OpenAI**: GPT-5.1, GPT-5 Pro
- **Claude**: Claude 4.5 Haiku, Claude 4.5 Sonnet
- **Gemini**: Gemini 2.5 Flash, Gemini 2.5 Pro, Gemini 3.0 Pro
- **GLM**: GLM-4 Air, GLM-4 AirX, GLM-4 Assistant, GLM-4 FlashX, GLM-4 Long, GLM-4 Plus, GLM-4.5, GLM-4.5 Air, GLM-4.5 AirX, GLM-4.5 Flash, GLM-4.5-X, GLM-4.6, GLM-4.6 Flash, GLM-4.6 Plus, GLM-4.7
- **Qwen**: Qwen Turbo, Qwen2.5 72B Instruct
- **Kimi**: Kimi K2 128K, Kimi K2 Flash
- **OpenRouter**: 包含多个免费和付费模型，如 AllenAI: Molmo2 8B、Arcee AI Trinity Mini、DeepSeek R1 系列、Gemini 系列、Llama 系列、Mistral 系列、NVIDIA Nemotron 系列、Qwen 系列、TNG 系列等（详见 `router.toml`）
- **Ollama**: GPT-OSS 20B (Ollama)
- **Vercel**: Gemini 2.5 Flash (Vercel)

完整模型列表请查看 `router.toml` 配置文件。

## API 使用示例

### OpenAI 兼容 API (推荐)

设置 base URL 为 `http://localhost:8000/v1` 即可使用任何 OpenAI 兼容客户端。

**cURL 示例:**

```bash
curl -X POST http://localhost:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "openai/gpt-5.1",
    "messages": [
      {"role": "user", "content": "Hello, how are you?"}
    ]
  }'
```

**Python SDK 示例:**

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8000/v1",
    api_key="your-api-key"
)

response = client.chat.completions.create(
    model="openai/gpt-5.1",
    messages=[{"role": "user", "content": "Hello!"}]
)
print(response.choices[0].message.content)
```

### 智能路由

根据标签自动选择模型：

```bash
curl -X POST http://localhost:8000/route/invoke \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "query": {
      "tags": ["coding", "high-quality"]
    },
    "request": {
      "prompt": "用 Python 写一个快速排序算法"
    }
  }'
```

## 认证系统

LLM Router 支持基于 API Key 的认证。本机请求（localhost）默认免认证，远程请求需要在 Header 中包含 `Authorization: Bearer <API_KEY>` 或 `X-API-Key: <API_KEY>`。

详细认证说明请参考 [API.md](docs/API.md)。

## 许可证

[根据项目实际情况填写]

## 贡献

欢迎提交 Issue 和 Pull Request！
