# LLM Router API Documentation

The LLM Router provides a RESTful API for managing LLM providers, models, and routing requests across multiple AI services.

## Base URL

All API endpoints are relative to `http://localhost:18000` (or your configured host and port).

## Authentication

### Authentication Strategy

The LLM Router implements a source-based authentication strategy:

1. **Local Requests (localhost/127.0.0.1)**:
   - ✅ **No authentication required** - Can access all endpoints directly
   - If authentication information is provided, permission restrictions (model limits, parameter limits, etc.) will still be applied
   - Suitable for local development and testing

2. **Remote Requests (other sources)**:
   - ❌ **Authentication required** (if authentication is enabled)
   - Must login to get a Session Token, or use API Key directly
   - Suitable for production environments and remote access

### Authentication Methods

#### Recommended: Session Token (Login First)

**Step 1: Login to get Session Token**

```bash
POST /auth/login
```

**Request Body:**
```json
{
  "api_key": "your-api-key"
}
```

Or use Authorization header:
```bash
POST /auth/login
Authorization: Bearer your-api-key
```

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_in": 86400,
  "message": "登录成功，请使用此 token 进行后续请求。使用 /auth/bind-model 绑定模型。"
}
```

**Step 2: Bind Model to Session (Optional but Recommended)**

Bind a model to your session token for easier use with OpenAI-compatible API:

```bash
POST /auth/bind-model
Authorization: Bearer <session-token>
```

**Request Body:**
```json
{
  "provider_name": "openai",
  "model_name": "gpt-5.1"
}
```

**Response:**
```json
{
  "message": "模型 openai/gpt-5.1 已绑定到 session",
  "provider_name": "openai",
  "model_name": "gpt-5.1"
}
```

**Step 3: Use Session Token in requests**

Use the token in one of the following ways:

1. **Authorization Bearer** (recommended):
   ```
   Authorization: Bearer <session-token>
   ```

2. **X-Session-Token header**:
   ```
   X-Session-Token: <session-token>
   ```

3. **Query parameter**:
   ```
   ?session_token=<session-token>
   ```

**Step 4: Logout (optional)**

```bash
POST /auth/logout
Authorization: Bearer <session-token>
```

#### Alternative: Direct API Key (Backward Compatible)

You can still use API Key directly (not recommended, lower security):

1. **Authorization Bearer**:
   ```
   Authorization: Bearer <api-key>
   ```

2. **X-API-Key header**:
   ```
   X-API-Key: <api-key>
   ```

3. **Query parameter**:
   ```
   ?api_key=<api-key>
   ```

### Public Endpoints

The following endpoints do not require authentication:
- `GET /health`
- `POST /auth/login`

### API Key Restrictions

Each API Key can have the following restrictions:
- **Model restrictions**: `allowed_models` - Can only call specified models
- **Provider restrictions**: `allowed_providers` - Can only call specified providers
- **Parameter limits**: `parameter_limits` - Automatically limit call parameters (e.g., max_tokens, temperature)
- **Expiration**: `expires_at` - API Key expiration timestamp (UTC)
- **Monthly quota**: `quota_tokens_monthly` - monthly token quota
- **IP allowlist**: `ip_allowlist` - supports single IP and CIDR

If restrictions are `null` or not set, there are no limits.

## Common Response Format

Most API endpoints return JSON responses with appropriate HTTP status codes.

## Endpoints

### Newly Added Compatibility Endpoints

- `POST /{provider}/v1/chat/completions`: OpenAI 兼容 chat completions，provider 在路径中，model 只需传模型名。
- `GET /route/pairs`: 获取配置的 strong/weak 模型对列表（来自 `router.toml` 的 `[[routing.pairs]]`）。
- `POST /v1/responses`: OpenAI Responses-compatible endpoint (for Codex CLI style calls).
- `POST /v1/messages/count_tokens`: Claude native token counting endpoint.
- `POST /v1/messages/batches`: Create Claude messages batch job.
- `GET /v1/messages/batches/{batch_id}`: Query Claude messages batch job.
- `POST /v1/messages/batches/{batch_id}/cancel`: Cancel Claude messages batch job.

### Stream 能力矩阵（Go Backend）

下表以当前 Go 后端实现为准（`src/api/routes.go` + `src/services/model_service.go`）：

| Endpoint | `stream` 参数 | 流式返回 | 备注 |
|---|---|---|---|
| `POST /v1/chat/completions` | 支持 | 支持（SSE） | `stream=true` 时透传上游流 |
| `POST /{provider}/v1/chat/completions` | 支持 | 支持（SSE） | 与标准端点一致 |
| `POST /v1/responses` | 支持 | 支持（SSE） | 内部映射到 chat completions stream |
| `POST /v1beta/models/{model}:streamGenerateContent` | N/A（流式端点） | 支持（SSE） | Gemini 兼容流式端点 |
| `POST /v1beta/models/{model}:generateContent` | 不支持 | 不支持 | 非流式 |
| `POST /v1/messages` | 不支持 | 不支持 | Claude messages 当前为非流式 |
| `POST /route/invoke` | 当前不支持 | 不支持 | 走非流式调用路径 |
| `POST /v1/audio/speech` | 不支持 | 不支持 | 返回完整音频文件 |
| `POST /v1/audio/transcriptions` | 不支持 | 不支持 | 非流式 |
| `POST /v1/audio/translations` | 不支持 | 不支持 | 非流式 |
| `POST /v1/images/generations` | 不支持 | 不支持 | 非流式 |
| `POST /v1/videos/generations` | 不支持 | 不支持 | 任务型接口，结果轮询 |

流式稳定性（当前实现）：
- 建连重试：流式连接建立失败时会做短退避重试（仅针对可重试错误）。
- 空闲超时：SSE 转发链路存在空闲读取超时保护，避免连接无响应长期悬挂。
- 客户端取消：客户端断连/请求取消会向上游传播取消信号并终止转发。

### Health Check

#### GET `/health`

Check if the service is running and healthy.

**Authentication:** Not required

**Response:**
```json
{
  "status": "ok"
}
```

**Example Usage:**

**Client Example:**
```text
from curl_cffi import requests

