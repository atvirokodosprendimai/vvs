package commands

import (
	"context"

	"github.com/atvirokodosprendimai/vvs/internal/modules/auth/domain"
)

// ChangeSelfPasswordCommand is used by an authenticated user changing their own password.
// CurrentPassword is verified before the change is applied.
type ChangeSelfPasswordCommand struct {
	UserID          string
	CurrentPassword string
	NewPassword     string
}

type ChangeSelfPasswordHandler struct {
	users domain.UserRepository
}

func NewChangeSelfPasswordHandler(users domain.UserRepository) *ChangeSelfPasswordHandler {
	return &ChangeSelfPasswordHandler{users: users}
}

func (h *ChangeSelfPasswordHandler) Handle(ctx context.Context, cmd ChangeSelfPasswordCommand) error {
	u, err := h.users.FindByID(ctx, cmd.UserID)
	if err != nil {
		return err
	}
	if !u.VerifyPassword(cmd.CurrentPassword) {
		return domain.ErrInvalidPassword
	}
	if err := u.ChangePassword(cmd.NewPassword); err != nil {
		return err
	}
	return h.users.Save(ctx, u)
}
