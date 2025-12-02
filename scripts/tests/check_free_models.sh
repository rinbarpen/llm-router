#!/usr/bin/env fish
# 检查哪些免费模型现在可以调用
#
# 前置条件：
# 1. 确保服务已启动: uv run llm-router
# 2. 确保已同步配置: uv run python sync_config.py

set API_URL "http://localhost:18000"

# 检查服务是否运行
if not curl -s "$API_URL/health" > /dev/null
    echo "❌ 错误: 服务未运行，请先启动服务: uv run llm-router"
    exit 1
end

echo "📋 获取免费模型列表..."
set FREE_MODELS (curl -s "$API_URL/models?tags=free" | python3 -c "
import sys, json
models = json.load(sys.stdin)
for m in models:
    print(f\"{m['provider_name']}|{m['name']}|{m.get('display_name', m['name'])}\")
")

if test -z "$FREE_MODELS"
    echo "⚠️  未找到带有 'free' 标签的模型"
    exit 0
end

echo "✅ 找到 "(count $FREE_MODELS)" 个免费模型"
echo ""
echo "🔍 正在测试模型可用性..."
echo ""

set AVAILABLE ""
set UNAVAILABLE ""

for model_line in $FREE_MODELS
    set parts (string split "|" $model_line)
    set provider $parts[1]
    set model_name $parts[2]
    set display_name $parts[3]
    
    echo -n "  测试 $provider/$model_name ... "
    
    set response (curl -s -X POST "$API_URL/models/$provider/$model_name/invoke" \
      -H "Content-Type: application/json" \
      -d '{
        "prompt": "hi",
        "parameters": {
          "max_tokens": 10,
          "temperature": 0.1
        }
      }' \
      -w "\n%{http_code}" \
      --max-time 15)
    
    set http_code (echo $response | tail -n 1)
    set body (echo $response | head -n -1)
    
    if test "$http_code" = "200"
        echo "✅ 可用"
        set AVAILABLE $AVAILABLE "$provider/$model_name|$display_name"
    else
        echo "❌ 不可用 (HTTP $http_code)"
        set UNAVAILABLE $UNAVAILABLE "$provider/$model_name|$display_name"
    end
end

echo ""
echo "=" (string repeat -n 80 "=")
echo "✅ 可用模型:"
echo "=" (string repeat -n 80 "=")

if test -n "$AVAILABLE"
    for model_info in $AVAILABLE
        set parts (string split "|" $model_info)
        echo "  ✓ $parts[1]"
        echo "    显示名称: $parts[2]"
        echo ""
    end
else
    echo "  (无)"
    echo ""
end

if test -n "$UNAVAILABLE"
    echo "=" (string repeat -n 80 "=")
    echo "❌ 不可用模型:"
    echo "=" (string repeat -n 80 "=")
    for model_info in $UNAVAILABLE
        set parts (string split "|" $model_info)
        echo "  ✗ $parts[1]"
        echo "    显示名称: $parts[2]"
        echo ""
    end
end

echo "=" (string repeat -n 80 "=")
set available_count (count $AVAILABLE)
set total_count (count $FREE_MODELS)
set unavailable_count (math $total_count - $available_count)
echo "📊 总结:"
echo "   总模型数: $total_count"
echo "   可用: $available_count"
echo "   不可用: $unavailable_count"
echo "=" (string repeat -n 80 "=")

if test $available_count -eq 0
    exit 1
else
    exit 0
end

