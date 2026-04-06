import axios from 'axios'
import type { 
  ModelRead,
  ModelInvokeRequest,
  ModelRouteRequest,
  InvokeResponse,
  ProviderRead,
  ProviderCreate,
  ProviderUpdate,
  ModelCreate,
  ModelUpdate,
  LoginRecord,
  EmbeddingsRequest,
  AudioSpeechRequest,
  AudioTranscriptionRequest,
  ImagesGenerationRequest,
  VideosGenerationRequest
} from './types'
import type { OAuthAccount } from './types'

// 从环境变量获取API基础URL，开发环境使用代理，生产环境使用配置的URL
const getApiBaseUrl = () => {
  // 开发环境：使用代理路径
  if (import.meta.env.DEV) {
    return '/api'
  }
  // 生产环境：使用环境变量配置的URL，默认为 /api
  return import.meta.env.VITE_API_BASE_URL || '/api'
}

const api = axios.create({
  baseURL: getApiBaseUrl(),
  timeout: 30000,
})

// 请求拦截器：携带 Session Token 或 API Key（仪表盘在需认证环境下访问后端）
export const SESSION_TOKEN_KEY = 'llm_router_session_token'
api.interceptors.request.use((config) => {
  const sessionToken =
    typeof localStorage !== 'undefined' ? localStorage.getItem(SESSION_TOKEN_KEY) : null
  const apiKey = import.meta.env.VITE_API_KEY
  if (sessionToken) {
    config.headers.set('Authorization', `Bearer ${sessionToken}`)
  } else if (apiKey) {
    config.headers.set('Authorization', `Bearer ${apiKey}`)
  }
  return config
})

// Monitor API - 仅包含后端保留的导出端点
export const monitorApi = {
  // 导出数据为JSON
  exportJSON: async (timeRangeHours: number = 24) => {
    const response = await api.get('/monitor/export/json', {
      params: { time_range_hours: timeRangeHours },
      responseType: 'blob',
    })
    return response.data as Blob
  },

  // 导出数据为Excel
  exportExcel: async (timeRangeHours: number = 24) => {
    const response = await api.get('/monitor/export/excel', {
      params: { time_range_hours: timeRangeHours },
      responseType: 'blob',
    })
    return response.data as Blob
  },

  // 下载监控导出包
  downloadDatabase: async () => {
    const response = await api.get('/monitor/database', {
      params: { format: 'zip' },
      responseType: 'blob',
    })
    return response.data as Blob
  },
}

// 登录记录 API（从 Redis 读取）
export interface LoginRecordsResponse {
  records: LoginRecord[]
  total: number
  redis_available?: boolean
}

export const loginRecordApi = {
  getLoginRecords: async (params?: {
    limit?: number
    offset?: number
    auth_type?: string
    is_success?: boolean
  }) => {
    const response = await api.get<LoginRecordsResponse>(
      '/monitor/login-records',
      { params }
    )
    return response.data
  },
}

export const oauthApi = {
  getAuthorizeUrl: async (provider: string, providerName: string, callbackUrl: string) => {
    const response = await api.get<{ url: string }>(
      `/auth/oauth/${provider}/authorize`,
      { params: { provider_name: providerName, callback_url: callbackUrl } }
    )
    return response.data.url
  },
  getStatus: async (provider: string, providerName: string) => {
    const response = await api.get<{ provider_name: string; has_oauth: boolean }>(
      `/auth/oauth/${provider}/status`,
      { params: { provider_name: providerName } }
    )
    return response.data
  },
  revoke: async (provider: string, providerName: string) => {
    const response = await api.post<{ provider_name: string; revoked: boolean }>(
      `/auth/oauth/${provider}/revoke`,
      { provider_name: providerName }
    )
    return response.data
  },
  listAccounts: async (provider: string, providerName: string) => {
    const response = await api.get<{ provider_name: string; accounts: OAuthAccount[] }>(
      `/auth/oauth/${provider}/accounts`,
      { params: { provider_name: providerName } }
    )
    return response.data.accounts
  },
  updateAccount: async (
    provider: string,
    providerName: string,
    accountId: number,
    payload: { account_name?: string; is_default?: boolean; is_active?: boolean; settings?: Record<string, any> }
  ) => {
    const response = await api.patch<OAuthAccount>(
      `/auth/oauth/${provider}/accounts/${accountId}`,
      payload,
      { params: { provider_name: providerName } }
    )
    return response.data
  },
  setDefaultAccount: async (provider: string, providerName: string, accountId: number) => {
    const response = await api.post<OAuthAccount>(
      `/auth/oauth/${provider}/accounts/${accountId}/default`,
      {},
      { params: { provider_name: providerName } }
    )
    return response.data
  },
  revokeAccount: async (provider: string, providerName: string, accountId: number) => {
    const response = await api.delete<{ provider_name: string; account_id: number; revoked: boolean }>(
      `/auth/oauth/${provider}/accounts/${accountId}`,
      { params: { provider_name: providerName } }
    )
    return response.data
  },
}

