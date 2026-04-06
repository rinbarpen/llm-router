package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rinbarpen/llm-router/src/schemas"
)

func (s *CatalogService) ListAPIKeyPolicyTemplates(ctx context.Context, teamTag, envTag string) ([]map[string]any, error) {
	args := []any{}
	sql := `SELECT id, name, team_tag, env_tag, policy, created_at, updated_at FROM api_key_policy_templates WHERE 1=1`
	if teamTag != "" {
		args = append(args, teamTag)
		sql += fmt.Sprintf(" AND team_tag = $%d", len(args))
	}
	if envTag != "" {
		args = append(args, envTag)
		sql += fmt.Sprintf(" AND env_tag = $%d", len(args))
	}
	sql += ` ORDER BY id ASC`
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("list policy templates: %w", err)
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var (
			id        int64
			name      string
			team      *string
			env       *string
			policyRaw []byte
			createdAt time.Time
			updatedAt time.Time
			policy    map[string]any
		)
		if err := rows.Scan(&id, &name, &team, &env, &policyRaw, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan policy template: %w", err)
		}
		_ = json.Unmarshal(policyRaw, &policy)
		out = append(out, map[string]any{
			"id":         id,
			"name":       name,
			"team_tag":   team,
			"env_tag":    env,
			"policy":     policy,
			"created_at": createdAt.UTC().Format(time.RFC3339),
			"updated_at": updatedAt.UTC().Format(time.RFC3339),
		})
	}
	return out, rows.Err()
}

func (s *CatalogService) CreateAPIKeyPolicyTemplate(ctx context.Context, name string, teamTag *string, envTag *string, policy map[string]any) (map[string]any, error) {
	if policy == nil {
		policy = map[string]any{}
	}
	policyRaw, _ := json.Marshal(policy)
	var (
		id        int64
		createdAt time.Time
		updatedAt time.Time
	)
	if err := s.pool.QueryRow(ctx, `
		INSERT INTO api_key_policy_templates(name, team_tag, env_tag, policy, created_at, updated_at)
		VALUES($1,$2,$3,$4,now(),now())
		RETURNING id, created_at, updated_at
	`, name, teamTag, envTag, policyRaw).Scan(&id, &createdAt, &updatedAt); err != nil {
		return nil, fmt.Errorf("create policy template: %w", err)
	}
	return map[string]any{
		"id":         id,
		"name":       name,
		"team_tag":   teamTag,
		"env_tag":    envTag,
		"policy":     policy,
		"created_at": createdAt.UTC().Format(time.RFC3339),
		"updated_at": updatedAt.UTC().Format(time.RFC3339),
	}, nil
}

func (s *CatalogService) UpdateAPIKeyPolicyTemplate(ctx context.Context, id int64, name string, teamTag *string, envTag *string, policy map[string]any) (map[string]any, error) {
	if policy == nil {
		policy = map[string]any{}
	}
	policyRaw, _ := json.Marshal(policy)
	var updatedAt time.Time
	if err := s.pool.QueryRow(ctx, `
		UPDATE api_key_policy_templates
		SET name = $2, team_tag = $3, env_tag = $4, policy = $5, updated_at = now()
		WHERE id = $1
		RETURNING updated_at
	`, id, name, teamTag, envTag, policyRaw).Scan(&updatedAt); err != nil {
		return nil, fmt.Errorf("update policy template: %w", err)
	}
	return map[string]any{
		"id":         id,
		"name":       name,
		"team_tag":   teamTag,
		"env_tag":    envTag,
		"policy":     policy,
		"updated_at": updatedAt.UTC().Format(time.RFC3339),
	}, nil
}

func (s *CatalogService) DeleteAPIKeyPolicyTemplate(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM api_key_policy_templates WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete policy template: %w", err)
	}
	return nil
}

