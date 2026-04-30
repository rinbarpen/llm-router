package migrate

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/rinbarpen/llm-router/src/config"
	"github.com/rinbarpen/llm-router/src/db"
	"github.com/rinbarpen/llm-router/src/services"

	_ "modernc.org/sqlite"
)

const bootstrapMarker = "sqlite_bootstrap_v1"

func Bootstrap(ctx context.Context, pool *db.Store, cfg config.Config) error {
	if err := ensureSchema(ctx, pool); err != nil {
		return err
	}
	if cfg.MigrateFromSQLite {
		alreadyDone, err := markerExists(ctx, pool, bootstrapMarker)
		if err != nil {
			return err
		}
		if !alreadyDone {
			if err := migrateMainSQLite(ctx, pool, cfg.SQLiteMainPath); err != nil {
				return err
			}
			if err := migrateMonitorSQLite(ctx, pool, cfg.SQLiteMonitorPath); err != nil {
				return err
			}
			if err := writeMarker(ctx, pool, bootstrapMarker); err != nil {
				return err
			}
		}
	}

	resolved, err := config.ResolveModelConfigPath(cfg.ModelConfigPath)
	if err != nil {
		slog.Warn("llm-router migrate: skip router.toml catalog sync", slog.Any("error", err))
		return nil
	}
	if err := services.SyncRouterTOMLWithPool(ctx, pool, resolved); err != nil {
		return fmt.Errorf("sync router.toml catalog: %w", err)
	}
	return nil
}

func normalizeSQLitePath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "sqlite+aiosqlite:///") {
		return "/" + strings.TrimPrefix(trimmed, "sqlite+aiosqlite:///")
	}
	if strings.HasPrefix(trimmed, "sqlite:///") {
		return "/" + strings.TrimPrefix(trimmed, "sqlite:///")
	}
	return trimmed
}

func ensureSchema(ctx context.Context, pool *db.Store) error {
	stmts := schemaStatements()
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("ensure schema failed: %w", err)
		}
	}
	patches := schemaPatchStatements()
	for _, stmt := range patches {
		if _, err := pool.Exec(ctx, stmt); err != nil && !isIgnorableSQLitePatchError(err) {
			return fmt.Errorf("ensure oauth schema patch failed: %w", err)
		}
	}
	if _, err := pool.Exec(ctx, `
		WITH ranked AS (
			SELECT id, provider_id, ROW_NUMBER() OVER (PARTITION BY provider_id ORDER BY id ASC) AS rn
			FROM provider_oauth_credentials
		)
		UPDATE provider_oauth_credentials
		SET is_default = (
			SELECT ranked.rn = 1
			FROM ranked
			WHERE ranked.id = provider_oauth_credentials.id
		)
		WHERE id IN (SELECT id FROM ranked)
	`); err != nil {
		return fmt.Errorf("normalize oauth defaults failed: %w", err)
	}
	return nil
}

func isIgnorableSQLitePatchError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate column name")
}

