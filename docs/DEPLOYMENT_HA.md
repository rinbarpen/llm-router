# 多机部署与高可用指南（Go Backend）

本文提供 LLM Router 的基础高可用部署范式：
- Docker Compose 多实例 + Nginx 反向代理
- Kubernetes 基础 Deployment/Service 示例
- 启动与运行时自检（`/health`、`/health/detail`）

## 1. Docker Compose 多实例

文件位置：
- `deploy/compose/docker-compose.ha.yml`
- `deploy/compose/nginx.ha.conf`

启动：

```bash
cd deploy/compose
docker compose -f docker-compose.ha.yml up -d --build
```

访问入口：
- Nginx 负载入口：`http://localhost:18080`
- 健康检查：`http://localhost:18080/health`
- 详细自检：`http://localhost:18080/health/detail`

说明：
- `llm-router-1` 与 `llm-router-2` 共用 PostgreSQL。
- Nginx `upstream` 默认 `least_conn`，并配置 `max_fails/fail_timeout`。
- SSE 接口通过代理透传，建议在生产网关适当增大 `read_timeout`。

## 2. Kubernetes 基础示例

文件位置：
- `deploy/k8s/llm-router-deployment.yaml`
- `deploy/k8s/llm-router-service.yaml`
- `deploy/k8s/llm-router-configmap-example.yaml`
- `deploy/k8s/llm-router-secrets-example.yaml`

应用顺序（示例）：

```bash
kubectl apply -f deploy/k8s/llm-router-configmap-example.yaml
kubectl apply -f deploy/k8s/llm-router-secrets-example.yaml
kubectl apply -f deploy/k8s/llm-router-deployment.yaml
kubectl apply -f deploy/k8s/llm-router-service.yaml
```

建议：
- `replicas >= 2`，并结合 HPA（后续可扩展）。
- 为 Deployment 增加 PodDisruptionBudget，避免维护窗口全量中断。
- 在 Ingress/网关层为流式接口设置更宽松超时。

## 3. 启动与运行时自检

### 3.1 基础健康检查

```bash
curl http://localhost:18000/health
```

### 3.2 详细自检

```bash
curl http://localhost:18000/health/detail
```

返回示例：

```json
{
  "overall_status": "ok",
  "checks": {
    "database": {"status": "ok"},
    "redis": {"status": "not_configured"},
    "upstream": {"status": "degraded", "note": "upstream active probe is not enabled in self-check yet"}
  }
}
```

状态建议：
- `overall_status=ok`：核心依赖可用。
- `overall_status=degraded`：服务可运行但存在风险项，建议告警并排查。

## 4. 会话与状态建议

- 运行态主状态统一落在 PostgreSQL，实例可无状态扩缩容。
- 若开启 Session Token 登录，建议前端统一走 API Key 或外部会话层，避免实例内会话粘连导致的体验不一致。
- 流式请求建议启用网关级连接数与速率限制，防止长连接压垮单节点。
