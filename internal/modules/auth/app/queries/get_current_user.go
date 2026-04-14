package queries

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/vvs/isp/internal/modules/auth/domain"
)

type GetCurrentUserHandler struct {
	users    domain.UserRepository
	sessions domain.SessionRepository
}

func NewGetCurrentUserHandler(users domain.UserRepository, sessions domain.SessionRepository) *GetCurrentUserHandler {
	return &GetCurrentUserHandler{users: users, sessions: sessions}
}

// Handle accepts the raw cookie token, hashes it, looks up the session, and returns the user.
// Returns nil, nil when the token is empty, expired, or not found.
func (h *GetCurrentUserHandler) Handle(ctx context.Context, rawToken string) (*domain.User, error) {
	if rawToken == "" {
		return nil, nil
	}

	sum := sha256.Sum256([]byte(rawToken))
	hash := hex.EncodeToString(sum[:])

	sess, err := h.sessions.FindByTokenHash(ctx, hash)
	if err != nil || sess == nil {
		return nil, nil
	}
	if sess.IsExpired() {
		return nil, nil
	}

	u, err := h.users.FindByID(ctx, sess.UserID)
	if err != nil {
		return nil, nil
	}
	return u, nil
}
