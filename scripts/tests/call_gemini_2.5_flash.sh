#!/usr/bin/env fish
# 调用 gemini-2.5-flash 的示例脚本
#
# 前置条件：
# 1. 确保服务已启动: uv run llm-router
# 2. 确保已同步配置: uv run python sync_config.py
# 3. 确保已设置 GEMINI_API_KEY 环境变量（在 .env 文件中或导出到环境）

# 设置变量
set API_URL "http://localhost:18000"
set PROVIDER "gemini"
set MODEL "gemini-2.5-flash"

# 如果启用了认证，设置 API Key（从环境变量读取）
# set API_KEY $LLM_ROUTER_ADMIN_KEY

# 检查服务是否运行
if not curl -s "$API_URL/health" > /dev/null
    echo "❌ 错误: 服务未运行，请先启动服务: uv run llm-router"
    exit 1
end

echo "调用 $PROVIDER/$MODEL 模型..."

# 方式1: 使用 prompt 参数（简单文本提示）
echo "=== 方式1: 使用 prompt 参数 ==="
curl -X POST "$API_URL/models/$PROVIDER/$MODEL/invoke" \
  -H "Content-Type: application/json" \
  (if set -q API_KEY; echo "-H \"Authorization: Bearer $API_KEY\""; end) \
  -d '{
    "prompt": "What is the capital of France?",
    "parameters": {
      "temperature": 0.7,
      "max_tokens": 150
    }
  }'

echo -e "\n\n"

# 方式2: 使用 messages 参数（对话格式）
echo "=== 方式2: 使用 messages 参数 ==="
curl -X POST "$API_URL/models/$PROVIDER/$MODEL/invoke" \
  -H "Content-Type: application/json" \
  (if set -q API_KEY; echo "-H \"Authorization: Bearer $API_KEY\""; end) \
  -d '{
    "messages": [
      {
        "role": "user",
        "content": "请用中文解释什么是人工智能"
      }
    ],
    "parameters": {
      "temperature": 0.8,
      "max_tokens": 500
    }
  }'

echo -e "\n\n"

# 方式3: 多轮对话
echo "=== 方式3: 多轮对话 ==="
curl -X POST "$API_URL/models/$PROVIDER/$MODEL/invoke" \
  -H "Content-Type: application/json" \
  (if set -q API_KEY; echo "-H \"Authorization: Bearer $API_KEY\""; end) \
  -d '{
    "messages": [
      {
        "role": "user",
        "content": "Hello, how are you?"
      },
      {
        "role": "assistant",
        "content": "I'\''m doing well, thank you! How can I help you today?"
      },
      {
        "role": "user",
        "content": "Can you explain quantum computing in simple terms?"
      }
    ],
    "parameters": {
      "temperature": 0.7
    }
  }'

echo -e "\n\n"

# 方式4: 使用路由功能（智能选择模型）
echo "=== 方式4: 使用路由功能 ==="
curl -X POST "$API_URL/route/invoke" \
  -H "Content-Type: application/json" \
  (if set -q API_KEY; echo "-H \"Authorization: Bearer $API_KEY\""; end) \
  -d '{
    "query": {
      "tags": ["fast", "chat"],
      "provider_types": ["gemini"]
    },
    "request": {
      "prompt": "Write a short poem about technology",
      "parameters": {
        "temperature": 0.9
      }
    }
  }'

