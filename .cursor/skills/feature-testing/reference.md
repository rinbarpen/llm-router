# API Reference & Test Templates

## 核心 API 响应格式

### 1. `/invoke` 接口响应
当调用 `/models/{provider}/{model}/invoke` 时，预期返回格式：

```json
{
  "output_text": "这是模型的回复内容",
  "raw": {
    "id": "...",
    "usage": {
      "prompt_tokens": 10,
      "completion_tokens": 20,
      "total_tokens": 30
    },
    "model": "..."
  }
}
```

### 2. OpenAI 兼容接口响应
当调用 `/v1/chat/completions` 或 `/{provider}/v1/chat/completions` 时，预期返回格式：

```json
{
  "id": "chatcmpl-...",
  "object": "chat.completion",
  "created": 1677652288,
  "model": "gpt-3.5-turbo",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello there!"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 9,
    "completion_tokens": 12,
    "total_tokens": 21
  }
}
```

## 常见错误码

| 状态码 | 含义 | 可能原因 |
| :--- | :--- | :--- |
| 401 | Unauthorized | API Key 缺失、错误或已过期 |
| 404 | Not Found | 模型 ID 不存在或 Provider 未配置 |
| 429 | Too Many Requests | 触发了限流策略 |
| 500 | Internal Server Error | 后端逻辑错误或上游 Provider API 故障 |
| 503 | Service Unavailable | 上游服务不可用 |

## 测试用例模板

### 用例 1：基础连通性测试
- **目标**：验证模型能正常接收请求并返回文本。
- **输入**：简单 prompt "1+1=?"
- **预期**：响应包含 "2"。

### 用例 2：流式输出测试
- **目标**：验证 `stream: true` 时能正确接收 SSE 流。
- **输入**：`{"stream": true, "messages": [...]}`
- **预期**：收到多个 `data: {...}` 块，最后以 `data: [DONE]` 结束。

### 用例 3：无效参数测试
- **目标**：验证错误处理逻辑。
- **输入**：设置 `temperature: 2.5` (超出范围)。
- **预期**：返回 400 或由上游透传的错误信息。
