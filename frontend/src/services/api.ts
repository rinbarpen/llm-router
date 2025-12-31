import axios from 'axios'
import type { 
  InvocationRead, 
  InvocationQuery, 
  StatisticsResponse, 
  TimeSeriesResponse,
  GroupedTimeSeriesResponse,
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

export const monitorApi = {
  // 获取调用历史列表
  getInvocations: async (query: Partial<InvocationQuery> = {}) => {
    const params = new URLSearchParams()
    if (query.model_id) params.append('model_id', query.model_id.toString())
    if (query.provider_id) params.append('provider_id', query.provider_id.toString())
    if (query.model_name) params.append('model_name', query.model_name)
    if (query.provider_name) params.append('provider_name', query.provider_name)
    if (query.status) params.append('status', query.status)
    if (query.start_time) params.append('start_time', query.start_time.toISOString())
    if (query.end_time) params.append('end_time', query.end_time.toISOString())
    params.append('limit', (query.limit || 100).toString())
    params.append('offset', (query.offset || 0).toString())
    params.append('order_by', query.order_by || 'started_at')
    params.append('order_desc', (query.order_desc !== false).toString())

    const response = await api.get('/monitor/invocations', { params })
    return response.data as {
      items: InvocationRead[]
      total: number
      limit: number
      offset: number
    }
  },

  // 获取单次调用详情
  getInvocationById: async (id: number) => {
    const response = await api.get(`/monitor/invocations/${id}`)
    return response.data as InvocationRead
  },

  // 获取统计信息
  getStatistics: async (timeRangeHours: number = 24, limit: number = 10) => {
    const response = await api.get('/monitor/statistics', {
      params: {
        time_range_hours: timeRangeHours,
        limit,
      },
    })
    return response.data as StatisticsResponse
  },

  // 获取时间序列数据
  getTimeSeries: async (granularity: 'hour' | 'day' | 'week' | 'month' = 'day', timeRangeHours: number = 168) => {
    const response = await api.get('/monitor/time-series', {
      params: {
        granularity,
        time_range_hours: timeRangeHours,
      },
    })
    return response.data as TimeSeriesResponse
  },

  // 获取分组时间序列数据
  getGroupedTimeSeries: async (
    groupBy: 'model' | 'provider',
    granularity: 'hour' | 'day' | 'week' | 'month' = 'day',
    timeRangeHours: number = 168
  ) => {
    const response = await api.get('/monitor/time-series/grouped', {
      params: {
        group_by: groupBy,
        granularity,
        time_range_hours: timeRangeHours,
      },
    })
    return response.data as GroupedTimeSeriesResponse
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
