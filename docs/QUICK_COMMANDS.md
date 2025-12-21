# LLM Router 常用请求指令

## 基础信息

- **API 地址**: `http://localhost:18000`
- **健康检查**: `GET /health`
- **认证**: 本机（localhost/127.0.0.1）默认免认证；远程或启用认证时需在请求头加入 `Authorization: Bearer <token 或 api_key>`.

## 免费模型列表

| Provider | Model Name | Display Name | 特点 |
|----------|-----------|--------------|------|
| openrouter | openrouter-llama-3.3-70b-instruct | Llama 3.3 70B Instruct | 高质量，开源 |
| openrouter | openrouter-gemma-3-27b-it | Gemma 3 27B IT | Google，指令调优 |
| openrouter | openrouter-glm-4.5-air | GLM-4.5 Air | 中文支持，快速 |
| openrouter | openrouter-grok-4.1-fast | Grok 4.1 Fast | xAI，快速响应 |

---

## 1. 健康检查

```bash
curl http://localhost:18000/health
```

---

## 2. 获取模型列表

### 获取所有模型
```bash
curl http://localhost:18000/models
```

### 获取免费模型
```bash
curl "http://localhost:18000/models?tags=free"
```

### 获取中文模型
```bash
curl "http://localhost:18000/models?tags=chinese"
```

### 获取 OpenRouter 模型
```bash
curl "http://localhost:18000/models?provider_types=openrouter"
```

---

## 3. 调用模型 - 简单文本提示

### 基础调用（英文）
```bash
curl -X POST "http://localhost:18000/models/openrouter/openrouter-llama-3.3-70b-instruct/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "What is the capital of France?",
    "parameters": {
      "temperature": 0.7,
      "max_tokens": 200
    }
  }'
```

### 中文调用（使用 GLM-4.5 Air）
```bash
curl -X POST "http://localhost:18000/models/openrouter/openrouter-glm-4.5-air/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "请用中文简单介绍一下人工智能",
    "parameters": {
      "temperature": 0.7,
      "max_tokens": 500
    }
  }'
```

### 快速调用（使用 Grok 4.1 Fast）
```bash
curl -X POST "http://localhost:18000/models/openrouter/openrouter-grok-4.1-fast/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Write a short poem about technology",
    "parameters": {
      "temperature": 0.9,
      "max_tokens": 300
    }
  }'
```

---

## 4. 调用模型 - 对话格式（Messages）

### 单轮对话
```bash
curl -X POST "http://localhost:18000/models/openrouter/openrouter-gemma-3-27b-it/invoke" \
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
      "max_tokens": 500
    }
  }'
```

### 多轮对话
```bash
curl -X POST "http://localhost:18000/models/openrouter/openrouter-llama-3.3-70b-instruct/invoke" \
  -H "Content-Type: application/json" \
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
        "content": "Can you explain machine learning?"
      }
    ],
    "parameters": {
      "temperature": 0.7
    }
  }'
```

### 带系统提示的对话
```bash
curl -X POST "http://localhost:18000/models/openrouter/openrouter-glm-4.5-air/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [
      {
        "role": "system",
        "content": "你是一个专业的AI助手，擅长用中文回答问题"
      },
      {
        "role": "user",
        "content": "请用Python写一个快速排序算法"
      }
    ],
    "parameters": {
      "temperature": 0.3,
      "max_tokens": 800
    }
  }'
```

---

## 5. 智能路由 - 自动选择模型

### 根据标签路由（选择免费快速模型）
```bash
curl -X POST "http://localhost:18000/route/invoke" \
  -H "Content-Type: application/json" \
  # 远程调用或启用认证时增加：-H "Authorization: Bearer YOUR_TOKEN_OR_API_KEY" \
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
  }'
```

### 根据 Provider 类型路由（仅 OpenRouter）
```bash
curl -X POST "http://localhost:18000/route/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "query": {
      "provider_types": ["openrouter"],
      "tags": ["free"]
    },
    "request": {
      "messages": [
        {
          "role": "user",
          "content": "Write a haiku about nature"
        }
      ],
      "parameters": {
        "temperature": 0.8,
        "max_tokens": 200
      }
    }
  }'
```

