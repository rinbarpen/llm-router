---
name: feature-testing
description: 用于测试 LLM Router 的新功能，特别是 API 接口测试。包含功能验证、API 兼容性测试和环境准备流程。当需要验证新开发的 Provider、模型或路由逻辑时使用。
---

# Feature Testing Skill

## 概述
本技能旨在指导如何对 LLM Router 的新功能进行系统化测试，确保 API 行为符合预期。

## 测试流程

### 1. 环境准备
在开始测试前，请确保：
- **代理设置**：如果测试远程 Provider，执行 `proxy_on` 开启代理。
- **环境变量**：检查 `.env` 文件，确保 `LLM_ROUTER_BASE_URL` 和 `LLM_ROUTER_API_KEY` 已正确配置。
- **服务启动**：确保后端服务已运行（通常在 `http://localhost:18000`）。

### 2. 功能验证 (Manual/cURL)
使用 cURL 快速验证基础连通性。

**调用指定模型：**
```bash
curl -X POST "${LLM_ROUTER_BASE_URL}/models/{provider}/{model}/invoke" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${LLM_ROUTER_API_KEY}" \
  -d '{
    "prompt": "Hello, who are you?",
    "parameters": {"temperature": 0.7}
  }'
```

### 3. API 兼容性测试 (OpenAI 格式)
验证 OpenAI 兼容接口是否正常工作。

```bash
# 标准端点（model 为 provider/model）
curl -X POST "${LLM_ROUTER_BASE_URL}/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${LLM_ROUTER_API_KEY}" \
  -d '{"model": "{model_id}", "messages": [{"role": "user", "content": "Hello"}], "stream": false}'

# Provider 在路径中（model 只需模型名）
curl -X POST "${LLM_ROUTER_BASE_URL}/openrouter/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{"model": "glm-4.5-air", "messages": [{"role": "user", "content": "Hello"}]}'
```

### 4. 自动化回归测试
运行项目自带的测试套件：
```bash
uv run pytest tests/test_openai_api.py
```

## 关键检查点
- [ ] 响应状态码是否为 200 OK。
- [ ] `output_text` 是否包含预期的模型回复。
- [ ] `usage` 字段（tokens 消耗）是否正确返回。
- [ ] 错误处理：对于无效的 API Key 或不存在的模型，是否返回 401 或 404。

## 更多资源
- 详细 API 响应格式请参考 [reference.md](reference.md)。
- 快速测试脚本请使用 [scripts/test_feature.py](scripts/test_feature.py)。
