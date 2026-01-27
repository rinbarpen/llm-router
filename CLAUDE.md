# LLM Router 项目介绍

## 项目概述

LLM Router 是一个统一的 LLM（大语言模型）路由服务，旨在解决多厂商 LLM API 集成复杂、切换困难、管理分散等问题。通过提供统一的 REST 接口、智能路由策略和完整的监控体系，让开发者能够轻松管理和使用来自不同供应商的 LLM 模型。

### 核心定位

- **统一接口**：屏蔽各厂商 API 差异，提供标准化的 REST API
- **智能路由**：基于标签系统自动选择最适合的模型
- **灵活管理**：支持远程 API 和本地模型，统一配置和管理
- **企业级特性**：访问控制、限流、监控、缓存等完整的企业级功能

### 解决的问题

1. **多厂商集成复杂**：不同厂商的 API 格式、认证方式、参数命名各不相同
2. **模型切换困难**：需要在代码中硬编码模型选择逻辑，难以动态调整
3. **成本控制困难**：无法统一监控和管理不同厂商的 API 调用成本
4. **访问控制缺失**：缺乏细粒度的权限管理和访问限制
5. **监控能力不足**：无法统一查看和分析所有模型的调用情况

## 核心架构

### 架构设计理念

LLM Router 采用分层架构设计，核心思想是**抽象与解耦**：

```
┌─────────────────────────────────────────────────────────┐
│                    API 层 (REST)                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐    │
│  │ 路由接口     │  │ 模型管理     │  │ 监控接口     │    │
│  └──────────────┘  └──────────────┘  └──────────────┘    │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│                   服务层 (Services)                       │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐    │
│  │ RouterEngine │  │ ModelService │  │ MonitorService│   │
│  └──────────────┘  └──────────────┘  └──────────────┘    │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐    │
│  │ RateLimiter  │  │ APIKeyService│  │ CacheService │    │
│  └──────────────┘  └──────────────┘  └──────────────┘    │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│                  Provider 层 (适配器)                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐    │
│  │   OpenAI     │  │   Claude     │  │   Gemini     │    │
│  └──────────────┘  └──────────────┘  └──────────────┘    │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐    │
│  │   Ollama     │  │    vLLM      │  │ Transformers │    │
│  └──────────────┘  └──────────────┘  └──────────────┘    │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│                   数据层 (SQLite)                         │
│  ┌──────────────┐  ┌──────────────┐                     │
│  │  主数据库     │  │  监控数据库   │                     │
│  └──────────────┘  └──────────────┘                     │
└─────────────────────────────────────────────────────────┘
```

### 关键设计原则

1. **Provider 抽象**：所有 LLM 提供商通过统一的 `BaseProviderClient` 接口接入
2. **配置驱动**：通过 TOML 配置文件管理所有 Provider 和模型，支持热加载
3. **标签路由**：基于标签系统实现智能模型选择，无需硬编码
4. **统一接口**：提供 OpenAI 兼容的 API，支持现有 SDK 无缝迁移
5. **可扩展性**：易于添加新的 Provider 和功能模块

## 主要组件

### 1. API 层 (`src/llm_router/api/`)

负责处理 HTTP 请求和响应，提供 RESTful API 接口。

**核心模块**：
- `app.py`：应用主入口，生命周期管理，配置热加载
- `routes.py`：路由定义，包括模型调用、路由、监控等接口
- `auth.py`：认证中间件，支持 API Key 和 Session Token
- `request_utils.py`：请求工具函数

**主要接口**：
- `/models`：模型管理（列表、创建、更新）
- `/models/{provider}/{model}/invoke`：直接调用指定模型
- `/route/invoke`：智能路由调用
- `/v1/chat/completions`：OpenAI 兼容接口
- `/monitor/*`：监控相关接口

### 2. 服务层 (`src/llm_router/services/`)

核心业务逻辑层，实现路由、模型管理、监控等功能。

**核心服务**：

- **RouterEngine**：路由引擎，实现智能模型选择逻辑
  - 根据标签、Provider 类型等条件选择模型
  - 处理 API Key 权限验证
  - 调用选定的模型并返回结果

