# 更新 OpenRouter 免费模型 (Update OpenRouter Free Models)

此命令用于从 OpenRouter API 获取最新的免费模型列表，并自动添加到 `router.toml` 配置文件中。

## 功能说明

1. **获取最新模型**：从 OpenRouter API (`https://openrouter.ai/api/v1/models`) 获取所有可用模型
2. **筛选免费模型**：自动识别免费模型（定价为 0 或模型 ID 包含 `:free`）
3. **对比现有配置**：与 `router.toml` 中已配置的模型对比，找出新模型
4. **自动生成配置**：为新模型自动生成完整的配置块，包括：
   - 模型名称（从 OpenRouter 模型 ID 自动生成）
   - 显示名称（添加 "(免费)" 后缀）
   - 标签（根据模型信息自动推断）
   - 配置参数（上下文窗口、视觉支持、工具支持、语言等）
5. **更新配置文件**：自动将新模型添加到 `router.toml` 的 OpenRouter Models 部分

## 执行步骤

### 前置条件

1. **网络连接**：
   需要能够访问 `https://openrouter.ai` API

2. **可选：API Key**：
   可以设置 `OPENROUTER_API_KEY` 环境变量以获得更好的 API 访问（某些情况下可能需要）
   ```bash
   export OPENROUTER_API_KEY="your-api-key"
   ```

3. **配置文件存在**：
   确保 `router.toml` 文件存在且格式正确

### 执行命令

运行更新脚本：

```bash
python3 scripts/update_openrouter_free_models.py
```

或者使用绝对路径：

```bash
cd /path/to/llm-router
python3 scripts/update_openrouter_free_models.py
```

## 输出说明

### 执行过程

脚本会显示：
- API 请求状态和获取的模型总数
- 筛选出的免费模型数量
- 已配置的模型数量
- 新发现的模型数量和列表
- 每个新模型的添加状态

### 示例输出

```
============================================================
更新 OpenRouter 免费模型
============================================================
使用环境变量中的 OPENROUTER_API_KEY
正在从 OpenRouter API 获取模型列表...
API 地址: https://openrouter.ai/api/v1/models
✓ 成功获取 500 个模型
✓ 筛选出 45 个免费模型
✓ 找到 16 个已配置的 OpenRouter 免费模型

✓ 发现 29 个新模型

新模型列表:
  - Model Name 1 (provider/model-1:free)
  - Model Name 2 (provider/model-2:free)
  ...

备份已创建: router.toml.backup
  ✓ 添加模型: model-1 (Model Name 1 (免费))
  ✓ 添加模型: model-2 (Model Name 2 (免费))
  ...

✓ 已添加 29 个新模型到 router.toml

============================================================
完成！
============================================================
```

## 配置生成规则

### 模型名称生成

- 从 OpenRouter 模型 ID 提取：`provider/model-name:free` -> `model-name`
- 自动转换为小写，使用连字符分隔
- 确保唯一性（如果已存在同名模型，自动添加数字后缀）

### 标签自动推断

脚本会根据以下信息自动推断标签：

- **必须标签**：`free`, `openrouter`
- **基础标签**：`chat`, `general`
- **提供者标签**：根据模型 ID 前缀推断
  - `meta-llama` -> `open-source`
  - `google` -> `google`
  - `qwen` -> `qwen`, `chinese`
  - `glm`, `z-ai` -> `glm`, `chinese`
  - `kimi`, `moonshotai` -> `kimi`, `chinese`
  - `mistral` -> `mistral`, `open-source`
  - `nvidia` -> `nvidia`
  - `openai` -> `openai`
- **功能标签**：根据模型名称和功能推断
  - 包含 `flash` 或 `fast` -> `fast`
  - 包含 `pro` 或 `plus` -> `high-quality`
  - 包含 `reasoning` 或 `think` -> `reasoning`
  - 包含 `long` 或 `context` -> `long-context`
  - 包含 `instruct` -> `instruction-tuned`
  - 包含 `coder` 或 `code` -> `coding`
  - 支持视觉 -> `image`
  - 支持函数调用 -> `function-call`

### 配置参数推断

- **context_window**：从 API 返回的 `context_length` 转换
  - 131072 -> "128k"
  - 1048576 -> "1M"
- **supports_vision**：从 API 的 `architecture.vision` 字段获取
- **supports_tools**：从 API 的 `architecture.function_calling` 字段获取
- **languages**：根据模型提供者推断
  - 中文模型提供者（qwen, glm, kimi, deepseek 等）-> `["zh", "en"]`
  - 其他 -> `["en"]`

## 注意事项

1. **自动备份**：脚本会在首次修改前创建 `router.toml.backup` 备份文件
2. **不覆盖现有模型**：只会添加新模型，不会修改或删除现有模型配置
3. **保持格式**：新添加的模型配置会保持与现有配置相同的格式和缩进
4. **插入位置**：新模型会添加到 OpenRouter Models 部分的末尾
5. **网络要求**：需要能够访问 OpenRouter API，某些网络环境可能需要代理

## 错误处理

- **API 请求失败**：显示错误信息，不修改配置文件
- **网络超时**：提示检查网络连接或重试
- **文件解析错误**：显示错误信息，保留备份文件
- **配置冲突**：如果生成的模型名称已存在，会自动添加后缀确保唯一性

## 后续步骤

更新完成后，建议：

1. **测试新模型**：运行 `test-and-clean-openrouter-free` 命令测试新添加的模型是否有效
2. **检查配置**：查看 `router.toml` 确认新模型配置正确
3. **重启服务**：如果服务正在运行，可能需要重启以加载新配置

## 相关文件

- `scripts/update_openrouter_free_models.py`：更新脚本
- `router.toml`：配置文件（会被更新）
- `router.toml.backup`：备份文件（首次运行时创建）
- `.cursor/commands/test-and-clean-openrouter-free.md`：测试和清理命令

## 技术细节

### OpenRouter API

- **端点**：`https://openrouter.ai/api/v1/models`
- **方法**：GET
- **认证**：可选，通过 `Authorization: Bearer <api_key>` 头
- **响应格式**：JSON，包含 `data` 数组，每个元素是一个模型对象

### 免费模型判断

模型被认为是免费的，如果满足以下任一条件：
1. 模型 ID 以 `:free` 结尾
2. `pricing.prompt == 0` 且 `pricing.completion == 0`

### 生成的配置格式

```toml
[[models]]
name = "model-name"
provider = "openrouter"
remote_identifier = "provider/model-name:free"
display_name = "Model Display Name (免费)"
tags = ["chat", "general", "openrouter", "free", ...]
[models.config]
context_window = "128k"
supports_vision = false
supports_tools = true
languages = ["en"]
```
