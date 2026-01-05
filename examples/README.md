# LLM Router 示例代码

本目录包含 LLM Router 的完整示例代码，涵盖简单调用、复杂调用和企业级使用场景。

## 目录结构

```
examples/
├── README.md                    # 本文件
├── python/                      # Python 示例（使用 curl_cffi）
│   ├── simple/                  # 简单调用示例
│   ├── advanced/                # 复杂调用示例
│   └── enterprise/              # 企业级使用示例
├── javascript/                  # JavaScript 示例
│   ├── simple/
│   ├── advanced/
│   └── enterprise/
├── typescript/                  # TypeScript 示例
│   ├── simple/
│   ├── advanced/
│   └── enterprise/
└── curl/                        # curl 命令脚本
    ├── simple.sh
    ├── advanced.sh
    └── enterprise.sh
```

## 快速开始

### 环境配置

1. 设置环境变量（可选，本机请求可免认证）：

```bash
export LLM_ROUTER_BASE_URL="http://localhost:18000"
export LLM_ROUTER_API_KEY="your-api-key"  # 远程请求时需要
```

或创建 `.env` 文件：

```bash
LLM_ROUTER_BASE_URL=http://localhost:18000
LLM_ROUTER_API_KEY=your-api-key
```

### Python 示例

#### 安装依赖

```bash
# 使用 uv（推荐）
uv sync

# 或使用 pip
pip install curl-cffi python-dotenv
```

#### 运行示例

```bash
# 简单调用
cd examples/python/simple
python health_check.py
python list_models.py
python basic_invoke.py
python simple_chat.py

# 复杂调用
cd ../advanced
python authentication.py
python routing.py
python multimodal.py
python streaming.py
python openai_compatible.py

# 企业级使用
cd ../enterprise
python batch_processing.py
python error_handling.py
python monitoring.py
python api_key_management.py
python retry_strategy.py
```

### JavaScript 示例

#### 运行示例

```bash
# Node.js 环境（需要 Node.js 18+）
cd examples/javascript/simple
node health-check.js
node list-models.js
node basic-invoke.js
node simple-chat.js
```

### TypeScript 示例

#### 编译和运行

```bash
# 需要先安装 TypeScript
npm install -g typescript ts-node

# 运行示例
cd examples/typescript/simple
ts-node health-check.ts
ts-node basic-invoke.ts
```

### curl 脚本

```bash
# 直接运行脚本
cd examples/curl
./simple.sh
./advanced.sh
./enterprise.sh
```

## 示例分类

### 简单调用示例

- **健康检查** (`health_check.py`, `health-check.js`, `health-check.ts`)
  - 检查服务运行状态

- **获取模型列表** (`list_models.py`, `list-models.js`)
  - 列出所有可用模型
  - 按标签、Provider 类型过滤

- **基础调用** (`basic_invoke.py`, `basic-invoke.js`, `basic-invoke.ts`)
  - 使用简单的 prompt 调用模型

- **简单对话** (`simple_chat.py`, `simple-chat.js`)
  - 使用 messages 格式进行多轮对话

### 复杂调用示例

- **认证流程** (`authentication.py`, `authentication.js`, `authentication.ts`)
  - 登录获取 Session Token
  - 绑定模型到 Session
  - 使用 Token 调用 API
  - 登出

- **智能路由** (`routing.py`, `routing.js`)
  - 根据标签自动选择模型
  - 根据 Provider 类型路由
  - 组合条件路由

- **多模态输入** (`multimodal.py`, `multimodal.js`)
  - 图像输入（URL 和 Base64）
  - 多张图像
  - 音频、视频输入

- **流式响应** (`streaming.py`, `streaming.js`)
  - 处理流式输出
  - OpenAI 兼容的流式 API

- **OpenAI 兼容 API** (`openai_compatible.py`, `openai_compatible_simple.py`, `openai-compatible.js`, `openai-compatible.ts`)
  - 使用标准 OpenAI 格式的 API（`/v1/chat/completions`）
  - model 参数在请求体中，格式为 `provider/model`
  - 无缝替换 OpenAI SDK
  - 支持流式响应（Server-Sent Events）

