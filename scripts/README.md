# 工具脚本

本目录提供 Go 后端的运维与开发辅助脚本。

## 核心脚本

- `start.sh`：本地启动后端/监控（`all|backend|monitor`）
- `start-db.sh`：旧版 PostgreSQL 辅助脚本（SQLite-only 运行模式不再需要）
- `import-db.sh`：旧版 SQLite -> PostgreSQL 导入脚本（SQLite-only 运行模式不再需要）
- `check-db.sh`：旧版 PostgreSQL 检查脚本
- `check-service.sh`：检查后端健康状态
- `test_apis.sh`：执行 API smoke（health/models/route/chat）
- `check-free-models.sh`：先健康检查，再执行 API smoke
- `list-supported-models.sh`：离线查看 `router.toml` 中声明的 provider，并合并 `data/model_sources/*.json` 中的 provider 模型来源；`router.toml` 不再要求维护 `[[models]]`
- `pricing_sync.sh`：从 `data/pricing/*.json` 同步模型定价
- `generate_api_key.sh`：生成 API Key（纯 shell）
- `qwen-tts-adapter`：QwenTTS 插件适配器；`voices` 支持扁平 `voices` 或“人物 -> 多音色”的 `characters[].voices[]` 目录格式，`synthesize` 通过 `QWEN_TTS_SYNTH_COMMAND` 委托给实际后端
- `funasr-asr-adapter`：FunASR ASR 插件适配器；`transcribe` 通过 FunASR `AutoModel.generate` 执行本地离线转写，要求运行环境已安装 `funasr`

## 快速用法

```bash
# 启动后端 + 监控
./scripts/start.sh

# 只启动后端
./scripts/start.sh backend

# 检查后端健康
./scripts/check-service.sh --url http://127.0.0.1:18000

# API smoke
./scripts/test_apis.sh

# 查看所有 provider 与模型
./scripts/list-supported-models.sh
./scripts/list-supported-models.sh --providers
./scripts/list-supported-models.sh --provider qwen --limit 10

# 定价同步
./scripts/pricing_sync.sh

# 查看 QwenTTS 角色
./scripts/qwen-tts-adapter voices --model qwen-tts-latest

# 调用 FunASR 本地转写
./scripts/funasr-asr-adapter transcribe --model paraformer-zh --input-file ./sample.wav --device cpu

# 生成 key
./scripts/generate_api_key.sh --length 40 --count 3
./scripts/generate_api_key.sh --env LLM_ROUTER_ADMIN_KEY --length 32
```

## 开机启动

- Linux: `scripts/linux/`
- macOS: `scripts/macos/`
- Windows: `scripts/windows/`

## 说明

- 脚本默认面向 Go 后端（`go run ./cmd/llm-router`）。
- 脚本不会自动安装依赖；请先准备 `go`、`curl`、`npm`、Docker（按需）。
