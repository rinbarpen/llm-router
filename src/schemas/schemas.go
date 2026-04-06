package schemas

import "time"

type Provider struct {
	ID        int64          `json:"id"`
	Name      string         `json:"name"`
	Type      string         `json:"type"`
	IsActive  bool           `json:"is_active"`
	BaseURL   *string        `json:"base_url,omitempty"`
	APIKey    *string        `json:"api_key,omitempty"`
	Settings  map[string]any `json:"settings,omitempty"`
	CreatedAt *time.Time     `json:"created_at,omitempty"`
	UpdatedAt *time.Time     `json:"updated_at,omitempty"`
}

type ProviderCreate struct {
	Name     string         `json:"name"`
	Type     string         `json:"type"`
	BaseURL  *string        `json:"base_url,omitempty"`
	APIKey   *string        `json:"api_key,omitempty"`
	Settings map[string]any `json:"settings,omitempty"`
}

type ProviderUpdate struct {
	BaseURL  *string        `json:"base_url,omitempty"`
	APIKey   *string        `json:"api_key,omitempty"`
	IsActive *bool          `json:"is_active,omitempty"`
	Settings map[string]any `json:"settings,omitempty"`
}

type Model struct {
	ID               int64          `json:"id"`
	ProviderID       int64          `json:"provider_id,omitempty"`
	ProviderName     string         `json:"provider_name"`
	Name             string         `json:"name"`
	DisplayName      *string        `json:"display_name,omitempty"`
	Description      *string        `json:"description,omitempty"`
	IsActive         bool           `json:"is_active"`
	RemoteIdentifier *string        `json:"remote_identifier,omitempty"`
	DefaultParams    map[string]any `json:"default_params,omitempty"`
	Config           map[string]any `json:"config,omitempty"`
	DownloadURI      *string        `json:"download_uri,omitempty"`
	LocalPath        *string        `json:"local_path,omitempty"`
	CreatedAt        *time.Time     `json:"created_at,omitempty"`
	UpdatedAt        *time.Time     `json:"updated_at,omitempty"`
}

type ModelCreate struct {
	ProviderName     string         `json:"provider_name"`
	Name             string         `json:"name"`
	DisplayName      *string        `json:"display_name,omitempty"`
	Description      *string        `json:"description,omitempty"`
	RemoteIdentifier *string        `json:"remote_identifier,omitempty"`
	DefaultParams    map[string]any `json:"default_params,omitempty"`
	Config           map[string]any `json:"config,omitempty"`
	DownloadURI      *string        `json:"download_uri,omitempty"`
	LocalPath        *string        `json:"local_path,omitempty"`
}

type ModelUpdate struct {
	DisplayName      *string        `json:"display_name,omitempty"`
	Description      *string        `json:"description,omitempty"`
	IsActive         *bool          `json:"is_active,omitempty"`
	RemoteIdentifier *string        `json:"remote_identifier,omitempty"`
	DefaultParams    map[string]any `json:"default_params,omitempty"`
	Config           map[string]any `json:"config,omitempty"`
	DownloadURI      *string        `json:"download_uri,omitempty"`
	LocalPath        *string        `json:"local_path,omitempty"`
}

type OpenAIModelsResponse struct {
	Object string              `json:"object"`
	Data   []OpenAIModelObject `json:"data"`
}

type OpenAIModelObject struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type APIKey struct {
	ID               int64          `json:"id"`
	Key              *string        `json:"key,omitempty"`
	Name             *string        `json:"name,omitempty"`
	IsActive         bool           `json:"is_active"`
	ExpiresAt        *time.Time     `json:"expires_at,omitempty"`
	QuotaTokensMonth *int64         `json:"quota_tokens_monthly,omitempty"`
	IPAllowlist      []string       `json:"ip_allowlist,omitempty"`
	AllowedModels    []string       `json:"allowed_models,omitempty"`
	AllowedProviders []string       `json:"allowed_providers,omitempty"`
	ParameterLimits  map[string]any `json:"parameter_limits,omitempty"`
	CreatedAt        *time.Time     `json:"created_at,omitempty"`
	UpdatedAt        *time.Time     `json:"updated_at,omitempty"`
}

type APIKeyCreate struct {
	Key              *string        `json:"key,omitempty"`
	Name             *string        `json:"name,omitempty"`
	ExpiresAt        *time.Time     `json:"expires_at,omitempty"`
	QuotaTokensMonth *int64         `json:"quota_tokens_monthly,omitempty"`
	IPAllowlist      []string       `json:"ip_allowlist,omitempty"`
	AllowedModels    []string       `json:"allowed_models,omitempty"`
	AllowedProviders []string       `json:"allowed_providers,omitempty"`
	ParameterLimits  map[string]any `json:"parameter_limits,omitempty"`
}