### 企业级使用示例

- **批量处理** (`batch_processing.py`, `batch-processing.js`)
  - 顺序批量处理
  - 并发批量处理
  - 异步批量处理

- **错误处理** (`error_handling.py`, `error-handling.js`, `error-handling.ts`)
  - HTTP 状态码处理
  - 异常捕获和重试
  - 错误分类和处理

- **监控** (`monitoring.py`, `monitoring.js`)
  - 查询调用历史（支持筛选和分页）
  - 获取使用统计（按时间范围）
  - 时间序列数据
  - 导出监控数据（JSON、Excel/CSV）
  - 下载监控数据库

- **API Key 管理** (`api_key_management.py`)
  - 创建、查询、更新、删除 API Key
  - 设置权限和限制

- **重试策略** (`retry_strategy.py`, `retry-strategy.js`)
  - 指数退避
  - 固定间隔
  - 智能重试

## 认证说明

### 本机请求（localhost/127.0.0.1）

- ✅ **不需要认证** - 可以直接访问所有端点
- 适用于本地开发和测试

### 远程请求

- ❌ **需要认证** - 必须提供 API Key 或 Session Token
- 推荐使用 Session Token（先登录）

### 认证方式

1. **Session Token（推荐）**：
   ```bash
   # 1. 登录获取 Session Token
   POST /auth/login
   Body: {"api_key": "your-api-key"}
   # 或使用 Header: Authorization: Bearer <api-key>
   
   # 2. 使用 Token（模型绑定在首次调用时自动完成）
   Authorization: Bearer <session-token>
   
   # 3. 登出（可选）
   POST /auth/logout
   Authorization: Bearer <session-token>
   ```

2. **直接使用 API Key**：
   ```bash
   Authorization: Bearer <api-key>
   ```

## API 端点概览

### 核心端点

- `GET /health` - 健康检查
- `GET /models` - 列出模型（支持筛选）
- `GET /models/{provider}/{model}` - 获取单个模型
- `POST /models/{provider}/{model}/invoke` - 调用模型
- `POST /route/invoke` - 智能路由调用
- `POST /v1/chat/completions` - OpenAI 兼容 API（标准端点）

### 认证端点

- `POST /auth/login` - 登录获取 Session Token
- `POST /auth/logout` - 登出使 Token 失效

### 管理端点

- `GET /providers` - 列出 Provider
- `POST /providers` - 创建 Provider
- `POST /models` - 创建模型
- `PATCH /models/{provider}/{model}` - 更新模型

### API Key 管理

- `GET /api-keys` - 列出所有 API Key
- `POST /api-keys` - 创建 API Key
- `GET /api-keys/{id}` - 获取 API Key 详情
- `PATCH /api-keys/{id}` - 更新 API Key
- `DELETE /api-keys/{id}` - 删除 API Key

### 监控端点

- `GET /monitor/invocations` - 获取调用历史
- `GET /monitor/invocations/{id}` - 获取调用详情
- `GET /monitor/statistics` - 获取统计信息
- `GET /monitor/time-series` - 获取时间序列数据
- `GET /monitor/export/json` - 导出 JSON 数据
- `GET /monitor/export/excel` - 导出 Excel/CSV 数据
- `GET /monitor/database` - 下载监控数据库

## 注意事项

1. **环境变量**：所有示例使用环境变量管理配置，避免硬编码敏感信息

2. **错误处理**：企业级示例包含完整的错误处理和重试机制

3. **免费模型**：示例默认使用 OpenRouter 的免费模型，避免依赖付费 API

4. **代码质量**：所有示例包含详细注释和文档字符串

5. **类型安全**：TypeScript 示例包含完整的类型定义

6. **OpenAI 兼容性**：`/v1/chat/completions` 端点完全兼容 OpenAI API，model 参数在请求体中，格式为 `provider/model`

## 更多资源

- [API 文档](../docs/API.md) - 完整的 API 参考
- [快速命令](../docs/QUICK_COMMANDS.md) - 常用 curl 命令
- [快速开始](../docs/QUICKSTART.md) - 快速启动指南

## 贡献

欢迎提交 Issue 和 Pull Request 来改进示例代码！

