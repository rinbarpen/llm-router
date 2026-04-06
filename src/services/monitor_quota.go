package services

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"time"
)

type QuotaDetailQuery struct {
	StartTime    *time.Time
	EndTime      *time.Time
	ProviderName string
	ModelName    string
	APIKeyID     *int64
	Limit        int
	Offset       int
}

func (s *CatalogService) GetQuotaDetails(ctx context.Context, q QuotaDetailQuery) ([]map[string]any, error) {
	if q.Limit <= 0 {
		q.Limit = 100
	}
	if q.Limit > 500 {
		q.Limit = 500
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
	args := make([]any, 0)
	where := " WHERE 1=1 "
	if q.StartTime != nil {
		args = append(args, q.StartTime.UTC())
		where += fmt.Sprintf(" AND started_at >= $%d", len(args))
	}
	if q.EndTime != nil {
		args = append(args, q.EndTime.UTC())
		where += fmt.Sprintf(" AND started_at <= $%d", len(args))
	}
	if q.ProviderName != "" {
		args = append(args, q.ProviderName)
		where += fmt.Sprintf(" AND provider_name = $%d", len(args))
	}
	if q.ModelName != "" {
		args = append(args, q.ModelName)
		where += fmt.Sprintf(" AND model_name = $%d", len(args))
	}
	if q.APIKeyID != nil {
		args = append(args, *q.APIKeyID)
		where += fmt.Sprintf(" AND api_key_id = $%d", len(args))
	}
	args = append(args, q.Limit, q.Offset)
	sql := `
		SELECT COALESCE(api_key_id,0) AS api_key_id,
		       COALESCE(api_key_name,'unknown') AS api_key_name,
		       provider_name,
		       model_name,
		       COUNT(*)::bigint AS requests,
		       COALESCE(SUM(total_tokens),0)::bigint AS total_tokens,
		       COALESCE(SUM(cost),0) AS total_cost
		FROM monitor_invocations
	` + where + `
		GROUP BY api_key_id, api_key_name, provider_name, model_name
		ORDER BY total_tokens DESC
		LIMIT $` + fmt.Sprintf("%d", len(args)-1) + ` OFFSET $` + fmt.Sprintf("%d", len(args))
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query quota details: %w", err)
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var (
			apiKeyID   int64
			apiKeyName string
			provider   string
			model      string
			requests   int64
			tokens     int64
			cost       float64
		)
		if err := rows.Scan(&apiKeyID, &apiKeyName, &provider, &model, &requests, &tokens, &cost); err != nil {
			return nil, fmt.Errorf("scan quota detail: %w", err)
		}
		out = append(out, map[string]any{
			"api_key_id":   apiKeyID,
			"api_key_name": apiKeyName,
			"provider":     provider,
			"model":        model,
			"requests":     requests,
			"total_tokens": tokens,
			"total_cost":   cost,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate quota details: %w", err)
	}
	return out, nil
}

func (s *CatalogService) ExportQuotaDetailsCSV(ctx context.Context, q QuotaDetailQuery) ([]byte, error) {
	rows, err := s.GetQuotaDetails(ctx, q)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"api_key_id", "api_key_name", "provider", "model", "requests", "total_tokens", "total_cost"})
	for _, row := range rows {
		_ = w.Write([]string{
			fmt.Sprintf("%v", row["api_key_id"]),
			fmt.Sprintf("%v", row["api_key_name"]),
			fmt.Sprintf("%v", row["provider"]),
			fmt.Sprintf("%v", row["model"]),
			fmt.Sprintf("%v", row["requests"]),
			fmt.Sprintf("%v", row["total_tokens"]),
			fmt.Sprintf("%v", row["total_cost"]),
		})
	}
	w.Flush()
	return buf.Bytes(), nil
}

func (s *CatalogService) GetBudgetAlerts(ctx context.Context) (map[string]any, error) {
	var day, week, month int64
	_ = s.pool.QueryRow(ctx, `
		SELECT threshold_day_tokens, threshold_week_tokens, threshold_month_tokens
		FROM monitor_budget_alert_settings
		WHERE id = 1
	`).Scan(&day, &week, &month)
	now := time.Now().UTC()
	dayTokens, _ := s.sumTokensSince(ctx, now.Add(-24*time.Hour))
	weekTokens, _ := s.sumTokensSince(ctx, now.Add(-7*24*time.Hour))
	monthTokens, _ := s.sumTokensSince(ctx, now.AddDate(0, -1, 0))
	return map[string]any{
		"thresholds": map[string]any{
			"day_tokens":   day,
			"week_tokens":  week,
			"month_tokens": month,
		},
		"current": map[string]any{
			"day_tokens":   dayTokens,
			"week_tokens":  weekTokens,
			"month_tokens": monthTokens,
		},
		"alerts": map[string]any{
			"day":   day > 0 && dayTokens >= day,
			"week":  week > 0 && weekTokens >= week,
			"month": month > 0 && monthTokens >= month,
		},
	}, nil
}

func (s *CatalogService) UpdateBudgetAlerts(ctx context.Context, day, week, month int64) (map[string]any, error) {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO monitor_budget_alert_settings(id, threshold_day_tokens, threshold_week_tokens, threshold_month_tokens, updated_at)
		VALUES(1,$1,$2,$3,now())
		ON CONFLICT(id) DO UPDATE SET
			threshold_day_tokens = EXCLUDED.threshold_day_tokens,
			threshold_week_tokens = EXCLUDED.threshold_week_tokens,
			threshold_month_tokens = EXCLUDED.threshold_month_tokens,
			updated_at = now()
	`, day, week, month)
	if err != nil {
		return nil, fmt.Errorf("update budget alerts: %w", err)
	}
	return s.GetBudgetAlerts(ctx)
}

func (s *CatalogService) sumTokensSince(ctx context.Context, start time.Time) (int64, error) {
	var tokens int64
	if err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(total_tokens), 0)::bigint FROM monitor_invocations WHERE started_at >= $1
	`, start).Scan(&tokens); err != nil {
		return 0, err
	}
	return tokens, nil
}