export const providerApi = {
  // 获取所有Provider
  getProviders: async () => {
    const response = await api.get('/providers')
    return response.data as ProviderRead[]
  },

  // 创建Provider
  createProvider: async (payload: ProviderCreate) => {
    const response = await api.post<ProviderRead>('/providers', payload)
    return response.data
  },

  // 更新Provider
  updateProvider: async (providerName: string, payload: ProviderUpdate) => {
    const response = await api.patch<ProviderRead>(`/providers/${providerName}`, payload)
    return response.data
  },
}

export const modelApi = {
  // 获取所有模型
  getModels: async (providerName?: string) => {
    const url = providerName ? `/models/${providerName}` : '/models'
    const response = await api.get(url)
    return response.data as ModelRead[]
  },

  // 获取特定Provider的模型
  getProviderModels: async (providerName: string) => {
    const response = await api.get(`/models/${providerName}`)
    return response.data as ModelRead[]
  },

  // 创建模型
  createModel: async (payload: ModelCreate) => {
    const response = await api.post<ModelRead>('/models', payload)
    return response.data
  },

  // 更新模型
  updateModel: async (providerName: string, modelName: string, payload: ModelUpdate) => {
    const response = await api.patch<ModelRead>(`/models/${providerName}/${modelName}`, payload)
    return response.data
  },

  // 调用特定模型
  invokeModel: async (providerName: string, modelName: string, request: ModelInvokeRequest) => {
    const response = await api.post<InvokeResponse>(`${getApiBaseUrl()}/models/${providerName}/${modelName}/invoke`, request)
    return response.data
  },

  // 路由调用
  routeModel: async (request: ModelRouteRequest) => {
    const response = await api.post<InvokeResponse>('/models/route', request)
    return response.data
  }
}

export const configApi = {
  // 从配置文件同步到数据库
  syncFromFile: async () => {
    const response = await api.post<{ success: boolean; message: string; config_file: string }>('/config/sync')
    return response.data
  },
}

export const pricingApi = {
  // 获取最新定价信息
  getLatestPricing: async () => {
    const response = await api.get<Record<string, any[]>>('/pricing/latest')
    return response.data
  },

  // 获取定价更新建议
  getPricingSuggestions: async () => {
    const response = await api.get<import('./types').PricingSuggestion[]>('/pricing/suggestions')
    return response.data
  },

  // 同步单个模型的定价
  syncModelPricing: async (modelId: number) => {
    const response = await api.post<import('./types').PricingSyncResponse>(`/pricing/sync/${modelId}`)
    return response.data
  },

  // 同步所有模型的定价
  syncAllPricing: async () => {
    const response = await api.post<{ success: boolean; message: string; results: any }>('/pricing/sync-all')
    return response.data
  },
}