func schemaStatements() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS go_bootstrap_migrations (
			name TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS providers (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			type TEXT NOT NULL,
			is_active BOOLEAN NOT NULL DEFAULT true,
			base_url TEXT,
			api_key TEXT,
			settings JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ
		)`,
		`CREATE TABLE IF NOT EXISTS provider_oauth_credentials (
			id INTEGER PRIMARY KEY,
			provider_id BIGINT NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
			provider_type TEXT NOT NULL,
			account_name TEXT,
			is_default BOOLEAN NOT NULL DEFAULT false,
			is_active BOOLEAN NOT NULL DEFAULT true,
			access_token TEXT,
			refresh_token TEXT,
			api_key TEXT,
			settings JSONB NOT NULL DEFAULT '{}'::jsonb,
			expires_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ
		)`,
		`CREATE TABLE IF NOT EXISTS api_keys (
			id INTEGER PRIMARY KEY,
			key TEXT UNIQUE,
			name TEXT,
			is_active BOOLEAN NOT NULL DEFAULT true,
			owner_type TEXT NOT NULL DEFAULT 'system',
			owner_id BIGINT,
			created_by_user_id BIGINT,
			expires_at TIMESTAMPTZ,
			quota_tokens_monthly BIGINT,
			ip_allowlist JSONB,
			allowed_models JSONB,
			allowed_providers JSONB,
			parameter_limits JSONB,
			created_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ
		)`,
		`CREATE TABLE IF NOT EXISTS models (
			id INTEGER PRIMARY KEY,
			provider_id BIGINT NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			display_name TEXT,
			description TEXT,
			is_active BOOLEAN NOT NULL DEFAULT true,
			remote_identifier TEXT,
			default_params JSONB NOT NULL DEFAULT '{}'::jsonb,
			config JSONB NOT NULL DEFAULT '{}'::jsonb,
			download_uri TEXT,
			local_path TEXT,
			created_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ,
			UNIQUE(provider_id, name)
		)`,
		`CREATE TABLE IF NOT EXISTS model_tags (
			model_id BIGINT NOT NULL REFERENCES models(id) ON DELETE CASCADE,
			tag_id BIGINT NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
			PRIMARY KEY(model_id, tag_id)
		)`,
		`CREATE TABLE IF NOT EXISTS rate_limits (
			id INTEGER PRIMARY KEY,
			model_id BIGINT NOT NULL UNIQUE REFERENCES models(id) ON DELETE CASCADE,
			max_requests BIGINT NOT NULL,
			per_seconds BIGINT NOT NULL,
			burst_size BIGINT,
			notes TEXT,
			config JSONB NOT NULL DEFAULT '{}'::jsonb
		)`,
		`CREATE TABLE IF NOT EXISTS monitor_invocations (
			id INTEGER PRIMARY KEY,
			model_id BIGINT NOT NULL,
			provider_id BIGINT NOT NULL,
			api_key_id BIGINT,
			api_key_name TEXT,
			auth_type TEXT,
			model_name TEXT NOT NULL,
			provider_name TEXT NOT NULL,
			started_at TIMESTAMPTZ NOT NULL,
			completed_at TIMESTAMPTZ,
			duration_ms DOUBLE PRECISION,
			first_token_ms DOUBLE PRECISION,
			stream_duration_ms DOUBLE PRECISION,
			stream_end_reason TEXT,
			status TEXT NOT NULL,
			error_message TEXT,
			request_prompt TEXT,
			request_messages JSONB,
			request_parameters JSONB NOT NULL DEFAULT '{}'::jsonb,
			response_text TEXT,
			response_text_length BIGINT,
			prompt_tokens BIGINT,
			completion_tokens BIGINT,
			total_tokens BIGINT,
			cost DOUBLE PRECISION,
			raw_response JSONB,
			created_at TIMESTAMPTZ
		)`,
		`CREATE TABLE IF NOT EXISTS api_key_usage_monthly (
			api_key_id BIGINT NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
			year_month TEXT NOT NULL,
			total_tokens BIGINT NOT NULL DEFAULT 0,
			total_requests BIGINT NOT NULL DEFAULT 0,
			total_cost DOUBLE PRECISION NOT NULL DEFAULT 0,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY (api_key_id, year_month)
		)`,
		`CREATE TABLE IF NOT EXISTS api_key_policy_templates (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			team_tag TEXT,
			env_tag TEXT,
			policy JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS api_key_policy_audit_logs (
			id INTEGER PRIMARY KEY,
			api_key_id BIGINT REFERENCES api_keys(id) ON DELETE SET NULL,
			action TEXT NOT NULL,
			payload JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS provider_model_catalog_cache (
			id INTEGER PRIMARY KEY,
			provider_name TEXT NOT NULL,
			model_name TEXT NOT NULL,
			metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
			fetched_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			UNIQUE(provider_name, model_name)
		)`,
		`CREATE TABLE IF NOT EXISTS monitor_budget_alert_settings (
			id INTEGER PRIMARY KEY,
			threshold_day_tokens BIGINT NOT NULL DEFAULT 0,
			threshold_week_tokens BIGINT NOT NULL DEFAULT 0,
			threshold_month_tokens BIGINT NOT NULL DEFAULT 0,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			display_name TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'active',
			is_platform_admin BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS user_password_credentials (
			user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
			password_hash TEXT NOT NULL,
			password_updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS user_email_codes (
			id INTEGER PRIMARY KEY,
			user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
			email TEXT NOT NULL,
			code TEXT NOT NULL,
			purpose TEXT NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			consumed_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS user_oauth_identities (
			id INTEGER PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			provider TEXT NOT NULL,
			subject TEXT NOT NULL,
			email TEXT,
			profile JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			UNIQUE(provider, subject)
		)`,
		`CREATE TABLE IF NOT EXISTS console_sessions (
			id INTEGER PRIMARY KEY,
			session_token TEXT NOT NULL UNIQUE,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			expires_at TIMESTAMPTZ NOT NULL,
			last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			revoked_at TIMESTAMPTZ,
			remote_addr TEXT,
			user_agent TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS teams (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			slug TEXT NOT NULL UNIQUE,
			description TEXT,
			status TEXT NOT NULL DEFAULT 'active',
			owner_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS team_members (
			id INTEGER PRIMARY KEY,
			team_id BIGINT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			role TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			invited_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			UNIQUE(team_id, user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS team_invites (
			id INTEGER PRIMARY KEY,
			team_id BIGINT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
			email TEXT NOT NULL,
			role TEXT NOT NULL,
			invite_token TEXT NOT NULL UNIQUE,
			status TEXT NOT NULL DEFAULT 'pending',
			expires_at TIMESTAMPTZ NOT NULL,
			invited_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS wallets (
			id INTEGER PRIMARY KEY,
			owner_type TEXT NOT NULL,
			owner_id BIGINT NOT NULL,
			currency TEXT NOT NULL DEFAULT 'CNY',
			balance NUMERIC(18,6) NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'active',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			UNIQUE(owner_type, owner_id)
		)`,
		`CREATE TABLE IF NOT EXISTS wallet_ledger_entries (
			id INTEGER PRIMARY KEY,
			wallet_id BIGINT NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
			entry_type TEXT NOT NULL,
			amount NUMERIC(18,6) NOT NULL,
			balance_before NUMERIC(18,6) NOT NULL,
			balance_after NUMERIC(18,6) NOT NULL,
			currency TEXT NOT NULL DEFAULT 'CNY',
			reason TEXT NOT NULL,
			reference_type TEXT,
			reference_id TEXT,
			metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS wallet_holds (
			id INTEGER PRIMARY KEY,
			wallet_id BIGINT NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
			reference_type TEXT NOT NULL,
			reference_id TEXT NOT NULL,
			amount NUMERIC(18,6) NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			expires_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			UNIQUE(reference_type, reference_id)
		)`,
		`CREATE TABLE IF NOT EXISTS recharge_orders (
			id INTEGER PRIMARY KEY,
			order_no TEXT NOT NULL UNIQUE,
			owner_type TEXT NOT NULL,
			owner_id BIGINT NOT NULL,
			wallet_id BIGINT REFERENCES wallets(id) ON DELETE SET NULL,
			amount NUMERIC(18,6) NOT NULL,
			currency TEXT NOT NULL DEFAULT 'CNY',
			status TEXT NOT NULL DEFAULT 'pending',
			payment_provider TEXT NOT NULL,
			subject TEXT,
			metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
			paid_at TIMESTAMPTZ,
			closed_at TIMESTAMPTZ,
			created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS payment_attempts (
			id INTEGER PRIMARY KEY,
			order_id BIGINT NOT NULL REFERENCES recharge_orders(id) ON DELETE CASCADE,
			provider TEXT NOT NULL,
			provider_trade_no TEXT,
			status TEXT NOT NULL DEFAULT 'pending',
			request_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
			response_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS payment_callbacks (
			id INTEGER PRIMARY KEY,
			provider TEXT NOT NULL,
			event_id TEXT NOT NULL,
			order_no TEXT,
			payload JSONB NOT NULL DEFAULT '{}'::jsonb,
			processed_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			UNIQUE(provider, event_id)
		)`,
	}
}

func schemaPatchStatements() []string {
	return []string{
		`ALTER TABLE provider_oauth_credentials ADD COLUMN IF NOT EXISTS account_name TEXT`,
		`ALTER TABLE provider_oauth_credentials ADD COLUMN IF NOT EXISTS is_default BOOLEAN NOT NULL DEFAULT false`,
		`ALTER TABLE provider_oauth_credentials ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT true`,
		`ALTER TABLE provider_oauth_credentials ADD COLUMN IF NOT EXISTS settings JSONB NOT NULL DEFAULT '{}'::jsonb`,
		`ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ`,
		`ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS quota_tokens_monthly BIGINT`,
		`ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS owner_type TEXT NOT NULL DEFAULT 'system'`,
		`ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS owner_id BIGINT`,
		`ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS created_by_user_id BIGINT`,
		`ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS ip_allowlist JSONB`,
		`ALTER TABLE monitor_invocations ADD COLUMN IF NOT EXISTS api_key_id BIGINT`,
		`ALTER TABLE monitor_invocations ADD COLUMN IF NOT EXISTS api_key_name TEXT`,
		`ALTER TABLE monitor_invocations ADD COLUMN IF NOT EXISTS auth_type TEXT`,
		`ALTER TABLE monitor_invocations ADD COLUMN IF NOT EXISTS first_token_ms DOUBLE PRECISION`,
		`ALTER TABLE monitor_invocations ADD COLUMN IF NOT EXISTS stream_duration_ms DOUBLE PRECISION`,
		`ALTER TABLE monitor_invocations ADD COLUMN IF NOT EXISTS stream_end_reason TEXT`,
		`CREATE INDEX IF NOT EXISTS idx_monitor_invocations_api_key_id ON monitor_invocations(api_key_id)`,
		`CREATE INDEX IF NOT EXISTS idx_monitor_invocations_started_at ON monitor_invocations(started_at)`,
		`CREATE INDEX IF NOT EXISTS idx_provider_model_catalog_cache_provider ON provider_model_catalog_cache(provider_name)`,
		`CREATE INDEX IF NOT EXISTS idx_provider_oauth_credentials_provider_id ON provider_oauth_credentials(provider_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_provider_oauth_default_active ON provider_oauth_credentials(provider_id) WHERE is_default = true AND is_active = true`,
		`CREATE INDEX IF NOT EXISTS idx_api_keys_owner ON api_keys(owner_type, owner_id)`,
		`CREATE INDEX IF NOT EXISTS idx_wallets_owner ON wallets(owner_type, owner_id)`,
		`CREATE INDEX IF NOT EXISTS idx_recharge_orders_owner ON recharge_orders(owner_type, owner_id)`,
		`UPDATE api_keys SET owner_type = 'system' WHERE owner_type IS NULL OR owner_type = ''`,
		`UPDATE provider_oauth_credentials
		 SET account_name = COALESCE(NULLIF(account_name, ''), 'oauth-' || provider_type || '-' || id::text)
		 WHERE account_name IS NULL OR account_name = ''`,
	}
}

func markerExists(ctx context.Context, pool *db.Store, marker string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM go_bootstrap_migrations WHERE name=$1)`, marker).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("query migration marker: %w", err)
	}
	return exists, nil
}

func writeMarker(ctx context.Context, pool *db.Store, marker string) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO go_bootstrap_migrations(name)
		VALUES($1)
		ON CONFLICT (name) DO UPDATE SET applied_at = now()
	`, marker)
	if err != nil {
		return fmt.Errorf("write migration marker: %w", err)
	}
	return nil
}

func migrateMainSQLite(ctx context.Context, pool *db.Store, rawPath string) error {
	path := normalizeSQLitePath(rawPath)
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	sdb, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("open sqlite main db: %w", err)
	}
	defer sdb.Close()

	if err := migrateProviders(ctx, sdb, pool); err != nil {
		return err
	}
	if err := migrateProviderOAuthCredentials(ctx, sdb, pool); err != nil {
		return err
	}
	if err := migrateAPIKeys(ctx, sdb, pool); err != nil {
		return err
	}
	if err := migrateModels(ctx, sdb, pool); err != nil {
		return err
	}
	if err := migrateModelTags(ctx, sdb, pool); err != nil {
		return err
	}
	if err := migrateRateLimits(ctx, sdb, pool); err != nil {
		return err
	}
	return nil
}

func migrateMonitorSQLite(ctx context.Context, pool *db.Store, rawPath string) error {
	path := normalizeSQLitePath(rawPath)
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	sdb, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("open sqlite monitor db: %w", err)
	}
	defer sdb.Close()

	rows, err := sdb.Query(`
		SELECT
			id, model_id, provider_id, model_name, provider_name,
			CAST(started_at AS TEXT), CAST(completed_at AS TEXT), duration_ms,
			status, error_message, request_prompt,
			CAST(request_messages AS TEXT), CAST(request_parameters AS TEXT),
			response_text, response_text_length,
			prompt_tokens, completion_tokens, total_tokens,
			cost, CAST(raw_response AS TEXT), CAST(created_at AS TEXT)
		FROM monitor_invocations
	`)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such table") {
			return nil
		}
		return fmt.Errorf("query sqlite monitor_invocations: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id, modelID, providerID, responseTextLength sql.NullInt64
			durationMS, cost                            sql.NullFloat64
			promptTokens, completionTokens, totalTokens sql.NullInt64
			modelName, providerName, status             sql.NullString
			startedAt, completedAt, createdAt           sql.NullString
			errorMessage, requestPrompt                 sql.NullString
			requestMessages, requestParameters          sql.NullString
			responseText, rawResponse                   sql.NullString
		)
		if err := rows.Scan(
			&id, &modelID, &providerID, &modelName, &providerName,
			&startedAt, &completedAt, &durationMS,
			&status, &errorMessage, &requestPrompt,
			&requestMessages, &requestParameters,
			&responseText, &responseTextLength,
			&promptTokens, &completionTokens, &totalTokens,
			&cost, &rawResponse, &createdAt,
		); err != nil {
			return fmt.Errorf("scan sqlite monitor_invocations: %w", err)
		}

		_, err := pool.Exec(ctx, `
			INSERT INTO monitor_invocations (
				id, model_id, provider_id, model_name, provider_name,
				started_at, completed_at, duration_ms,
				status, error_message, request_prompt,
				request_messages, request_parameters,
				response_text, response_text_length,
				prompt_tokens, completion_tokens, total_tokens,
				cost, raw_response, created_at
			)
			VALUES (
				$1,$2,$3,$4,$5,
				nullif($6,''), nullif($7,''), $8,
				$9,$10,$11,
				nullif($12,'')::jsonb, nullif($13,'')::jsonb,
				$14,$15,
				$16,$17,$18,
				$19, nullif($20,'')::jsonb, nullif($21,'')
			)
			ON CONFLICT (id) DO UPDATE SET
				status = EXCLUDED.status,
				error_message = EXCLUDED.error_message,
				response_text = EXCLUDED.response_text,
				total_tokens = EXCLUDED.total_tokens,
				cost = EXCLUDED.cost
		`,
			id.Int64, modelID.Int64, providerID.Int64, modelName.String, providerName.String,
			nullableString(startedAt), nullableString(completedAt), nullableFloat(durationMS),
			status.String, nullableString(errorMessage), nullableString(requestPrompt),
			jsonOrDefault(requestMessages, "[]"), jsonOrDefault(requestParameters, "{}"),
			nullableString(responseText), nullableInt(responseTextLength),
			nullableInt(promptTokens), nullableInt(completionTokens), nullableInt(totalTokens),
			nullableFloat(cost), jsonOrDefault(rawResponse, "{}"), nullableString(createdAt),
		)
		if err != nil {
			return fmt.Errorf("insert monitor_invocations(%d): %w", id.Int64, err)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate sqlite monitor_invocations: %w", err)
	}
	return nil
}

func migrateProviders(ctx context.Context, sdb *sql.DB, pool *db.Store) error {
	rows, err := sdb.Query(`
		SELECT id, name, type, is_active, base_url, api_key,
		       CAST(settings AS TEXT), CAST(created_at AS TEXT), CAST(updated_at AS TEXT)
		FROM providers
	`)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such table") {
			return nil
		}
		return fmt.Errorf("query sqlite providers: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id, isActive                   sql.NullInt64
			name, pType, baseURL, apiKey   sql.NullString
			settings, createdAt, updatedAt sql.NullString
		)
		if err := rows.Scan(&id, &name, &pType, &isActive, &baseURL, &apiKey, &settings, &createdAt, &updatedAt); err != nil {
			return fmt.Errorf("scan sqlite providers: %w", err)
		}

		_, err := pool.Exec(ctx, `
			INSERT INTO providers (id, name, type, is_active, base_url, api_key, settings, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,nullif($7,'')::jsonb,nullif($8,''),nullif($9,''))
			ON CONFLICT (id) DO UPDATE SET
				name=EXCLUDED.name,
				type=EXCLUDED.type,
				is_active=EXCLUDED.is_active,
				base_url=EXCLUDED.base_url,
				api_key=EXCLUDED.api_key,
				settings=EXCLUDED.settings,
				updated_at=EXCLUDED.updated_at
		`, id.Int64, name.String, pType.String, isActive.Int64 == 1, nullableString(baseURL), nullableString(apiKey), jsonOrDefault(settings, "{}"), nullableString(createdAt), nullableString(updatedAt))
		if err != nil {
			return fmt.Errorf("insert provider(%d): %w", id.Int64, err)
		}
	}
	return rows.Err()
}

func migrateProviderOAuthCredentials(ctx context.Context, sdb *sql.DB, pool *db.Store) error {
	rows, err := sdb.Query(`
		SELECT id, provider_id, provider_type, access_token, refresh_token, api_key,
		       CAST(expires_at AS TEXT), CAST(created_at AS TEXT), CAST(updated_at AS TEXT)
		FROM provider_oauth_credentials
	`)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such table") {
			return nil
		}
		return fmt.Errorf("query sqlite provider_oauth_credentials: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id, providerID                                  sql.NullInt64
			providerType, accessToken, refreshToken, apiKey sql.NullString
			expiresAt, createdAt, updatedAt                 sql.NullString
		)
		if err := rows.Scan(&id, &providerID, &providerType, &accessToken, &refreshToken, &apiKey, &expiresAt, &createdAt, &updatedAt); err != nil {
			return fmt.Errorf("scan sqlite provider_oauth_credentials: %w", err)
		}
		_, err := pool.Exec(ctx, `
			INSERT INTO provider_oauth_credentials (
				id, provider_id, provider_type, access_token, refresh_token, api_key, expires_at, created_at, updated_at
			)
			VALUES ($1,$2,$3,$4,$5,$6,nullif($7,''),nullif($8,''),nullif($9,''))
			ON CONFLICT (id) DO UPDATE SET
				access_token=EXCLUDED.access_token,
				refresh_token=EXCLUDED.refresh_token,
				api_key=EXCLUDED.api_key,
				expires_at=EXCLUDED.expires_at,
				updated_at=EXCLUDED.updated_at
		`, id.Int64, providerID.Int64, providerType.String, nullableString(accessToken), nullableString(refreshToken), nullableString(apiKey), nullableString(expiresAt), nullableString(createdAt), nullableString(updatedAt))
		if err != nil {
			return fmt.Errorf("insert provider_oauth_credentials(%d): %w", id.Int64, err)
		}
	}
	return rows.Err()
}

func migrateAPIKeys(ctx context.Context, sdb *sql.DB, pool *db.Store) error {
	rows, err := sdb.Query(`
		SELECT id, key, name, is_active,
		       CAST(allowed_models AS TEXT), CAST(allowed_providers AS TEXT), CAST(parameter_limits AS TEXT),
		       CAST(created_at AS TEXT), CAST(updated_at AS TEXT)
		FROM api_keys
	`)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such table") {
			return nil
		}
		return fmt.Errorf("query sqlite api_keys: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id, isActive                    sql.NullInt64
			key, name                       sql.NullString
			allowedModels, allowedProviders sql.NullString
			parameterLimits                 sql.NullString
			createdAt, updatedAt            sql.NullString
		)
		if err := rows.Scan(&id, &key, &name, &isActive, &allowedModels, &allowedProviders, &parameterLimits, &createdAt, &updatedAt); err != nil {
			return fmt.Errorf("scan sqlite api_keys: %w", err)
		}
		_, err := pool.Exec(ctx, `
			INSERT INTO api_keys (
				id, key, name, is_active, allowed_models, allowed_providers, parameter_limits, created_at, updated_at
			)
			VALUES (
				$1,$2,$3,$4,
				nullif($5,'')::jsonb, nullif($6,'')::jsonb, nullif($7,'')::jsonb,
				nullif($8,''), nullif($9,'')
			)
			ON CONFLICT (id) DO UPDATE SET
				key=EXCLUDED.key,
				name=EXCLUDED.name,
				is_active=EXCLUDED.is_active,
				allowed_models=EXCLUDED.allowed_models,
				allowed_providers=EXCLUDED.allowed_providers,
				parameter_limits=EXCLUDED.parameter_limits,
				updated_at=EXCLUDED.updated_at
		`, id.Int64, nullableString(key), nullableString(name), isActive.Int64 == 1,
			jsonOrDefault(allowedModels, "[]"), jsonOrDefault(allowedProviders, "[]"), jsonOrDefault(parameterLimits, "{}"),
			nullableString(createdAt), nullableString(updatedAt),
		)
		if err != nil {
			return fmt.Errorf("insert api_keys(%d): %w", id.Int64, err)
		}
	}
	return rows.Err()
}

func migrateModels(ctx context.Context, sdb *sql.DB, pool *db.Store) error {
	rows, err := sdb.Query(`
		SELECT id, provider_id, name, display_name, description, is_active, remote_identifier,
		       CAST(default_params AS TEXT), CAST(config AS TEXT), download_uri, local_path,
		       CAST(created_at AS TEXT), CAST(updated_at AS TEXT)
		FROM models
	`)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such table") {
			return nil
		}
		return fmt.Errorf("query sqlite models: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id, providerID, isActive                         sql.NullInt64
			name, displayName, description, remoteIdentifier sql.NullString
			defaultParams, cfgJSON                           sql.NullString
			downloadURI, localPath                           sql.NullString
			createdAt, updatedAt                             sql.NullString
		)
		if err := rows.Scan(
			&id, &providerID, &name, &displayName, &description, &isActive, &remoteIdentifier,
			&defaultParams, &cfgJSON, &downloadURI, &localPath,
			&createdAt, &updatedAt,
		); err != nil {
			return fmt.Errorf("scan sqlite models: %w", err)
		}
		_, err := pool.Exec(ctx, `
			INSERT INTO models (
				id, provider_id, name, display_name, description, is_active, remote_identifier,
				default_params, config, download_uri, local_path, created_at, updated_at
			)
			VALUES (
				$1,$2,$3,$4,$5,$6,$7,
				nullif($8,'')::jsonb, nullif($9,'')::jsonb, $10, $11, nullif($12,''), nullif($13,'')
			)
			ON CONFLICT (id) DO UPDATE SET
				provider_id=EXCLUDED.provider_id,
				name=EXCLUDED.name,
				display_name=EXCLUDED.display_name,
				description=EXCLUDED.description,
				is_active=EXCLUDED.is_active,
				remote_identifier=EXCLUDED.remote_identifier,
				default_params=EXCLUDED.default_params,
				config=EXCLUDED.config,
				download_uri=EXCLUDED.download_uri,
				local_path=EXCLUDED.local_path,
				updated_at=EXCLUDED.updated_at
		`,
			id.Int64, providerID.Int64, name.String, nullableString(displayName), nullableString(description), isActive.Int64 == 1, nullableString(remoteIdentifier),
			jsonOrDefault(defaultParams, "{}"), jsonOrDefault(cfgJSON, "{}"), nullableString(downloadURI), nullableString(localPath), nullableString(createdAt), nullableString(updatedAt),
		)
		if err != nil {
			return fmt.Errorf("insert models(%d): %w", id.Int64, err)
		}
	}
	return rows.Err()
}

func migrateModelTags(ctx context.Context, sdb *sql.DB, pool *db.Store) error {
	rows, err := sdb.Query(`SELECT model_id, tag_id FROM model_tags`)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such table") {
			return nil
		}
		return fmt.Errorf("query sqlite model_tags: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var modelID, tagID sql.NullInt64
		if err := rows.Scan(&modelID, &tagID); err != nil {
			return fmt.Errorf("scan sqlite model_tags: %w", err)
		}
		_, err := pool.Exec(ctx, `
			INSERT INTO model_tags (model_id, tag_id)
			VALUES ($1, $2)
			ON CONFLICT (model_id, tag_id) DO NOTHING
		`, modelID.Int64, tagID.Int64)
		if err != nil {
			return fmt.Errorf("insert model_tags(%d,%d): %w", modelID.Int64, tagID.Int64, err)
		}
	}
	return rows.Err()
}

func migrateRateLimits(ctx context.Context, sdb *sql.DB, pool *db.Store) error {
	rows, err := sdb.Query(`
		SELECT id, model_id, max_requests, per_seconds, burst_size, notes, CAST(config AS TEXT)
		FROM rate_limits
	`)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such table") {
			return nil
		}
		return fmt.Errorf("query sqlite rate_limits: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id, modelID, maxRequests, perSeconds, burstSize sql.NullInt64
			notes, cfgJSON                                  sql.NullString
		)
		if err := rows.Scan(&id, &modelID, &maxRequests, &perSeconds, &burstSize, &notes, &cfgJSON); err != nil {
			return fmt.Errorf("scan sqlite rate_limits: %w", err)
		}
		_, err := pool.Exec(ctx, `
			INSERT INTO rate_limits (id, model_id, max_requests, per_seconds, burst_size, notes, config)
			VALUES ($1,$2,$3,$4,$5,$6,nullif($7,'')::jsonb)
			ON CONFLICT (id) DO UPDATE SET
				model_id=EXCLUDED.model_id,
				max_requests=EXCLUDED.max_requests,
				per_seconds=EXCLUDED.per_seconds,
				burst_size=EXCLUDED.burst_size,
				notes=EXCLUDED.notes,
				config=EXCLUDED.config
		`, id.Int64, modelID.Int64, maxRequests.Int64, perSeconds.Int64, nullableInt(burstSize), nullableString(notes), jsonOrDefault(cfgJSON, "{}"))
		if err != nil {
			return fmt.Errorf("insert rate_limits(%d): %w", id.Int64, err)
		}
	}
	return rows.Err()
}

func nullableString(v sql.NullString) any {
	if !v.Valid {
		return nil
	}
	if strings.TrimSpace(v.String) == "" {
		return nil
	}
	return v.String
}

func nullableInt(v sql.NullInt64) any {
	if !v.Valid {
		return nil
	}
	return v.Int64
}

func nullableFloat(v sql.NullFloat64) any {
	if !v.Valid {
		return nil
	}
	return v.Float64
}

func jsonOrDefault(v sql.NullString, fallback string) string {
	if !v.Valid || strings.TrimSpace(v.String) == "" {
		return fallback
	}
	raw := strings.TrimSpace(v.String)
	if json.Valid([]byte(raw)) {
		return raw
	}
	return fallback
}
