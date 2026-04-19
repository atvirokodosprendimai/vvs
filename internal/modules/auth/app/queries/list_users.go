package queries

import (
	"context"
	"time"

	"github.com/vvs/isp/internal/modules/auth/domain"
)

type UserRow struct {
	ID        string
	Username  string
	FullName  string
	Division  string
	Role      domain.Role
	CreatedAt time.Time
}

type ListUsersHandler struct {
	users domain.UserRepository
}

func NewListUsersHandler(users domain.UserRepository) *ListUsersHandler {
	return &ListUsersHandler{users: users}
}

func (h *ListUsersHandler) Handle(ctx context.Context) ([]UserRow, error) {
	all, err := h.users.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]UserRow, len(all))
	for i, u := range all {
		rows[i] = UserRow{
			ID:        u.ID,
			Username:  u.Username,
			FullName:  u.FullName,
			Division:  u.Division,
			Role:      u.Role,
			CreatedAt: u.CreatedAt,
		}
	}
	return rows, nil
}
