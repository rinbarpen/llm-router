package db

import (
	"time"

	"github.com/google/uuid"
)

// LoginRecord mirrors src/llm_router/db/login_models.py.
type LoginRecord struct {
	ID               string
	Timestamp        time.Time
	IPAddress        string
	AuthType         string
	IsSuccess        bool
	APIKeyID         *int64
	SessionTokenHash *string
	IsLocal          bool
}

func NewLoginRecord(ipAddress, authType string, isSuccess bool) LoginRecord {
	return LoginRecord{
		ID:        uuid.NewString(),
		Timestamp: NowUTC(),
		IPAddress: ipAddress,
		AuthType:  authType,
		IsSuccess: isSuccess,
	}
}
