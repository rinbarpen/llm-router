# Repository Guidelines

## Project Structure & Module Organization
- Go backend entrypoint: `cmd/llm-router/`.
- API layer: `src/api/`.
- Core services/routing logic: `src/services/`.
- Provider adapters: `src/providers/`.
- Persistence/config/migration: `src/db/`, `src/config/`, `src/migrate/`.
- Frontend monitor (React + Vite + TS): `examples/monitor/`.
- Utility/manual scripts: `scripts/`, `examples/`.

## Build, Test, and Development Commands
- `go mod download`: install Go dependencies.
- `go run ./cmd/llm-router`: start backend server.
- `./scripts/start.sh`: start backend + monitor in local dev mode.
- `./scripts/start.sh backend` / `./scripts/start.sh monitor`: start one side only.
- `go test ./...`: run Go regression suite.
- `cd examples/monitor && npm install && npm run dev`: run monitor UI locally.
- `cd examples/monitor && npm run build`: type-check and build monitor production assets.

## Coding Style & Naming Conventions
- Go: `gofmt`, short focused packages, `camelCase` identifiers, exported symbols with clear doc comments.
- TypeScript/React: functional components in `PascalCase` (for example `ModelManagement.tsx`), hooks prefixed with `use`.
- Keep provider-specific behavior isolated inside `src/providers/*` and avoid cross-provider branching in route handlers.

## Testing Guidelines
- Framework: Go `testing` package.
- Place tests alongside packages or under `src/*` as `*_test.go`.
- Add/update tests for any API contract, routing decision, auth rule, or provider integration change.
- Run `go test ./...` before opening a PR.

## Commit & Pull Request Guidelines
- Follow Conventional Commit style seen in history (`feat:`, `refactor:`, `release:`), e.g. `feat: add provider retry backoff`.
- Keep commits scoped and atomic; separate refactors from behavior changes.
- PRs should include: purpose, key files changed, test commands run + results, config/env impacts (`router.toml`, `.env`, ports).
- Include screenshots/GIFs for `examples/monitor/` UI changes and link related issues/tasks.

## Security & Configuration Tips
- Never commit real API keys; use `.env` (from `.env.example`) and `api_key_env` in `router.toml`.
- Validate `router.toml` changes carefully, especially provider names, model IDs, and monitor/server port alignment.
