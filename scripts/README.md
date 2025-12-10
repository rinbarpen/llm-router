# 工具脚本

本目录包含 LLM Router 的各种工具脚本，包括开机启动脚本和 API Key 生成工具。

## 目录结构

```
scripts/
├── generate_api_key.py      # 生成 API Key 的 Python 脚本
├── generate_api_key.sh      # 生成 API Key 的 Shell 包装脚本
├── linux/                   # Linux systemd 服务文件
├── macos/                   # macOS launchd 服务文件
├── windows/                 # Windows 任务计划脚本
└── tests/                   # 测试脚本
```

## API Key 生成工具

### 快速使用

```bash
# 生成一个默认长度的 API Key（32 字符）
python scripts/generate_api_key.py

# 或使用 shell 脚本
./scripts/generate_api_key.sh
```

### 高级用法

```bash
# 生成指定长度的 key
python scripts/generate_api_key.py --length 64

# 生成多个 key
python scripts/generate_api_key.py --count 5

# 生成并输出为环境变量格式（方便添加到 .env 文件）
python scripts/generate_api_key.py --env LLM_ROUTER_ADMIN_KEY

# 生成多个 key 并输出为环境变量格式（逗号分隔）
python scripts/generate_api_key.py --count 3 --env LLM_ROUTER_ADMIN_KEY

# 添加前缀（如 sk-）
python scripts/generate_api_key.py --prefix sk- --length 40
```

### 使用示例

```bash
# 1. 生成管理员 key
python scripts/generate_api_key.py --env LLM_ROUTER_ADMIN_KEY --length 32
# 输出: LLM_ROUTER_ADMIN_KEY=xxx...

# 2. 将输出添加到 .env 文件
python scripts/generate_api_key.py --env LLM_ROUTER_ADMIN_KEY >> .env

# 3. 生成多个受限 key
python scripts/generate_api_key.py --count 3 --length 40
```

### 安全说明

- 生成的 API Key 使用 Python `secrets` 模块，确保加密安全
- 默认长度 32 字符，建议至少 16 字符
- 自动排除容易混淆的字符（0, O, I, l）
- 生成的 key 包含字母、数字和部分特殊字符（-、_）

## 开机启动脚本

## 目录结构

```
scripts/
├── linux/          # Linux systemd 服务文件
│   ├── install.sh              # 安装脚本
│   ├── uninstall.sh            # 卸载脚本
│   ├── llm-router-backend.service
│   └── llm-router-frontend.service
├── macos/          # macOS launchd 服务文件
│   ├── install.sh              # 安装脚本
│   ├── uninstall.sh            # 卸载脚本
│   ├── com.llmrouter.backend.plist
│   └── com.llmrouter.frontend.plist
└── windows/        # Windows 任务计划脚本
    ├── install-backend.ps1     # 后端安装脚本
    ├── install-frontend.ps1     # 前端安装脚本
    └── uninstall.ps1            # 卸载脚本
```

## Linux (systemd)

### 安装

```bash
cd scripts/linux
sudo ./install.sh
```

脚本会：
1. 检测项目路径和用户信息
2. 询问要安装的服务（后端/前端/两者）
3. 创建 systemd 服务文件
4. 启用开机自启

### 服务管理

```bash
# 启动服务
sudo systemctl start llm-router-backend
sudo systemctl start llm-router-frontend

# 停止服务
sudo systemctl stop llm-router-backend
sudo systemctl stop llm-router-frontend

# 查看状态
sudo systemctl status llm-router-backend
sudo systemctl status llm-router-frontend

# 查看日志
sudo journalctl -u llm-router-backend -f
sudo journalctl -u llm-router-frontend -f

# 禁用开机自启
sudo systemctl disable llm-router-backend
sudo systemctl disable llm-router-frontend
```

### 卸载

```bash
cd scripts/linux
sudo ./uninstall.sh
```

## macOS (launchd)

### 安装

```bash
cd scripts/macos
./install.sh
```

