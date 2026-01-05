import axios from 'axios'
import type { 
  ModelRead,
  ModelInvokeRequest,
  ModelRouteRequest,
  InvokeResponse
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

export const modelApi = {
  // 获取所有模型
  getModels: async () => {
    const response = await api.get('/models')
    return response.data as ModelRead[]
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

export default api
