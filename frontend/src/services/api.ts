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
  ModelUpdate
} from './types'

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

  // 下载数据库文件
  downloadDatabase: async () => {
    const response = await api.get('/monitor/database', {
      responseType: 'blob',
    })
    return response.data as Blob
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

export default api
