#!/bin/bash
# LLM Router Linux 服务卸载脚本

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}LLM Router Linux 服务卸载脚本${NC}"
echo ""

# 检查是否为 root
if [ "$EUID" -ne 0 ]; then 
    echo -e "${RED}错误: 请使用 sudo 运行此脚本${NC}"
    exit 1
fi

# 询问卸载哪些服务
echo -e "${YELLOW}请选择要卸载的服务:${NC}"
echo "1) 仅后端服务"
echo "2) 仅前端服务"
echo "3) 后端 + 前端服务"
read -p "请选择 (1-3): " choice

BACKEND_UNINSTALL=false
FRONTEND_UNINSTALL=false

case $choice in
    1)
        BACKEND_UNINSTALL=true
        ;;
    2)
        FRONTEND_UNINSTALL=true
        ;;
    3)
        BACKEND_UNINSTALL=true
        FRONTEND_UNINSTALL=true
        ;;
    *)
        echo -e "${RED}无效选择${NC}"
        exit 1
        ;;
esac

# 停止并卸载后端服务
if [ "$BACKEND_UNINSTALL" = true ]; then
    echo ""
    echo -e "${GREEN}卸载后端服务...${NC}"
    
    if systemctl is-active --quiet llm-router-backend.service 2>/dev/null; then
        systemctl stop llm-router-backend.service
        echo "后端服务已停止"
    fi
    
    if systemctl is-enabled --quiet llm-router-backend.service 2>/dev/null; then
        systemctl disable llm-router-backend.service
        echo "后端服务已禁用"
    fi
    
    if [ -f "/etc/systemd/system/llm-router-backend.service" ]; then
        rm /etc/systemd/system/llm-router-backend.service
        echo "后端服务文件已删除"
    fi
fi

# 停止并卸载前端服务
if [ "$FRONTEND_UNINSTALL" = true ]; then
    echo ""
    echo -e "${GREEN}卸载前端服务...${NC}"
    
    if systemctl is-active --quiet llm-router-frontend.service 2>/dev/null; then
        systemctl stop llm-router-frontend.service
        echo "前端服务已停止"
    fi
    
    if systemctl is-enabled --quiet llm-router-frontend.service 2>/dev/null; then
        systemctl disable llm-router-frontend.service
        echo "前端服务已禁用"
    fi
    
    if [ -f "/etc/systemd/system/llm-router-frontend.service" ]; then
        rm /etc/systemd/system/llm-router-frontend.service
        echo "前端服务文件已删除"
    fi
fi

# 重新加载 systemd
if [ "$BACKEND_UNINSTALL" = true ] || [ "$FRONTEND_UNINSTALL" = true ]; then
    echo ""
    echo -e "${GREEN}重新加载 systemd...${NC}"
    systemctl daemon-reload
fi

echo ""
echo -e "${GREEN}卸载完成！${NC}"

