# 模型标签 (Tags) 参考指南

LLM Router 使用标签系统来实现智能路由。通过为模型分配标签，你可以根据功能、性能或语言能力动态地选择最合适的模型，而无需在代码中硬编码特定的模型名称。

## 1. 标签命名规范

为了保证路由的准确性和配置的一致性，建议遵循以下命名规范：

- **格式**: 使用全小写字母，单词之间使用连字符 `-` 连接（例如 `high-quality`）。
- **交集匹配**: 路由接口 (`/route/invoke`) 执行的是标签的**交集匹配**。如果请求中指定了 `["chat", "fast"]`，则只有同时具备这两个标签的模型才会被选中。

## 2. 推荐标签分类

虽然系统支持自定义任何字符串作为标签，但建议参考以下分类进行标注：

### 功能特性 (Capabilities)
- `chat`: 标准对话/聊天模型。
- `reasoning`: 具有强化推理能力的模型（如 OpenAI o1, DeepSeek-R 系列）。
- `coding`: 针对编程、代码生成和补全进行了优化的模型。
- `math`: 擅长数学计算和复杂逻辑证明的模型。
- `vision`: 支持图像输入、分析和理解的视觉模型（多模态）。
- `audio`: 支持音频输入或生成的模型。
- `video`: 支持视频处理或生成的模型。
- `tool-calling` / `function-call`: 支持函数调用/工具使用的模型。
- `agent`: 具有较强指令遵循能力，适合作为 Agent 调用。

### 性能与成本 (Performance & Cost)
- `fast`: 响应速度较快的中型规模模型。
- `fastest`: 极速响应的小型或蒸馏模型（如 GPT-4o-mini, Flash 系列）。
- `high-quality`: 旗舰级模型，输出最准确、能力最强（如 Opus, GPT-4o, Pro 系列）。
- `affordable`: 成本极低的模型。
- `long-context`: 支持极长上下文窗口（如 128k, 1M+）。

### 语言能力 (Languages)
- `chinese`: 针对中文语境进行了深度优化。
- `english`: 针对英文语境优化。
- `multilingual`: 原生支持多种主流语言。

### 版本与状态 (Version & Status)
- `latest`: 当前提供商提供的最新主流版本。
- `legacy`: 旧版本模型，仅用于向下兼容。

## 3. 配置示例 (`router.toml`)

在配置文件中，你可以为每个模型定义一组标签：

```toml
[[models]]
name = "gpt-4o"
provider = "openai"
tags = ["chat", "high-quality", "vision", "latest"]

[[models]]
name = "deepseek-reasoner"
provider = "deepseek"
tags = ["chat", "reasoning", "coding", "math"]

[[models]]
name = "gemini-2.5-flash"
provider = "gemini"
tags = ["chat", "fastest", "multilingual", "latest"]
```

## 4. 路由调用示例

使用标签进行智能路由调用：

```bash
curl -X POST http://localhost:8000/route/invoke \
  -H "Content-Type: application/json" \
  -d '{
    "query": {
      "tags": ["reasoning", "coding"]
    },
    "request": {
      "messages": [{"role": "user", "content": "帮我写一个复杂的分布式锁算法逻辑"}]
    }
  }'
```

系统将自动在所有注册模型中寻找同时具备 `reasoning` 和 `coding` 标签的可用模型并执行任务。

## 5. 高级功能说明

### 工具调用 (Tool Calling)
对于支持 `tool-calling` 的模型，你可以通过 `parameters` 字段透传定义：

```bash
curl -X POST http://localhost:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "现在的天气如何？"}],
    "parameters": {
      "tools": [{
        "type": "function",
        "function": { "name": "get_weather", "parameters": {...} }
      }]
    }
  }'
```

### 多模态支持 (Multimodal)
标注为 `vision` 或 `audio` 的模型表示其后端具备处理非文本数据的能力。在当前版本中，建议通过 URL 或 Base64 编码在文本消息中描述，或者利用特定的 Provider `parameters` 进行扩展。