response = requests.get("http://localhost:18000/health")
print(response.json())  # {"status": "ok"}
```

**JavaScript:**
```javascript
const response = await fetch('http://localhost:18000/health');
const data = await response.json();
console.log(data);  // {status: "ok"}
```

**curl:**
```bash
curl http://localhost:18000/health
```

---

### Authentication

#### POST `/auth/login`

Login with API Key to get a Session Token.

**Authentication:** Not required (this is the login endpoint)

**Request Body:**
```json
{
  "api_key": "your-api-key"
}
```

**Alternative:** You can also provide the API Key via Authorization header:
```
Authorization: Bearer your-api-key
```

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_in": 86400,
  "message": "登录成功，请使用此 token 进行后续请求。使用 /auth/bind-model 绑定模型。"
}
```

**Error Responses:**
- `401 Unauthorized`: API Key not provided
- `403 Forbidden`: Invalid API Key

**Example Usage:**

**Client Example:**
```text
from curl_cffi import requests

# 方式 1: 使用请求体
response = requests.post(
    "http://localhost:18000/auth/login",
    json={"api_key": "your-api-key"}
)
data = response.json()
token = data["token"]

# 方式 2: 使用 Authorization header
response = requests.post(
    "http://localhost:18000/auth/login",
    headers={"Authorization": "Bearer your-api-key"}
)
```

**JavaScript:**
```javascript
// 方式 1: 使用请求体
const response = await fetch('http://localhost:18000/auth/login', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({api_key: 'your-api-key'})
});
const data = await response.json();
const token = data.token;

// 方式 2: 使用 Authorization header
const response2 = await fetch('http://localhost:18000/auth/login', {
    method: 'POST',
    headers: {'Authorization': 'Bearer your-api-key'}
});
```

**curl:**
```bash
# 方式 1: 使用请求体
curl -X POST http://localhost:18000/auth/login \
  -H "Content-Type: application/json" \
  -d '{"api_key": "your-api-key"}'

# 方式 2: 使用 Authorization header
curl -X POST http://localhost:18000/auth/login \
  -H "Authorization: Bearer your-api-key"
```

---

#### POST `/auth/bind-model`

Bind a model to your session token. This allows you to use the OpenAI-compatible API without specifying the model in each request.

**Authentication:** Required (Session Token or API Key)

**Request Headers:**
```
Authorization: Bearer <session-token>
```

**Request Body:**
```json
{
  "provider_name": "openai",
  "model_name": "gpt-5.1"
}
```

**Response:**
```json
{
  "message": "模型 openai/gpt-5.1 已绑定到 session",
  "provider_name": "openai",
  "model_name": "gpt-5.1"
}
```

**Error Responses:**
- `400 Bad Request`: Missing provider_name or model_name, or model is inactive
- `401 Unauthorized`: Session Token not provided
- `403 Forbidden`: API Key does not have permission to access the model
- `404 Not Found`: Model not found, or session not found/expired

**Note:** You can also bind a model automatically by specifying it in the `/v1/chat/completions` request. The model will be automatically bound to your session for future requests.

---

#### POST `/auth/logout`

Logout and invalidate the Session Token.

**Authentication:** Required (Session Token or API Key)

**Request Headers:**
```
Authorization: Bearer <session-token>
```

**Response:**
```json
{
  "message": "登出成功"
}
```

**Error Responses:**
- `401 Unauthorized`: Session Token not provided
- `404 Not Found`: Session not found or already expired

---

### Provider Management

#### GET `/providers`

List all configured providers.

**Authentication:** Required for remote requests (optional for local requests)

**Response:**
```json
[
  {
    "id": 1,
    "name": "openai",
    "type": "openai",
    "is_active": true,
    "base_url": "https://api.openai.com"
  },
  {
    "id": 2,
    "name": "gemini",
    "type": "gemini",
    "is_active": true,
    "base_url": null
  }
]
```

#### POST `/providers`

Create or update a provider.

**Authentication:** Required for remote requests (optional for local requests)

**Request Body:**
```json
{
  "name": "openai",
  "type": "openai",
  "base_url": "https://api.openai.com",
  "api_key": "sk-...",
  "is_active": true,
  "settings": {}
}
```

**Parameters:**

- `name` (string, required): Unique identifier for the provider
- `type` (string, required): Provider type (one of: "openai", "codex_cli", "opencode_cli", "kimi_code_cli", "qwen_code_cli", "claude_code_cli", "gemini", "claude", "openrouter", "glm", "kimi", "qwen")
- `base_url` (string, optional): Base URL for the provider API
- `api_key` (string, optional): API key for the provider
- `is_active` (boolean, default: true): Whether the provider is active
- `settings` (object, optional): Additional provider-specific settings

**Response:**
```json
{
  "id": 1,
  "name": "openai",
  "type": "openai",
  "is_active": true,
  "base_url": "https://api.openai.com"
}
```

#### GET `/providers/{provider_name}/supported-models`

List model IDs reported by the upstream provider when live discovery is supported.

**Authentication:** Required for remote requests (optional for local requests)

---

#### GET `/providers/{provider_name}/remote-models`

List provider-native models with metadata. This is the upstream provider catalog, not the llm-router local `models` table.

Query parameters:

- `refresh` (boolean, optional): force a fresh provider request instead of using the short-lived runtime cache.

#### POST `/providers/{provider_name}/models/sync`

Synchronize one provider's upstream models into the local database. New models are enabled by default, existing manual fields are preserved, and auto-managed models missing upstream are disabled.

Request body:

```json
{"default_new_model_active": true}
```

#### POST `/providers/models/sync`

Synchronize all providers that support live model discovery.

#### GET `/model-updates/runs`

Return recent provider model sync runs.

### Model Management

#### GET `/models`

List all available models with optional filtering.

**Authentication:** Required for remote requests (optional for local requests)

**Query Parameters:**

- `tag` or `tags` (string, optional): Filter models by tags (repeated parameter or comma-separated values)
- `provider_type` or `provider_types` (string, optional): Filter models by provider type (repeated parameter or comma-separated values)
- `include_inactive` (boolean, default: false): Include inactive models in the response

**Example:**
```
GET /models?tags=chat,general&provider_types=openai,gemini&include_inactive=true
```

**Example Usage:**

