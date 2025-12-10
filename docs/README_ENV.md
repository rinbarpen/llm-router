# 环境变量加载说明

## .env 文件自动加载

项目现在支持自动加载 `.env` 文件，无需手动导出环境变量。

### 工作原理

1. **应用启动时自动加载**：当运行 `uv run llm-router` 时，会自动查找项目根目录下的 `.env` 文件并加载其中的环境变量。

2. **配置同步时自动加载**：运行 `uv run python sync_config.py` 时，也会自动加载 `.env` 文件，确保 API Keys 正确保存到数据库。

### 使用方式

1. **创建 `.env` 文件**（如果不存在）：
   ```bash
   cp .env.example .env
   ```

2. **编辑 `.env` 文件**，添加 API Keys：
   ```bash
   GEMINI_API_KEY=your-gemini-api-key
   OPENAI_API_KEY=your-openai-api-key
   ANTHROPIC_API_KEY=your-anthropic-api-key
   LLM_ROUTER_ADMIN_KEY=your-admin-api-key  # 用于管理/远程调用或登录获取 session
   # ... 其他 API Keys
   ```

3. **启动服务**（自动加载 .env）：
   ```bash
   uv run llm-router
   ```

4. **同步配置**（自动加载 .env 并更新数据库）：
   ```bash
   uv run python sync_config.py
   ```

### 注意事项

- `.env` 文件应该放在项目根目录（与 `router.toml` 同级）
- `.env` 文件不应提交到版本控制系统（已在 `.gitignore` 中）
- 如果修改了 `.env` 文件中的 API Keys，需要：
  1. 运行 `uv run python sync_config.py` 更新数据库
  2. 重启服务（如果服务正在运行）
- 本机（localhost/127.0.0.1）请求默认免认证，但远程调用或启用认证时需确保 `LLM_ROUTER_ADMIN_KEY` 等密钥已配置并生效。

### 验证 .env 文件是否加载

```bash
# 测试环境变量是否加载
uv run python -c "import llm_router.config; import os; print('GEMINI_API_KEY:', '已加载' if 'GEMINI_API_KEY' in os.environ else '未加载')"
```

### 技术实现

- 使用 `python-dotenv` 库加载 `.env` 文件
- 在 `src/llm_router/config.py` 模块导入时自动加载
- 使用 `override=False` 参数，不覆盖已存在的环境变量