### 选择中文模型
```bash
curl -X POST "http://localhost:18000/route/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "query": {
      "tags": ["chinese", "free"]
    },
    "request": {
      "prompt": "写一首关于春天的短诗",
      "parameters": {
        "temperature": 0.9,
        "max_tokens": 300
      }
    }
  }'
```

---

## 6. 常用参数说明

### temperature（温度）
- **范围**: 0.0 - 2.0
- **说明**: 控制输出的随机性
  - `0.1-0.3`: 更确定、更聚焦的输出（适合事实性回答）
  - `0.7-0.9`: 平衡的创造性（适合对话）
  - `1.0-2.0`: 高创造性（适合创意写作）

### max_tokens（最大令牌数）
- **说明**: 限制生成的最大长度
- **建议**: 
  - 简短回答: 50-200
  - 中等回答: 200-500
  - 长回答: 500-2000

### top_p（核采样）
- **范围**: 0.0 - 1.0
- **说明**: 控制输出的多样性（通常与 temperature 一起使用）

### frequency_penalty（频率惩罚）
- **范围**: -2.0 - 2.0
- **说明**: 减少重复内容（正值）或增加重复（负值）

### presence_penalty（存在惩罚）
- **范围**: -2.0 - 2.0
- **说明**: 鼓励（负值）或避免（正值）讨论新话题

---

## 7. 格式化输出（使用 jq）

### 只显示输出文本
```bash
curl -X POST "http://localhost:18000/models/openrouter/openrouter-glm-4.5-air/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "你好",
    "parameters": {"max_tokens": 100}
  }' | jq -r '.output_text'
```

### 显示完整响应（格式化）
```bash
curl -X POST "http://localhost:18000/models/openrouter/openrouter-llama-3.3-70b-instruct/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Hello",
    "parameters": {"max_tokens": 100}
  }' | jq '.'
```

### 显示令牌使用情况
```bash
curl -X POST "http://localhost:18000/models/openrouter/openrouter-gemma-3-27b-it/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Test",
    "parameters": {"max_tokens": 100}
  }' | jq '.raw.usage'
```

---

## 8. 认证方式

### 推荐方式：先登录后请求（Session Token）

#### 步骤 1: 登录获取 Session Token

```bash
# 方式1: 通过请求体传递 API Key
curl -X POST "http://localhost:18000/auth/login" \
  -H "Content-Type: application/json" \
  -d '{
    "api_key": "YOUR_API_KEY"
  }'
```

```bash
# 方式2: 通过 Authorization 头传递 API Key
curl -X POST "http://localhost:18000/auth/login" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{}'
```

**响应示例：**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_in": 86400,
  "message": "登录成功，请使用此 token 进行后续请求。使用 /auth/bind-model 绑定模型。"
}
```

#### 步骤 2: 绑定模型到 Session（可选，推荐用于 OpenAI 兼容 API）

```bash
curl -X POST "http://localhost:18000/auth/bind-model" \
  -H "Authorization: Bearer YOUR_SESSION_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "provider_name": "openai",
    "model_name": "gpt-5.1"
  }'
```

**响应示例：**
```json
{
  "message": "模型 openai/gpt-5.1 已绑定到 session",
  "provider_name": "openai",
  "model_name": "gpt-5.1"
}
```

#### 步骤 3: 使用 Session Token 进行请求

```bash
# 使用 Authorization Bearer（推荐）
curl -X POST "http://localhost:18000/models/openrouter/openrouter-glm-4.5-air/invoke" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_SESSION_TOKEN" \
  -d '{
    "prompt": "你好",
    "parameters": {"max_tokens": 100}
  }'
```

```bash
# 使用 X-Session-Token 头
curl -X POST "http://localhost:18000/models/openrouter/openrouter-glm-4.5-air/invoke" \
  -H "Content-Type: application/json" \
  -H "X-Session-Token: YOUR_SESSION_TOKEN" \
  -d '{
    "prompt": "你好",
    "parameters": {"max_tokens": 100}
  }'
