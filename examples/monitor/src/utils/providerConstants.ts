/**
 * Provider 相关常量与工具（与后端默认值对齐）
 */

const PROVIDER_TYPE_ALIASES: Record<string, string> = {
  bigmodel: 'glm',
  'z.ai': 'glm',
  transformers_local: 'transformers',
  vllm_local: 'vllm',
  ollama_local: 'ollama',
}

export function canonicalizeProviderType(type?: string): string | undefined {
  if (!type) return type
  return PROVIDER_TYPE_ALIASES[type] ?? type
}

export const DEFAULT_PROVIDER_BASE_URLS: Record<string, string> = {
  openai: 'https://api.openai.com/v1',
  codex_cli: 'https://api.openai.com/v1',
  opencode_cli: 'https://api.openai.com/v1',
  kimi_code_cli: 'https://api.moonshot.cn/v1',
  qwen_code_cli: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
  openrouter: 'https://openrouter.ai/api/v1',
  azure_openai: 'https://YOUR_RESOURCE_NAME.openai.azure.com',
  huggingface: 'https://api-inference.huggingface.co',
  gemini: 'https://generativelanguage.googleapis.com',
  claude: 'https://api.anthropic.com',
  claude_code_cli: 'https://api.anthropic.com',
  grok: 'https://api.x.ai/v1',
  deepseek: 'https://api.deepseek.com',
  qwen: 'https://dashscope.aliyuncs.com',
  kimi: 'https://api.moonshot.cn',
  glm: 'https://open.bigmodel.cn/api/paas/v4',
  bigmodel: 'https://open.bigmodel.cn/api/paas/v4',
  'z.ai': 'https://api.z.ai/api/paas/v4',
  minimax: 'https://api.minimax.chat/v1',
  doubao: 'https://ark.cn-beijing.volces.com/api/v3',
  ollama: 'http://127.0.0.1:11434',
  transformers: 'http://127.0.0.1:8000/v1',
  vllm: 'http://localhost:8000',
  remote_http: 'https://example.com',
  custom_http: 'https://example.com',
  groq: 'https://api.groq.com/openai/v1',
  siliconflow: 'https://api.siliconflow.cn/v1',
  aihubmix: 'https://aihubmix.com/v1',
  volcengine: 'https://ark.cn-beijing.volces.com/api/v3',
}

const API_KEY_URL_MAP: Record<string, string> = {
  openai: 'https://platform.openai.com/api-keys',
  codex_cli: 'https://platform.openai.com/api-keys',
  opencode_cli: 'https://platform.openai.com/api-keys',
  kimi_code_cli: 'https://platform.moonshot.ai',
  qwen_code_cli: 'https://modelstudio.console.alibabacloud.com/?tab=playground#/api-key',
  claude: 'https://console.anthropic.com/settings/keys',
  gemini: 'https://aistudio.google.com/app/apikey',
  openrouter: 'https://openrouter.ai/settings/keys',
  glm: 'https://bigmodel.cn/dev/api',
  bigmodel: 'https://bigmodel.cn/dev/api',
  'z.ai': 'https://z.ai/manage-apikey/apikey-list',
  kimi: 'https://platform.moonshot.ai',
  qwen: 'https://modelstudio.console.alibabacloud.com/?tab=playground#/api-key',
  groq: 'https://console.groq.com/keys',
  deepseek: 'https://platform.deepseek.com/api_keys',
  siliconflow: 'https://cloud.siliconflow.cn/account/ak',
  aihubmix: 'https://aihubmix.com/token',
  volcengine: 'https://console.volcengine.com/ark',
  huggingface: 'https://huggingface.co/settings/tokens',
}

