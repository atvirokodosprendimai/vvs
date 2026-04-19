package commands

import (
	"context"

	"github.com/atvirokodosprendimai/vvs/internal/modules/auth/domain"
)

type CreateUserCommand struct {
	Username string
	Password string
	Role     domain.Role
}

type CreateUserHandler struct {
	users domain.UserRepository
}

func NewCreateUserHandler(users domain.UserRepository) *CreateUserHandler {
	return &CreateUserHandler{users: users}
}

func (h *CreateUserHandler) Handle(ctx context.Context, cmd CreateUserCommand) (*domain.User, error) {
	existing, err := h.users.FindByUsername(ctx, cmd.Username)
	if err != nil && err != domain.ErrUserNotFound {
		return nil, err
	}
	if existing != nil {
		return nil, domain.ErrUsernameTaken
	}

	u, err := domain.NewUser(cmd.Username, cmd.Password, cmd.Role)
	if err != nil {
		return nil, err
	}

	if err := h.users.Save(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}
