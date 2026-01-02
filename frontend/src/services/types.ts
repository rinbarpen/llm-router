export type InvocationStatus = 'success' | 'error'

export interface InvocationRead {
  id: number
  model_id: number
  provider_id: number
  model_name: string
  provider_name: string
  started_at: string
  completed_at: string | null
  duration_ms: number | null
  status: InvocationStatus
  error_message: string | null
  request_prompt: string | null
  request_messages: Array<{ role: string; content: string }> | null
  request_parameters: Record<string, any>
  response_text: string | null
  response_text_length: number | null
  prompt_tokens: number | null
  completion_tokens: number | null
  total_tokens: number | null
  cost: number | null  // 成本（USD）
  raw_response: Record<string, any> | null
  created_at: string
}

export interface InvocationQuery {
  model_id?: number
  provider_id?: number
  model_name?: string
  provider_name?: string
  status?: InvocationStatus
  start_time?: Date
  end_time?: Date
  limit?: number
  offset?: number
  order_by?: 'started_at' | 'duration_ms' | 'total_tokens'
  order_desc?: boolean
}

export interface TimeRangeStatistics {
  time_range: string
  total_calls: number
  success_calls: number
  error_calls: number
  success_rate: number
  total_tokens: number
  avg_duration_ms: number | null
  total_cost: number | null  // 总成本（USD）
}

export interface ModelStatistics {
  model_id: number
  model_name: string
  provider_name: string
  total_calls: number
  success_calls: number
  error_calls: number
  success_rate: number
  total_tokens: number
  prompt_tokens: number
  completion_tokens: number
  avg_duration_ms: number | null
  total_duration_ms: number
  total_cost: number | null  // 总成本（USD）
}

export interface StatisticsResponse {
  overall: TimeRangeStatistics
  by_model: ModelStatistics[]
  recent_errors: InvocationRead[]
}

export interface TimeSeriesDataPoint {
  timestamp: string
  total_calls: number
  success_calls: number
  error_calls: number
  total_tokens: number
  prompt_tokens?: number
  completion_tokens?: number
}

export interface TimeSeriesResponse {
  granularity: 'hour' | 'day' | 'week' | 'month'
  data: TimeSeriesDataPoint[]
}

export interface GroupedTimeSeriesDataPoint {
  timestamp: string
  group_name: string
  total_calls: number
  success_calls: number
  error_calls: number
  total_tokens: number
  prompt_tokens?: number
  completion_tokens?: number
}

export interface GroupedTimeSeriesResponse {
  granularity: 'hour' | 'day' | 'week' | 'month'
  group_by: 'model' | 'provider'
  data: GroupedTimeSeriesDataPoint[]
}

// New Types for Chat/Playground

export interface ModelRead {
  id: number
  provider_id: number
  provider_name: string
  name: string
  type: string // 'chat' | 'completion' | 'embedding'
  cost_per_1k_tokens?: number
  cost_per_1k_completion_tokens?: number
  is_active: boolean
  tags: string[]
}

export interface ModelInvokeRequest {
  prompt?: string
  messages?: Array<{ role: string; content: string }>
  parameters?: Record<string, any>
  stream?: boolean
}

export interface ModelQuery {
  tags?: string[]
  provider_types?: string[]
  include_inactive?: boolean
}

export interface ModelRouteRequest {
  query: ModelQuery
  request: ModelInvokeRequest
}

export interface InvokeResponse {
  output_text: string
  raw?: any
  usage?: {
    prompt_tokens: number
    completion_tokens: number
    total_tokens: number
  }
}
