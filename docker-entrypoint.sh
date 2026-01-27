#!/bin/bash
set -e

# 如果前端dist目录存在且共享volume已挂载，复制前端文件到共享volume
if [ -d "/app/frontend/dist" ] && [ -d "/app/frontend-dist" ]; then
    echo "复制前端构建产物到共享volume..."
    cp -r /app/frontend/dist/* /app/frontend-dist/
    echo "前端文件复制完成"
fi

# 启动后端服务
exec "$@"
