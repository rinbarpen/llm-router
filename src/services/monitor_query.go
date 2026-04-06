package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rinbarpen/llm-router/src/schemas"
)

type InvocationQueryOptions struct {
	ModelID      *int64
	ProviderID   *int64
	APIKeyID     *int64
	ModelName    string
	ProviderName string
	Status       string
	AuthType     string
	StartTime    *time.Time
	EndTime      *time.Time
	Limit        int
	Offset       int
	OrderBy      string
	OrderDesc    bool
}

func (s *CatalogService) QueryInvocations(ctx context.Context, opts InvocationQueryOptions) ([]schemas.MonitorInvocation, error) {
	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	if opts.Limit > 200 {
		opts.Limit = 200
	}
	if opts.Offset < 0 {
		opts.Offset = 0
	}
	orderBy := "id"
	switch strings.TrimSpace(opts.OrderBy) {
	case "started_at", "duration_ms", "total_tokens", "cost", "id":
		orderBy = strings.TrimSpace(opts.OrderBy)
	}
	orderDir := "DESC"
	if !opts.OrderDesc {
		orderDir = "ASC"
	}
	where := make([]string, 0)
	args := make([]any, 0)
	arg := 1
	add := func(cond string, val any) {
		where = append(where, fmt.Sprintf(cond, arg))
		args = append(args, val)
		arg++
	}
	if opts.ModelID != nil {
		add("model_id = $%d", *opts.ModelID)
	}
	if opts.ProviderID != nil {
		add("provider_id = $%d", *opts.ProviderID)
	}
	if opts.APIKeyID != nil {
		add("api_key_id = $%d", *opts.APIKeyID)
	}
	if strings.TrimSpace(opts.ModelName) != "" {
		add("model_name = $%d", strings.TrimSpace(opts.ModelName))
	}
	if strings.TrimSpace(opts.ProviderName) != "" {
		add("provider_name = $%d", strings.TrimSpace(opts.ProviderName))
	}
	if strings.TrimSpace(opts.Status) != "" {
		add("status = $%d", strings.TrimSpace(opts.Status))
	}
	if strings.TrimSpace(opts.AuthType) != "" {
		add("auth_type = $%d", strings.TrimSpace(opts.AuthType))
	}
	if opts.StartTime != nil {
		add("started_at >= $%d", opts.StartTime.UTC())
	}
	if opts.EndTime != nil {
		add("started_at <= $%d", opts.EndTime.UTC())
	}
	sql := `
		SELECT id, model_id, provider_id, api_key_id, api_key_name, auth_type,
		       model_name, provider_name, started_at, completed_at, duration_ms,
		       first_token_ms, stream_duration_ms, stream_end_reason,
		       status, error_message, request_prompt, response_text, response_text_length,
		       prompt_tokens, completion_tokens, total_tokens, cost, created_at
		FROM monitor_invocations
	`
	if len(where) > 0 {
		sql += " WHERE " + strings.Join(where, " AND ")
	}
	sql += fmt.Sprintf(" ORDER BY %s %s LIMIT $%d OFFSET $%d", orderBy, orderDir, arg, arg+1)
	args = append(args, opts.Limit, opts.Offset)

	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query monitor invocations: %w", err)
	}
	defer rows.Close()
	out := make([]schemas.MonitorInvocation, 0)
	for rows.Next() {
		var item schemas.MonitorInvocation
		if err := rows.Scan(
			&item.ID, &item.ModelID, &item.ProviderID, &item.APIKeyID, &item.APIKeyName, &item.AuthType,
			&item.ModelName, &item.ProviderName, &item.StartedAt, &item.CompletedAt, &item.DurationMS,
			&item.FirstTokenMS, &item.StreamDurationMS, &item.StreamEndReason,
			&item.Status, &item.ErrorMessage, &item.RequestPrompt, &item.ResponseText, &item.ResponseTextLength,
			&item.PromptTokens, &item.CompletionTokens, &item.TotalTokens, &item.Cost, &item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan monitor invocation: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate monitor invocations: %w", err)
	}
	return out, nil
}
