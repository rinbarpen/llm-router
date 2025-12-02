#!/usr/bin/env fish
# 测试 openrouter-glm-4.5-air 模型的脚本
#
# 前置条件：
# 1. 确保服务已启动: uv run llm-router
# 2. 确保已同步配置: uv run python sync_config.py
# 3. 确保已设置 OPENROUTER_API_KEY 环境变量（在 .env 文件中或导出到环境）

# 设置变量
set API_URL "http://localhost:18000"
set PROVIDER "openrouter"
set MODEL "openrouter-glm-4.5-air"

# 如果启用了认证，设置 API Key（从环境变量读取）
# set API_KEY $LLM_ROUTER_ADMIN_KEY

# 检查服务是否运行
if not curl -s "$API_URL/health" > /dev/null
    echo "❌ 错误: 服务未运行，请先启动服务: uv run llm-router"
    exit 1
end

echo "测试 $PROVIDER/$MODEL 模型..."
echo ""

# 方式1: 使用 prompt 参数（简单文本提示 - 中文）
echo "=== 方式1: 使用 prompt 参数（中文测试） ==="
curl -X POST "$API_URL/models/$PROVIDER/$MODEL/invoke" \
  -H "Content-Type: application/json" \
  (if set -q API_KEY; echo "-H \"Authorization: Bearer $API_KEY\""; end) \
  -d '{
    "prompt": "请用中文简单介绍一下人工智能的发展历史",
    "parameters": {
      "temperature": 0.7,
      "max_tokens": 500
    }
  }'

echo -e "\n\n"

# 方式2: 使用 messages 参数（对话格式 - 中文）
echo "=== 方式2: 使用 messages 参数（中文对话） ==="
curl -X POST "$API_URL/models/$PROVIDER/$MODEL/invoke" \
  -H "Content-Type: application/json" \
  (if set -q API_KEY; echo "-H \"Authorization: Bearer $API_KEY\""; end) \
  -d '{
    "messages": [
      {
        "role": "user",
        "content": "请解释一下什么是机器学习，并给出一个简单的例子"
      }
    ],
    "parameters": {
      "temperature": 0.8,
      "max_tokens": 600
    }
  }'

echo -e "\n\n"

# 方式3: 多轮对话（中英文混合）
echo "=== 方式3: 多轮对话（中英文混合） ==="
curl -X POST "$API_URL/models/$PROVIDER/$MODEL/invoke" \
  -H "Content-Type: application/json" \
  (if set -q API_KEY; echo "-H \"Authorization: Bearer $API_KEY\""; end) \
  -d '{
    "messages": [
      {
        "role": "user",
        "content": "Hello, can you speak both Chinese and English?"
      },
      {
        "role": "assistant",
        "content": "Yes, I can communicate in both Chinese and English. How can I help you?"
      },
      {
        "role": "user",
        "content": "请用中文解释一下深度学习和神经网络的区别"
      }
    ],
    "parameters": {
      "temperature": 0.7
    }
  }'

echo -e "\n\n"

# 方式4: 编程相关测试（GLM 擅长中文和编程）
echo "=== 方式4: 编程相关测试 ==="
curl -X POST "$API_URL/models/$PROVIDER/$MODEL/invoke" \
  -H "Content-Type: application/json" \
  (if set -q API_KEY; echo "-H \"Authorization: Bearer $API_KEY\""; end) \
  -d '{
    "messages": [
      {
        "role": "user",
        "content": "请用 Python 写一个快速排序算法，并添加中文注释"
      }
    ],
    "parameters": {
      "temperature": 0.3,
      "max_tokens": 800
    }
  }'

echo -e "\n\n"

# 方式5: 使用路由功能（通过标签选择模型）
echo "=== 方式5: 使用路由功能（通过标签选择） ==="
curl -X POST "$API_URL/route/invoke" \
  -H "Content-Type: application/json" \
  (if set -q API_KEY; echo "-H \"Authorization: Bearer $API_KEY\""; end) \
  -d '{
    "query": {
      "tags": ["chinese", "free", "fast"],
      "provider_types": ["openrouter"]
    },
    "request": {
      "prompt": "写一首关于春天的短诗",
      "parameters": {
        "temperature": 0.9,
        "max_tokens": 300
      }
    }
  }'

echo -e "\n\n"
echo "✅ 测试完成！"