```

```bash
# 使用查询参数
curl -X POST "http://localhost:18000/models/openrouter/openrouter-glm-4.5-air/invoke?session_token=YOUR_SESSION_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "你好",
    "parameters": {"max_tokens": 100}
  }'
```

#### 步骤 3: 登出（可选）

```bash
curl -X POST "http://localhost:18000/auth/logout" \
  -H "Authorization: Bearer YOUR_SESSION_TOKEN"
```

### 兼容方式：直接使用 API Key（向后兼容）

如果不想使用登录流程，仍然可以直接使用 API Key（不推荐，安全性较低）：

```bash
curl -X POST "http://localhost:18000/models/openrouter/openrouter-glm-4.5-air/invoke" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "prompt": "你好",
    "parameters": {"max_tokens": 100}
  }'
```

或者使用查询参数：

```bash
curl -X POST "http://localhost:18000/models/openrouter/openrouter-glm-4.5-air/invoke?api_key=YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "你好",
    "parameters": {"max_tokens": 100}
  }'
```

### 认证策略说明

**基于来源的认证策略：**

1. **本机请求（localhost/127.0.0.1）**：
   - ✅ **不需要认证**，可以直接访问所有端点
   - 如果提供了认证信息，仍然会应用相应的权限限制（如模型限制、参数限制等）
   - 适用于本地开发和测试

2. **远程请求（其他来源）**：
   - ❌ **必须认证**（如果启用了认证）
   - 需要先登录获取 Session Token，或直接使用 API Key
   - 适用于生产环境和远程访问

### 认证方式对比

| 方式 | 安全性 | 推荐度 | 说明 |
|------|--------|--------|------|
| Session Token（登录后） | 高 | ⭐⭐⭐⭐⭐ | 推荐方式，API Key 不会在每次请求中传输 |
| 直接使用 API Key | 中 | ⭐⭐⭐ | 向后兼容，但 API Key 会在每次请求中传输 |
| 本机请求（免认证） | - | - | 仅限 localhost，自动跳过认证 |

### 完整示例：使用 Session Token

```bash
# 1. 登录
TOKEN=$(curl -s -X POST "http://localhost:18000/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"api_key": "YOUR_API_KEY"}' | jq -r '.token')

# 2. 使用 Token 调用模型
curl -X POST "http://localhost:18000/models/openrouter/openrouter-glm-4.5-air/invoke" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "prompt": "你好",
    "parameters": {"max_tokens": 100}
  }'

# 3. 登出（可选）
curl -X POST "http://localhost:18000/auth/logout" \
  -H "Authorization: Bearer $TOKEN"
```

### 本机请求示例（免认证）

**注意：** 以下示例仅在本机（localhost）访问时有效，远程访问仍需要认证。

```bash
# 本机请求可以直接调用，无需认证
curl -X POST "http://localhost:18000/models/openrouter/openrouter-glm-4.5-air/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "你好",
    "parameters": {"max_tokens": 100}
  }'

# 本机请求也可以使用认证信息（用于权限限制）
curl -X POST "http://localhost:18000/models/openrouter/openrouter-glm-4.5-air/invoke" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_SESSION_TOKEN" \
  -d '{
    "prompt": "你好",
    "parameters": {"max_tokens": 100}
  }'
```

---

## 9. 常用场景示例

### 编程问题（使用低温度）
```bash
curl -X POST "http://localhost:18000/models/openrouter/openrouter-llama-3.3-70b-instruct/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Write a Python function to calculate factorial",
    "parameters": {
      "temperature": 0.2,
      "max_tokens": 300
    }
  }'
```

### 创意写作（使用高温度）
```bash
curl -X POST "http://localhost:18000/models/openrouter/openrouter-grok-4.1-fast/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "Write a creative story about a robot learning to paint",
    "parameters": {
      "temperature": 0.9,
      "max_tokens": 500
    }
  }'
