package db

import "time"

// MonitorInvocation stores invocation records in an independent monitor database.
type MonitorInvocation struct {
	ID int64

	ModelID      int64
	ProviderID   int64
	ModelName    string
	ProviderName string

	StartedAt   time.Time
	CompletedAt *time.Time
	DurationMS  *float64

	Status       InvocationStatus
	ErrorMessage *string

	RequestPrompt     *string
	RequestMessages   []map[string]any
	RequestParameters map[string]any

	ResponseText       *string
	ResponseTextLength *int64

	PromptTokens     *int64
	CompletionTokens *int64
	TotalTokens      *int64

	Cost        *float64
	RawResponse map[string]any

	CreatedAt *time.Time
}