**Client Example:**
```text
from curl_cffi import requests

# 获取所有模型
response = requests.get("http://localhost:18000/models")
models = response.json()

# 按标签过滤
response = requests.get(
    "http://localhost:18000/models",
    params={"tags": "free,chinese"}
)

# 按 Provider 类型过滤
response = requests.get(
    "http://localhost:18000/models",
    params={"provider_types": "openrouter"}
)
```

**JavaScript:**
```javascript
// 获取所有模型
const response = await fetch('http://localhost:18000/models');
const models = await response.json();

// 按标签过滤
const params = new URLSearchParams({tags: 'free,chinese'});
const response2 = await fetch(`http://localhost:18000/models?${params}`);
```

**curl:**
```bash
# 获取所有模型
curl http://localhost:18000/models

# 按标签过滤
curl "http://localhost:18000/models?tags=free,chinese"

# 按 Provider 类型过滤
curl "http://localhost:18000/models?provider_types=openrouter"
```

**Response:**
```json
[
  {
    "id": 1,
    "name": "gpt-5.1",
    "display_name": "GPT-5.1",
    "description": null,
    "provider_id": 1,
    "provider_name": "openai",
    "provider_type": "openai",
    "tags": ["chat", "general", "image", "audio", "reasoning"],
    "default_params": {},
    "config": {
      "context_window": "128k",
      "supports_vision": true,
      "supports_tools": true,
      "languages": ["en"],
      "cost_per_1k_tokens": 0.0005,
      "cost_per_1k_completion_tokens": 0.003
    },
    "rate_limit": {
      "max_requests": 50,
      "per_seconds": 60,
      "burst_size": null,
      "notes": null,
      "config": {}
    },
    "local_path": null
  }
]
```

#### POST `/models`

Register a new model.

**Authentication:** Required for remote requests (optional for local requests)

**Request Body:**
```json
{
  "name": "gpt-5.1",
  "provider_name": "openai",
  "display_name": "GPT-5.1",
  "description": "OpenAI's most advanced model",
  "remote_identifier": null,
  "is_active": true,
  "tags": ["chat", "general", "image"],
  "default_params": {},
  "config": {
    "context_window": "128k",
    "supports_vision": true
  },
  "download_uri": null,
  "local_path": null,
  "rate_limit": {
    "max_requests": 50,
    "per_seconds": 60
  }
}
```

**Parameters:**

- `name` (string, required): Model name identifier
- `provider_id` (integer, optional): ID of the provider (alternative to provider_name)
- `provider_name` (string, optional): Name of the provider (alternative to provider_id)
- `display_name` (string, optional): Display name for the model
- `description` (string, optional): Model description
- `remote_identifier` (string, optional): Remote identifier for the model (if different from name)
- `is_active` (boolean, default: true): Whether the model is active
- `tags` (array of strings, optional): Tags for the model
- `default_params` (object, optional): Default parameters for model calls
- `config` (object, optional): Provider-specific configuration
- `download_uri` (string, optional): URI for downloading the model
- `local_path` (string, optional): Local path where the model is stored
- `rate_limit` (object, optional): Rate limiting configuration

**Response:**
```json
{
  "id": 1,
  "name": "gpt-5.1",
  "display_name": "GPT-5.1",
  "description": "OpenAI's most advanced model",
  "provider_id": 1,
  "provider_name": "openai",
  "provider_type": "openai",
  "tags": ["chat", "general", "image"],
  "default_params": {},
  "config": {
    "context_window": "128k",
    "supports_vision": true
  },
  "rate_limit": {
    "max_requests": 50,
    "per_seconds": 60
  },
  "local_path": null
}
```

#### PATCH `/models/{provider_name}/{model_name}`

Update an existing model.

**Path Parameters:**

- `provider_name` (string): Name of the provider
- `model_name` (string): Name of the model

**Request Body:**
```json
{
  "display_name": "Updated GPT-5.1",
  "is_active": true,
  "tags": ["chat", "general", "image", "fast"],
  "rate_limit": {
    "max_requests": 100,
    "per_seconds": 60
  }
}
```

All fields are optional and only provided fields will be updated.

**Response:**
```json
{
  "id": 1,
  "name": "gpt-5.1",
  "display_name": "Updated GPT-5.1",
  "description": "OpenAI's most advanced model",
  "provider_id": 1,
  "provider_name": "openai",
  "provider_type": "openai",
  "tags": ["chat", "general", "image", "fast"],
  "default_params": {},
  "config": {
    "context_window": "128k",
    "supports_vision": true
  },
  "rate_limit": {
    "max_requests": 100,
    "per_seconds": 60
  },
  "local_path": null
}
```

---

### Model Invocation

#### POST `/models/{provider_name}/{model_name}/invoke`

Directly invoke a specific model.

**Authentication:** Required for remote requests (optional for local requests)

**Note:** If authentication is provided, model and parameter restrictions will be applied.

**Path Parameters:**

- `provider_name` (string): Name of the provider
- `model_name` (string): Name of the model

**Request Body:**
```json
{
  "prompt": "What is the capital of France?",
  "messages": [
    {
      "role": "user",
      "content": "What is the capital of France?"
    }
  ],
  "parameters": {
    "temperature": 0.7,
    "max_tokens": 150
  },
  "stream": false
}
```

**Parameters:**

- `prompt` (string, optional): Simple text prompt (alternative to messages)
- `messages` (array of objects, optional): Chat messages in role/content format (alternative to prompt)
- `parameters` (object, optional): Model-specific parameters
- `stream` (boolean, default: false): Whether to stream the response
- `batch` (array of objects, optional): List of `ModelInvokeRequest` objects for concurrent processing. If provided, top-level `prompt` and `messages` are ignored for the batch call.

Either `prompt`, `messages`, or `batch` must be provided.

**Example Usage:**

**Client Example:**
```text
from curl_cffi import requests

# 使用 prompt
response = requests.post(
    "http://localhost:18000/models/openrouter/llama-3.3-70b-instruct/invoke",
    json={
        "prompt": "What is Python?",
        "parameters": {"temperature": 0.7, "max_tokens": 200}
    }
)
data = response.json()
print(data["output_text"])

