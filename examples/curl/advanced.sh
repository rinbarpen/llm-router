#!/bin/bash
# 复杂调用示例
# 演示认证、路由、多模态、流式响应等高级功能

BASE_URL="${LLM_ROUTER_BASE_URL:-http://localhost:18000}"
API_KEY="${LLM_ROUTER_API_KEY:-}"

echo "============================================================"
echo "LLM Router 复杂调用示例"
echo "============================================================"
echo

# 1. 认证流程
if [ -n "$API_KEY" ]; then
    echo "1. 登录获取 Session Token"
    echo "------------------------------------------------------------"
    TOKEN=$(curl -s -X POST "${BASE_URL}/auth/login" \
      -H "Content-Type: application/json" \
      -d "{\"api_key\": \"${API_KEY}\"}" | jq -r '.token')
    
    if [ -n "$TOKEN" ] && [ "$TOKEN" != "null" ]; then
        echo "✓ 登录成功，Token: ${TOKEN:0:20}..."
        echo
        
        echo "2. 使用 Token 调用 OpenAI 兼容 API（自动绑定模型）"
        echo "------------------------------------------------------------"
        echo "注意: 模型绑定在首次调用时自动完成"
        curl -s -X POST "${BASE_URL}/v1/chat/completions" \
          -H "Content-Type: application/json" \
          -H "Authorization: Bearer ${TOKEN}" \
          -d '{
            "model": "openrouter/llama-3.3-70b-instruct",
            "messages": [
              {"role": "user", "content": "Test binding"}
            ]
          }' | jq '{content: .choices[0].message.content}'
        echo
        
        echo "3. 使用 Token 调用模型"
        echo "------------------------------------------------------------"
        curl -s -X POST "${BASE_URL}/models/openrouter/llama-3.3-70b-instruct/invoke" \
          -H "Content-Type: application/json" \
          -H "Authorization: Bearer ${TOKEN}" \
          -d '{
            "prompt": "What is Python?",
            "parameters": {
              "temperature": 0.7,
              "max_tokens": 200
            }
          }' | jq '{output: .output_text, tokens: .raw.usage.total_tokens}'
        echo
    else
        echo "✗ 登录失败，请检查 API Key"
        echo
    fi
else
    echo "⚠ 未设置 LLM_ROUTER_API_KEY，跳过认证示例"
    echo
fi

# 4. 获取 routing pairs
echo "4. 获取 strong/weak 模型对列表"
echo "------------------------------------------------------------"
curl -s "${BASE_URL}/route/pairs" | jq .
echo

# 5. 智能路由
echo "5. 智能路由（根据标签自动选择模型）"
echo "------------------------------------------------------------"
curl -s -X POST "${BASE_URL}/route/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "query": {
      "tags": ["free", "fast"]
    },
    "request": {
      "prompt": "What is 2+2?",
      "parameters": {
        "temperature": 0.1,
        "max_tokens": 50
      }
    }
  }' | jq '{output: .output_text, model: .raw.model}'
echo

# 5b. 使用 routing_pair 指定模型对（需 router.toml 配置 [[routing.pairs]]）
echo "5b. 使用 routing_pair 调用"
echo "------------------------------------------------------------"
curl -s -X POST "${BASE_URL}/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "strong",
    "routing_mode": "strong",
    "routing_pair": "gemini-3",
    "messages": [{"role": "user", "content": "What is 2+2?"}],
    "max_tokens": 50
  }' | jq '{content: .choices[0].message.content}'
echo

# 6. OpenAI 兼容 API
echo "6. OpenAI 兼容 API"
echo "------------------------------------------------------------"
echo "6a. Provider 在路径中（model 只需模型名）"
curl -s -X POST "${BASE_URL}/openrouter/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama-3.3-70b-instruct",
    "messages": [{"role": "user", "content": "Say hi"}],
    "max_tokens": 30
  }' | jq '{content: .choices[0].message.content}'
echo
echo "6b. 标准端点（model 为 provider/model）"
curl -s -X POST "${BASE_URL}/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "openrouter/llama-3.3-70b-instruct",
    "messages": [
      {
        "role": "user",
        "content": "Hello! How are you?"
      }
    ],
    "temperature": 0.7,
    "max_tokens": 100
  }' | jq '{content: .choices[0].message.content, tokens: .usage.total_tokens}'
echo

# 7. 流式响应（OpenAI 兼容）
echo "7. 流式响应（OpenAI 兼容）"
echo "------------------------------------------------------------"
echo "注意: 流式响应使用 Server-Sent Events 格式"
curl -s -X POST "${BASE_URL}/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "openrouter/llama-3.3-70b-instruct",
    "messages": [
      {"role": "user", "content": "Count from 1 to 5"}
    ],
    "stream": true,
    "max_tokens": 50
  }' | grep -o '"content":"[^"]*"' | head -5
echo

echo "提示:"
echo "- 认证流程：登录 -> 绑定模型 -> 使用 Token"
echo "- GET /route/pairs 可获取配置的 strong/weak 模型对"
echo "- routing_pair 参数可指定 [[routing.pairs]] 中的 pair 名称"
echo "- OpenAI 兼容 API 可以无缝替换 OpenAI SDK"

