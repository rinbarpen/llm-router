#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SERVICE_DIR="/etc/systemd/system"

if [[ "$EUID" -ne 0 ]]; then
  echo "错误: 请使用 sudo 运行此脚本" >&2
  exit 1
fi

INSTALL_USER="${SUDO_USER:-$USER}"
INSTALL_HOME=$(eval echo "~$INSTALL_USER")

SERVICE_FILE="$SERVICE_DIR/llm-router-pricing-sync.service"
TIMER_FILE="$SERVICE_DIR/llm-router-pricing-sync.timer"

sed \
  -e "s|__USER__|$INSTALL_USER|g" \
  -e "s|__PROJECT_ROOT__|$PROJECT_ROOT|g" \
  -e "s|__USER_HOME__|$INSTALL_HOME|g" \
  "$SCRIPT_DIR/llm-router-pricing-sync.service" > "$SERVICE_FILE"

cp "$SCRIPT_DIR/llm-router-pricing-sync.timer" "$TIMER_FILE"

systemctl daemon-reload
systemctl enable --now llm-router-pricing-sync.timer

echo "已安装并启用定时器: llm-router-pricing-sync.timer"
echo "查看状态: sudo systemctl status llm-router-pricing-sync.timer"
echo "查看下次执行: sudo systemctl list-timers llm-router-pricing-sync.timer"
echo "手动执行一次: sudo systemctl start llm-router-pricing-sync.service"
