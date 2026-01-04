#!/bin/bash
# 简单调用示例
# 演示基础的 API 调用，包括健康检查、模型列表、基础调用等

BASE_URL="${LLM_ROUTER_BASE_URL:-http://localhost:18000}"
API_KEY="${LLM_ROUTER_API_KEY:-}"

# 颜色输出
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo "============================================================"
echo "LLM Router 简单调用示例"
echo "============================================================"
echo

# 1. 健康检查
echo "1. 健康检查"
echo "------------------------------------------------------------"
curl -s "${BASE_URL}/health" | jq .
echo

# 2. 获取模型列表
echo "2. 获取所有模型"
echo "------------------------------------------------------------"
curl -s "${BASE_URL}/models" | jq '.[0:3] | .[] | {name: .name, provider: .provider_name, tags: .tags}'
echo

# 3. 按标签过滤
echo "3. 获取免费模型"
echo "------------------------------------------------------------"
curl -s "${BASE_URL}/models?tags=free" | jq '.[0:3] | .[] | {name: .name, provider: .provider_name}'
echo

# 4. 基础调用
echo "4. 基础文本调用"
echo "------------------------------------------------------------"
curl -X POST "${BASE_URL}/models/openrouter/openrouter-llama-3.3-70b-instruct/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "What is the capital of France?",
    "parameters": {
      "temperature": 0.7,
      "max_tokens": 200
    }
  }' | jq '{output: .output_text, tokens: .raw.usage.total_tokens}'
echo

# 5. 使用 messages 格式
echo "5. 使用 messages 格式"
echo "------------------------------------------------------------"
curl -X POST "${BASE_URL}/models/openrouter/openrouter-llama-3.3-70b-instruct/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [
      {
        "role": "user",
        "content": "Explain quantum computing in simple terms"
      }
    ],
    "parameters": {
      "temperature": 0.7,
      "max_tokens": 300
    }
  }' | jq '{output: .output_text, tokens: .raw.usage.total_tokens}'
echo

echo "提示:"
echo "- 本机请求（localhost）默认免认证"
echo "- 远程请求需要设置 LLM_ROUTER_API_KEY 环境变量"
echo "- 使用 jq 格式化 JSON 输出（可选）"