func (s *CatalogService) ApplyAPIKeyPolicyTemplate(ctx context.Context, templateID int64, apiKeyIDs []int64) (map[string]any, error) {
	var policyRaw []byte
	if err := s.pool.QueryRow(ctx, `SELECT policy FROM api_key_policy_templates WHERE id = $1`, templateID).Scan(&policyRaw); err != nil {
		return nil, fmt.Errorf("load policy template: %w", err)
	}
	policy := map[string]any{}
	_ = json.Unmarshal(policyRaw, &policy)
	updated := 0
	for _, id := range apiKeyIDs {
		patch := map[string]any{}
		for k, v := range policy {
			patch[k] = v
		}
		if err := s.ApplyAPIKeyPolicyPatch(ctx, id, patch); err != nil {
			return nil, err
		}
		updated++
	}
	return map[string]any{
		"template_id":  templateID,
		"updated_keys": updated,
	}, nil
}

func (s *CatalogService) ApplyAPIKeyPolicyPatch(ctx context.Context, apiKeyID int64, patch map[string]any) error {
	current, err := s.GetAPIKey(ctx, apiKeyID)
	if err != nil {
		return err
	}
	update := schemas.APIKeyUpdate{
		Name:             current.Name,
		IsActive:         &current.IsActive,
		ExpiresAt:        current.ExpiresAt,
		QuotaTokensMonth: current.QuotaTokensMonth,
		IPAllowlist:      current.IPAllowlist,
		AllowedModels:    current.AllowedModels,
		AllowedProviders: current.AllowedProviders,
		ParameterLimits:  current.ParameterLimits,
	}
	if v, ok := patch["quota_tokens_monthly"]; ok {
		if q, ok := asInt64(v); ok {
			update.QuotaTokensMonth = &q
		}
	}
	if v, ok := patch["ip_allowlist"].([]any); ok {
		items := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				items = append(items, s)
			}
		}
		update.IPAllowlist = items
	}
	if v, ok := patch["allowed_models"].([]any); ok {
		items := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				items = append(items, s)
			}
		}
		update.AllowedModels = items
	}
	if v, ok := patch["allowed_providers"].([]any); ok {
		items := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				items = append(items, s)
			}
		}
		update.AllowedProviders = items
	}
	if v, ok := patch["parameter_limits"].(map[string]any); ok {
		update.ParameterLimits = v
	}
	if _, err := s.UpdateAPIKey(ctx, apiKeyID, update); err != nil {
		return err
	}
	payloadRaw, _ := json.Marshal(patch)
	_, _ = s.pool.Exec(ctx, `
		INSERT INTO api_key_policy_audit_logs(api_key_id, action, payload, created_at)
		VALUES($1,'batch_apply',$2,now())
	`, apiKeyID, payloadRaw)
	return nil
}

func asInt64(v any) (int64, bool) {
	switch t := v.(type) {
	case int:
		return int64(t), true
	case int32:
		return int64(t), true
	case int64:
		return t, true
	case float32:
		return int64(t), true
	case float64:
		return int64(t), true
	default:
		return 0, false
	}
}

func (s *CatalogService) ListAPIKeyPolicyAudit(ctx context.Context, limit int, offset int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, api_key_id, action, payload, created_at
		FROM api_key_policy_audit_logs
		ORDER BY id DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list policy audit logs: %w", err)
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var (
			id        int64
			apiKeyID  *int64
			action    string
			payload   []byte
			createdAt time.Time
			data      map[string]any
		)
		if err := rows.Scan(&id, &apiKeyID, &action, &payload, &createdAt); err != nil {
			return nil, fmt.Errorf("scan policy audit log: %w", err)
		}
		_ = json.Unmarshal(payload, &data)
		out = append(out, map[string]any{
			"id":         id,
			"api_key_id": apiKeyID,
			"action":     action,
			"payload":    data,
			"created_at": createdAt.UTC().Format(time.RFC3339),
		})
	}
	return out, rows.Err()
}
