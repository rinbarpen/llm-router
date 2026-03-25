#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${LLM_ROUTER_BASE_URL:-http://127.0.0.1:18000}"
API_KEY="${LLM_ROUTER_API_KEY:-}"

header_auth=()
if [[ -n "${API_KEY}" ]]; then
  header_auth=( -H "Authorization: Bearer ${API_KEY}" )
fi

echo "[1/4] health"
curl -fsS "${BASE_URL}/health" >/dev/null

echo "[2/4] models"
curl -fsS "${BASE_URL}/v1/models" "${header_auth[@]}" >/dev/null

echo "[3/4] route pairs"
curl -fsS "${BASE_URL}/route/pairs" "${header_auth[@]}" >/dev/null

echo "[4/4] chat completions (best effort)"
if ! curl -fsS "${BASE_URL}/v1/chat/completions" \
  -H 'Content-Type: application/json' \
  "${header_auth[@]}" \
  -d '{"model":"openai/gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}' >/dev/null; then
  echo "chat completion check skipped/failed (ensure model exists in router.toml)"
fi

echo "API smoke passed"
