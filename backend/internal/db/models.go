package db

import "time"

type ProviderType string

const (
	ProviderTypeRemoteHTTP    ProviderType = "remote_http"
	ProviderTypeTransformers  ProviderType = "transformers"
	ProviderTypeOllama        ProviderType = "ollama"
	ProviderTypeVLLM          ProviderType = "vllm"
	ProviderTypeCustomHTTP    ProviderType = "custom_http"
	ProviderTypeOpenAI        ProviderType = "openai"
	ProviderTypeGemini        ProviderType = "gemini"
	ProviderTypeClaude        ProviderType = "claude"
	ProviderTypeGrok          ProviderType = "grok"
	ProviderTypeDeepseek      ProviderType = "deepseek"
	ProviderTypeQwen          ProviderType = "qwen"
	ProviderTypeKimi          ProviderType = "kimi"
	ProviderTypeGLM           ProviderType = "glm"
	ProviderTypeOpenRouter    ProviderType = "openrouter"
	ProviderTypeAzureOpenAI   ProviderType = "azure_openai"
	ProviderTypeHuggingFace   ProviderType = "huggingface"
	ProviderTypeMiniMax       ProviderType = "minimax"
	ProviderTypeDoubao        ProviderType = "doubao"
	ProviderTypeGroq          ProviderType = "groq"
	ProviderTypeSiliconFlow   ProviderType = "siliconflow"
	ProviderTypeAIHubMix      ProviderType = "aihubmix"
	ProviderTypeVolcengine    ProviderType = "volcengine"
	ProviderTypeCodexCLI      ProviderType = "codex_cli"
	ProviderTypeClaudeCodeCLI ProviderType = "claude_code_cli"
	ProviderTypeOpenCodeCLI   ProviderType = "opencode_cli"
	ProviderTypeKimiCodeCLI   ProviderType = "kimi_code_cli"
	ProviderTypeQwenCodeCLI   ProviderType = "qwen_code_cli"
)

type InvocationStatus string

const (
	InvocationStatusSuccess InvocationStatus = "success"
	InvocationStatusError   InvocationStatus = "error"
)

type Provider struct {
	ID        int64
	Name      string
	Type      ProviderType
	IsActive  bool
	BaseURL   *string
	APIKey    *string
	Settings  map[string]any
	CreatedAt *time.Time
	UpdatedAt *time.Time
}

type ProviderOAuthCredential struct {
	ID           int64
	ProviderID   int64
	ProviderType string
	AccessToken  *string
	RefreshToken *string
	APIKey       *string
	ExpiresAt    *time.Time
	CreatedAt    *time.Time
	UpdatedAt    *time.Time
}

type Model struct {
	ID               int64
	ProviderID       int64
	Name             string
	DisplayName      *string
	Description      *string
	IsActive         bool
	RemoteIdentifier *string
	DefaultParams    map[string]any
	Config           map[string]any
	DownloadURI      *string
	LocalPath        *string
	CreatedAt        *time.Time
	UpdatedAt        *time.Time
}

type RateLimit struct {
	ID          int64
	ModelID     int64
	MaxRequests int64
	PerSeconds  int64
	BurstSize   *int64
	Notes       *string
	Config      map[string]any
}

// APIKey maps python's Tag(APIKey alias) row.
type APIKey struct {
	ID               int64
	Key              *string
	Name             *string
	IsActive         bool
	AllowedModels    []string
	AllowedProviders []string
	ParameterLimits  map[string]any
	CreatedAt        *time.Time
	UpdatedAt        *time.Time
}

type ModelTag struct {
	ModelID int64
	TagID   int64
}
