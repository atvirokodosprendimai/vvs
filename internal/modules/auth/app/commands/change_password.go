package commands

import (
	"context"

	"github.com/vvs/isp/internal/modules/auth/domain"
)

type ChangePasswordCommand struct {
	UserID      string
	NewPassword string
}

type ChangePasswordHandler struct {
	users domain.UserRepository
}

func NewChangePasswordHandler(users domain.UserRepository) *ChangePasswordHandler {
	return &ChangePasswordHandler{users: users}
}

func (h *ChangePasswordHandler) Handle(ctx context.Context, cmd ChangePasswordCommand) error {
	u, err := h.users.FindByID(ctx, cmd.UserID)
	if err != nil {
		return err
	}
	if err := u.ChangePassword(cmd.NewPassword); err != nil {
		return err
	}
	return h.users.Save(ctx, u)
}
