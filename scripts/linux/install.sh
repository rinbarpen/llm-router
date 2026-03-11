#!/bin/bash
# LLM Router Linux 服务安装脚本

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
SERVICE_DIR="/etc/systemd/system"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}LLM Router Linux 服务安装脚本${NC}"
echo ""

# 检查是否为 root
if [ "$EUID" -ne 0 ]; then 
    echo -e "${RED}错误: 请使用 sudo 运行此脚本${NC}"
    exit 1
fi

# 获取当前用户（运行脚本的用户）
INSTALL_USER="${SUDO_USER:-$USER}"
INSTALL_HOME=$(eval echo ~$INSTALL_USER)

echo -e "${YELLOW}安装信息:${NC}"
echo "  项目目录: $PROJECT_ROOT"
echo "  安装用户: $INSTALL_USER"
echo "  用户目录: $INSTALL_HOME"
echo ""

# 检查项目目录是否存在
if [ ! -d "$PROJECT_ROOT" ]; then
    echo -e "${RED}错误: 项目目录不存在: $PROJECT_ROOT${NC}"
    exit 1
fi

# 检查 uv 是否安装
if ! command -v uv &> /dev/null; then
    echo -e "${YELLOW}警告: 未找到 uv，请确保已安装${NC}"
fi

# 检查 npm 是否安装（监控界面需要）
if ! command -v npm &> /dev/null; then
    echo -e "${YELLOW}警告: 未找到 npm，监控界面服务将无法启动${NC}"
fi

# 询问安装哪些服务
echo -e "${YELLOW}请选择要安装的服务:${NC}"
echo "1) 仅后端服务"
echo "2) 仅监控界面服务"
echo "3) 后端 + 监控界面服务"
read -p "请选择 (1-3): " choice

BACKEND_INSTALL=false
FRONTEND_INSTALL=false

case $choice in
    1)
        BACKEND_INSTALL=true
        ;;
    2)
        FRONTEND_INSTALL=true
        ;;
    3)
        BACKEND_INSTALL=true
        FRONTEND_INSTALL=true
        ;;
    *)
        echo -e "${RED}无效选择${NC}"
        exit 1
        ;;
esac

# 安装后端服务
if [ "$BACKEND_INSTALL" = true ]; then
    echo ""
    echo -e "${GREEN}安装后端服务...${NC}"
    
    # 创建服务文件
    cat > "$SERVICE_DIR/llm-router-backend.service" <<EOF
[Unit]
Description=LLM Router Backend Service
After=network.target

[Service]
Type=simple
User=$INSTALL_USER
WorkingDirectory=$PROJECT_ROOT
Environment="PATH=$INSTALL_HOME/.local/bin:/usr/local/bin:/usr/bin:/bin"
ExecStart=$INSTALL_HOME/.local/bin/uv run llm-router
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

# 安全设置
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF

    echo -e "${GREEN}后端服务文件已创建${NC}"
fi

# 安装监控界面服务
if [ "$FRONTEND_INSTALL" = true ]; then
    echo ""
    echo -e "${GREEN}安装监控界面服务...${NC}"
    
    # 检查监控目录
    if [ ! -d "$PROJECT_ROOT/monitor" ]; then
        echo -e "${RED}错误: 监控目录不存在: $PROJECT_ROOT/monitor${NC}"
        exit 1
    fi
    
    # 创建服务文件
    cat > "$SERVICE_DIR/llm-router-monitor.service" <<EOF
[Unit]
Description=LLM Router Monitor Service
After=network.target llm-router-backend.service
Requires=llm-router-backend.service

[Service]
Type=simple
User=$INSTALL_USER
WorkingDirectory=$PROJECT_ROOT/monitor
Environment="PATH=$INSTALL_HOME/.local/bin:/usr/local/bin:/usr/bin:/bin"
Environment="NODE_ENV=production"
ExecStart=/usr/bin/npm run dev
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

# 安全设置
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF

    echo -e "${GREEN}监控界面服务文件已创建${NC}"
fi

# 重新加载 systemd
echo ""
echo -e "${GREEN}重新加载 systemd...${NC}"
systemctl daemon-reload

# 启用服务
if [ "$BACKEND_INSTALL" = true ]; then
    systemctl enable llm-router-backend.service
    echo -e "${GREEN}后端服务已启用（开机自启）${NC}"
fi

if [ "$FRONTEND_INSTALL" = true ]; then
    systemctl enable llm-router-monitor.service
    echo -e "${GREEN}监控界面服务已启用（开机自启）${NC}"
fi

echo ""
echo -e "${GREEN}安装完成！${NC}"
echo ""
echo "服务管理命令:"
if [ "$BACKEND_INSTALL" = true ]; then
    echo "  启动: sudo systemctl start llm-router-backend"
    echo "  停止: sudo systemctl stop llm-router-backend"
    echo "  状态: sudo systemctl status llm-router-backend"
    echo "  日志: sudo journalctl -u llm-router-backend -f"
fi
if [ "$FRONTEND_INSTALL" = true ]; then
    echo "  启动: sudo systemctl start llm-router-monitor"
    echo "  停止: sudo systemctl stop llm-router-monitor"
    echo "  状态: sudo systemctl status llm-router-monitor"
    echo "  日志: sudo journalctl -u llm-router-monitor -f"
fi
echo ""
echo "现在可以启动服务:"
if [ "$BACKEND_INSTALL" = true ]; then
    echo "  sudo systemctl start llm-router-backend"
fi
if [ "$FRONTEND_INSTALL" = true ]; then
    echo "  sudo systemctl start llm-router-monitor"
fi