```

### 翻译任务
```bash
curl -X POST "http://localhost:18000/models/openrouter/openrouter-glm-4.5-air/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [
      {
        "role": "user",
        "content": "请将以下英文翻译成中文：Hello, how are you?"
      }
    ],
    "parameters": {
      "temperature": 0.3,
      "max_tokens": 200
    }
  }'
```

### 代码解释
```bash
curl -X POST "http://localhost:18000/models/openrouter/openrouter-gemma-3-27b-it/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [
      {
        "role": "user",
        "content": "Explain what this code does:\ndef fibonacci(n):\n    if n <= 1:\n        return n\n    return fibonacci(n-1) + fibonacci(n-2)"
      }
    ],
    "parameters": {
      "temperature": 0.5,
      "max_tokens": 400
    }
  }'
```

---

## 10. 多模态输入（图像、音频、视频、文件）

### 图像输入

#### OpenAI 兼容格式（GPT-4 Vision, Claude 等）
```bash
curl -X POST "http://localhost:18000/models/openai/gpt-5.1/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [
      {
        "role": "user",
        "content": [
          {
            "type": "text",
            "text": "请描述这张图片"
          },
          {
            "type": "image_url",
            "image_url": {
              "url": "https://example.com/image.jpg"
            }
          }
        ]
      }
    ],
    "parameters": {
      "max_tokens": 300
    }
  }'
```

#### Base64 编码图像
```bash
curl -X POST "http://localhost:18000/models/openai/gpt-5.1/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [
      {
        "role": "user",
        "content": [
          {
            "type": "text",
            "text": "这张图片里有什么？"
          },
          {
            "type": "image_url",
            "image_url": {
              "url": "data:image/jpeg;base64,/9j/4AAQSkZJRg..."
            }
          }
        ]
      }
    ],
    "parameters": {
      "max_tokens": 300
    }
  }'
```

#### 多张图片
```bash
curl -X POST "http://localhost:18000/models/openai/gpt-5.1/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [
      {
        "role": "user",
        "content": [
          {
            "type": "text",
            "text": "比较这两张图片的差异"
          },
          {
            "type": "image_url",
            "image_url": {
              "url": "https://example.com/image1.jpg"
            }
          },
          {
            "type": "image_url",
            "image_url": {
              "url": "https://example.com/image2.jpg"
            }
          }
        ]
      }
    ],
    "parameters": {
      "max_tokens": 500
    }
  }'
```

### Gemini 格式（图像输入）

#### 使用 URL
```bash
curl -X POST "http://localhost:18000/models/gemini/gemini-2.5-pro/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [
      {
        "role": "user",
        "content": "请分析这张图片: https://example.com/image.jpg"
      }
    ],
    "parameters": {
      "max_tokens": 300
    }
  }'
```

#### 使用 Base64（Gemini 格式）
```bash
curl -X POST "http://localhost:18000/models/gemini/gemini-2.5-pro/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [
      {
        "role": "user",
        "content": "data:image/jpeg;base64,/9j/4AAQSkZJRg..."
      }
    ],
    "parameters": {
      "max_tokens": 300
    }
  }'
```

### 音频输入

#### OpenAI 格式（Whisper API）
```bash
# 注意：音频通常需要通过文件上传，这里展示 JSON 格式的示例
curl -X POST "http://localhost:18000/models/openai/gpt-5.1/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [
      {
        "role": "user",
        "content": [
          {
            "type": "text",
            "text": "请转写这段音频"
          },
          {
            "type": "input_audio",
            "input_audio": {
              "data": "base64_encoded_audio_data",
              "format": "mp3"
            }
          }
        ]
      }
    ],
    "parameters": {
      "max_tokens": 1000
    }
  }'
```

#### Gemini 音频格式
```bash
curl -X POST "http://localhost:18000/models/gemini/gemini-2.5-pro/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [
      {
        "role": "user",
        "content": "data:audio/mp3;base64,base64_encoded_audio_data"
      }
    ],
    "parameters": {
      "max_tokens": 1000
    }
  }'