脚本会：
1. 检测项目路径和用户信息
2. 询问要安装的服务（后端/前端/两者）
3. 创建 launchd plist 文件到 `~/Library/LaunchAgents/`
4. 加载并启动服务

### 服务管理

```bash
# 启动服务
launchctl start com.llmrouter.backend
launchctl start com.llmrouter.frontend

# 停止服务
launchctl stop com.llmrouter.backend
launchctl stop com.llmrouter.frontend

# 查看状态
launchctl list | grep llmrouter

# 查看日志
tail -f ~/workspace/sxy/gym/llm-router/logs/backend.log
tail -f ~/workspace/sxy/gym/llm-router/logs/frontend.log

# 卸载服务（停止并删除）
launchctl unload ~/Library/LaunchAgents/com.llmrouter.backend.plist
launchctl unload ~/Library/LaunchAgents/com.llmrouter.frontend.plist
```

### 卸载

```bash
cd scripts/macos
./uninstall.sh
```

## Windows (任务计划程序)

### 安装后端

1. 以**管理员身份**打开 PowerShell
2. 运行安装脚本：

```powershell
cd scripts\windows
.\install-backend.ps1
```

### 安装前端

```powershell
.\install-frontend.ps1
```

### 服务管理

```powershell
# 启动服务
Start-ScheduledTask -TaskName "LLMRouter-Backend"
Start-ScheduledTask -TaskName "LLMRouter-Frontend"

# 停止服务
Stop-ScheduledTask -TaskName "LLMRouter-Backend"
Stop-ScheduledTask -TaskName "LLMRouter-Frontend"

# 查看状态
Get-ScheduledTask -TaskName "LLMRouter-Backend"
Get-ScheduledTask -TaskName "LLMRouter-Frontend"

# 查看任务计划（图形界面）
taskschd.msc
```

### 卸载

```powershell
cd scripts\windows
.\uninstall.ps1
```

## 注意事项

### 前置要求

1. **已安装依赖**：
   - 后端：`uv` 已安装并在 PATH 中
   - 前端：`npm` 已安装并在 PATH 中

2. **配置文件**：
   - 确保 `router.toml` 已正确配置
   - 确保 `.env` 文件包含必要的 API Keys

3. **项目路径**：
   - 脚本会自动检测项目路径
   - 如果项目路径不是默认路径，可能需要修改脚本中的路径

### 路径配置

如果项目不在默认路径，需要修改：

- **Linux**: 编辑 `install.sh` 中的 `PROJECT_ROOT` 变量
- **macOS**: 编辑 `install.sh` 中的 `PROJECT_ROOT` 变量
- **Windows**: 运行脚本时指定路径：
  ```powershell
  .\install-backend.ps1 -ProjectPath "C:\path\to\llm-router"
  ```

### 日志位置

- **Linux**: 使用 `journalctl` 查看日志
- **macOS**: `~/workspace/sxy/gym/llm-router/logs/`
- **Windows**: 任务计划程序的执行历史

### 故障排查

1. **服务无法启动**：
   - 检查 `uv` 和 `npm` 是否在 PATH 中
   - 检查项目路径是否正确
   - 查看日志文件

2. **端口冲突**：
   - 检查 `router.toml` 中的端口配置
   - 确保端口未被其他程序占用

3. **权限问题**：
   - Linux/macOS: 确保脚本有执行权限
   - Windows: 确保以管理员身份运行

## 手动配置（高级）

如果自动安装脚本不满足需求，可以手动配置：

### Linux systemd

复制服务文件到 `/etc/systemd/system/`，修改路径和用户，然后：

```bash
sudo systemctl daemon-reload
sudo systemctl enable llm-router-backend
sudo systemctl start llm-router-backend
```

### macOS launchd

复制 plist 文件到 `~/Library/LaunchAgents/`，修改路径，然后：

```bash
launchctl load ~/Library/LaunchAgents/com.llmrouter.backend.plist
```

### Windows 任务计划

1. 打开"任务计划程序" (`taskschd.msc`)
2. 创建基本任务
3. 设置触发器为"计算机启动时"
4. 设置操作为运行脚本

