package domain

import (
	"context"
	"errors"
)

var (
	ErrRoleNotFound  = errors.New("role not found")
	ErrRoleBuiltin   = errors.New("built-in role cannot be deleted")
	ErrRoleInUse     = errors.New("role is assigned to one or more users")
	ErrRoleNameEmpty = errors.New("role name is required")
	ErrRoleExists    = errors.New("role name already taken")
	ErrInvalidRole   = errors.New("invalid role")
)

// RoleDefinition is a named role stored in the roles table.
type RoleDefinition struct {
	Name        Role
	DisplayName string
	IsBuiltin   bool
	CanWrite    bool
}

// RoleRepository is the port for role persistence.
type RoleRepository interface {
	List(ctx context.Context) ([]RoleDefinition, error)
	FindByName(ctx context.Context, name Role) (*RoleDefinition, error)
	Save(ctx context.Context, r *RoleDefinition) error
	Delete(ctx context.Context, name Role) error
}
