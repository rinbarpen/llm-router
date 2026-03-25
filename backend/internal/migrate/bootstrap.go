package migrate

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rinbarpen/llm-router/backend/internal/config"
	"github.com/rinbarpen/llm-router/backend/internal/services"

	_ "modernc.org/sqlite"
)

const bootstrapMarker = "sqlite_bootstrap_v1"

func Bootstrap(ctx context.Context, pool *pgxpool.Pool, cfg config.Config) error {
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
		log.Printf("llm-router migrate: skip router.toml catalog sync: %v", err)
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

func ensureSchema(ctx context.Context, pool *pgxpool.Pool) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS go_bootstrap_migrations (
			name TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS providers (
			id BIGINT PRIMARY KEY,
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
			id BIGINT PRIMARY KEY,
			provider_id BIGINT NOT NULL UNIQUE REFERENCES providers(id) ON DELETE CASCADE,
			provider_type TEXT NOT NULL,
			access_token TEXT,
			refresh_token TEXT,
			api_key TEXT,
			expires_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ
		)`,
		`CREATE TABLE IF NOT EXISTS api_keys (
			id BIGINT PRIMARY KEY,
			key TEXT UNIQUE,
			name TEXT,
			is_active BOOLEAN NOT NULL DEFAULT true,
			allowed_models JSONB,
			allowed_providers JSONB,
			parameter_limits JSONB,
			created_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ
		)`,
		`CREATE TABLE IF NOT EXISTS models (
			id BIGINT PRIMARY KEY,
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
			id BIGINT PRIMARY KEY,
			model_id BIGINT NOT NULL UNIQUE REFERENCES models(id) ON DELETE CASCADE,
			max_requests BIGINT NOT NULL,
			per_seconds BIGINT NOT NULL,
			burst_size BIGINT,
			notes TEXT,
			config JSONB NOT NULL DEFAULT '{}'::jsonb
		)`,
		`CREATE TABLE IF NOT EXISTS monitor_invocations (
			id BIGINT PRIMARY KEY,
			model_id BIGINT NOT NULL,
			provider_id BIGINT NOT NULL,
			model_name TEXT NOT NULL,
			provider_name TEXT NOT NULL,
			started_at TIMESTAMPTZ NOT NULL,
			completed_at TIMESTAMPTZ,
			duration_ms DOUBLE PRECISION,
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
	}

	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("ensure schema failed: %w", err)
		}
	}
	return nil
}

func markerExists(ctx context.Context, pool *pgxpool.Pool, marker string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM go_bootstrap_migrations WHERE name=$1)`, marker).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("query migration marker: %w", err)
	}
	return exists, nil
}

func writeMarker(ctx context.Context, pool *pgxpool.Pool, marker string) error {
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

func migrateMainSQLite(ctx context.Context, pool *pgxpool.Pool, rawPath string) error {
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

func migrateMonitorSQLite(ctx context.Context, pool *pgxpool.Pool, rawPath string) error {
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

func migrateProviders(ctx context.Context, sdb *sql.DB, pool *pgxpool.Pool) error {
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

func migrateProviderOAuthCredentials(ctx context.Context, sdb *sql.DB, pool *pgxpool.Pool) error {
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

func migrateAPIKeys(ctx context.Context, sdb *sql.DB, pool *pgxpool.Pool) error {
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

func migrateModels(ctx context.Context, sdb *sql.DB, pool *pgxpool.Pool) error {
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

func migrateModelTags(ctx context.Context, sdb *sql.DB, pool *pgxpool.Pool) error {
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

func migrateRateLimits(ctx context.Context, sdb *sql.DB, pool *pgxpool.Pool) error {
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