- **ModelService**：模型管理服务
  - 模型的增删改查
  - 模型查询和过滤（按标签、Provider 等）
  - 模型配置管理

- **MonitorService**：监控服务
  - 记录所有模型调用历史
  - 提供统计分析和时间序列数据
  - 使用独立的监控数据库，不影响主业务

- **RateLimiterManager**：限流管理器
  - 按模型配置限流策略
  - 支持令牌桶算法
  - 自动加载和刷新限流配置

- **APIKeyService**：API Key 管理服务
  - API Key 的创建、验证、权限管理
  - 支持模型限制、Provider 限制、参数限制

- **CacheService**：缓存服务
  - 优化 API 查询性能
  - 支持 TTL 和自动清理

### 3. Provider 层 (`src/llm_router/providers/`)

Provider 适配器层，实现不同厂商 API 的统一接入。

**Provider 类型**：

- **远程 API Provider**：
  - `openai_compatible.py`：OpenAI 兼容 API（OpenAI, Grok, DeepSeek, Qwen, Kimi, GLM, OpenRouter）
  - `anthropic.py`：Anthropic Claude API
  - `gemini.py`：Google Gemini API
  - `remote_http.py`：通用 HTTP Provider

- **本地模型 Provider**：
  - `ollama_local.py`：Ollama 本地模型
  - `vllm_local.py`：vLLM 本地推理服务
  - `transformers_local.py`：Transformers 本地模型

**设计模式**：
- 所有 Provider 继承 `BaseProviderClient`
- 实现统一的 `invoke()` 方法
- 支持多 API Key 轮换和自动重试

### 4. 数据层 (`src/llm_router/db/`)

数据持久化层，使用 SQLAlchemy 异步 ORM。

**数据库设计**：

- **主数据库**：存储 Provider、模型、API Key、限流配置等
  - `Provider`：Provider 配置
  - `Model`：模型配置
  - `APIKey`：API Key 配置
  - `RateLimit`：限流配置

- **监控数据库**：独立存储调用历史，避免影响主业务性能
  - `Invocation`：调用记录
  - 支持时间序列查询和统计分析

### 5. 配置层 (`src/llm_router/config.py`, `model_config.py`)

配置管理模块，支持从 TOML 文件和环境变量加载配置。

**配置来源优先级**：
1. 环境变量
2. `router.toml` 配置文件
3. 默认值

**配置热加载**：使用 `watchfiles` 监控配置文件变化，自动重新加载。

### 6. 前端监控界面 (`frontend/`)

基于 React + TypeScript + Ant Design 的监控 Dashboard。

**主要功能**：
- 实时调用监控
- 统计图表（时间序列、模型分布等）
- 模型管理界面
- 调用历史查询

## 技术栈

### 后端技术

- **Web 框架**：Starlette（轻量级 ASGI 框架）
- **异步 HTTP 客户端**：httpx, curl-cffi
- **数据库 ORM**：SQLAlchemy (异步)
- **数据库**：SQLite（主数据库 + 监控数据库）
- **配置管理**：Pydantic, TOML
- **依赖管理**：uv
- **测试框架**：pytest, pytest-asyncio

### 前端技术

- **框架**：React 18 + TypeScript
- **UI 组件库**：Ant Design
- **图表库**：Recharts
- **构建工具**：Vite
- **HTTP 客户端**：Axios

### 核心依赖

```python
# 主要依赖
starlette>=0.50.0          # Web 框架
sqlalchemy[asyncio]>=2.0.44  # ORM
pydantic>=2.12.4          # 数据验证
httpx>=0.27.0             # HTTP 客户端
uvicorn[standard]>=0.38.0  # ASGI 服务器
```

## 核心特性

### 1. 智能路由系统

基于标签的路由策略，无需硬编码模型选择逻辑：

```python
# 根据任务类型自动选择模型
query = {
    "tags": ["coding", "high-quality"],
    "provider_types": ["openai", "claude"]
}
# 系统会自动选择最合适的模型
```

