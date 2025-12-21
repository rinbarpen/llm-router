#!/bin/bash
# LLM Router macOS 服务安装脚本

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
LAUNCH_AGENTS_DIR="$HOME/Library/LaunchAgents"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}LLM Router macOS 服务安装脚本${NC}"
echo ""

INSTALL_USER="$USER"
INSTALL_HOME="$HOME"

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

# 检查 npm 是否安装（前端需要）
if ! command -v npm &> /dev/null; then
    echo -e "${YELLOW}警告: 未找到 npm，前端服务将无法启动${NC}"
fi

# 创建日志目录
mkdir -p "$PROJECT_ROOT/logs"

# 询问安装哪些服务
echo -e "${YELLOW}请选择要安装的服务:${NC}"
echo "1) 仅后端服务"
echo "2) 仅前端服务"
echo "3) 后端 + 前端服务"
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

# 获取 uv 路径
UV_PATH=$(which uv || echo "$INSTALL_HOME/.local/bin/uv")

# 安装后端服务
if [ "$BACKEND_INSTALL" = true ]; then
    echo ""
    echo -e "${GREEN}安装后端服务...${NC}"
    
    # 创建 plist 文件
    cat > "$LAUNCH_AGENTS_DIR/com.llmrouter.backend.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.llmrouter.backend</string>
    
    <key>ProgramArguments</key>
    <array>
        <string>$UV_PATH</string>
        <string>run</string>
        <string>llm-router</string>
    </array>
    
    <key>WorkingDirectory</key>
    <string>$PROJECT_ROOT</string>
    
    <key>RunAtLoad</key>
    <true/>
    
    <key>KeepAlive</key>
    <true/>
    
    <key>StandardOutPath</key>
    <string>$PROJECT_ROOT/logs/backend.log</string>
    
    <key>StandardErrorPath</key>
    <string>$PROJECT_ROOT/logs/backend.error.log</string>
    
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>$INSTALL_HOME/.local/bin:/usr/local/bin:/usr/bin:/bin</string>
    </dict>
</dict>
</plist>
EOF

    # 加载服务
    launchctl load "$LAUNCH_AGENTS_DIR/com.llmrouter.backend.plist" 2>/dev/null || true
    echo -e "${GREEN}后端服务已安装并启动${NC}"
fi

# 安装前端服务
if [ "$FRONTEND_INSTALL" = true ]; then
    echo ""
    echo -e "${GREEN}安装前端服务...${NC}"
    
    # 检查前端目录
    if [ ! -d "$PROJECT_ROOT/frontend" ]; then
        echo -e "${RED}错误: 前端目录不存在: $PROJECT_ROOT/frontend${NC}"
        exit 1
    fi
    
    # 获取 npm 路径
    NPM_PATH=$(which npm || echo "/usr/local/bin/npm")
    
    # 创建 plist 文件
    cat > "$LAUNCH_AGENTS_DIR/com.llmrouter.frontend.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.llmrouter.frontend</string>
    
    <key>ProgramArguments</key>
    <array>
        <string>$NPM_PATH</string>
        <string>run</string>
        <string>dev</string>
    </array>
    
    <key>WorkingDirectory</key>
    <string>$PROJECT_ROOT/frontend</string>
    
    <key>RunAtLoad</key>
    <true/>
    
    <key>KeepAlive</key>
    <true/>
    
    <key>StandardOutPath</key>
    <string>$PROJECT_ROOT/logs/frontend.log</string>
    
    <key>StandardErrorPath</key>
    <string>$PROJECT_ROOT/logs/frontend.error.log</string>
    
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>$INSTALL_HOME/.local/bin:/usr/local/bin:/usr/bin:/bin</string>
        <key>NODE_ENV</key>
        <string>production</string>
    </dict>
</dict>
</plist>
EOF

    # 加载服务
    launchctl load "$LAUNCH_AGENTS_DIR/com.llmrouter.frontend.plist" 2>/dev/null || true
    echo -e "${GREEN}前端服务已安装并启动${NC}"
fi

echo ""
echo -e "${GREEN}安装完成！${NC}"
echo ""
echo "服务管理命令:"
if [ "$BACKEND_INSTALL" = true ]; then
    echo "  启动: launchctl start com.llmrouter.backend"
    echo "  停止: launchctl stop com.llmrouter.backend"
    echo "  状态: launchctl list | grep llmrouter"
    echo "  日志: tail -f $PROJECT_ROOT/logs/backend.log"
fi
if [ "$FRONTEND_INSTALL" = true ]; then
    echo "  启动: launchctl start com.llmrouter.frontend"
    echo "  停止: launchctl stop com.llmrouter.frontend"
    echo "  状态: launchctl list | grep llmrouter"
    echo "  日志: tail -f $PROJECT_ROOT/logs/frontend.log"
fi

