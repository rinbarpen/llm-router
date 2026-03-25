# LLM Router

统一的 LLM 路由服务，支持多厂商 API（OpenAI, Gemini, Claude, GLM, Qwen, Kimi, OpenRouter 等）及本地模型（Ollama, vLLM, Transformers），提供统一的 REST 接口、灵活的标签路由策略、完整的调用监控和细粒度的 API Key 访问控制。

## 功能特性

### 核心功能

- **多供应商支持 (Multi-Provider Support)**：连接到各种 LLM 供应商，包括 OpenAI, Gemini, Claude, GLM, Qwen, Kimi, OpenRouter, xAI (Grok), DeepSeek 等。
- **模型管理 (Model Management)**：使用标签、速率限制和自定义设置注册、配置和管理模型（详见 [docs/TAGS.md](docs/TAGS.md)）。
- **智能路由 (Intelligent Routing)**：根据任务类型（标签）、供应商类型和其他标准自动选择最佳模型。
- **OpenAI API 兼容 (OpenAI API Compatibility)**：提供标准的 OpenAI 兼容端点（`/v1/chat/completions`、`/{provider}/v1/chat/completions`、`/v1/models`），支持无缝替换 OpenAI SDK。
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
- **PostgreSQL 后端**：使用 PostgreSQL 持久化存储 Provider、模型配置及调用记录，并支持从 SQLite 一次性导入。

## 快速开始

### 1. 安装依赖

后端基于 Go：

```bash
# 克隆或下载项目
cd llm-router

# 初始化并安装依赖
go mod download
```

如果没有安装 Go（建议 1.24+），请先安装：
```bash
go version
```

### 2. 配置文件

编辑 `router.toml` 文件，配置 Provider 和模型。

创建 `.env` 文件（如果不存在），填入各 Provider 的 API Key：

```bash
# 从模板文件复制 .env 文件
cp .env.example .env
```

编辑 `.env` 文件，填入各 Provider 的 API Key。

### 3. 启动服务

#### 一键启动前后端（本地开发推荐）

```bash
./scripts/start.sh
```

也可以按模式单独启动：

```bash
./scripts/start.sh backend
./scripts/start.sh monitor
```

`./scripts/start.sh` 使用 Go 后端，并在启动前检查 PostgreSQL 可达性。  
脚本会检查 `go`、`curl`、`npm` 和监控界面依赖是否已安装；不会自动执行安装。缺少依赖时，请先运行 `go mod download` 或 `cd examples/monitor && npm install`。

#### 启动后端

```bash
# 使用 Go（推荐）
go run ./cmd/llm-router
```

服务将根据 `router.toml` 中的 `[server]` 配置启动。如果 `router.toml` 存在于项目根目录，系统会自动读取其中的端口配置。

**配置优先级**：环境变量 > router.toml > 默认值（8000）

验证服务运行：
```bash
# 根据 router.toml 中的配置访问（如配置了 18000 端口）
curl http://localhost:18000/health

# 或默认端口
curl http://localhost:18000/health
```

**注意**：健康检查端点无需认证，本机请求也无需认证。

#### 启动监控界面（可选）

```bash
cd examples/monitor
npm install  # 首次运行需要
npm run dev
```

访问 `http://localhost:3000`（或配置的端口）查看监控界面。

**详细启动说明请参考 [QUICKSTART.md](docs/QUICKSTART.md)**

### 4. 运行测试

```bash
go test ./...
```

默认运行仓库内 Go 测试（`cmd/`、`src/`）。

说明：`examples/`、`scripts/` 下脚本用于手工验证与运维，不纳入核心 `go test` 回归。

## 配置文件详解

### 模型定价多来源更新（常用模型 + 免费模型）

后端提供以下定价接口：
- `GET /pricing/latest`
- `GET /pricing/suggestions`
- `POST /pricing/sync/{model_id}`
- `POST /pricing/sync-all`

默认内置了常用模型及免费模型目录（`openai/claude/gemini/deepseek/qwen/kimi/glm/groq`），并支持通过环境变量挂接远程来源，远程失败时自动回退内置目录。

```bash
# provider -> pricing source URL(JSON)
# URL 响应可为 list 或 {"models":[...]} / {"data":[...]}
# 每条记录至少包含模型名 + 输入/输出价格：
# model_name|name|model|id + input_price_per_1k/output_price_per_1k
export LLM_ROUTER_PRICING_SOURCE_URLS='{
  "openai":"https://example.com/openai-pricing.json",
  "claude":"https://example.com/claude-pricing.json",
  "gemini":"https://example.com/gemini-pricing.json",
  "groq":"https://example.com/groq-pricing.json",
  "qwen":"https://example.com/qwen-pricing.json",
  "deepseek":"https://example.com/deepseek-pricing.json",
  "kimi":"https://example.com/kimi-pricing.json",
  "glm":"https://example.com/glm-pricing.json"
}'
```