**路由策略**：
- 标签匹配：根据功能标签（coding, reasoning, image 等）选择
- Provider 过滤：限制可用的 Provider 类型
- 优先级选择：在多个候选模型中智能选择

### 2. 统一接口设计

**OpenAI 兼容 API**：
```python
# 使用标准 OpenAI SDK，只需修改 base_url
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8000/v1",
    api_key="your-key"
)
# 可以调用任何已注册的模型
response = client.chat.completions.create(
    model="claude/claude-4.5-sonnet",  # 使用 Claude
    messages=[{"role": "user", "content": "Hello"}]
)
```

**统一 REST API**：
- 所有模型使用相同的接口格式
- 自动处理不同 Provider 的 API 差异
- 统一的错误处理和响应格式

### 3. 灵活的配置系统

**TOML 配置文件**：
```toml
[[providers]]
name = "openai"
type = "openai"
api_key_env = "OPENAI_API_KEY"

[[models]]
name = "gpt-4o"
provider = "openai"
tags = ["chat", "general", "high-quality"]
```

**配置热加载**：
- 修改配置文件后自动重新加载
- 无需重启服务
- 支持动态添加/删除模型和 Provider

### 4. 企业级访问控制

**细粒度权限管理**：
- 模型级别限制：限制可调用的模型
- Provider 级别限制：限制可用的 Provider
- 参数限制：限制请求参数（如 max_tokens）

**认证方式**：
- API Key 认证
- Session Token（支持登录绑定模型）
- 本机请求免认证（开发友好）

### 5. 完整的监控体系

**调用监控**：
- 记录所有模型调用历史
- 统计各模型的调用次数、成功率、延迟
- 时间序列数据分析

**独立监控数据库**：
- 不影响主业务性能
- 支持大量历史数据存储
- 快速查询和分析

### 6. 限流控制

**按模型配置限流**：
```toml
[models.rate_limit]
max_requests = 50
per_seconds = 60
```

**特性**：
- 令牌桶算法
- 支持突发流量（burst_size）
- 自动限流保护

### 7. 多模态支持

支持标记模型的多模态能力：
- `image`：图像输入
- `audio`：音频输入
- `video`：视频输入

系统可以根据任务类型自动选择支持相应模态的模型。

## 项目结构

```
llm-router/
├── src/llm_router/          # 后端核心代码
│   ├── api/                 # API 层
│   │   ├── app.py           # 应用主入口
│   │   ├── routes.py        # 路由定义
│   │   ├── auth.py          # 认证中间件
│   │   └── ...
│   ├── services/            # 服务层
│   │   ├── router_engine.py # 路由引擎
│   │   ├── model_service.py # 模型管理
│   │   ├── monitor_service.py # 监控服务
│   │   └── ...
│   ├── providers/           # Provider 适配器
│   │   ├── base.py          # 基类
│   │   ├── openai_compatible.py
│   │   ├── anthropic.py
│   │   ├── gemini.py
│   │   └── ...
│   ├── db/                  # 数据层
│   │   ├── models.py        # 数据模型
│   │   ├── monitor_models.py # 监控数据模型
│   │   └── ...
│   ├── config.py            # 配置管理
│   └── ...
├── frontend/                # 前端监控界面
│   ├── src/
│   │   ├── components/      # React 组件
│   │   ├── services/        # API 服务
│   │   └── ...
│   └── ...
├── examples/                # 使用示例
│   ├── python/
│   ├── javascript/
│   └── curl/
├── docs/                    # 文档
│   ├── API.md
│   ├── QUICKSTART.md
│   └── ...
├── tests/                   # 测试代码
├── router.toml             # 配置文件
├── pyproject.toml          # Python 项目配置
└── README.md               # 项目说明
```

## 使用场景

### 1. 多厂商模型统一管理

**场景**：需要同时使用 OpenAI、Claude、Gemini 等多个厂商的模型

**解决方案**：
- 统一配置所有 Provider 和模型
- 通过统一接口调用，无需关心各厂商 API 差异
- 统一监控和管理所有调用

