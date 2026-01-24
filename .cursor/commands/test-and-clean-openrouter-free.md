# 测试并清理 OpenRouter 免费模型 (Test and Clean OpenRouter Free Models)

此命令用于自动测试 `router.toml` 中所有标记为 `free` 的 OpenRouter 模型，并移除无效的模型配置。

## 功能说明

1. **自动测试**：从 `router.toml` 中提取所有 `provider="openrouter"` 且 `tags` 包含 `"free"` 的模型
2. **验证有效性**：通过调用 API 测试每个模型是否可用
3. **自动清理**：移除所有测试失败的无效模型配置
4. **生成报告**：保存测试结果到 `test_results.json`

## 执行步骤

### 前置条件

1. **确保服务运行**：
   确保 LLM Router 服务正在运行在 `http://localhost:18000`
   ```bash
   # 检查服务状态
   curl http://localhost:18000/health
   ```

2. **确保 API Key 配置**：
   确保已设置 `OPENROUTER_API_KEY` 环境变量（如果需要）

### 执行命令

运行测试和清理脚本：

```bash
python3 scripts/test_and_clean_openrouter_free.py
```

或者使用项目根目录下的脚本：

```bash
python3 test_all_openrouter_free.py
python3 remove_invalid_models.py
```

## 输出说明

### 测试过程

脚本会显示：
- 每个模型的测试进度 `[1/35]`
- 测试结果（成功 ✓ 或失败 ✗）
- 失败原因（如果失败）

### 测试总结

脚本会输出：
- 总模型数
- 有效模型数量和列表
- 无效模型数量和列表

### 清理操作

如果发现无效模型：
- 自动创建备份文件 `router.toml.backup`
- 从 `router.toml` 中移除所有无效模型配置
- 显示已移除的模型列表

### 结果文件

- `test_results.json`：包含有效和无效模型的 JSON 格式结果
- `router.toml.backup`：配置文件备份（首次运行时创建）

## 注意事项

1. **服务必须运行**：测试需要 LLM Router 服务在运行，否则会连接失败
2. **网络连接**：测试需要能够访问 OpenRouter API
3. **API Key**：某些模型可能需要有效的 OpenRouter API Key
4. **备份文件**：脚本会自动创建备份，但建议在重要操作前手动备份
5. **超时设置**：每个模型的测试超时时间为 30 秒

## 示例输出

```
============================================================
测试并清理 OpenRouter 免费模型
============================================================
配置文件: /path/to/router.toml
API 地址: http://localhost:18000/v1/chat/completions

找到 35 个免费模型需要测试

[1/35] Meta: Llama 3.3.70B Instruct (免费) (llama-3.3-70b-instruct)
  ✓ 成功
[2/35] Google: Gemma 3.27B (免费) (gemma-3-27b-it)
  ✓ 成功
...

============================================================
测试总结
============================================================
总模型数: 35
有效模型: 16
无效模型: 19

✓ 有效模型列表 (16):
  - llama-3.3-70b-instruct (Meta: Llama 3.3.70B Instruct (免费))
  ...

✗ 无效模型列表 (19):
  - deepseek-v3.1-nex-n1 (Nex AGI: DeepSeek V3.1 Nex N1 (免费))
  ...

============================================================
清理无效模型
============================================================
备份已创建: router.toml.backup
  移除模型: deepseek-v3.1-nex-n1
  ...

✓ 已移除 19 个无效模型
```

## 相关文件

- `scripts/test_and_clean_openrouter_free.py`：整合的测试和清理脚本
- `test_all_openrouter_free.py`：仅测试脚本
- `remove_invalid_models.py`：仅清理脚本
- `test_results.json`：测试结果文件
- `router.toml.backup`：配置文件备份
