#!/bin/bash
# 企业级使用示例
# 演示批量处理、监控、API Key 管理等企业级功能

BASE_URL="${LLM_ROUTER_BASE_URL:-http://localhost:18000}"
API_KEY="${LLM_ROUTER_API_KEY:-}"

echo "============================================================"
echo "LLM Router 企业级使用示例"
echo "============================================================"
echo

# 1. 批量调用（使用循环）
echo "1. 批量调用示例"
echo "------------------------------------------------------------"
PROMPTS=(
    "What is Python?"
    "What is JavaScript?"
    "What is Rust?"
)

for prompt in "${PROMPTS[@]}"; do
    echo "处理: ${prompt}"
    curl -s -X POST "${BASE_URL}/models/openrouter/openrouter-llama-3.3-70b-instruct/invoke" \
      -H "Content-Type: application/json" \
      -d "{
        \"prompt\": \"${prompt}\",
        \"parameters\": {
          \"temperature\": 0.7,
          \"max_tokens\": 100
        }
      }" | jq -r '.output_text' | head -c 100
    echo "..."
    echo
done

# 2. 获取调用历史
if [ -n "$API_KEY" ]; then
    echo "2. 获取调用历史"
    echo "------------------------------------------------------------"
    curl -s -X GET "${BASE_URL}/monitor/invocations?limit=5" \
      -H "Authorization: Bearer ${API_KEY}" | jq '.[] | {id: .id, model: "\(.provider_name)/\(.model_name)", status: .status, tokens: .total_tokens}'
    echo
    
    # 3. 获取统计信息
    echo "3. 获取使用统计（24小时）"
    echo "------------------------------------------------------------"
    curl -s -X GET "${BASE_URL}/monitor/statistics?time_range=24h" \
      -H "Authorization: Bearer ${API_KEY}" | jq '.overall | {total_calls: .total_calls, success_rate: .success_rate, total_tokens: .total_tokens}'
    echo
    
    # 4. API Key 管理（列出所有 Key）
    echo "4. 列出所有 API Key"
    echo "------------------------------------------------------------"
    curl -s -X GET "${BASE_URL}/api-keys" \
      -H "Authorization: Bearer ${API_KEY}" | jq '.[] | {id: .id, name: .name, is_active: .is_active}'
    echo
else
    echo "⚠ 未设置 LLM_ROUTER_API_KEY，跳过需要认证的示例"
    echo
fi

echo "提示:"
echo "- 批量处理可以使用并行请求提高效率"
echo "- 监控功能可以帮助追踪 API 使用情况"
echo "- API Key 管理需要管理员权限"

