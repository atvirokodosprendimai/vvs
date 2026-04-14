package commands

import (
	"context"

	"github.com/vvs/isp/internal/modules/auth/domain"
)

type LogoutCommand struct {
	TokenHash string
}

type LogoutHandler struct {
	sessions domain.SessionRepository
}

func NewLogoutHandler(sessions domain.SessionRepository) *LogoutHandler {
	return &LogoutHandler{sessions: sessions}
}

func (h *LogoutHandler) Handle(ctx context.Context, cmd LogoutCommand) error {
	sess, err := h.sessions.FindByTokenHash(ctx, cmd.TokenHash)
	if err != nil {
		return nil // already gone — treat as success
	}
	return h.sessions.DeleteByID(ctx, sess.ID)
}
