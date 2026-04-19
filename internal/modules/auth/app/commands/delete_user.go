package commands

import (
	"context"

	"github.com/atvirokodosprendimai/vvs/internal/modules/auth/domain"
)

type DeleteUserCommand struct {
	ID string
}

type DeleteUserHandler struct {
	users    domain.UserRepository
	sessions domain.SessionRepository
}

func NewDeleteUserHandler(users domain.UserRepository, sessions domain.SessionRepository) *DeleteUserHandler {
	return &DeleteUserHandler{users: users, sessions: sessions}
}

func (h *DeleteUserHandler) Handle(ctx context.Context, cmd DeleteUserCommand) error {
	if _, err := h.users.FindByID(ctx, cmd.ID); err != nil {
		return err
	}
	if err := h.sessions.DeleteByUserID(ctx, cmd.ID); err != nil {
		return err
	}
	return h.users.Delete(ctx, cmd.ID)
}
