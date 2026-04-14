package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrSessionExpired = errors.New("session expired")

type Session struct {
	ID        string
	UserID    string
	TokenHash string
	CreatedAt time.Time
	ExpiresAt time.Time
}

func NewSession(userID, tokenHash string, ttl time.Duration) *Session {
	now := time.Now().UTC()
	return &Session{
		ID:        uuid.Must(uuid.NewV7()).String(),
		UserID:    userID,
		TokenHash: tokenHash,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}
}

func (s *Session) IsExpired() bool {
	return time.Now().UTC().After(s.ExpiresAt)
}
