#!/usr/bin/env bash

set -euo pipefail

SERVICE_FILE="/etc/systemd/system/llm-router-pricing-sync.service"
TIMER_FILE="/etc/systemd/system/llm-router-pricing-sync.timer"

if [[ "$EUID" -ne 0 ]]; then
  echo "错误: 请使用 sudo 运行此脚本" >&2
  exit 1
fi

systemctl disable --now llm-router-pricing-sync.timer 2>/dev/null || true
systemctl stop llm-router-pricing-sync.service 2>/dev/null || true

rm -f "$SERVICE_FILE" "$TIMER_FILE"

systemctl daemon-reload

echo "已卸载: llm-router-pricing-sync.service / llm-router-pricing-sync.timer"
