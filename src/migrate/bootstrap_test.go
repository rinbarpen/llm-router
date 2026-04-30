package migrate

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rinbarpen/llm-router/src/config"
	"github.com/rinbarpen/llm-router/src/db"
)

func TestNormalizeSQLitePath(t *testing.T) {
	got := normalizeSQLitePath("sqlite+aiosqlite:///tmp/a.db")
	if got != "/tmp/a.db" {
		t.Fatalf("normalizeSQLitePath() = %q, want /tmp/a.db", got)
	}
	got = normalizeSQLitePath("/tmp/b.db")
	if got != "/tmp/b.db" {
		t.Fatalf("normalizeSQLitePath passthrough = %q", got)
	}
}

func TestSchemaStatementsIncludePlatformTablesAndAPIKeyOwnership(t *testing.T) {
	stmts := schemaStatements()
	patches := schemaPatchStatements()

	requiredTables := []string{
		"users",
		"user_password_credentials",
		"user_email_codes",
		"user_oauth_identities",
		"console_sessions",
		"teams",
		"team_members",
		"team_invites",
		"wallets",
		"wallet_ledger_entries",
		"wallet_holds",
		"recharge_orders",
		"payment_attempts",
		"payment_callbacks",
	}
	for _, table := range requiredTables {
		if !containsSQL(stmts, "CREATE TABLE IF NOT EXISTS "+table) {
			t.Fatalf("missing table bootstrap for %s", table)
		}
	}

	requiredPatches := []string{
		"ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS owner_type TEXT",
		"ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS owner_id BIGINT",
		"ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS created_by_user_id BIGINT",
		"UPDATE api_keys SET owner_type = 'system'",
	}
	for _, patch := range requiredPatches {
		if !containsSQL(patches, patch) {
			t.Fatalf("missing schema patch %q", patch)
		}
	}
}

func TestBootstrapCreatesSQLiteSchema(t *testing.T) {
	ctx := context.Background()
	store, err := db.Connect(ctx, filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer store.Close()

	cfg := config.Config{
		SQLitePath:        filepath.Join(t.TempDir(), "router.db"),
		MigrateFromSQLite: false,
		ModelConfigPath:   filepath.Join(t.TempDir(), "missing-router.toml"),
	}
	if err := Bootstrap(ctx, store, cfg); err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	if err := Bootstrap(ctx, store, cfg); err != nil {
		t.Fatalf("Bootstrap() second run error = %v", err)
	}

	for _, table := range []string{"providers", "models", "api_keys", "monitor_invocations", "users", "wallets"} {
		var name string
		if err := store.QueryRow(ctx, `SELECT name FROM sqlite_master WHERE type = 'table' AND name = $1`, table).Scan(&name); err != nil {
			t.Fatalf("table %s missing after bootstrap: %v", table, err)
		}
	}
}

func containsSQL(stmts []string, want string) bool {
	for _, stmt := range stmts {
		if strings.Contains(stmt, want) {
			return true
		}
	}
	return false
}
