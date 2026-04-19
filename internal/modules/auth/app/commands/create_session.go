package commands

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"

	"github.com/vvs/isp/internal/modules/auth/domain"
)

// CreateSessionHandler creates a session for an already-verified user.
// Used in the TOTP login flow: credentials are verified first, then this
// is called after the TOTP code is confirmed.
type CreateSessionHandler struct {
	sessions domain.SessionRepository
}

func NewCreateSessionHandler(sessions domain.SessionRepository) *CreateSessionHandler {
	return &CreateSessionHandler{sessions: sessions}
}

func (h *CreateSessionHandler) Handle(ctx context.Context, userID string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(sum[:])
	sess := domain.NewSession(userID, hash, SessionTTL)
	if err := h.sessions.Save(ctx, sess); err != nil {
		return "", err
	}
	return token, nil
}
