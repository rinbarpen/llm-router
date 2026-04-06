# 渠道镜像与第三方代理：配置示例与排障

本文面向 OpenAI 兼容渠道（如 OpenRouter、镜像网关、自建代理）给出可直接落地的配置模板与排障路径。

## 1. 典型配置（单渠道多镜像账号）

```toml
[routing]
load_balance_strategy = "weighted"          # round_robin | weighted | least_failure
channel_fallback = ["openrouter", "openai"] # 主渠道失败后回退

[routing.provider_weights]
openrouter = 3
openai = 1

[routing.circuit_breaker]
enabled = true
failure_threshold = 3
cooldown_seconds = 30
half_open_max_requests = 1

[[providers]]
name = "openrouter"
type = "openrouter"
base_url = "https://openrouter.ai/api/v1"
api_key_env = "OPENROUTER_API_KEY"

[providers.settings]
# 推荐：主 key + 备用 key 组合，触发 429/5xx 后自动切换
accounts = [
  { name = "main",   api_key_env = "OPENROUTER_API_KEY",        is_default = true,  priority = 100, max_in_flight = 8, cooldown_seconds = 30 },
  { name = "backup", api_key_env = "OPENROUTER_API_KEY_BACKUP", is_default = false, priority = 80,  max_in_flight = 6, cooldown_seconds = 30 }
]
```

## 2. 第三方代理/镜像模板

```toml
[[providers]]
name = "openai-mirror-a"
type = "openai"
base_url = "https://mirror-a.example.com/v1"
api_key_env = "MIRROR_A_KEY"

[providers.settings]
accounts = [
  { name = "a-main", api_key_env = "MIRROR_A_KEY", priority = 100, max_requests = 120, per_seconds = 60 },
  { name = "a-bak",  api_key_env = "MIRROR_A_KEY_BAK", priority = 90, max_requests = 60, per_seconds = 60 }
]
```

建议：
- `base_url` 固定到 provider 的稳定入口（不要带 `/chat/completions`）。
- `accounts` 至少准备 1 个备份 key。
- 对高流量渠道设置 `max_requests/per_seconds/max_in_flight`，避免单 key 被打爆。

## 3. 最小验证命令

```bash
# 1) 服务健康
curl http://localhost:18000/health

# 2) 检查渠道实时状态（并发/失败率/延迟/熔断）
curl http://localhost:18000/monitor/channel-load

# 3) 压测一个会触发上游限流的模型，观察是否自动切换账号/渠道
curl -X POST http://localhost:18000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "openrouter/gpt-4o-mini",
    "messages": [{"role":"user","content":"hello"}],
    "max_tokens": 32
  }'
```

## 4. 常见故障与定位

### 4.1 现象：请求持续 429

排查顺序：
1. 检查 `providers.settings.accounts` 是否只有单 key。
2. 检查 key 是否配置了 `max_requests/per_seconds` 且过低。
3. 调 `GET /monitor/channel-load` 看目标 provider 的 `failure_rate` 与 `circuit_open`。
4. 若所有账号都在冷却，增大账号池或降低并发。

### 4.2 现象：请求直接 4xx（如 400/401/403）且不回退

说明：
- 业务/鉴权类错误默认不视为可重试，不会做渠道回退。

建议：
1. 校验 API key 与上游权限。
2. 校验 `model` 是否真实可用。
3. 校验请求体参数是否合法（如模型不支持 `response_format`）。

### 4.3 现象：某渠道长期不可用

排查：
1. 确认 `base_url` 可达且 TLS/证书正常。
2. 临时将 `failure_threshold` 调高，确认是否过敏熔断。
3. 调整 `cooldown_seconds` 与 `half_open_max_requests`，避免恢复探测过慢。

## 5. 生产配置建议

- 至少两层容错：`accounts` 内 key 级切换 + `channel_fallback` 渠道级切换。
- `weighted` 适合主备流量比例；`least_failure` 适合多镜像质量不稳定场景。
- 周期观察 `/monitor/channel-load`，按失败率和延迟调整权重。
