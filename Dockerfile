# Multi-stage Dockerfile for LLM Router (Go backend + monitor UI)

# ============================================
# Stage 1: Monitor build
# ============================================
FROM node:20-alpine AS monitor-builder
WORKDIR /app

COPY router.toml ./
COPY examples/monitor/package.json examples/monitor/package-lock.json ./examples/monitor/
WORKDIR /app/examples/monitor
RUN npm ci --only=production=false
COPY examples/monitor/ ./
RUN npm run build

# ============================================
# Stage 2: Go backend build
# ============================================
FROM golang:1.24-alpine AS go-builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/llm-router ./cmd/llm-router

# ============================================
# Stage 3: Runtime
# ============================================
FROM debian:bookworm-slim AS runtime
WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

COPY --from=go-builder /out/llm-router /app/llm-router
COPY --from=monitor-builder /app/examples/monitor/dist /app/monitor/dist

COPY router.toml* ./
COPY .env.example ./.env.example
COPY docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod +x /app/docker-entrypoint.sh

RUN mkdir -p /app/data /app/model_store /app/monitor-dist

ENV LLM_ROUTER_HOST=0.0.0.0
ENV LLM_ROUTER_PORT=18000
ENV LLM_ROUTER_MODEL_STORE=/app/data/models

EXPOSE 18000

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:18000/health || exit 1

ENTRYPOINT ["/app/docker-entrypoint.sh"]
CMD ["/app/llm-router"]
