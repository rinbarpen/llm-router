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

If restrictions are `null` or not set, there are no limits.

## Common Response Format

Most API endpoints return JSON responses with appropriate HTTP status codes.

## Endpoints

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

**Python (curl_cffi):**
```python
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

**Python (curl_cffi):**
```python
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
- `type` (string, required): Provider type (one of: "openai", "gemini", "claude", "openrouter", "glm", "kimi", "qwen")
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

---

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

**Python (curl_cffi):**
```python
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
  "name": "gpt-4o",
  "provider_name": "openai",
  "display_name": "GPT-4o",
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
  "display_name": "Updated GPT-4o",
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

Either `prompt` or `messages` must be provided.

**Example Usage:**

**Python (curl_cffi):**
```python
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
curl -X POST "http://localhost:18000/models/openrouter/openrouter-llama-3.3-70b-instruct/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "What is Python?",
    "parameters": {
      "temperature": 0.7,
      "max_tokens": 200
    }
  }'

# 使用 messages
curl -X POST "http://localhost:18000/models/openrouter/openrouter-llama-3.3-70b-instruct/invoke" \
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

---

### OpenAI-Compatible API

The LLM Router provides a standard OpenAI-compatible API endpoint that follows the OpenAI API format. This allows you to use the router as a drop-in replacement for OpenAI's API with minimal code changes.

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

- `model` (string, required): Model identifier in the format `provider_name/model_name`.
  - Example: `"openrouter/glm-4.5-air"`, `"openai/gpt-5.1"`, `"claude/claude-4.5-sonnet"`
  - If using session binding (see below), this parameter can be omitted.
  - Alternatively, you can use a full remote model identifier to call models not configured in the database.
- `messages` (array, required): Array of message objects with `role` and `content` fields. Supported roles: `system`, `user`, `assistant`.
- `temperature` (number, optional): Sampling temperature (0-2). Default varies by model.
- `top_p` (number, optional): Nucleus sampling parameter.
- `max_tokens` (integer, optional): Maximum number of tokens to generate.
- `stop` (string or array, optional): Stop sequences.
- `presence_penalty` (number, optional): Presence penalty (-2.0 to 2.0).
- `frequency_penalty` (number, optional): Frequency penalty (-2.0 to 2.0).
- `stream` (boolean, optional): Whether to stream the response. Currently not supported.
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

**Python (curl_cffi):**
```python
from curl_cffi import requests

# 标准 OpenAI API 端点（无需登录，model 在请求体中）
url = "http://localhost:18000/v1/chat/completions"
headers = {
    "Content-Type": "application/json",
    "Authorization": f"Bearer {token}"  # 可选，本机请求可省略
}
payload = {
    "model": "openrouter/glm-4.5-air",  # model 在请求体中
    "messages": [{"role": "user", "content": "Hello!"}],
    "temperature": 0.7,
    "max_tokens": 100
}

response = requests.post(url, json=payload, headers=headers)
data = response.json()
print(data["choices"][0]["message"]["content"])
```

**JavaScript:**
```javascript
// 标准调用
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
    max_tokens=100
)

print(response.choices[0].message.content)
```

**使用 Session 绑定模型（可选，推荐）：**
```python
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

### Intelligent Routing

#### POST `/route/invoke`

Route a request to an appropriate model based on query criteria.

**Authentication:** 本机（localhost/127.0.0.1）可免认证；远程请求需按配置提供 Session Token 或 API Key。

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

**Python (curl_cffi):**
```python
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
  "model_name": "gpt-4o",
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