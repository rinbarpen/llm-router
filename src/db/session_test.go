package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestConnectOpensSQLiteStoreAndEnforcesRollback(t *testing.T) {
	ctx := context.Background()
	store, err := Connect(ctx, filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer store.Close()

	if _, err := store.Exec(ctx, `CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := store.Exec(ctx, `INSERT INTO items(name) VALUES ($1)`, "before"); err != nil {
		t.Fatalf("insert with numbered placeholder: %v", err)
	}

	tx, err := store.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO items(name) VALUES ($1)`, "rolled-back"); err != nil {
		t.Fatalf("insert in tx: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}

	var count int
	if err := store.QueryRow(ctx, `SELECT COUNT(*) FROM items`).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("row count = %d, want 1", count)
	}
}