```

### 视频输入

#### Gemini 视频格式
```bash
curl -X POST "http://localhost:18000/models/gemini/gemini-3.0-pro/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [
      {
        "role": "user",
        "content": "请分析这个视频: https://example.com/video.mp4"
      }
    ],
    "parameters": {
      "max_tokens": 500
    }
  }'
```

#### Base64 编码视频
```bash
curl -X POST "http://localhost:18000/models/gemini/gemini-3.0-pro/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [
      {
        "role": "user",
        "content": "data:video/mp4;base64,base64_encoded_video_data"
      }
    ],
    "parameters": {
      "max_tokens": 500
    }
  }'
```

### 文件输入（文档、PDF 等）

#### 通过 URL 引用文件
```bash
curl -X POST "http://localhost:18000/models/openai/gpt-5.1/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [
      {
        "role": "user",
        "content": [
          {
            "type": "text",
            "text": "请总结这个 PDF 文档的主要内容"
          },
          {
            "type": "file_url",
            "file_url": {
              "url": "https://example.com/document.pdf",
              "format": "pdf"
            }
          }
        ]
      }
    ],
    "parameters": {
      "max_tokens": 1000
    }
  }'
```

#### Base64 编码文件
```bash
curl -X POST "http://localhost:18000/models/openai/gpt-5.1/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [
      {
        "role": "user",
        "content": [
          {
            "type": "text",
            "text": "分析这个文档"
          },
          {
            "type": "file",
            "file": {
              "data": "base64_encoded_file_data",
              "mime_type": "application/pdf"
            }
          }
        ]
      }
    ],
    "parameters": {
      "max_tokens": 1000
    }
  }'
```

### 混合多模态输入（文本 + 图像 + 音频）

```bash
curl -X POST "http://localhost:18000/models/gemini/gemini-3.0-pro/invoke" \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [
      {
        "role": "user",
        "content": "请分析这张图片和这段音频，并给出综合结论。图片: https://example.com/image.jpg, 音频: https://example.com/audio.mp3"
      }
    ],
    "parameters": {
      "max_tokens": 800
    }
  }'
```

### 重要说明

**多模态支持取决于具体的 Provider 和模型：**

1. **OpenAI 兼容格式**（GPT-4 Vision, Claude 等）：
   - 支持 `content` 为数组格式，包含 `text` 和 `image_url` 对象
   - 格式示例：
     ```json
     {
       "role": "user",
       "content": [
         {"type": "text", "text": "描述图片"},
         {"type": "image_url", "image_url": {"url": "..."}}
       ]
     }
     ```

2. **Gemini 格式**：
   - 支持在 `content` 字符串中直接使用 URL 或 Base64 数据 URI
   - 格式示例：
     ```json
     {
       "role": "user",
       "content": "data:image/jpeg;base64,..."
     }
     ```

3. **当前实现限制**：
   - 如果模型不支持多模态，多模态内容可能会被忽略或返回错误
   - 建议先查询模型的 `supports_vision`、`supports_audio` 等配置
   - 某些 Provider 可能需要特定的格式，请参考对应 Provider 的文档

4. **检查模型能力**：
   ```bash
   # 查看模型配置，确认是否支持多模态
   curl "http://localhost:18000/models" | jq '.[] | select(.config.supports_vision == true)'
   ```

### 支持的模型列表

#### 支持图像的模型
- `openai/gpt-5.1` - 支持图像输入
- `openai/gpt-5-pro` - 支持图像输入
- `claude/claude-4.5-sonnet` - 支持图像输入
- `claude/claude-4.5-haiku` - 支持图像输入
- `gemini/gemini-2.5-flash` - 支持图像输入
- `gemini/gemini-2.5-pro` - 支持图像、音频、视频
- `gemini/gemini-3.0-pro` - 支持图像、音频、视频

#### 支持音频的模型
- `gemini/gemini-2.5-pro` - 支持音频输入
- `gemini/gemini-3.0-pro` - 支持音频输入

#### 支持视频的模型
- `gemini/gemini-2.5-pro` - 支持视频输入
- `gemini/gemini-3.0-pro` - 支持视频输入

### 多模态使用提示

1. **图像格式**:
   - 支持的格式：JPEG, PNG, GIF, WebP
   - 推荐大小：小于 20MB
   - 可以使用 URL 或 Base64 编码

2. **音频格式**:
   - 支持的格式：MP3, WAV, FLAC, AAC
   - 推荐大小：小于 25MB
   - 采样率：建议 16kHz 或更高

3. **视频格式**:
   - 支持的格式：MP4, MOV, AVI
   - 推荐大小：小于 100MB
   - 分辨率：建议 720p 或更低

4. **文件格式**:
   - PDF, DOCX, TXT, CSV 等
   - 大小限制取决于具体模型

5. **Base64 编码**:
   ```bash
   # 将文件转换为 Base64
   base64 -i image.jpg | tr -d '\n'
   
   # 或使用 Python
   python3 -c "import base64; print(base64.b64encode(open('image.jpg', 'rb').read()).decode())"
   ```

---

## 11. 错误处理

### 检查响应状态码
```bash
curl -X POST "http://localhost:18000/models/openrouter/openrouter-glm-4.5-air/invoke" \
  -H "Content-Type: application/json" \
  -w "\nHTTP Status: %{http_code}\n" \
  -d '{
    "prompt": "test",
    "parameters": {"max_tokens": 10}
  }'
