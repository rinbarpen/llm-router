---
name: update-provider-models
description: Updates models and related metadata (context window, vision, tools, tags) for each LLM Router provider. Prefers provider APIs where available (e.g. OpenRouter), uses web search for official docs when no API exists. Writes to router.toml and reminds to sync config to DB. Use when the user asks to update provider models, refresh model lists, sync model information from OpenAI/Claude/Gemini/OpenRouter, or when asked to update 各 provider 的模型 / 同步模型列表 / 用搜索更新模型信息.
---

# Update Provider Models

## When to Use

Apply this skill when the user asks to:
- Update models and related information for each provider
- Refresh or sync model lists (OpenAI, Claude, Gemini, OpenRouter, etc.)
- Use search to find latest model IDs and metadata, then update configuration

## Strategy

1. **Provider has model list API**  
   Prefer calling the API to fetch models. For OpenRouter, use or reference the project script `scripts/update_openrouter_free_models.py` (fetch API → parse → generate `[[models]]` blocks).

2. **No public model list API**  
   Use **web search** or **URL fetch** (e.g. MCP `web_search`, `mcp_web_fetch`) to find official model pages (OpenAI Models docs, Anthropic model list, Google AI Gemini docs). Extract model id, display name, context length, supports_vision, supports_tools, then map to TOML.

## Workflow

1. **Scope** — Determine which providers to update (all or user-specified).
2. **Per provider**  
   - If API or project script exists → fetch via API / run or emulate script.  
   - Else → web search or fetch official docs, parse model list and attributes.
3. **Map to TOML** — Convert results to `[[models]]` + `[models.rate_limit]` (optional) + `[models.config]`. Deduplicate by `name`.
4. **Edit router.toml** — Insert or update `[[models]]` blocks inside the correct provider section (e.g. `# OpenAI Models`, `# Claude Models`). Do not change `[server]`, `[frontend]`, or `[[providers]]`.
5. **Remind** — After editing, tell the user to run **config sync to DB** (e.g. frontend “同步配置” or `POST /config/sync`). This is an existing project feature; the skill does not implement it.

## router.toml Writing Rules

- **Only change model-related content.** Do not modify server, frontend, or provider connection config.
- **Block structure** (match existing style in project `router.toml`):
  - `[[models]]`: `name`, `provider`, `display_name`, `tags`. Add `remote_identifier` when the provider uses it (e.g. Claude, Gemini).
  - `[models.rate_limit]`: optional; use same keys as existing (e.g. `max_requests`, `per_seconds`).
  - `[models.config]`: `context_window` (e.g. `"128k"`, `"1M"`), `supports_vision`, `supports_tools`, `languages` (e.g. `["en"]` or `["zh", "en"]`).
- **Placement** — Add new models inside the corresponding provider section; keep section headers and order; do not break existing structure.

## OpenRouter

- Prefer the existing script: `scripts/update_openrouter_free_models.py` (fetches from OpenRouter API, filters free models, appends to router.toml). When the user wants OpenRouter models updated, suggest running it or follow its pattern for API → parse → TOML.

## After Updating

Remind the user to **sync configuration to the database** so the running app sees the new models:
- Frontend: use “同步配置” (or equivalent).
- API: `POST /config/sync`.

## Reference

- Provider doc links and TOML field details: [reference.md](reference.md).
