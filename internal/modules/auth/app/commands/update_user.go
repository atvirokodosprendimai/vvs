package commands

import (
	"context"
	"errors"

	"github.com/vvs/isp/internal/modules/auth/domain"
)

var ErrForbidden = errors.New("forbidden")

type UpdateUserCommand struct {
	ActorID  string
	UserID   string
	FullName string
	Division string
	Role     domain.Role
}

type UpdateUserHandler struct {
	users domain.UserRepository
}

func NewUpdateUserHandler(users domain.UserRepository) *UpdateUserHandler {
	return &UpdateUserHandler{users: users}
}

func (h *UpdateUserHandler) Handle(ctx context.Context, cmd UpdateUserCommand) error {
	actor, err := h.users.FindByID(ctx, cmd.ActorID)
	if err != nil {
		return err
	}

	target, err := h.users.FindByID(ctx, cmd.UserID)
	if err != nil {
		return err
	}

	isSelf := actor.ID == target.ID

	if actor.IsAdmin() {
		// Admin: can update all fields for any user
		target.UpdateProfile(cmd.FullName, cmd.Division)
		if cmd.Role != "" && cmd.Role != target.Role {
			if err := target.ChangeRole(cmd.Role); err != nil {
				return err
			}
		}
	} else if isSelf {
		// Self-service: full name only; division and role stay unchanged
		target.UpdateProfile(cmd.FullName, target.Division)
	} else {
		return ErrForbidden
	}

	return h.users.Save(ctx, target)
}
