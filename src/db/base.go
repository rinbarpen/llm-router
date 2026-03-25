package db

import "time"

const DefaultDBFilename = "llm_router.db"

// NowUTC returns the current UTC timestamp.
func NowUTC() time.Time {
	return time.Now().UTC()
}

// BoolFromInt mirrors common SQLite boolean encoding (0/1).
func BoolFromInt(v int64) bool {
	return v != 0
}
