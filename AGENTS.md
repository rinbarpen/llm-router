# Repository Guidelines

## Project Structure & Module Organization
- Backend Python package: `src/llm_router/`.
- API layer: `src/llm_router/api/` (Starlette routes, auth, request handling).
- Core services and routing logic: `src/llm_router/services/`.
- Provider adapters: `src/llm_router/providers/` (OpenAI, Gemini, Claude, local/CLI providers).
- Persistence models/session code: `src/llm_router/db/`.
- Frontend monitor (React + Vite + TS): `examples/monitor/`.
- Automated tests: `tests/` (core regression). Utility/manual scripts: `scripts/`, `examples/`.

## Build, Test, and Development Commands
- `uv sync`: install backend dependencies from `pyproject.toml`/`uv.lock`.
- `uv run llm-router`: start backend server.
- `./scripts/start.sh`: start backend + monitor in local dev mode.
- `./scripts/start.sh backend` / `./scripts/start.sh monitor`: start one side only.
- `uv run pytest`: run core regression suite under `tests/`.
- `uv run pytest -q tests/test_api.py tests/test_openai_api.py tests/test_auth.py`: fast API-focused check.
- `cd examples/monitor && npm install && npm run dev`: run monitor UI locally.
- `cd examples/monitor && npm run build`: type-check and build monitor production assets.

## Coding Style & Naming Conventions
- Python: PEP 8, 4-space indentation, type hints for public/service APIs, `snake_case` for functions/modules, `PascalCase` for classes.
- TypeScript/React: functional components in `PascalCase` (for example `ModelManagement.tsx`), hooks prefixed with `use`.
- Keep provider-specific behavior isolated inside `src/llm_router/providers/*` and avoid cross-provider branching in route handlers.

## Testing Guidelines
- Framework: `pytest` with `pytest-asyncio` (`asyncio_mode = auto`).
- Place tests in `tests/` as `test_*.py`; mirror package/function names where practical.
- Add/update tests for any API contract, routing decision, auth rule, or provider integration change.
- Prefer focused tests with explicit fixtures in `tests/conftest.py` and run `uv run pytest` before opening a PR.

## Commit & Pull Request Guidelines
- Follow Conventional Commit style seen in history (`feat:`, `refactor:`, `release:`), e.g. `feat: add provider retry backoff`.
- Keep commits scoped and atomic; separate refactors from behavior changes.
- PRs should include: purpose, key files changed, test commands run + results, config/env impacts (`router.toml`, `.env`, ports).
- Include screenshots/GIFs for `examples/monitor/` UI changes and link related issues/tasks.

## Security & Configuration Tips
- Never commit real API keys; use `.env` (from `.env.example`) and `api_key_env` in `router.toml`.
- Validate `router.toml` changes carefully, especially provider names, model IDs, and monitor/server port alignment.
