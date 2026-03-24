# llm-router Go backend (bootstrap)

This directory contains the initial Go implementation for backend migration.

## Implemented in this milestone

- Go service entrypoint (`cmd/llm-router`)
- HTTP router with `/health` + explicit `501` fallback for not-yet-migrated routes
- PostgreSQL connection bootstrap
- Startup bootstrap migration framework:
  - Creates required PostgreSQL tables
  - Imports data from SQLite main/monitor databases once
  - Writes idempotent bootstrap marker (`go_bootstrap_migrations`)
- `scripts/start.sh` compatibility toggle:
  - default Go backend
  - set `LLM_ROUTER_BACKEND_IMPL=python` to start Python backend
  - preflight PostgreSQL reachability check before Go backend startup

## Environment variables

- `LLM_ROUTER_BACKEND_IMPL` (default `go`; set `python` to switch launcher in `scripts/start.sh`)
- `LLM_ROUTER_HOST` (default `0.0.0.0`)
- `LLM_ROUTER_PORT` (default `8000`)
- `LLM_ROUTER_PG_DSN` or `LLM_ROUTER_POSTGRES_DSN`
- `LLM_ROUTER_MIGRATE_FROM_SQLITE` (default `true`)
- `LLM_ROUTER_SQLITE_MAIN_PATH` (default `data/llm_router.db`)
- `LLM_ROUTER_SQLITE_MONITOR_PATH` (default `data/llm_datas.db`)

## Run (when Go toolchain is installed)

```bash
cd backend
go mod tidy
go run ./cmd/llm-router
```
