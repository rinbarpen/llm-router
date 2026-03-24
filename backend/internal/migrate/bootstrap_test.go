package migrate

import "testing"

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