const API_KEY_LINK_TEXT_MAP: Record<string, string> = {
  openai: '前往 OpenAI 获取密钥',
  codex_cli: '前往 OpenAI 获取密钥',
  opencode_cli: '前往 OpenAI 获取密钥',
  kimi_code_cli: '前往 Moonshot 获取密钥',
  qwen_code_cli: '前往阿里云获取密钥',
  claude: '前往 Anthropic 获取密钥',
  gemini: '前往 Google AI Studio 获取密钥',
  openrouter: '前往 OpenRouter 获取密钥',
  glm: '前往智谱开放平台获取密钥',
  bigmodel: '前往智谱开放平台获取密钥',
  'z.ai': '前往 z.ai 获取密钥',
  kimi: '前往 Moonshot 获取密钥',
  qwen: '前往阿里云获取密钥',
  groq: '前往 Groq 获取密钥',
  deepseek: '前往 DeepSeek 获取密钥',
  siliconflow: '前往硅基流动获取密钥',
  aihubmix: '前往 AiHubMix 获取密钥',
  volcengine: '前往火山引擎获取密钥',
  huggingface: '前往 Hugging Face 获取密钥',
}

/** 展示用标签：bigmodel/glm → 智谱 bigmodel，z.ai/glm-z → z.ai，其余用 type */
export function getProviderDisplayLabel(provider: { name: string; type: string }): string {
  if (provider.name === 'bigmodel' || provider.name === 'glm') return '智谱 bigmodel'
  if (provider.name === 'z.ai' || provider.name === 'glm-z') return 'z.ai'
  if (provider.type === 'glm' && provider.name.toLowerCase().includes('z.ai')) return 'z.ai'
  return provider.type
}

/** 仅根据 provider 名称返回展示名（用于列表标题、模型路径等） */
export function getProviderDisplayName(name: string): string {
  if (name === 'glm-z') return 'z.ai'
  if (name === 'glm' || name === 'bigmodel') return 'bigmodel'
  return name
}

export function getApiKeyUrl(providerName?: string, providerType?: string): string | null {
  const canonicalType = canonicalizeProviderType(providerType)
  return (providerName && API_KEY_URL_MAP[providerName]) || (canonicalType && API_KEY_URL_MAP[canonicalType]) || null
}

export function getApiKeyLinkText(providerName?: string, providerType?: string): string {
  const canonicalType = canonicalizeProviderType(providerType)
  return (
    (providerName && API_KEY_LINK_TEXT_MAP[providerName]) ||
    (canonicalType && API_KEY_LINK_TEXT_MAP[canonicalType]) ||
    '点击这里获取密钥'
  )
}

const OPENAI_LIKE_TYPES = new Set([
  'openai',
  'grok',
  'groq',
  'deepseek',
  'kimi',
  'siliconflow',
  'aihubmix',
  'volcengine',
  'glm',
  'minimax',
  'doubao',
  'huggingface',
  'transformers',
  'vllm',
  'remote_http',
  'custom_http',
  'azure_openai',
])

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
  const type = canonicalizeProviderType(provider.type) ?? provider.type
  const { name } = provider
  if (type === 'openrouter') return `${cleanUrl}/chat/completions`
  if (type === 'volcengine' || type === 'doubao') return `${cleanUrl}/chat/completions`
  if (OPENAI_LIKE_TYPES.has(type)) {
    const hasV1 = /\/v1$/i.test(cleanUrl)
    return hasV1 ? `${cleanUrl}/chat/completions` : `${cleanUrl}/v1/chat/completions`
  }
  if (type === 'gemini') return `${cleanUrl}/v1beta/models/<model>:generateContent`
  if (type === 'claude') return `${cleanUrl}/v1/messages`
  if (type === 'claude_code_cli') return `${cleanUrl}/v1/messages`
  if (type === 'opencode_cli' || type === 'kimi_code_cli' || type === 'qwen_code_cli') return `${cleanUrl}/v1/responses`
  if (type === 'codex_cli') return `${cleanUrl}/v1/responses`
  if (type === 'qwen') return `${cleanUrl}/compatible-mode/v1/chat/completions`
  if (type === 'ollama') return `${cleanUrl}/api/chat`
  if (type === 'glm' || name === 'z.ai') return `${cleanUrl}/chat/completions`
  return `${cleanUrl}/chat/completions`
}
