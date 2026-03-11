# Reference: Provider Docs and TOML Fields

## Official Model List Sources

Use these for web search or fetch when updating models.

| Provider | Model list / docs | Notes |
|----------|------------------|--------|
| OpenAI | https://platform.openai.com/docs/models | Models overview and capabilities |
| OpenAI | https://platform.openai.com/docs/api-reference/models/list | List models via API (GET /v1/models) |
| Anthropic | https://docs.anthropic.com/en/docs/about-claude/models/all-models | All Claude models |
| Anthropic | https://docs.anthropic.com/en/api/models-list | List models API |
| Google Gemini | https://ai.google.dev/gemini-api/docs/models | Gemini models and specs |
| Google Gemini | https://ai.google.dev/api/models | Models API reference |
| OpenRouter | https://openrouter.ai/api/v1/models | Public list; project uses `scripts/update_openrouter_free_models.py` |

Other providers (GLM, Qwen, Kimi, DeepSeek, etc.): search for “{provider} API models” or “{provider} 模型列表” and use official docs.

---

## router.toml Model Block

Only **model-related** keys below. Do not add or change `[server]`, `[monitor]`, or `[[providers]]` in this skill.

### `[[models]]` (required)

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Unique model id in this project (e.g. `gpt-5.2`, `claude-4.5-sonnet`). Used in routes and API. |
| `provider` | string | Must match a `[[providers]].name` (e.g. `openai`, `claude`, `gemini`, `openrouter`). |
| `display_name` | string | Human-readable name (e.g. `GPT-5.2`, `Claude Opus 4.6`). |
| `tags` | array of strings | e.g. `["chat", "general", "image", "function-call", "high-quality"]`. Used for routing. |
| `remote_identifier` | string (optional) | Provider’s own model id when it differs from `name`. Required for Claude and Gemini when the API expects a specific id (e.g. `claude-sonnet-4-5-20250929`, `gemini-3-pro-preview`). |

### `[models.rate_limit]` (optional)

| Field | Type | Description |
|-------|------|-------------|
| `max_requests` | integer | Max requests per window. |
| `per_seconds` | integer | Window length in seconds. |

Omit if no rate limit is needed.

### `[models.config]` (required)

| Field | Type | Description |
|-------|------|-------------|
| `context_window` | string | e.g. `"128k"`, `"256k"`, `"1M"`. Use `k` for thousands, `M` for millions. |
| `supports_vision` | boolean | `true` if the model accepts image input. |
| `supports_tools` | boolean | `true` if the model supports function/tool calling. |
| `languages` | array of strings | e.g. `["en"]` or `["zh", "en"]`. |

---

## remote_identifier

- **OpenAI**: Usually not needed; `name` is the model id sent to the API.
- **Claude (Anthropic)**: Set when the API expects a dated or specific id (e.g. `claude-sonnet-4-5-20250929`). See `src/llm_router/providers/anthropic.py` (uses `model.remote_identifier`).
- **Gemini**: Set when the API id differs from `name` (e.g. `gemini-3-pro-preview`). See `src/llm_router/providers/gemini.py` (uses `model.remote_identifier`).

Schema and DB fields: `src/llm_router/schemas.py` (`ModelCreate`, `ModelUpdate`, `ModelRead`), `src/llm_router/db/models.py` (`Model`).
