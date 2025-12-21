#!/bin/bash
# LLM Router macOS 服务卸载脚本

set -e

LAUNCH_AGENTS_DIR="$HOME/Library/LaunchAgents"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}LLM Router macOS 服务卸载脚本${NC}"
echo ""

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
    
    if launchctl list | grep -q "com.llmrouter.backend"; then
        launchctl unload "$LAUNCH_AGENTS_DIR/com.llmrouter.backend.plist" 2>/dev/null || true
        echo "后端服务已停止"
    fi
    
    if [ -f "$LAUNCH_AGENTS_DIR/com.llmrouter.backend.plist" ]; then
        rm "$LAUNCH_AGENTS_DIR/com.llmrouter.backend.plist"
        echo "后端服务文件已删除"
    fi
fi

# 停止并卸载前端服务
if [ "$FRONTEND_UNINSTALL" = true ]; then
    echo ""
    echo -e "${GREEN}卸载前端服务...${NC}"
    
    if launchctl list | grep -q "com.llmrouter.frontend"; then
        launchctl unload "$LAUNCH_AGENTS_DIR/com.llmrouter.frontend.plist" 2>/dev/null || true
        echo "前端服务已停止"
    fi
    
    if [ -f "$LAUNCH_AGENTS_DIR/com.llmrouter.frontend.plist" ]; then
        rm "$LAUNCH_AGENTS_DIR/com.llmrouter.frontend.plist"
        echo "前端服务文件已删除"
    fi
fi

echo ""
echo -e "${GREEN}卸载完成！${NC}"