# 使用 messages
response = requests.post(
    "http://localhost:18000/models/openrouter/llama-3.3-70b-instruct/invoke",
    json={
        "messages": [
            {"role": "user", "content": "What is Python?"}
        ],
        "parameters": {"temperature": 0.7, "max_tokens": 200}
    }
)
```

**JavaScript:**
```javascript
// 使用 prompt
const response = await fetch(
    'http://localhost:18000/models/openrouter/llama-3.3-70b-instruct/invoke',
    {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({
            prompt: 'What is Python?',
            parameters: {temperature: 0.7, max_tokens: 200}
        })
    }
);
const data = await response.json();
console.log(data.output_text);
```

**curl:**
```bash
# 使用 prompt
curl -X POST "http://localhost:18000/models/openrouter/llama-3.3-70b-instruct/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "What is Python?",
    "parameters": {
      "temperature": 0.7,
      "max_tokens": 200
    }
  }'

# 使用 messages
curl -X POST "http://localhost:18000/models/openrouter/llama-3.3-70b-instruct/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [
      {"role": "user", "content": "What is Python?"}
    ],
    "parameters": {
      "temperature": 0.7,
      "max_tokens": 200
    }
  }'
```

**Response:**
```json
{
  "output_text": "The capital of France is Paris.",
  "cost": 0.00045,
  "raw": {
    "model": "gpt-5.1",
    "created": 1234567890,
    "usage": {
      "prompt_tokens": 10,
      "completion_tokens": 5,
      "total_tokens": 15
    }
  }
}
```

#### Batch Invocation (Concurrent)

You can process multiple requests concurrently by using the `batch` field in the request body. This is supported by both direct invocation and intelligent routing.

**Request Body:**
```json
{
  "batch": [
    {
      "prompt": "Tell me a joke",
      "parameters": {"max_tokens": 50}
    },
    {
      "prompt": "What is 1+1?",
      "parameters": {"max_tokens": 10}
    }
  ]
}
```

**Response:**
```json
{
  "output_text": "Batch processing completed",
  "batch": [
    {
      "output_text": "Why did the chicken cross the road?...",
      "cost": 0.00001,
      "raw": {...}
    },
    {
      "output_text": "1 + 1 = 2",
      "cost": 0.000005,
      "raw": {...}
    }
  ],
  "cost": 0.000015
}
```

**Note:** Batch requests do not support streaming. If `stream: true` is provided with `batch`, it will be ignored and a full response will be returned.

---

### OpenAI-Compatible API

The LLM Router provides a standard OpenAI-compatible API endpoint that follows the OpenAI API format. This allows you to use the router as a drop-in replacement for OpenAI's API with minimal code changes.

**Chat 调用方式概览：**

| 方式 | 端点 | model 格式 | 适用场景 |
|------|------|------------|----------|
| Provider 在路径 | `POST /{provider}/v1/chat/completions` | 仅模型名（如 `glm-4.5-air`） | 明确指定 provider，避免重复前缀 |
| 标准端点 | `POST /v1/chat/completions` | `provider/model`（如 `openrouter/glm-4.5-air`） | 通用、可替换 OpenAI SDK |
| 直接 Invoke | `POST /models/{provider}/{model}/invoke` | URL 路径 | 指定具体模型 |

#### POST `/{provider}/v1/chat/completions`（Provider 在路径中）

当 provider 在路径中时，请求体 `model` 只需传模型名，避免 `openrouter/openrouter/xxx` 等重复前缀错误。

**端点示例**：`POST /openrouter/v1/chat/completions`

**Request Body:**
```json
{
  "model": "glm-4.5-air",
  "messages": [{"role": "user", "content": "Hello!"}],
  "max_tokens": 100
}
```

若 `model` 含 `provider/model` 且前缀与路径一致，会自动 strip 前缀。

**模型命名规则：**

- **Provider 名**：如 `openrouter`、`openai`、`claude` 等
- **模型名**：使用数据库中的本地模型名；若通过兼容模式从 `router.toml` 导入 `[[models]]`，仍然取其中的 `name`，**不是** `remote_identifier`
- 示例：`glm-4.5-air`（调用格式 `openrouter/glm-4.5-air`）对应 `remote_identifier` 如 `z-ai/glm-4.5-air:free`

**curl 示例：**
```bash
# 推荐：仅传模型名
curl -X POST "http://localhost:18000/openrouter/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{"model": "glm-4.5-air", "messages": [{"role": "user", "content": "Hello"}], "max_tokens": 50}'

