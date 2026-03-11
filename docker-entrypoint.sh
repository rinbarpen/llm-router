#!/bin/bash
set -e

# 如果监控界面dist目录存在且共享volume已挂载，复制监控界面文件到共享volume
if [ -d "/app/monitor/dist" ] && [ -d "/app/monitor-dist" ]; then
    echo "复制监控界面构建产物到共享volume..."
    cp -r /app/monitor/dist/* /app/monitor-dist/
    echo "监控界面文件复制完成"
fi

# 启动后端服务
exec "$@"
