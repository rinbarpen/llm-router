functions:
 - aihubmix, analysis, audio, chat, cheap, chinese, claude, claude_code_cli, code, code-execution, codex_cli, coding, deepseek, fast, free, function-call, gemini, general, glm, google, groq, high-quality, image, instruction-tuned, kimi, kimi_code_cli, local, long-context, mistral, nvidia, ollama, opencode_cli, open-source, openai, openrouter, qwen, qwen_code_cli, reasoning, siliconflow, summary, tts, uncensored, video, vllm, volcengine, web-search, writing, 

abilities:
 - audio, function-call, image, long-context, reasoning, video, 

sources:
 - aihubmix, azure-openai, azure_openai, claude, claude_code_cli, codex_cli, deepseek, deepseek (cn), doubao, doubao (cn), doubao (global), gemini, glm, glm (cn), glm (global), groq, huggingface, kimi, kimi (cn), kimi (global), kimi_code_cli, minimax, minimax (cn), minimax (global), ollama, ollama (cn), opencode_cli, openai, openrouter, qwen, qwen (cn), qwen (global), qwen_code_cli, siliconflow, vllm, volcengine, ...

features:
 - cheap, chinese, fast, free, high-quality, local, open-source, 

### Functions (功能)
- `general`: 通用模型
- `chat`: 聊天/对话优化
- `writing`: 创意写作/文案
- `coding`: 代码生成/辅助
- `summary`: 文本摘要
- `analysis`: 数据分析/逻辑分析
- `instruction-tuned`: 指令遵循优化
- `planning`: 任务规划

### Abilities (能力)
- `image`: 多模态-视觉/图像
- `audio`: 多模态-音频
- `video`: 多模态-视频
- `reasoning`: 强化推理 (如 o1, R1)
- `long-context`: 长上下文支持 (128k+)
- `function-call`: 工具调用支持
- `web-search`: 联网搜索能力
- `code-execution`: 代码执行能力
- `agentic`: Agent 适用

### Sources (来源/厂商)
- `qwen`, `kimi`, `openai`, `claude`, `gemini`, `google`, `glm`, `openrouter`, `x-ai`, `mistral`, `ollama`, `vllm`, `custom`

### Features (特性)
- `cheap`: 低成本
- `free`: 免费
- `fast`: 响应速度快
- `chinese`: 中文能力强
- `local`: 本地部署
- `open-source`: 开源模型
- `high-quality`: 旗舰级/高质量
- `uncensored`: 无审查/低内容限制

## 2. 配置示例 (`router.toml`)

`router.toml` 现在主要声明 provider 与 routing。下面的 `[[models]]` 仅作为兼容旧版手工导入示例；默认模型目录以数据库为主，也可以由 provider discovery / `data/model_sources/*.json` 自动补齐。

```toml
[routing]
default_pair = "gemini-3"

[[routing.pairs]]
name = "gemini-3"
strong_model = "gemini/gemini-3.0-pro"
weak_model = "gemini/gemini-3.0-flash"

[[routing.pairs]]
name = "gemini-2.5"
strong_model = "gemini/gemini-2.5-pro"
weak_model = "gemini/gemini-2.5-flash"

[[models]]
name = "gpt-4o"
provider = "openai"
tags = ["chat", "general", "image", "function-call", "web-search", "code-execution", "openai", "high-quality"]

[[models]]
name = "deepseek-r1"
provider = "openrouter"
tags = ["chat", "reasoning", "coding", "openrouter", "chinese", "free"]
```

## 3. 路由调用示例

```bash
curl -X POST http://localhost:18000/route/invoke \
  -H "Content-Type: application/json" \
  -d '{
    "query": {
      "tags": ["reasoning", "coding"]
    },
    "request": {
      "messages": [{"role": "user", "content": "写一个分布式锁的 Python 实现"}]
    }
  }'
```