远程记录支持 `unit=per_token`（会自动换算为每 1k token 价格）；当输入和输出都为 `0` 时，会自动标记为免费模型/免费层。

也支持本地文件来源（`file://` 或绝对路径）：

```bash
export LLM_ROUTER_PRICING_SOURCE_URLS='{
  "openai":"file:///abs/path/pricing/openai.json",
  "qwen":"/abs/path/pricing/qwen.json"
}'
```

### router.toml 结构

配置文件采用 TOML 格式，包含以下部分：

#### 服务器配置

```toml
[server]
host = "0.0.0.0"  # 可选，默认 0.0.0.0
port = 8000       # 可选，默认 8000
```

#### 监控界面配置

```toml
[monitor]
port = 3000                    # 监控界面开发服务器端口
api_url = "http://localhost:18000"  # 后端API地址（开发环境代理用）
api_base_url = "/api"          # 生产环境API基础路径
```

#### 路由对配置

```toml
[routing]
default_pair = "gemini-3"  # 默认使用的路由对

[[routing.pairs]]
name = "gemini-3"
strong_model = "gemini/gemini-3.0-pro"   # 强模型（复杂任务）
weak_model = "gemini/gemini-3.0-flash"   # 弱模型（简单/快速任务）

[[routing.pairs]]
name = "gemini-2.5"
strong_model = "gemini/gemini-2.5-pro"
weak_model = "gemini/gemini-2.5-flash"
```

通过 `POST /route` 接口使用路由对，系统根据 `role` 和 `task` 自动选择强/弱模型。

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

远程 API：
- `openai`: OpenAI API
- `azure_openai`: Azure OpenAI
- `claude`: Anthropic Claude API（`/v1/messages`、count_tokens、batches），需 API Key
- `gemini`: Google Gemini API
- `deepseek`: DeepSeek API（OpenAI 兼容）
- `glm`: 智谱 AI GLM（国内版/国际版，通过 `base_url` 区分）
- `qwen`: 阿里云通义千问
- `kimi`: 月之暗面 Kimi
- `doubao`: 字节跳动豆包
- `minimax`: MiniMax
- `openrouter`: OpenRouter
- `groq`: Groq
- `siliconflow`: 硅基流动
- `aihubmix`: AiHubMix API 网关
- `volcengine`: 火山引擎
- `huggingface`: Hugging Face Inference API

本地 CLI（使用本机登录态，无需 API Key）：
- `codex_cli`: Codex CLI / OpenAI Responses API（`/v1/responses`）
- `opencode_cli`: OpenCode CLI（`/v1/responses`）
- `kimi_code_cli`: Kimi Code CLI（`/v1/responses`）
- `qwen_code_cli`: Qwen Code CLI（`/v1/responses`）
- `claude_code_cli`: Claude Code CLI（`/v1/messages`）

**本地 Code CLI 项目级权限设置（推荐）**

```toml
[[providers]]
name = "claude_code_cli"
type = "claude_code_cli"
[providers.settings]
executable = "claude"
permission_mode = "bypassPermissions"
workspace_root = "/abs/path/to/your/project"
default_workspace_path = "/abs/path/to/your/project"
enforce_workspace_scope = true
```

说明：
- `workspace_root`：权限边界根目录（必须绝对路径）。
- `default_workspace_path`：未显式传 `workspace_path` 时使用的目录。
- 若未配置 `default_workspace_path`，默认使用 `.llmrouter/workdir/{project_id}`（`project_id` 基于项目绝对路径稳定计算）。
- `enforce_workspace_scope=true`：当请求 `workspace_path` 超出 `workspace_root` 时直接拒绝。
- `codex_cli` 额外支持：`approval_policy`、`sandbox_mode`、`network_access`（app-server 路径）。

本地推理服务：
- `ollama`: 本地 Ollama
- `vllm`: 本地 vLLM
- `transformers`: 本地 Transformers

**DeepSeek 配置示例**：
```toml
[[providers]]
name = "deepseek (cn)"
type = "deepseek"
api_key_env = "DEEPSEEK_API_KEY"
base_url = "https://api.deepseek.com"
```