# 若传 provider/model 且前缀与路径一致，会自动 strip 前缀
curl -X POST "http://localhost:18000/openrouter/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{"model": "openrouter/glm-4.5-air", "messages": [{"role": "user", "content": "Hello"}], "max_tokens": 50}'
```

---

#### POST `/v1/chat/completions`

Standard OpenAI chat completions endpoint. The `model` parameter is specified in the request body, following OpenAI's standard format.

**Authentication:** Required for remote requests (optional for local requests)

**Request Body:**
```json
{
  "model": "openrouter/glm-4.5-air",
  "messages": [
    {
      "role": "user",
      "content": "Hello!"
    }
  ],
  "temperature": 0.7,
  "max_tokens": 150
}
```

**部分可用模型示例：**

- `openai/gpt-5.1`、`openai/gpt-5-pro`：OpenAI 最新模型
- `claude/claude-4.5-sonnet`、`claude/claude-4.5-haiku`：Anthropic Claude 系列
- `gemini/gemini-2.5-flash`、`gemini/gemini-3.0-pro`：Google Gemini 系列
- `glm/glm-4.7`、`glm/glm-4.6-plus`：智谱 GLM 系列
- `qwen/qwen2.5-72b-instruct`、`qwen/qwen-turbo`：阿里云通义千问系列
- `kimi/kimi-k2-128k`、`kimi/kimi-k2-flash`：月之暗面 Kimi 系列
- `openrouter/llama-3.3-70b-instruct`：Meta Llama 3.3（免费）
- `openrouter/qwen3-next-80b-a3b-instruct`：Qwen 3 系列（免费）
- `openrouter/nemotron-3-nano-30b-a3b`：NVIDIA Nemotron 3 Nano（免费）
- `openrouter/deepseek-r1-0528`：DeepSeek R1 系列（免费）
- `ollama/gpt-oss-20b`：Ollama 本地模型
- 更多模型请查看 `router.toml` 配置文件

**Parameters:**

- `model` (string, optional): Model identifier in the format `provider_name/model_name`，也支持 `strong|weak|stronge` 别名。
  - Example: `"openrouter/glm-4.5-air"`, `"openai/gpt-5.1"`, `"claude/claude-4.5-sonnet"`
  - `strong|weak|stronge` 会映射到会话绑定或配置中的强/弱模型。
  - If using session binding (see below), this parameter can be omitted.
  - Alternatively, you can use a full remote model identifier to call models not configured in the database.
- `routing_mode` (string, optional): `auto|strong|weak|stronge`，用于自动或强弱档路由
- `routing_pair` (string, optional): pair 名称，从 `[[routing.pairs]]` 选取 strong/weak 模型对。未指定时使用 `default_pair` 或 `default_strong_model`/`default_weak_model`
- `messages` (array, required): Array of message objects with `role` and `content` fields. Supported roles: `system`, `user`, `assistant`.
- `temperature` (number, optional): Sampling temperature (0-2). Default varies by model.
- `top_p` (number, optional): Nucleus sampling parameter.
- `max_tokens` (integer, optional): Maximum number of tokens to generate.
- `stop` (string or array, optional): Stop sequences.
- `presence_penalty` (number, optional): Presence penalty (-2.0 to 2.0).
- `frequency_penalty` (number, optional): Frequency penalty (-2.0 to 2.0).
- `stream` (boolean, optional): 是否启用流式响应。`/v1/chat/completions` 支持 `stream=true` 并返回 SSE。
- `n` (integer, optional): Number of completions to generate. Default: 1.
- `user` (string, optional): User identifier for tracking.

**Response:**
```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1677652288,
  "model": "openai/gpt-5.1",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! How can I help you today?"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 9,
    "total_tokens": 19,
    "cost": 0.00057
  }
}
```

**Example Usage:**

**Client Example:**
```text
from curl_cffi import requests

headers = {"Content-Type": "application/json"}
if token:
    headers["Authorization"] = f"Bearer {token}"  # 可选，本机请求可省略

# 方式 1：Provider 在路径中，model 只需模型名
url = "http://localhost:18000/openrouter/v1/chat/completions"
payload = {"model": "glm-4.5-air", "messages": [{"role": "user", "content": "Hello!"}], "max_tokens": 100}

# 方式 2：标准端点，model 为 provider/model
url = "http://localhost:18000/v1/chat/completions"
payload = {"model": "openrouter/glm-4.5-air", "messages": [{"role": "user", "content": "Hello!"}], "max_tokens": 100}

response = requests.post(url, json=payload, headers=headers)
data = response.json()
print(data["choices"][0]["message"]["content"])
```

**JavaScript:**
```javascript
// 方式 1：Provider 在路径中
const url = 'http://localhost:18000/openrouter/v1/chat/completions';
// payload: { model: 'glm-4.5-air', messages: [...] }

