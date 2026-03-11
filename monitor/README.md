# LLM Router Monitor Frontend

模型调用监控系统的前端界面。

## 安装依赖

```bash
cd frontend
npm install
```

或使用 yarn:

```bash
yarn install
```

## 端口配置

前端端口配置支持两种方式：

### 方式1：在 router.toml 中配置（推荐）

在项目根目录的 `router.toml` 文件中添加 `[frontend]` 配置部分：

```toml
[frontend]
port = 3000                    # 前端开发服务器端口
api_url = "http://localhost:8000"  # 后端API服务器地址（开发环境代理用）
api_base_url = "/api"          # 生产环境API基础路径
```

**注意**：如果 `api_url` 未配置，系统会自动根据 `[server]` 配置构建API地址。

### 方式2：使用环境变量

在 `frontend/.env` 文件中配置（会覆盖 router.toml 中的配置）：

```bash
# 前端开发服务器端口
VITE_PORT=3000

# 后端API服务器地址（用于开发环境代理）
VITE_API_URL=http://localhost:8000

# 生产环境API基础路径
VITE_API_BASE_URL=/api
```

### 配置优先级

环境变量 > router.toml > 默认值

### 配置示例

**示例1：后端运行在 9000 端口**

```toml
[server]
port = 9000

[frontend]
port = 3000
api_url = "http://localhost:9000"
```

**示例2：前端运行在 4000 端口**

```toml
[frontend]
port = 4000
api_url = "http://localhost:8000"
```

**示例3：自动根据服务器配置构建API地址**

```toml
[server]
host = "0.0.0.0"
port = 18000

[frontend]
port = 3000
# api_url 未配置，会自动构建为 http://localhost:18000
```

## 开发

启动开发服务器：

```bash
npm run dev
```

前端会通过代理访问后端API。确保后端服务已启动。

## 构建

构建生产版本:

```bash
npm run build
```

构建产物在 `dist/` 目录。

生产环境部署时，确保：
1. 前端静态文件由Web服务器（如Nginx）提供
2. Web服务器配置了 `/api` 路径的代理，指向后端服务
3. 或者修改 `api_base_url` 为完整的后端URL

## 功能

- **统计信息面板**: 显示总体统计、按模型统计、最近错误
- **调用历史列表**: 查看所有模型调用记录，支持筛选和分页
- **调用详情**: 查看单次调用的详细信息，包括请求、响应、原始数据
- **实时更新**: 自动刷新统计数据（每5秒）

## 技术栈

- React 18
- TypeScript
- Ant Design 5
- Vite
- Axios
