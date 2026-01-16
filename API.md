# LLM Router API Documentation

The LLM Router provides a RESTful API for managing LLM providers, models, and routing requests across multiple AI services.

## Base URL

All API endpoints are relative to `http://localhost:8000` (or your configured host and port).

## Authentication

For most endpoints, no authentication is required. However, providers may require API keys which are configured in the server configuration.

## Common Response Format

Most API endpoints return JSON responses with appropriate HTTP status codes.

## Endpoints

### Health Check

#### GET `/health`

Check if the service is running and healthy.

**Response:**
```json
{
  "status": "ok"
}
```

---

### Provider Management

#### GET `/providers`

List all configured providers.

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

**Query Parameters:**

- `tag` or `tags` (string, optional): Filter models by tags (repeated parameter or comma-separated values)
- `provider_type` or `provider_types` (string, optional): Filter models by provider type (repeated parameter or comma-separated values)
- `include_inactive` (boolean, default: false): Include inactive models in the response

**Example:**
```
GET /models?tags=chat,general&provider_types=openai,gemini&include_inactive=true
```

**Response:**
```json
[
  {
    "id": 1,
    "name": "gpt-4o",
    "display_name": "GPT-4o",
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
      "languages": ["en"]
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
  "name": "gpt-4o",
  "display_name": "GPT-4o",
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
  "name": "gpt-4o",
  "display_name": "Updated GPT-4o",
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

**Response:**
```json
{
  "output_text": "The capital of France is Paris.",
  "raw": {
    "model": "gpt-4o",
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

### Intelligent Routing

#### POST `/route/invoke`

Route a request to an appropriate model based on query criteria. See [TAGS.md](TAGS.md) for a comprehensive guide on recommended model tags.

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
  "raw": {
    "model": "gpt-4o",
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

### OpenAI Compatible API

The router provides standard OpenAI-compatible endpoints, allowing you to use existing OpenAI clients (like the official OpenAI SDK) by simply changing the `base_url`.

#### GET `/v1/models`

List all available models in an OpenAI-compatible format. The `id` field in the response corresponds to the unique model names registered in the router.

**Response:**
```json
{
  "object": "list",
  "data": [
    {
      "id": "gpt-4o",
      "object": "model",
      "created": 1677610602,
      "owned_by": "llm-router"
    }
  ]
}
```

#### POST `/v1/chat/completions`

Standard OpenAI chat completion endpoint. The router uses the `model` parameter to select the appropriate model from its registered models. If multiple providers offer the same model name, the router will select the one with the highest priority configuration.

**Request Body:**
Standard OpenAI Chat Completion request body (supports `model`, `messages`, `temperature`, `top_p`, `n`, `stream`, `stop`, `max_tokens`, etc.).

**Example:**
```json
{
  "model": "gpt-4o",
  "messages": [
    {
      "role": "user",
      "content": "Hello!"
    }
  ],
  "temperature": 0.7
}
```

**Response:**
Standard OpenAI Chat Completion response body.

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
- `404 Not Found`: Resource not found
- `429 Too Many Requests`: Rate limit exceeded
- `500 Internal Server Error`: Server error

---

## Rate Limiting

The LLM Router implements rate limiting per model with configurable limits:

- `max_requests`: Maximum number of requests allowed
- `per_seconds`: Time period in seconds for the rate limit
- `burst_size`: Optional burst size (if larger than max_requests)

Rate limits are automatically applied and tracked per model, ensuring providers stay within their API limits.