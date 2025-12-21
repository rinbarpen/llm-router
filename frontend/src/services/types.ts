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
}

export interface TimeSeriesResponse {
  granularity: 'hour' | 'day' | 'week' | 'month'
  data: TimeSeriesDataPoint[]
}

