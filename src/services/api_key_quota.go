package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func yearMonth(now time.Time) string {
	return now.UTC().Format("2006-01")
}

func (s *CatalogService) GetAPIKeyMonthlyUsage(ctx context.Context, apiKeyID int64, ym string) (int64, float64, error) {
	var (
		tokens int64
		cost   float64
	)
	if err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(total_tokens, 0), COALESCE(total_cost, 0)
		FROM api_key_usage_monthly
		WHERE api_key_id = $1 AND year_month = $2
	`, apiKeyID, ym).Scan(&tokens, &cost); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("query api key monthly usage: %w", err)
	}
	return tokens, cost, nil
}

func (s *CatalogService) CheckAPIKeyQuota(ctx context.Context, apiKeyID int64, quotaTokensMonthly int64) error {
	if quotaTokensMonthly <= 0 {
		return nil
	}
	tokens, _, err := s.GetAPIKeyMonthlyUsage(ctx, apiKeyID, yearMonth(time.Now()))
	if err != nil {
		return err
	}
	if tokens >= quotaTokensMonthly {
		return fmt.Errorf("monthly token quota exceeded")
	}
	return nil
}

func (s *CatalogService) AccumulateAPIKeyUsage(ctx context.Context, apiKeyID int64, tokens int64, cost float64) error {
	if apiKeyID <= 0 {
		return nil
	}
	if tokens < 0 {
		tokens = 0
	}
	if cost < 0 {
		cost = 0
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO api_key_usage_monthly(api_key_id, year_month, total_tokens, total_requests, total_cost, updated_at)
		VALUES($1, $2, $3, 1, $4, now())
		ON CONFLICT(api_key_id, year_month) DO UPDATE SET
			total_tokens = api_key_usage_monthly.total_tokens + EXCLUDED.total_tokens,
			total_requests = api_key_usage_monthly.total_requests + 1,
			total_cost = api_key_usage_monthly.total_cost + EXCLUDED.total_cost,
			updated_at = now()
	`, apiKeyID, yearMonth(time.Now()), tokens, cost)
	if err != nil {
		return fmt.Errorf("accumulate api key usage: %w", err)
	}
	if err := s.applyWalletDebitForAPIKey(ctx, apiKeyID, cost, tokens); err != nil && !errors.Is(err, ErrInsufficientBalance) {
		return fmt.Errorf("deduct wallet balance: %w", err)
	}
	return nil
}