```

### 查看错误详情
```bash
curl -X POST "http://localhost:18000/models/openrouter/invalid-model/invoke" \
  -H "Content-Type: application/json" \
  -d '{"prompt": "test"}' 2>&1 | jq '.'
```

---

## 提示

1. **服务地址**: 默认是 `http://localhost:18000`，如果修改了配置请相应调整
2. **超时设置**: 某些模型响应较慢，可以添加 `--max-time 30` 参数增加超时时间
3. **JSON 格式**: 确保 JSON 格式正确，特别注意引号的转义（在 shell 中使用单引号包裹 JSON）
4. **中文内容**: 使用支持中文的模型（如 `openrouter-glm-4.5-air`）可以获得更好的中文体验
5. **免费模型**: 免费模型可能有速率限制，建议不要过于频繁调用

---

## 快速参考

```bash
# 健康检查
curl http://localhost:18000/health

# 登录获取 Session Token（推荐）
TOKEN=$(curl -s -X POST "http://localhost:18000/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"api_key": "YOUR_API_KEY"}' | jq -r '.token')

# 获取免费模型列表
curl "http://localhost:18000/models?tags=free" \
  -H "Authorization: Bearer $TOKEN"

# 获取支持图像的模型
curl "http://localhost:18000/models" \
  -H "Authorization: Bearer $TOKEN" | jq '.[] | select(.config.supports_vision == true)'

# 简单调用（英文）
curl -X POST "http://localhost:18000/models/openrouter/openrouter-llama-3.3-70b-instruct/invoke" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"prompt": "Hello", "parameters": {"max_tokens": 100}}'

# 简单调用（中文）
curl -X POST "http://localhost:18000/models/openrouter/openrouter-glm-4.5-air/invoke" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"prompt": "你好", "parameters": {"max_tokens": 100}}'

# 图像输入（OpenAI 格式）
curl -X POST "http://localhost:18000/models/openai/gpt-5.1/invoke" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "messages": [{
      "role": "user",
      "content": [
        {"type": "text", "text": "描述图片"},
        {"type": "image_url", "image_url": {"url": "https://example.com/image.jpg"}}
      ]
    }],
    "parameters": {"max_tokens": 300}
  }'

# 智能路由
curl -X POST "http://localhost:18000/route/invoke" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query": {"tags": ["free"]}, "request": {"prompt": "test", "parameters": {"max_tokens": 100}}}'

# 登出（可选）
curl -X POST "http://localhost:18000/auth/logout" \
  -H "Authorization: Bearer $TOKEN"
```