### 2. 智能模型选择

**场景**：根据任务类型自动选择最合适的模型

**解决方案**：
- 为模型配置标签（coding, reasoning, image 等）
- 使用路由接口，系统自动选择
- 无需在代码中硬编码模型选择逻辑

### 3. 成本优化

**场景**：需要在高质量模型和低成本模型之间平衡

**解决方案**：
- 配置多个模型，设置成本标签（cheap, free, high-quality）
- 根据任务重要性选择不同模型
- 统一监控成本，优化使用策略

### 4. 本地模型集成

**场景**：需要同时使用云端 API 和本地部署的模型

**解决方案**：
- 支持 Ollama、vLLM、Transformers 等本地模型
- 统一接口调用，本地和云端模型无差异
- 可以灵活切换，降低对云端 API 的依赖

### 5. 企业级访问控制

**场景**：需要为不同团队/应用分配不同的模型访问权限

**解决方案**：
- 创建多个 API Key，配置不同的权限
- 限制可调用的模型、Provider 和参数
- 统一管理和监控所有 API Key 的使用情况

### 6. 开发和生产环境切换

**场景**：开发时使用本地模型，生产环境使用云端 API

**解决方案**：
- 通过配置文件轻松切换
- 使用相同的接口，无需修改代码
- 支持环境变量覆盖配置

## 项目特色

### 1. 开发者友好

- **零学习成本**：OpenAI 兼容 API，现有代码可直接使用
- **配置简单**：TOML 配置文件，易于理解和维护
- **热加载**：修改配置无需重启
- **丰富示例**：提供 Python、JavaScript、cURL 等多种示例

### 2. 企业级特性

- **完整监控**：调用历史、统计分析、实时监控
- **访问控制**：细粒度权限管理
- **限流保护**：防止 API 滥用
- **高可用**：多 API Key 轮换、自动重试

### 3. 高度可扩展

- **易于添加 Provider**：实现 `BaseProviderClient` 接口即可
- **插件化设计**：各模块独立，易于扩展
- **配置驱动**：通过配置而非代码实现功能

### 4. 性能优化

- **异步架构**：基于 Starlette 的异步处理
- **缓存机制**：减少数据库查询
- **独立监控库**：不影响主业务性能

## 快速开始

### 1. 安装

```bash
# 使用 uv 安装依赖
uv sync
```

### 2. 配置

编辑 `router.toml` 配置 Provider 和模型，在 `.env` 中设置 API Key。

### 3. 启动

```bash
# 启动后端
uv run llm-router

# 启动前端监控（可选）
cd frontend && npm run dev
```

### 4. 使用

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8000/v1",
    api_key="your-key"
)

response = client.chat.completions.create(
    model="openai/gpt-4o",
    messages=[{"role": "user", "content": "Hello!"}]
)
```

详细使用说明请参考：
- [README.md](README.md) - 项目总体说明
- [QUICKSTART.md](docs/QUICKSTART.md) - 快速启动指南
- [API.md](docs/API.md) - API 文档
- [TAGS.md](TAGS.md) - 标签系统说明

## 技术亮点

### 1. 异步架构

- 基于 Python asyncio 和 Starlette
- 支持高并发请求处理
- 异步数据库操作，提升性能

### 2. 类型安全

- 使用 Pydantic 进行数据验证
- TypeScript 前端类型检查
- 减少运行时错误

### 3. 配置热加载

- 使用 `watchfiles` 监控文件变化
- 自动重新加载配置
- 无需重启服务

### 4. 独立监控数据库

- 主业务和监控数据分离
- 避免监控数据影响主业务性能
- 支持大量历史数据存储

### 5. Provider 抽象

- 统一的 Provider 接口
- 易于添加新的 Provider
- 代码复用率高

## 未来规划

- [ ] 流式输出支持
- [ ] 更多本地模型 Provider
- [ ] 模型缓存和结果缓存
- [ ] 更丰富的监控指标
- [ ] 分布式部署支持
- [ ] 更多认证方式（OAuth、JWT 等）

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

[根据项目实际情况填写]
