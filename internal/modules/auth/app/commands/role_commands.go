package commands

import (
	"context"
	"strings"

	"github.com/atvirokodosprendimai/vvs/internal/modules/auth/domain"
)

// CreateRoleCommand creates a new custom role.
type CreateRoleCommand struct {
	Name        string
	DisplayName string
	CanWrite    bool
}

type CreateRoleHandler struct {
	roles domain.RoleRepository
}

func NewCreateRoleHandler(roles domain.RoleRepository) *CreateRoleHandler {
	return &CreateRoleHandler{roles: roles}
}

func (h *CreateRoleHandler) Handle(ctx context.Context, cmd CreateRoleCommand) (*domain.RoleDefinition, error) {
	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return nil, domain.ErrRoleNameEmpty
	}
	existing, err := h.roles.FindByName(ctx, domain.Role(name))
	if err != nil && err != domain.ErrRoleNotFound {
		return nil, err
	}
	if existing != nil {
		return nil, domain.ErrRoleExists
	}
	rd := &domain.RoleDefinition{
		Name:        domain.Role(name),
		DisplayName: strings.TrimSpace(cmd.DisplayName),
		IsBuiltin:   false,
		CanWrite:    cmd.CanWrite,
	}
	if err := h.roles.Save(ctx, rd); err != nil {
		return nil, err
	}
	return rd, nil
}

// DeleteRoleCommand deletes a custom role.
type DeleteRoleCommand struct {
	Name domain.Role
}

type DeleteRoleHandler struct {
	roles domain.RoleRepository
}

func NewDeleteRoleHandler(roles domain.RoleRepository) *DeleteRoleHandler {
	return &DeleteRoleHandler{roles: roles}
}

func (h *DeleteRoleHandler) Handle(ctx context.Context, cmd DeleteRoleCommand) error {
	return h.roles.Delete(ctx, cmd.Name)
}