export const quotaApi = {
  getQuotaDetails: async (params?: {
    start_time?: string
    end_time?: string
    provider_name?: string
    model_name?: string
    api_key_id?: number
    limit?: number
    offset?: number
  }) => {
    const response = await api.get<Array<Record<string, any>>>('/monitor/quota-details', { params })
    return response.data
  },
  exportQuotaDetails: async (format: 'csv' | 'json' = 'csv') => {
    const response = await api.get('/monitor/quota-details/export', {
      params: { format },
      responseType: format === 'csv' ? 'blob' : 'json',
    })
    return response.data
  },
  getBudgetAlerts: async () => {
    const response = await api.get<Record<string, any>>('/monitor/budget-alerts')
    return response.data
  },
  updateBudgetAlerts: async (payload: { day_tokens: number; week_tokens: number; month_tokens: number }) => {
    const response = await api.put<Record<string, any>>('/monitor/budget-alerts', payload)
    return response.data
  },
}

export const policyTemplateApi = {
  list: async (params?: { team_tag?: string; env_tag?: string }) => {
    const response = await api.get<Array<Record<string, any>>>('/api-key-policy-templates', { params })
    return response.data
  },
  create: async (payload: Record<string, any>) => {
    const response = await api.post<Record<string, any>>('/api-key-policy-templates', payload)
    return response.data
  },
  update: async (id: number, payload: Record<string, any>) => {
    const response = await api.patch<Record<string, any>>(`/api-key-policy-templates/${id}`, payload)
    return response.data
  },
  remove: async (id: number) => {
    await api.delete(`/api-key-policy-templates/${id}`)
  },
  batchApply: async (payload: { template_id: number; api_key_ids: number[] }) => {
    const response = await api.post<Record<string, any>>('/api-keys/batch-apply-policy', payload)
    return response.data
  },
  audit: async (params?: { limit?: number; offset?: number }) => {
    const response = await api.get<Array<Record<string, any>>>('/api-keys/policy-audit', { params })
    return response.data
  },
}

export const providerCatalogApi = {
  sync: async (providerName: string) => {
    const response = await api.post<Record<string, any>>(`/providers/${providerName}/catalog-models/sync`)
    return response.data
  },
  list: async (providerName: string) => {
    const response = await api.get<{ provider_name: string; models: Array<Record<string, any>> }>(`/providers/${providerName}/catalog-models`)
    return response.data
  },
  reconcile: async (providerName: string) => {
    const response = await api.get<Record<string, any>>(`/providers/${providerName}/model-reconciliation`)
    return response.data
  },
}

export const multimodalApi = {
  embeddings: async (payload: EmbeddingsRequest) => {
    const response = await api.post('/v1/embeddings', payload)
    return response.data as Record<string, any>
  },

  speech: async (payload: AudioSpeechRequest) => {
    const response = await api.post('/v1/audio/speech', payload, {
      responseType: 'blob',
    })
    return response.data as Blob
  },

  transcribe: async (payload: AudioTranscriptionRequest) => {
    const fileDataUrl = await fileToDataUrl(payload.file)
    const response = await api.post('/v1/audio/transcriptions', {
      model: payload.model,
      file: fileDataUrl,
      prompt: payload.prompt,
      response_format: payload.response_format,
      temperature: payload.temperature,
      language: payload.language,
    })
    return response.data as Record<string, any>
  },

  translate: async (payload: AudioTranscriptionRequest) => {
    const fileDataUrl = await fileToDataUrl(payload.file)
    const response = await api.post('/v1/audio/translations', {
      model: payload.model,
      file: fileDataUrl,
      prompt: payload.prompt,
      response_format: payload.response_format,
      temperature: payload.temperature,
    })
    return response.data as Record<string, any>
  },

  generateImage: async (payload: ImagesGenerationRequest) => {
    const response = await api.post('/v1/images/generations', payload)
    return response.data as Record<string, any>
  },

  generateVideo: async (payload: VideosGenerationRequest) => {
    const response = await api.post('/v1/videos/generations', payload)
    return response.data as Record<string, any>
  },

  getVideoJob: async (jobId: string) => {
    const response = await api.get(`/v1/videos/generations/${jobId}`)
    return response.data as Record<string, any>
  },
}

const fileToDataUrl = (file: File): Promise<string> =>
  new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result))
    reader.onerror = () => reject(reader.error)
    reader.readAsDataURL(file)
  })

export default api
