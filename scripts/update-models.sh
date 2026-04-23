#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

if [[ -f "$REPO_ROOT/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$REPO_ROOT/.env"
  set +a
fi

show_usage() {
  cat <<EOF
LLM Router model update

Usage:
  $0 [--all] [--provider NAME_OR_TYPE ...] [--config PATH] [--source-dir DIR] [--dry-run] [--json]

Options:
  -p, --provider NAME_OR_TYPE  Update providers matching this name or type. Repeatable.
      --all                    Update all providers (default when no provider is supplied).
      --config PATH            router.toml path. Defaults to LLM_ROUTER_MODEL_CONFIG or router.toml.
      --source-dir DIR         Static model source directory. Defaults to [model_updates].source_dir.
      --dry-run                Compute changes without deleting DB rows or writing router.toml.
      --json                   Emit JSON.
  -h, --help                   Show this help.

Examples:
  $0 --provider openrouter
  $0 --provider "qwen (cn)" --provider gemini
  $0 --all --json
  $0 --dry-run --provider claude
EOF
}

args=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    -p|--provider)
      if [[ $# -lt 2 ]]; then
        echo "missing value for $1" >&2
        exit 2
      fi
      args+=(--provider "$2")
      shift 2
      ;;
    --all|--dry-run|--json)
      args+=("$1")
      shift
      ;;
    --config|--source-dir|--timeout)
      if [[ $# -lt 2 ]]; then
        echo "missing value for $1" >&2
        exit 2
      fi
      args+=("$1" "$2")
      shift 2
      ;;
    -h|--help)
      show_usage
      exit 0
      ;;
    *)
      echo "unknown option: $1" >&2
      show_usage >&2
      exit 2
      ;;
  esac
done

cd "$REPO_ROOT"
exec go run ./cmd/update-models "${args[@]}"
