export type InvocationStatus = 'success' | 'error'

export interface LoginRecord {
  id: string
  timestamp: string
  ip_address: string
  auth_type: string  // 'api_key' | 'session_token' | 'none'
  is_success: boolean
  api_key_id: number | null
  session_token_hash: string | null
  is_local: boolean
}

export interface InvocationRead {
  id: number
  model_id: number
  provider_id: number
  model_name: string
  provider_name: string
  started_at: string
  completed_at: string | null
  duration_ms: number | null
  first_token_ms: number | null
  stream_duration_ms: number | null
  stream_end_reason: string | null
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

export type ProviderType =
  | 'openai'
  | 'gemini'
  | 'claude'
  | 'codex_cli'
  | 'claude_code_cli'
  | 'opencode_cli'
  | 'kimi_code_cli'
  | 'qwen_code_cli'
  | 'openrouter'
  | 'azure_openai'
  | 'huggingface'
  | 'minimax'
  | 'doubao'
  | 'glm'
  | 'kimi'
  | 'qwen'
  | 'grok'
  | 'groq'
  | 'deepseek'
  | 'siliconflow'
  | 'aihubmix'
  | 'volcengine'
  | 'ollama'
  | 'transformers'
  | 'vllm'
  | 'remote_http'
  | 'custom_http'
  | 'bigmodel'
  | 'z.ai'
  | 'transformers_local'
  | 'vllm_local'
  | 'ollama_local'

export interface ProviderRead {
  id: number
  name: string
  type: ProviderType
  is_active: boolean
  base_url: string | null
  api_key?: string | null
}

export interface ProviderWithDetails extends ProviderRead {
  api_key?: string | null
  settings?: Record<string, any>
}

export interface ProviderUpdate {
  type?: ProviderType
  base_url?: string | null
  api_key?: string | null
  is_active?: boolean
  settings?: Record<string, any>
}

export interface ProviderCreate {
  name: string
  type: ProviderType
  base_url?: string | null
  api_key?: string | null
  is_active?: boolean
  settings?: Record<string, any>
}

export interface ParameterLimits {
  max_tokens?: number | null
  temperature?: number | null
  top_p?: number | null
  frequency_penalty?: number | null
  presence_penalty?: number | null
  custom_limits?: Record<string, any>
  [key: string]: any
}

export interface APIKeyRead {
  id: number
  key?: string | null
  name?: string | null
  is_active: boolean
  owner_type?: string | null
  owner_id?: number | null
  created_by_user_id?: number | null
  expires_at?: string | null
  quota_tokens_monthly?: number | null
  ip_allowlist?: string[]
  allowed_models?: string[]
  allowed_providers?: string[]
  parameter_limits?: ParameterLimits
  created_at?: string | null
  updated_at?: string | null
}

export interface APIKeyCreate {
  key?: string | null
  name?: string | null
  owner_type?: string | null
  owner_id?: number | null
  created_by_user_id?: number | null
  expires_at?: string | null
  quota_tokens_monthly?: number | null
  ip_allowlist?: string[]
  allowed_models?: string[]
  allowed_providers?: string[]
  parameter_limits?: ParameterLimits
}

export interface APIKeyUpdate {
  name?: string | null
  is_active?: boolean
  owner_type?: string | null
  owner_id?: number | null
  created_by_user_id?: number | null
  expires_at?: string | null
  quota_tokens_monthly?: number | null
  ip_allowlist?: string[]
  allowed_models?: string[]
  allowed_providers?: string[]
  parameter_limits?: ParameterLimits
}

export interface RateLimitConfig {
  max_requests: number
  per_seconds: number
  burst_size?: number | null
  notes?: string | null
  config?: Record<string, any>
}

export interface ModelRead {
  id: number
  provider_id: number
  provider_name: string
  provider_type: ProviderType
  name: string
  display_name: string | null
  description: string | null
  tags: string[]
  default_params: Record<string, any>
  config: Record<string, any>
  rate_limit: RateLimitConfig | null
  local_path: string | null
  is_active?: boolean
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

// Model Management Types
export interface ModelCreate {
  name: string
  provider_id?: number
  provider_name?: string
  display_name?: string
  description?: string
  remote_identifier?: string
  is_active?: boolean
  tags?: string[]
  default_params?: Record<string, any>
  config?: Record<string, any>
  download_uri?: string
  local_path?: string
  rate_limit?: RateLimitConfig
}

export interface ModelUpdate {
  display_name?: string
  description?: string
  is_active?: boolean
  tags?: string[]
  default_params?: Record<string, any>
  config?: Record<string, any>
  download_uri?: string
  local_path?: string
  rate_limit?: RateLimitConfig
}

export interface RemoteProviderModel {
  provider_name: string
  provider_type: ProviderType
  model_name: string
  local_name: string
  display_name?: string
  remote_identifier: string
  metadata?: Record<string, any>
}

export interface ModelUpdateRun {
  provider_name: string
  started_at?: string
  completed_at?: string
  added: string[]
  updated: string[]
  deleted?: string[]
  disabled?: string[]
  skipped?: string[]
  error?: string
  backup_path?: string
}

export interface ModelUpdateResult {
  started_at?: string
  completed_at?: string
  provider_runs: ModelUpdateRun[]
  backup_path?: string
}

// 定价相关类型
export interface ModelPricingInfo {
  model_name: string
  provider: string
  input_price_per_1k: number
  output_price_per_1k: number
  source: string
  last_updated: string
  notes?: string | null
}

export interface PricingSuggestion {
  model_id: number
  model_name: string
  provider_name: string
  current_input_price?: number | null
  current_output_price?: number | null
  latest_input_price?: number | null
  latest_output_price?: number | null
  has_update: boolean
  pricing_info?: ModelPricingInfo | null
}

export interface PricingSyncResponse {
  success: boolean
  message: string
  updated_pricing?: ModelPricingInfo | null
}

export interface OAuthAccount {
  id: number
  provider_id: number
  provider_name: string
  provider_type: string
  account_name: string
  is_default: boolean
  is_active: boolean
  access_token?: string | null
  refresh_token?: string | null
  api_key?: string | null
  expires_at?: string | null
  settings?: Record<string, any>
  created_at?: string | null
  updated_at?: string | null
}

export interface ConsoleUser {
  id: number
  email: string
  display_name: string
  status: string
  roles: string[]
  created_at?: string | null
  updated_at?: string | null
}

export interface ConsoleUserUpdate {
  display_name?: string | null
  status?: string | null
}

export interface ConsoleSession {
  token?: string
  expires_at?: string | null
  user: ConsoleUser
}

export interface TeamRead {
  id: number
  name: string
  slug: string
  status: string
  description?: string | null
  created_at?: string | null
  updated_at?: string | null
}

export interface TeamCreate {
  name: string
  slug: string
  description?: string | null
}

export interface TeamMemberCreate {
  user_id: number
  role: string
}

export interface TeamInviteRead {
  id: number
  team_id: number
  email: string
  role: string
  invite_token: string
  status: string
  expires_at?: string | null
  created_at?: string | null
  updated_at?: string | null
}

export interface TeamInviteCreate {
  email: string
  role: string
}

export interface TeamMemberRead {
  id: number
  team_id: number
  user_id: number
  user_email: string
  display_name: string
  role: string
  status: string
  created_at?: string | null
  updated_at?: string | null
}

export interface TeamMemberUpdate {
  role?: string | null
  status?: string | null
}

export interface WalletRead {
  id: number
  owner_type: string
  owner_id: number
  currency: string
  balance: number
  status: string
  created_at?: string | null
  updated_at?: string | null
}

export interface RechargeOrderRead {
  id: number
  order_no: string
  owner_type: string
  owner_id: number
  amount: number
  currency: string
  status: string
  payment_provider: string
  created_at?: string | null
  updated_at?: string | null
}

export interface RechargeCheckout {
  provider: string
  order_no: string
  payment_url?: string
  qr_code_text?: string
}

export interface EmbeddingsRequest {
  model: string
  input: string | string[]
  encoding_format?: 'float' | 'base64'
  dimensions?: number
  user?: string
}

export interface AudioSpeechRequest {
  model: string
  input: string
  voice: string
  response_format?: string
  speed?: number
}

export interface TTSPluginInfo {
  name: string
  default_model?: string
  models: string[]
}

export interface TTSVoiceInfo {
  id: string
  display_name?: string
  character?: string
  character_display_name?: string
  timbre?: string
  timbre_display_name?: string
  downloaded: boolean
  downloading?: boolean
  error?: string
}

export interface AudioTranscriptionRequest {
  model: string
  file: File
  prompt?: string
  response_format?: string
  temperature?: number
  language?: string
}

export interface ImagesGenerationRequest {
  model: string
  prompt: string
  n?: number
  size?: string
  quality?: string
  response_format?: 'url' | 'b64_json'
  style?: string
}

export interface VideosGenerationRequest {
  model: string
  prompt: string
  size?: string
  duration?: number
  fps?: number
  response_format?: 'url' | 'b64_json'
}

export type ChatRole = 'system' | 'user' | 'assistant' | 'tool'

export interface ChatMessage {
  id: string
  role: ChatRole
  content: string
  createdAt: string
  toolCalls?: ChatToolCall[]
}

export interface ChatToolCall {
  id: string
  index: number
  type: string
  name: string
  arguments: string
}

export interface ChatToolCallDelta {
  id?: string
  index: number
  type?: string
  name?: string
  argumentsPart?: string
}

export interface ChatUsage {
  prompt_tokens?: number
  completion_tokens?: number
  total_tokens?: number
  cost?: number
}

export interface ChatCompletionChoice {
  index: number
  message?: {
    role: ChatRole
    content: string
    tool_calls?: Array<{
      id?: string
      type?: string
      function?: {
        name?: string
        arguments?: string
      }
    }>
  }
  delta?: {
    role?: ChatRole
    content?: string
    tool_calls?: Array<{
      index: number
      id?: string
      type?: string
      function?: {
        name?: string
        arguments?: string
      }
    }>
  }
  finish_reason?: string | null
}

export interface ChatCompletionResponse {
  id?: string
  object?: string
  created?: number
  model?: string
  choices?: ChatCompletionChoice[]
  usage?: ChatUsage
  cost?: number
  [key: string]: any
}

export interface ChatCompletionRequest {
  model: string
  messages: Array<{
    role: ChatRole
    content:
      | string
      | Array<
          | { type: 'text'; text: string }
          | { type: 'image_url'; image_url: { url: string } }
        >
  }>
  stream?: boolean
  temperature?: number
  max_tokens?: number
  top_p?: number
  tools?: Array<Record<string, any>>
  tool_choice?: string | Record<string, any>
  skills?: Array<Record<string, any>> | string[]
  [key: string]: any
}

export interface ChatDebugTrace {
  request: ChatCompletionRequest
  response?: ChatCompletionResponse
  events?: ChatCompletionResponse[]
  error?: string
}

export interface ChatSettings {
  model: string
  temperature: number
  maxTokens: number
  topP: number
  stream: boolean
  systemPrompt: string
  toolsJson?: string
  skillsJson?: string
  toolChoiceJson?: string
  extraBodyJson?: string
}

export interface ChatSession {
  id: string
  title: string
  createdAt: string
  updatedAt: string
  settings: ChatSettings
  messages: ChatMessage[]
  traces: ChatDebugTrace[]
}
