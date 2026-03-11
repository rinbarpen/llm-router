/**
 * Provider 相关常量与工具（与后端默认值对齐）
 */

export const DEFAULT_PROVIDER_BASE_URLS: Record<string, string> = {
  openai: 'https://api.openai.com/v1',
  codex_cli: 'https://api.openai.com/v1',
  openrouter: 'https://openrouter.ai/api/v1',
  gemini: 'https://generativelanguage.googleapis.com',
  claude: 'https://api.anthropic.com',
  claude_code: 'https://api.anthropic.com',
  grok: 'https://api.x.ai/v1',
  deepseek: 'https://api.deepseek.com',
  qwen: 'https://dashscope.aliyuncs.com',
  kimi: 'https://api.moonshot.cn',
  bigmodel: 'https://open.bigmodel.cn/api/paas/v4',
  'z.ai': 'https://api.z.ai/api/paas/v4',
  ollama: 'http://127.0.0.1:11434',
  vllm: 'http://localhost:8000',
  groq: 'https://api.groq.com/openai/v1',
  siliconflow: 'https://api.siliconflow.cn/v1',
  aihubmix: 'https://aihubmix.com/v1',
  volcengine: 'https://ark.cn-beijing.volces.com/api/v3',
}

const API_KEY_URL_MAP: Record<string, string> = {
  openai: 'https://platform.openai.com/api-keys',
  codex_cli: 'https://platform.openai.com/api-keys',
  claude: 'https://console.anthropic.com/settings/keys',
  claude_code: 'https://console.anthropic.com/settings/keys',
  gemini: 'https://aistudio.google.com/app/apikey',
  openrouter: 'https://openrouter.ai/settings/keys',
  bigmodel: 'https://bigmodel.cn/dev/api',
  'z.ai': 'https://z.ai/manage-apikey/apikey-list',
  kimi: 'https://platform.moonshot.ai',
  qwen: 'https://modelstudio.console.alibabacloud.com/?tab=playground#/api-key',
  groq: 'https://console.groq.com/keys',
  deepseek: 'https://platform.deepseek.com/api_keys',
  siliconflow: 'https://cloud.siliconflow.cn/account/ak',
  aihubmix: 'https://aihubmix.com/token',
  volcengine: 'https://console.volcengine.com/ark',
}

const API_KEY_LINK_TEXT_MAP: Record<string, string> = {
  openai: '前往 OpenAI 获取密钥',
  codex_cli: '前往 OpenAI 获取密钥',
  claude: '前往 Anthropic 获取密钥',
  claude_code: '前往 Anthropic 获取密钥',
  gemini: '前往 Google AI Studio 获取密钥',
  openrouter: '前往 OpenRouter 获取密钥',
  bigmodel: '前往智谱开放平台获取密钥',
  'z.ai': '前往 z.ai 获取密钥',
  kimi: '前往 Moonshot 获取密钥',
  qwen: '前往阿里云获取密钥',
  groq: '前往 Groq 获取密钥',
  deepseek: '前往 DeepSeek 获取密钥',
  siliconflow: '前往硅基流动获取密钥',
  aihubmix: '前往 AiHubMix 获取密钥',
  volcengine: '前往火山引擎获取密钥',
}

/** 查找 API 密钥链接时，将 glm/glm-z 规范为 bigmodel/z.ai */
function normalizeProviderNameForApiKey(name?: string): string | undefined {
  if (name === 'glm') return 'bigmodel'
  if (name === 'glm-z') return 'z.ai'
  return name
}

/** 展示用标签：bigmodel/glm → 智谱 bigmodel，z.ai/glm-z → z.ai，其余用 type */
export function getProviderDisplayLabel(provider: { name: string; type: string }): string {
  if (provider.name === 'bigmodel' || provider.name === 'glm') return '智谱 bigmodel'
  if (provider.name === 'z.ai' || provider.name === 'glm-z') return 'z.ai'
  return provider.type
}

/** 仅根据 provider 名称返回展示名（用于列表标题、模型路径等） */
export function getProviderDisplayName(name: string): string {
  if (name === 'glm-z') return 'z.ai'
  if (name === 'glm' || name === 'bigmodel') return 'bigmodel'
  return name
}

export function getApiKeyUrl(providerName?: string, providerType?: string): string | null {
  const name = normalizeProviderNameForApiKey(providerName) ?? providerName
  const type = normalizeProviderNameForApiKey(providerType) ?? providerType
  return (name && API_KEY_URL_MAP[name]) || (type && API_KEY_URL_MAP[type]) || null
}

export function getApiKeyLinkText(providerName?: string, providerType?: string): string {
  const name = normalizeProviderNameForApiKey(providerName) ?? providerName
  const type = normalizeProviderNameForApiKey(providerType) ?? providerType
  return (name && API_KEY_LINK_TEXT_MAP[name]) || (type && API_KEY_LINK_TEXT_MAP[type]) || '点击这里获取密钥'
}

const OPENAI_LIKE_TYPES = new Set(['openai', 'grok', 'groq', 'deepseek', 'kimi', 'siliconflow', 'aihubmix', 'volcengine'])

/**
 * 根据 Provider 类型生成 API 地址预览
 */
export function getApiUrlPreview(
  baseUrl: string | null | undefined,
  provider: { name: string; type: string } | null
): string {
  if (!baseUrl) return ''
  const cleanUrl = baseUrl.replace(/\/+$/, '')
  if (!provider) return `${cleanUrl}/chat/completions`
  const { type, name } = provider
  if (type === 'openrouter') return `${cleanUrl}/chat/completions`
  if (type === 'volcengine' || type === 'doubao') return `${cleanUrl}/chat/completions`
  if (OPENAI_LIKE_TYPES.has(type)) {
    const hasV1 = /\/v1$/i.test(cleanUrl)
    return hasV1 ? `${cleanUrl}/chat/completions` : `${cleanUrl}/v1/chat/completions`
  }
  if (type === 'gemini') return `${cleanUrl}/v1beta/models/<model>:generateContent`
  if (type === 'claude') return `${cleanUrl}/v1/messages`
  if (type === 'claude_code') return `${cleanUrl}/v1/messages`
  if (type === 'codex_cli') return `${cleanUrl}/v1/responses`
  if (type === 'qwen') return `${cleanUrl}/compatible-mode/v1/chat/completions`
  if (type === 'ollama') return `${cleanUrl}/api/chat`
  if (type === 'bigmodel' || name === 'z.ai') return `${cleanUrl}/chat/completions`
  return `${cleanUrl}/chat/completions`
}
