# 更新项目文档 (Update Project Documentation)

此命令用于根据 `router.toml` 中的最新配置自动更新项目文档（包括 `TAGS.md`、`README.md` 和 `docs/API.md`）。

## 执行步骤

1. **运行更新脚本**：
   执行 `scripts/update_docs.py` 以更新 `TAGS.md` 并生成模型汇总信息。
   ```bash
   python3 scripts/update_docs.py
   ```

2. **同步 `TAGS.md`**：
   确认 `TAGS.md` 已被脚本更新。如果脚本未能完全覆盖所有分类，请手动微调。

3. **更新 `README.md`**：
   根据脚本输出的 "Model Summary"，更新 `README.md` 中的 "Provider 配置" 和 "模型配置" 部分，确保列出的模型与 `router.toml` 一致。

4. **更新 `docs/API.md`**：
   确保 `docs/API.md` 中的示例代码和模型列表反映了最新的 Provider 和模型。

## 注意事项
- 每次修改 `router.toml` 后都应运行此命令。
- 脚本会自动提取标签和厂商信息，但 `README.md` 和 `docs/API.md` 的描述性文字可能仍需人工或 AI 进一步润色。

