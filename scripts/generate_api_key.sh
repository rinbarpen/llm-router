#!/usr/bin/env bash
# 生成 LLM Router API Key（无 Python 依赖）

set -euo pipefail

LENGTH=32
COUNT=1
PREFIX=""
ENV_NAME=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --length)
      LENGTH="${2:-32}"
      shift 2
      ;;
    --count)
      COUNT="${2:-1}"
      shift 2
      ;;
    --prefix)
      PREFIX="${2:-}"
      shift 2
      ;;
    --env)
      ENV_NAME="${2:-}"
      shift 2
      ;;
    -h|--help)
      cat <<'EOF'
用法:
  ./scripts/generate_api_key.sh [--length N] [--count N] [--prefix STR] [--env VAR]
EOF
      exit 0
      ;;
    *)
      echo "错误: 未知参数 $1" >&2
      exit 1
      ;;
  esac
done

if ! [[ "${LENGTH}" =~ ^[0-9]+$ ]] || (( LENGTH < 8 )); then
  echo "错误: --length 必须为 >= 8 的整数" >&2
  exit 1
fi
if ! [[ "${COUNT}" =~ ^[0-9]+$ ]] || (( COUNT < 1 )); then
  echo "错误: --count 必须为 >= 1 的整数" >&2
  exit 1
fi

gen_key() {
  local body_len=$1
  local body
  body="$(openssl rand -base64 128 | tr -dc 'A-Za-z0-9_-' | head -c "${body_len}")"
  printf '%s%s\n' "${PREFIX}" "${body}"
}

if [[ -n "${ENV_NAME}" ]]; then
  if (( COUNT == 1 )); then
    printf '%s=%s\n' "${ENV_NAME}" "$(gen_key "${LENGTH}")"
  else
    keys=()
    for ((i=0; i<COUNT; i++)); do
      keys+=("$(gen_key "${LENGTH}")")
    done
    (IFS=,; printf '%s=%s\n' "${ENV_NAME}" "${keys[*]}")
  fi
  exit 0
fi

for ((i=0; i<COUNT; i++)); do
  gen_key "${LENGTH}"
done