// 方式 2：标准调用
const url = 'http://localhost:18000/v1/chat/completions';
const response = await fetch(url, {
    method: 'POST',
    headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`  // 可选，本机请求可省略
    },
    body: JSON.stringify({
        model: 'openrouter/glm-4.5-air',  // model 在请求体中
        messages: [{role: 'user', content: 'Hello!'}],
        temperature: 0.7,
        max_tokens: 100
    })
});
const data = await response.json();
console.log(data.choices[0].message.content);
```

**curl:**
```bash
# 标准调用（本机请求可省略 Authorization header）
# 也可改用 POST /{provider}/v1/chat/completions 在路径中指定 provider
curl -X POST http://localhost:18000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "model": "openrouter/glm-4.5-air",
    "messages": [{"role": "user", "content": "Hello!"}],
    "temperature": 0.7,
    "max_tokens": 100
  }'
```

**使用 OpenAI SDK（完全兼容）：**
```text
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
    max_tokens=100
)

print(response.choices[0].message.content)
```

**使用 Session 绑定模型（可选，推荐）：**
```text
# 1. 登录并绑定模型
response = requests.post(
    "http://localhost:18000/auth/login",
    json={"api_key": "your-api-key"}
)
token = response.json()["token"]

# 2. 绑定模型到 Session
requests.post(
    "http://localhost:18000/auth/bind-model",
    headers={"Authorization": f"Bearer {token}"},
    json={
        "provider_name": "openrouter",
        "model_name": "glm-4.5-air"
    }
)

# 3. 使用 OpenAI 兼容 API（可以不指定 model，使用绑定的模型）
url = "http://localhost:18000/v1/chat/completions"
payload = {
    # model 参数可以省略，使用 session 绑定的模型
    "messages": [{"role": "user", "content": "Hello!"}],
    "temperature": 0.7,
    "max_tokens": 100
}

response = requests.post(url, json=payload, headers={
    "Authorization": f"Bearer {token}",
    "Content-Type": "application/json"
})
```

---

#### POST `/v1/audio/speech`

OpenAI-compatible text-to-speech endpoint.

**Authentication:** Required for remote requests (optional for local requests)

**Request Body:**
```json
{
  "model": "qwen (cn)/qwen3-tts-flash",
  "input": "请用自然的中文读出这句话。",
  "voice": "Cherry",
  "response_format": "mp3"
}
```

**Notes:**
- The target model must declare `tts` capability via `config.capabilities.tts = true` or a `tts` tag.
- `ProviderType.QWEN` uses DashScope's native TTS API internally and is still exposed through `/v1/audio/speech`.
- This endpoint currently returns the complete audio file and does not stream partial audio chunks.

**Response:**
- `200 OK` with `audio/*` bytes.

#### POST `/v1/audio/transcriptions`

OpenAI-compatible speech-to-text endpoint.

**Multipart Example:**
```bash
curl -X POST http://localhost:18000/v1/audio/transcriptions \
  -F "model=local-speaches/faster-whisper-large-v3" \
  -F "file=@sample.wav"
```

**FunASR Plugin Example:**
```bash
curl -X POST http://localhost:18000/v1/audio/transcriptions \
  -F "model=plugin:funasr/paraformer-zh" \
  -F "file=@sample.wav"
```

**JSON Example:**
```json
{
  "model": "local-speaches/faster-whisper-large-v3",
  "file": "data:audio/wav;base64,UklGRi4uLg=="
}
```

**Notes:**
- Non-plugin target models must declare `asr` capability.
- For local deployment, the recommended integration path is an OpenAI-compatible speech service such as `speaches` or `vLLM[audio]`, registered as a normal `openai` provider.
- For local offline FunASR, configure `[plugins.asr.funasr]` and call `plugin:funasr/<model_id>`. The FunASR plugin supports transcription only; `/v1/audio/translations` is not supported.

---

### Intelligent Routing


#### POST `/route`

仅返回路由决策（模型和调用参数），不执行推理请求。适用于上游系统自行调用 OpenAI-compatible 客户端的场景。

**Authentication:** 本机（localhost/127.0.0.1）默认可免认证；远程请求需按配置提供 Session Token 或 API Key。

**Request Body:**
```json
{
  "model": "openrouter/gpt-4o",
  "role": "planner",
  "task": "worker",
  "trace_id": "trace-123",
  "routing_mode": "auto",
  "routing_pair": "gemini-3",
  "temperature": 0.2,
  "max_tokens": 1024
}
```

**Parameters:**
- `model` (string, optional): 手动指定模型，支持 `provider/model` 或别名 `strong|weak|stronge`
- `role` (string, optional): 调用角色（如 `supervisor/planner/writer/tester/docupdater`）
- `task` (string, optional): 调用任务类型（如 `routing/worker`）
- `trace_id` (string, optional): 追踪 ID
- `model_hint` (string, optional, deprecated): 兼容旧字段，建议改用 `model`
- `routing_mode` (string, optional): `auto|strong|weak|stronge`，用于自动或强弱档路由
- `routing_pair` (string, optional): pair 名称，从 `router.toml` 的 `[[routing.pairs]]` 选取 strong/weak 模型对。未指定时使用 `default_pair` 或 `default_strong_model`/`default_weak_model`
- `temperature` (number, optional): 覆盖默认温度
- `max_tokens` (integer, optional): 覆盖默认最大输出长度

**模型解析优先级（strong/weak）：** session 绑定 > `routing_pair` > `default_pair` > `default_strong_model`/`default_weak_model`

**Response:**
```json
{
  "model": "openrouter/gpt-4o",
  "base_url": "https://openrouter.ai/api/v1",
  "temperature": 0.2,
  "max_tokens": 1024,
  "provider": "openrouter"
}
```


#### GET `/route/pairs`

获取配置的 strong/weak 模型对列表（来自 `router.toml` 的 `[[routing.pairs]]`）。

**Authentication:** 本机默认可免认证；远程请求需 Session Token 或 API Key。

**Response:**
```json
{
  "default_pair": "gemini-3",
  "pairs": [
    {
      "name": "gemini-3",
      "strong_model": "gemini/gemini-3.0-pro",
      "weak_model": "gemini/gemini-3.0-flash"
    },
    {
      "name": "gemini-2.5",
      "strong_model": "gemini/gemini-2.5-pro",
      "weak_model": "gemini/gemini-2.5-flash"
    }
  ]
}
```


#### POST `/route/invoke`

Route a request to an appropriate model based on query criteria.

**Authentication:** 本机（localhost/127.0.0.1）默认可免认证（可通过环境变量 `LLM_ROUTER_ALLOW_LOCAL_WITHOUT_AUTH=false` 或 `router.toml` 的 `[server]` 下 `allow_local_without_auth = false` 关闭）；远程请求需按配置提供 Session Token 或 API Key。

**Request Body:**
```json
{
  "query": {
    "tags": ["chat", "fast"],
    "provider_types": ["openai", "gemini"],
    "include_inactive": false
  },
  "request": {
    "messages": [
      {
        "role": "user",
        "content": "What is the capital of France?"
      }
    ],
    "parameters": {
      "temperature": 0.7
    },
    "stream": false
  }
}
```

**Parameters:**

- `query` (object, required): Criteria for selecting a model to route to
  - `tags` (array of strings, optional): Tags to match
  - `provider_types` (array of strings, optional): Provider types to include
  - `include_inactive` (boolean, default: false): Include inactive models
- `request` (object, required): The actual request to send to the selected model (same format as direct invocation)

**Response:**
```json
{
  "output_text": "The capital of France is Paris.",
  "cost": 0.00045,
  "raw": {
    "model": "gpt-5.1",
    "created": 1234567890,
    "usage": {
      "prompt_tokens": 10,
      "completion_tokens": 5,
      "total_tokens": 15
    }
  }
}
```

**Quick curl (本机免认证示例):**
```bash
curl -X POST "http://localhost:18000/route/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "query": {"tags": ["chat","general"], "provider_types": ["openai","gemini","claude"]},
    "request": {"messages": [{"role": "user", "content": "Hello, how are you?"}], "stream": false}
  }'
```

**带认证示例（远程或强制认证场景）:**
```bash
curl -X POST "http://localhost:18000/route/invoke" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your-session-token-or-api-key>" \
  -d '{
    "query": {"tags": ["chat","general"], "provider_types": ["openai","gemini","claude"]},
    "request": {"messages": [{"role": "user", "content": "Hello, how are you?"}], "stream": false}
  }'
```

**Example Usage:**

**Client Example:**
```text
from curl_cffi import requests

response = requests.post(
    "http://localhost:18000/route/invoke",
    json={
        "query": {
            "tags": ["free", "fast"],
            "provider_types": ["openrouter"]
        },
        "request": {
            "prompt": "What is 2+2?",
            "parameters": {"temperature": 0.1, "max_tokens": 50}
        }
    }
)
data = response.json()
print(data["output_text"])
```

**JavaScript:**
```javascript
const response = await fetch('http://localhost:18000/route/invoke', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({
        query: {
            tags: ['free', 'fast'],
            provider_types: ['openrouter']
        },
        request: {
            prompt: 'What is 2+2?',
            parameters: {temperature: 0.1, max_tokens: 50}
        }
    })
});
const data = await response.json();
console.log(data.output_text);
```

---

## Error Responses

When an error occurs, the API returns an appropriate HTTP status code and an error message:

```json
{
  "detail": "Error message describing the issue"
}
```

Common status codes include:

- `400 Bad Request`: Invalid request parameters
- `401 Unauthorized`: Authentication required or invalid credentials
- `403 Forbidden`: Invalid API Key or Session Token, or access denied
- `404 Not Found`: Resource not found
- `429 Too Many Requests`: Rate limit exceeded
- `500 Internal Server Error`: Server error

### Common Chat Completions Errors（Chat 调用常见错误）

**错误：`模型 openrouter/openrouter/nemotron-nano-9b-v2 不存在`**

- **原因**：在 `/{provider}/v1/chat/completions` 中，`model` 仍传了 `provider/model`，导致重复前缀。
- **处理**：使用 provider-in-path 时，`model` 只传模型名（如 `nemotron-nano-9b-v2`）；或改用 `POST /v1/chat/completions` 并传 `model: "openrouter/nemotron-nano-9b-v2"`。

### Provider-Specific Errors（上游 Provider 错误）

当请求经由 LLM Router 转发到上游 Provider（如 OpenRouter、OpenAI）时，若上游返回错误，Router 会将原始错误透传。常见情况：

**OpenRouter 403：`This model is not available in your region.`**

- **含义**：所选模型在你所在地区不可用（由 OpenRouter/上游模型商限制）。
- **常见原因**：OpenAI 等模型对部分国家/地区有访问限制。
- **处理建议**：
  1. 改用 OpenRouter 上无区域限制的模型，例如：
     - `openrouter/glm-4.5-air`（免费）
     - `openrouter/llama-3.3-70b-instruct`（免费）
     - `openrouter/gemini-2.0-flash-exp`（免费）
  2. 使用代理或 VPN 变更请求出口地区（需自行承担合规风险）。
  3. 改用其他 Provider（如 `aihubmix/gpt-4o`、`openai/gpt-4o` 等）若其支持你所在地区。

**OpenRouter 其他 403**

- 账户余额不足：多数付费模型需至少 $5 余额，请到 openrouter.ai/account 充值。
- API Key 权限不足：检查 Key 的 scope 或重新生成。

---

---

### API Key Management

#### POST `/api-keys`

Create a new API Key.

**Authentication:** Required for remote requests (optional for local requests)

**Request Body:**
```json
{
  "key": "my-api-key",
  "name": "My API Key",
  "is_active": true,
  "expires_at": "2026-12-31T00:00:00Z",
  "quota_tokens_monthly": 5000000,
  "ip_allowlist": ["203.0.113.8", "10.0.0.0/8"],
  "allowed_models": ["openai/gpt-5.1", "claude/claude-4.5-sonnet"],
  "allowed_providers": ["openai"],
  "parameter_limits": {
    "max_tokens": 2000,
    "temperature": 0.7
  }
}
```

**Response:**
```json
{
  "id": 1,
  "key": "my-api-key",
  "name": "My API Key",
  "is_active": true,
  "expires_at": "2026-12-31T00:00:00Z",
  "quota_tokens_monthly": 5000000,
  "ip_allowlist": ["203.0.113.8", "10.0.0.0/8"],
  "allowed_models": ["openai/gpt-5.1", "claude/claude-4.5-sonnet"],
  "allowed_providers": ["openai"],
  "parameter_limits": {
    "max_tokens": 2000,
    "temperature": 0.7
  },
  "created_at": "2024-01-01T00:00:00",
  "updated_at": "2024-01-01T00:00:00"
}
```

---

#### GET `/api-keys`

List all API Keys.

**Authentication:** Required for remote requests (optional for local requests)

**Response:**
```json
[
  {
    "id": 1,
    "key": "my-api-key",
    "name": "My API Key",
    "is_active": true,
    "allowed_models": null,
    "allowed_providers": null,
    "parameter_limits": null,
    "created_at": "2024-01-01T00:00:00",
    "updated_at": "2024-01-01T00:00:00"
  }
]
```

---

#### GET `/api-keys/{id}`

Get a specific API Key by ID.

**Authentication:** Required for remote requests (optional for local requests)

**Path Parameters:**
- `id` (integer): API Key ID

**Response:**
```json
{
  "id": 1,
  "key": "my-api-key",
  "name": "My API Key",
  "is_active": true,
  "allowed_models": null,
  "allowed_providers": null,
  "parameter_limits": null,
  "created_at": "2024-01-01T00:00:00",
  "updated_at": "2024-01-01T00:00:00"
}
```

---

#### PATCH `/api-keys/{id}`

Update an existing API Key.

**Authentication:** Required for remote requests (optional for local requests)

**Path Parameters:**
- `id` (integer): API Key ID

**Request Body:**
```json
{
  "name": "Updated API Key Name",
  "is_active": false,
  "allowed_models": ["openai/gpt-5.1"],
  "parameter_limits": {
    "max_tokens": 1000
  }
}
```

All fields are optional and only provided fields will be updated.

**Response:**
```json
{
  "id": 1,
  "key": "my-api-key",
  "name": "Updated API Key Name",
  "is_active": false,
  "allowed_models": ["openai/gpt-5.1"],
  "allowed_providers": null,
  "parameter_limits": {
    "max_tokens": 1000
  },
  "created_at": "2024-01-01T00:00:00",
  "updated_at": "2024-01-01T01:00:00"
}
```

---

#### DELETE `/api-keys/{id}`

Delete an API Key.

**Authentication:** Required for remote requests (optional for local requests)

**Path Parameters:**
- `id` (integer): API Key ID

**Response:**
- `204 No Content`: Successfully deleted

---

## Rate Limiting

The LLM Router implements rate limiting per model with configurable limits:

- `max_requests`: Maximum number of requests allowed
- `per_seconds`: Time period in seconds for the rate limit
- `burst_size`: Optional burst size (if larger than max_requests)

Rate limits are automatically applied and tracked per model, ensuring providers stay within their API limits.

---

## Monitoring Endpoints

The LLM Router provides monitoring endpoints to track API usage and performance.

### GET `/monitor/invocations`

Get invocation history with optional filtering.

**Authentication:** Required for remote requests (optional for local requests)

**Query Parameters:**
- `model_id` (integer, optional): Filter by model ID
- `provider_id` (integer, optional): Filter by provider ID
- `model_name` (string, optional): Filter by model name
- `provider_name` (string, optional): Filter by provider name
- `status` (string, optional): Filter by status (success, error)
- `api_key_id` (integer, optional): Filter by caller API Key ID
- `auth_type` (string, optional): Filter by auth type (`api_key` / `session_token`)
- `start_time` (datetime, optional): Start time for filtering
- `end_time` (datetime, optional): End time for filtering
- `limit` (integer, default: 100): Maximum number of results
- `offset` (integer, default: 0): Offset for pagination
- `order_by` (string, default: "started_at"): Field to order by
- `order_desc` (boolean, default: true): Order descending

**Response:**
```json
[
  {
    "id": 1,
    "model_id": 1,
    "provider_id": 1,
    "model_name": "gpt-5.1",
    "provider_name": "openai",
    "started_at": "2024-01-01T00:00:00",
    "completed_at": "2024-01-01T00:00:01",
    "duration_ms": 1000.0,
    "status": "success",
    "error_message": null,
    "prompt_tokens": 10,
    "completion_tokens": 5,
    "total_tokens": 15,
    "cost": 0.00045,
    "created_at": "2024-01-01T00:00:00"
  }
]
```

---

### GET `/monitor/invocations/{id}`

Get a specific invocation by ID.

**Authentication:** Required for remote requests (optional for local requests)

**Path Parameters:**
- `id` (integer): Invocation ID

**Response:**
```json
{
  "id": 1,
  "model_id": 1,
  "provider_id": 1,
  "model_name": "gpt-5.1",
  "provider_name": "openai",
  "started_at": "2024-01-01T00:00:00",
  "completed_at": "2024-01-01T00:00:01",
  "duration_ms": 1000.0,
  "status": "success",
  "error_message": null,
  "request_prompt": "What is the capital of France?",
  "request_messages": null,
  "request_parameters": {"temperature": 0.7},
  "response_text": "The capital of France is Paris.",
  "response_text_length": 25,
  "prompt_tokens": 10,
  "completion_tokens": 5,
  "total_tokens": 15,
  "cost": 0.00045,
  "raw_response": {...},
  "created_at": "2024-01-01T00:00:00"
}
```

---

### GET `/monitor/statistics`

---

### GET `/monitor/quota-details`

按调用方/API Key、渠道、模型聚合额度明细，支持 `start_time`、`end_time`、`provider_name`、`model_name`、`api_key_id`、`limit`、`offset` 查询参数。

### GET `/monitor/quota-details/export`

导出额度明细，`format=csv|json`（默认 `csv`）。

### GET `/monitor/budget-alerts`

查询日/周/月 token 阈值与当前告警状态。

### PUT `/monitor/budget-alerts`

更新预算阈值，Body:

```json
{
  "day_tokens": 1000000,
  "week_tokens": 5000000,
  "month_tokens": 20000000
}
```

### API Key Policy Templates

- `GET /api-key-policy-templates`
- `POST /api-key-policy-templates`
- `PATCH /api-key-policy-templates/{id}`
- `DELETE /api-key-policy-templates/{id}`
- `POST /api-keys/batch-apply-policy`
- `GET /api-keys/policy-audit`

### Provider Model Catalog

- `GET /providers/{provider_name}/remote-models`
- `POST /providers/{provider_name}/models/sync`
- `POST /providers/models/sync`
- `GET /model-updates/runs`
- `POST /providers/{provider_name}/catalog-models/sync`
- `GET /providers/{provider_name}/catalog-models`
- `GET /providers/{provider_name}/model-reconciliation`

### Metadata Override

- `PATCH /models/{provider_name}/{model_name}/metadata-override`

将手工元信息覆盖写入 `models.config.metadata_override`，用于标签、上下文长度、价格等元信息覆盖。

Get usage statistics.

**Authentication:** Required for remote requests (optional for local requests)

**Query Parameters:**
- `time_range` (string, optional): Time range (e.g., "1h", "24h", "7d")

**Response:**
```json
{
  "overall": {
    "time_range": "24h",
    "total_calls": 1000,
    "success_calls": 950,
    "error_calls": 50,
    "success_rate": 0.95,
    "total_tokens": 50000,
    "avg_duration_ms": 1200.0,
    "total_cost": 1.25
  },
  "by_model": [
    {
      "model_id": 1,
      "model_name": "gpt-5.1",
      "provider_name": "openai",
      "total_calls": 500,
      "success_calls": 480,
      "error_calls": 20,
      "success_rate": 0.96,
      "total_tokens": 25000,
      "prompt_tokens": 15000,
      "completion_tokens": 10000,
      "avg_duration_ms": 1500.0,
      "total_duration_ms": 750000.0,
      "total_cost": 0.75
    }
  ],
  "recent_errors": [...]
}
```

---

### GET `/monitor/login-records`

Get login/auth records (stored in Redis). Records are written on each auth attempt (success or failure) and on `/auth/login` success.

**Authentication:** Required for remote requests (optional for local requests)

**Query Parameters:**
- `limit` (integer, default: 100, max: 500): Maximum number of results
- `offset` (integer, default: 0): Offset for pagination
- `auth_type` (string, optional): Filter by auth type (`api_key`, `session_token`, `none`)
- `is_success` (boolean, optional): Filter by success/failure

**Response:**
```json
{
  "records": [
    {
      "id": "uuid",
      "timestamp": "2024-01-01T00:00:00",
      "ip_address": "127.0.0.1",
      "auth_type": "api_key",
      "is_success": true,
      "api_key_id": null,
      "session_token_hash": null,
      "is_local": true
    }
  ],
  "total": 100
}
```

---

### GET `/monitor/time-series`

Get time series data for usage metrics.

**Authentication:** Required for remote requests (optional for local requests)

**Query Parameters:**
- `granularity` (string, optional): Time granularity (hour, day, week, month)
- `start_time` (datetime, optional): Start time
- `end_time` (datetime, optional): End time

**Response:**
```json
{
  "granularity": "hour",
  "data": [
    {
      "timestamp": "2024-01-01T00:00:00",
      "total_calls": 100,
      "success_calls": 95,
      "error_calls": 5,
      "total_tokens": 5000
    }
  ]
}
```
