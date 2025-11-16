# LLM Router

LLM Router is a flexible and scalable routing service for Large Language Models (LLMs) that provides unified access to multiple AI providers with rate limiting, model management, and intelligent routing capabilities.

## Features

- **Multi-Provider Support**: Connect to various LLM providers including OpenAI, Gemini, Claude, GLM, Qwen, Kimi, and OpenRouter
- **Model Management**: Register, configure, and manage models with tags, rate limits, and custom settings
- **Rate Limiting**: Built-in rate limiting with configurable limits per model and provider
- **Intelligent Routing**: Route requests based on tags, provider types, and other criteria
- **Configuration via TOML**: Easy configuration using TOML files with support for multiple providers and models
- **RESTful API**: Comprehensive API for managing providers, models and routing requests
- **SQLite Backend**: Persistent storage for provider and model configurations

## Installation

```bash
# Clone the repository
git clone <repository-url>
cd llm-router

# Install dependencies using uv (recommended)
uv sync

# Or install using pip
pip install -e .
```

## Configuration

1. Copy the example configuration file:
```bash
cp router.example.toml router.toml
```

2. Edit `router.toml` to configure your providers and models, setting up API keys and rate limits as needed.

3. Set your environment variables for API keys:
```bash
export OPENAI_API_KEY="your-openai-api-key"
export GEMINI_API_KEY="your-gemini-api-key"
export ANTHROPIC_API_KEY="your-anthropic-api-key"
# ... add other provider keys as needed
```

## Usage

### Running the Server

```bash
# Start the server with default settings (runs on 0.0.0.0:8000)
llm-router

# Or with custom host/port
LLM_ROUTER_HOST=127.0.0.1 LLM_ROUTER_PORT=8001 llm-router
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `LLM_ROUTER_HOST` | Host to bind the server to | `0.0.0.0` |
| `LLM_ROUTER_PORT` | Port to bind the server to | `8000` |
| `LLM_ROUTER_DATABASE_URL` | Database connection string | `sqlite:///llm_router.db` |
| `LLM_ROUTER_MODEL_STORE` | Directory for storing model files | `./model_store` |
| `LLM_ROUTER_DOWNLOAD_CACHE` | Directory for download cache | None |
| `LLM_ROUTER_DOWNLOAD_CONCURRENCY` | Number of concurrent downloads | `2` |
| `LLM_ROUTER_DEFAULT_TIMEOUT` | Request timeout in seconds | `60.0` |
| `LLM_ROUTER_LOG_LEVEL` | Log level | `INFO` |
| `LLM_ROUTER_MODEL_CONFIG` | Path to model configuration file | None |

## Supported Providers

The LLM Router supports multiple LLM providers:

- **OpenAI**: GPT-4, GPT-4o, GPT-4o Mini, O1 series
- **Google Gemini**: Gemini 2.5 Flash, Gemini 1.5 Pro
- **Anthropic Claude**: Claude 3.7 Sonnet, Claude 3.5 Haiku
- **Zhipu AI GLM**: GLM-4 Plus, GLM-4 Flash
- **Alibaba Tongyi Qwen**: Qwen2.5 72B Instruct, Qwen Turbo
- **Moonshot/Kimi**: Moonshot v1 models
- **OpenRouter**: Access to models through OpenRouter

## API Documentation

See [API.md](API.md) for detailed API documentation.

## Example Usage

### Direct Model Invocation

```bash
curl -X POST http://localhost:8000/models/openai/gpt-4o/invoke \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [
      {"role": "user", "content": "Hello, how are you?"}
    ]
  }'
```

### Intelligent Routing

```bash
curl -X POST http://localhost:8000/route/invoke \
  -H "Content-Type: application/json" \
  -d '{
    "query": {
      "tags": ["chat", "fast"],
      "provider_types": ["openai", "gemini"]
    },
    "request": {
      "messages": [
        {"role": "user", "content": "What is the capital of France?"}
      ]
    }
  }'
```

## Running Tests

```bash
# Run all tests
pytest

# Run specific test file
pytest tests/test_api.py
```

## License

MIT License - see the [LICENSE](LICENSE) file for details.