package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var pgPlaceholderRE = regexp.MustCompile(`\$\d+`)

type Store struct {
	db *sql.DB
}

type Tx struct {
	tx *sql.Tx
}

func Connect(ctx context.Context, path string) (*Store, error) {
	path = normalizeSQLitePath(path)
	if path == "" {
		return nil, fmt.Errorf("sqlite path is required")
	}
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("create sqlite data dir: %w", err)
		}
	}
	conn, err := sql.Open("sqlite", sqliteDSN(path))
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)
	conn.SetConnMaxLifetime(0)

	store := &Store{db: conn}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := store.Ping(pingCtx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	for _, pragma := range []string{
		`PRAGMA foreign_keys = ON`,
		`PRAGMA journal_mode = WAL`,
		`PRAGMA busy_timeout = 5000`,
	} {
		if _, err := store.Exec(ctx, pragma); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("configure sqlite: %w", err)
		}
	}
	return store, nil
}

func sqliteDSN(path string) string {
	if path == ":memory:" {
		return "file::memory:?_time_format=sqlite"
	}
	u := url.URL{Scheme: "file", Path: path}
	q := u.Query()
	q.Set("_time_format", "sqlite")
	u.RawQuery = q.Encode()
	return u.String()
}

func (s *Store) Close() {
	if s != nil && s.db != nil {
		_ = s.db.Close()
	}
}

func (s *Store) Ping(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("sqlite store unavailable")
	}
	return s.db.PingContext(ctx)
}

func (s *Store) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return s.db.ExecContext(ctx, rewriteSQL(query), args...)
}

func (s *Store) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return s.db.QueryContext(ctx, rewriteSQL(query), args...)
}

func (s *Store) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return s.db.QueryRowContext(ctx, rewriteSQL(query), args...)
}

func (s *Store) Begin(ctx context.Context) (*Tx, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &Tx{tx: tx}, nil
}

func (t *Tx) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.tx.ExecContext(ctx, rewriteSQL(query), args...)
}

func (t *Tx) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return t.tx.QueryContext(ctx, rewriteSQL(query), args...)
}

func (t *Tx) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return t.tx.QueryRowContext(ctx, rewriteSQL(query), args...)
}

func (t *Tx) Commit(ctx context.Context) error {
	_ = ctx
	return t.tx.Commit()
}

func (t *Tx) Rollback(ctx context.Context) error {
	_ = ctx
	return t.tx.Rollback()
}

func normalizeSQLitePath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(trimmed, "sqlite+aiosqlite:///"):
		return strings.TrimPrefix(trimmed, "sqlite+aiosqlite:///")
	case strings.HasPrefix(trimmed, "sqlite:///"):
		return strings.TrimPrefix(trimmed, "sqlite:///")
	case strings.HasPrefix(trimmed, "sqlite://"):
		return strings.TrimPrefix(trimmed, "sqlite://")
	default:
		return trimmed
	}
}

func rewriteSQL(query string) string {
	out := pgPlaceholderRE.ReplaceAllString(query, "?")
	replacements := []struct {
		old string
		new string
	}{
		{`::jsonb`, ``},
		{`::float8`, ``},
		{`::bigint`, ``},
		{`::text`, ``},
		{`TIMESTAMPTZ`, `TIMESTAMP`},
		{`now()`, `(strftime('%Y-%m-%d %H:%M:%f+00:00','now'))`},
		{`FOR UPDATE`, ``},
		{`ADD COLUMN IF NOT EXISTS`, `ADD COLUMN`},
	}
	for _, repl := range replacements {
		out = strings.ReplaceAll(out, repl.old, repl.new)
	}
	return out
}