type APIKeyUpdate struct {
	Name             *string        `json:"name,omitempty"`
	IsActive         *bool          `json:"is_active,omitempty"`
	ExpiresAt        *time.Time     `json:"expires_at,omitempty"`
	QuotaTokensMonth *int64         `json:"quota_tokens_monthly,omitempty"`
	IPAllowlist      []string       `json:"ip_allowlist,omitempty"`
	AllowedModels    []string       `json:"allowed_models,omitempty"`
	AllowedProviders []string       `json:"allowed_providers,omitempty"`
	ParameterLimits  map[string]any `json:"parameter_limits,omitempty"`
}

type LoginRequest struct {
	APIKey string `json:"api_key"`
}

type OAuthAccount struct {
	ID           int64          `json:"id"`
	ProviderID   int64          `json:"provider_id"`
	ProviderName string         `json:"provider_name"`
	ProviderType string         `json:"provider_type"`
	AccountName  string         `json:"account_name"`
	IsDefault    bool           `json:"is_default"`
	IsActive     bool           `json:"is_active"`
	AccessToken  *string        `json:"access_token,omitempty"`
	RefreshToken *string        `json:"refresh_token,omitempty"`
	APIKey       *string        `json:"api_key,omitempty"`
	ExpiresAt    *time.Time     `json:"expires_at,omitempty"`
	Settings     map[string]any `json:"settings,omitempty"`
	CreatedAt    *time.Time     `json:"created_at,omitempty"`
	UpdatedAt    *time.Time     `json:"updated_at,omitempty"`
}

type OAuthAccountUpdate struct {
	AccountName *string        `json:"account_name,omitempty"`
	IsDefault   *bool          `json:"is_default,omitempty"`
	IsActive    *bool          `json:"is_active,omitempty"`
	Settings    map[string]any `json:"settings,omitempty"`
}

type MonitorInvocation struct {
	ID                 int64      `json:"id"`
	ModelID            int64      `json:"model_id"`
	ProviderID         int64      `json:"provider_id"`
	APIKeyID           *int64     `json:"api_key_id,omitempty"`
	APIKeyName         *string    `json:"api_key_name,omitempty"`
	AuthType           *string    `json:"auth_type,omitempty"`
	ModelName          string     `json:"model_name"`
	ProviderName       string     `json:"provider_name"`
	StartedAt          *time.Time `json:"started_at,omitempty"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
	DurationMS         *float64   `json:"duration_ms,omitempty"`
	FirstTokenMS       *float64   `json:"first_token_ms,omitempty"`
	StreamDurationMS   *float64   `json:"stream_duration_ms,omitempty"`
	StreamEndReason    *string    `json:"stream_end_reason,omitempty"`
	Status             string     `json:"status"`
	ErrorMessage       *string    `json:"error_message,omitempty"`
	RequestPrompt      *string    `json:"request_prompt,omitempty"`
	ResponseText       *string    `json:"response_text,omitempty"`
	ResponseTextLength *int64     `json:"response_text_length,omitempty"`
	PromptTokens       *int64     `json:"prompt_tokens,omitempty"`
	CompletionTokens   *int64     `json:"completion_tokens,omitempty"`
	TotalTokens        *int64     `json:"total_tokens,omitempty"`
	Cost               *float64   `json:"cost,omitempty"`
	CreatedAt          *time.Time `json:"created_at,omitempty"`
}

// TimeSeriesDataPoint matches Python TimeSeriesDataPoint for /monitor/time-series.
type TimeSeriesDataPoint struct {
	Timestamp        time.Time `json:"timestamp"`
	TotalCalls       int64     `json:"total_calls"`
	SuccessCalls     int64     `json:"success_calls"`
	ErrorCalls       int64     `json:"error_calls"`
	TotalTokens      int64     `json:"total_tokens"`
	PromptTokens     int64     `json:"prompt_tokens"`
	CompletionTokens int64     `json:"completion_tokens"`
	TotalCost        *float64  `json:"total_cost,omitempty"`
}

// TimeSeriesResponse matches Python TimeSeriesResponse.
type TimeSeriesResponse struct {
	Granularity string                `json:"granularity"`
	Data        []TimeSeriesDataPoint `json:"data"`
}

// GroupedTimeSeriesDataPoint matches Python GroupedTimeSeriesDataPoint.
type GroupedTimeSeriesDataPoint struct {
	Timestamp        time.Time `json:"timestamp"`
	GroupName        string    `json:"group_name"`
	TotalCalls       int64     `json:"total_calls"`
	SuccessCalls     int64     `json:"success_calls"`
	ErrorCalls       int64     `json:"error_calls"`
	TotalTokens      int64     `json:"total_tokens"`
	PromptTokens     int64     `json:"prompt_tokens"`
	CompletionTokens int64     `json:"completion_tokens"`
	TotalCost        *float64  `json:"total_cost,omitempty"`
}

// GroupedTimeSeriesResponse matches Python GroupedTimeSeriesResponse.
type GroupedTimeSeriesResponse struct {
	Granularity string                       `json:"granularity"`
	GroupBy     string                       `json:"group_by"`
	Data        []GroupedTimeSeriesDataPoint `json:"data"`
}
