package services

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// LoginRecord is a lightweight projection of login_audit rows.
type LoginRecord struct {
	ID         int64      `json:"id"`
	APIKeyID   *int64     `json:"api_key_id,omitempty"`
	APIKeyName *string    `json:"api_key_name,omitempty"`
	RemoteAddr *string    `json:"remote_addr,omitempty"`
	UserAgent  *string    `json:"user_agent,omitempty"`
	CreatedAt  *time.Time `json:"created_at,omitempty"`
}

// LoginRecordService manages login_audit read/write operations.
type LoginRecordService struct {
	pool *pgxpool.Pool
}

func NewLoginRecordService(pool *pgxpool.Pool) *LoginRecordService {
	return &LoginRecordService{pool: pool}
}

func (s *LoginRecordService) Record(ctx context.Context, apiKeyID *int64, apiKeyName *string, remoteAddr *string, userAgent *string) error {
	if s == nil || s.pool == nil {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO login_audit(api_key_id, api_key_name, remote_addr, user_agent, created_at)
		VALUES ($1, $2, $3, $4, now())
	`, apiKeyID, apiKeyName, remoteAddr, userAgent)
	if err != nil {
		return fmt.Errorf("insert login record: %w", err)
	}
	return nil
}

func (s *LoginRecordService) List(ctx context.Context, limit int) ([]LoginRecord, error) {
	if s == nil || s.pool == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, api_key_id, api_key_name, remote_addr, user_agent, created_at
		FROM login_audit
		ORDER BY id DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query login records: %w", err)
	}
	defer rows.Close()

	out := make([]LoginRecord, 0)
	for rows.Next() {
		var item LoginRecord
		if err := rows.Scan(&item.ID, &item.APIKeyID, &item.APIKeyName, &item.RemoteAddr, &item.UserAgent, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan login record: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate login records: %w", err)
	}
	return out, nil
}