**GLM 双版本配置示例**：
```toml
# 国内版（质谱轻言）
[[providers]]
name = "glm (cn)"
type = "glm"
api_key_env = "GLM_API_KEY"
base_url = "https://open.bigmodel.cn/api/paas/v4"
[providers.settings]
endpoint = "/chat/completions"

# 国际版（z.ai）
[[providers]]
name = "glm (global)"
type = "glm"
api_key_env = "GLM_API_KEY"
base_url = "https://api.z.ai/api/paas/v4"
[providers.settings]
endpoint = "/chat/completions"
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

- **claude_code_cli（本地 CLI）**: Claude Default, Claude Sonnet, Claude Opus, Claude Haiku, Claude Sonnet 1M Context, Claude OpusPlan, 以及各固定版本（claude-opus-4-6, claude-sonnet-4-6, claude-haiku-4-5 等）
- **codex_cli（本地 CLI）**: GPT-5.3 Codex
- **opencode_cli（本地 CLI）**: OpenCode Default
- **kimi_code_cli（本地 CLI）**: Kimi Code Default
- **qwen_code_cli（本地 CLI）**: Qwen Code Default
- **openai**: GPT-5.2, GPT-5.2 Pro, GPT-5 Mini, GPT-5 Nano, GPT-5.1, GPT-5 Pro
- **claude**: Claude Opus 4.6, Claude Opus 4.5, Claude 4.5 Sonnet, Claude 4.5 Haiku
- **gemini**: Gemini 2.5 Flash, Gemini 2.5 Pro, Gemini 3.0 Pro, Gemini 3.0 Flash
- **glm**: GLM-5, GLM-4.7
- **qwen**: Qwen Plus, Qwen Max, Qwen2.5 72B Instruct, Qwen Turbo, Qwen3 TTS 系列
- **kimi**: Kimi Latest, Kimi K2 128K, Kimi K2 Flash
- **deepseek**: DeepSeek Chat, DeepSeek Reasoner
- **groq**: Llama 3.1 8B Instant, Llama 3.3 70B Versatile, GPT OSS 120B, GPT OSS 20B
- **siliconflow**: DeepSeek V3, Qwen2 7B Instruct
- **aihubmix**: GPT-4o Mini, GPT-4o
- **volcengine**: 豆包 Pro 32K
- **ollama**: GPT-OSS 20B
- **vllm**: vLLM 默认模型
- **openrouter**: OpenRouter Auto Free, Meta Llama 3.3 70B, Z.AI GLM 4.5 Air, Xiaomi MIMO V2 Flash, DeepSeek R1 系列, Qwen3 系列, Gemini 系列, NVIDIA Nemotron 系列, Mistral 系列等（含大量免费模型）
- **vercel**: Gemini 2.5 Flash (Vercel)

完整模型列表请查看 `router.toml` 配置文件。

## API 使用示例

### OpenAI 兼容 API (推荐)

设置 base URL 为 `http://localhost:18000/v1` 即可使用任何 OpenAI 兼容客户端。

**cURL 示例:**

```bash
# 方式 1：Provider 在路径中，model 只需模型名
curl -X POST http://localhost:18000/openrouter/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "glm-4.5-air", "messages": [{"role": "user", "content": "Hello"}]}'

# 方式 2：标准端点，model 为 provider/model
curl -X POST http://localhost:18000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "openai/gpt-5.1",
    "messages": [{"role": "user", "content": "Hello, how are you?"}]
  }'
```

**JavaScript SDK 示例:**

```javascript
import OpenAI from "openai";

const client = new OpenAI({
  baseURL: "http://localhost:18000/v1",
  apiKey: "your-api-key",
});

const response = await client.chat.completions.create({
  model: "openai/gpt-5.1",
  messages: [{ role: "user", content: "Hello!" }],
});
console.log(response.choices[0]?.message?.content);
```

### 智能路由

根据标签自动选择模型：

```bash
curl -X POST http://localhost:18000/route/invoke \
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



### 轻量路由决策（给上游 Agent 系统）

如果上游系统只需要“选模型”而不希望由 llm-router 代调，可使用 `POST /route`：

```bash
curl -X POST http://localhost:18000/route \
  -H "Content-Type: application/json" \
  -d '{
    "role": "planner",
    "task": "worker",
    "trace_id": "trace-123",
    "model_hint": "openrouter/gpt-4o"
  }'
```

示例响应：

```json
{
  "model": "openrouter/gpt-4o",
  "base_url": "https://openrouter.ai/api/v1",
  "api_key": "sk-***",
  "temperature": 0.2,
  "max_tokens": 1024,
  "provider": "openrouter"
}
```

## 认证系统

LLM Router 支持基于 API Key 的认证。本机请求（localhost）默认免认证，远程请求需要在 Header 中包含 `Authorization: Bearer <API_KEY>` 或 `X-API-Key: <API_KEY>`。

详细认证说明请参考 [API.md](docs/API.md)。

## 许可证

本项目采用 [MIT License](LICENSE) 许可证。

Copyright (c) 2025 rinbarpen

## 贡献

欢迎提交 Issue 和 Pull Request！
