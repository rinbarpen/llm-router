# 多阶段构建 Dockerfile for LLM Router

# ============================================
# 阶段1: 监控界面构建
# ============================================
FROM node:20-alpine AS monitor-builder

# 设置工作目录为项目根目录
WORKDIR /app

# 先复制router.toml（监控界面构建时需要读取配置）
COPY router.toml ./

# 复制监控界面依赖文件
COPY monitor/package.json monitor/package-lock.json ./monitor/

# 进入监控目录安装依赖
WORKDIR /app/monitor
RUN npm ci --only=production=false

# 复制监控界面源代码
COPY monitor/ ./

# 构建监控界面（此时router.toml在/app目录，监控界面可以正确读取）
RUN npm run build

# ============================================
# 阶段2: 后端运行
# ============================================
FROM python:3.10-slim AS backend

# 设置工作目录
WORKDIR /app

# 安装系统依赖
RUN apt-get update && apt-get install -y \
    curl \
    && rm -rf /var/lib/apt/lists/*

# 安装 uv (Python包管理器)
RUN pip install --no-cache-dir uv

# 复制项目配置文件
COPY pyproject.toml uv.lock ./

# 复制后端源代码（uv sync 会构建本地包，必须先存在 src/）
COPY src/ ./src/

# 复制 README.md（pyproject.toml 中声明了 readme = "README.md"）
COPY README.md ./

# 安装Python依赖
RUN uv sync --frozen --no-dev

# 从监控界面构建阶段复制构建产物
COPY --from=monitor-builder /app/monitor/dist ./monitor/dist

# 复制配置文件（如果存在）
COPY router.toml* ./
COPY .env.example ./.env.example

# 创建数据目录、模型存储目录和监控界面共享目录
RUN mkdir -p /app/data /app/model_store /app/monitor-dist

# 复制启动脚本
COPY docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod +x /app/docker-entrypoint.sh

# 设置环境变量
ENV PYTHONPATH=/app
ENV PATH="/app/.venv/bin:$PATH"
ENV LLM_ROUTER_DATABASE_URL="sqlite+aiosqlite:////app/data/llm_router.db"
ENV LLM_ROUTER_MONITOR_DATABASE_URL="sqlite+aiosqlite:////app/data/llm_datas.db"
ENV LLM_ROUTER_MODEL_STORE="/app/data/models"

# 暴露端口
EXPOSE 18000

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:18000/health || exit 1

# 设置入口点
ENTRYPOINT ["/app/docker-entrypoint.sh"]

# 启动命令
CMD ["uv", "run", "llm-router"]
