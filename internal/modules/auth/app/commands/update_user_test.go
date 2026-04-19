package commands_test

import (
	"context"
	"testing"

	"github.com/atvirokodosprendimai/vvs/internal/modules/auth/app/commands"
	"github.com/atvirokodosprendimai/vvs/internal/modules/auth/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeStubRepo(users ...*domain.User) *stubUserRepo {
	m := make(map[string]*domain.User, len(users))
	for _, u := range users {
		m[u.ID] = u
	}
	return &stubUserRepo{users: m}
}

func makeUser(t *testing.T, id, username string, role domain.Role) *domain.User {
	t.Helper()
	u, err := domain.NewUser(username, "password123", role)
	require.NoError(t, err)
	u.ID = id
	return u
}

func TestUpdateUser_AdminCanUpdateAllFields(t *testing.T) {
	admin := makeUser(t, "admin-1", "admin", domain.RoleAdmin)
	target := makeUser(t, "user-1", "alice", domain.RoleOperator)
	repo := makeStubRepo(admin, target)
	h := commands.NewUpdateUserHandler(repo)

	err := h.Handle(context.Background(), commands.UpdateUserCommand{
		ActorID:  "admin-1",
		UserID:   "user-1",
		FullName: "Alice Smith",
		Division: "Engineering",
		Role:     domain.RoleViewer,
	})
	require.NoError(t, err)

	updated, _ := repo.FindByID(context.Background(), "user-1")
	assert.Equal(t, "Alice Smith", updated.FullName)
	assert.Equal(t, "Engineering", updated.Division)
	assert.Equal(t, domain.RoleViewer, updated.Role)
}

func TestUpdateUser_SelfCanUpdateFullNameOnly(t *testing.T) {
	user := makeUser(t, "user-1", "bob", domain.RoleOperator)
	user.Division = "Sales"
	repo := makeStubRepo(user)
	h := commands.NewUpdateUserHandler(repo)

	err := h.Handle(context.Background(), commands.UpdateUserCommand{
		ActorID:  "user-1",
		UserID:   "user-1",
		FullName: "Bob Jones",
		Division: "HACKED",    // should be ignored
		Role:     domain.RoleAdmin, // should be ignored
	})
	require.NoError(t, err)

	updated, _ := repo.FindByID(context.Background(), "user-1")
	assert.Equal(t, "Bob Jones", updated.FullName)
	assert.Equal(t, "Sales", updated.Division)       // unchanged
	assert.Equal(t, domain.RoleOperator, updated.Role) // unchanged
}

func TestUpdateUser_NonAdminCannotEditOtherUser(t *testing.T) {
	actor := makeUser(t, "user-1", "bob", domain.RoleOperator)
	target := makeUser(t, "user-2", "alice", domain.RoleOperator)
	repo := makeStubRepo(actor, target)
	h := commands.NewUpdateUserHandler(repo)

	err := h.Handle(context.Background(), commands.UpdateUserCommand{
		ActorID:  "user-1",
		UserID:   "user-2",
		FullName: "Alice Hacked",
	})
	assert.ErrorIs(t, err, commands.ErrForbidden)
}

func TestUpdateUser_UnknownUserReturnsError(t *testing.T) {
	admin := makeUser(t, "admin-1", "admin", domain.RoleAdmin)
	repo := makeStubRepo(admin)
	h := commands.NewUpdateUserHandler(repo)

	err := h.Handle(context.Background(), commands.UpdateUserCommand{
		ActorID: "admin-1",
		UserID:  "no-such-user",
	})
	assert.ErrorIs(t, err, domain.ErrUserNotFound)
}

func TestUpdateUser_InvalidRoleReturnsError(t *testing.T) {
	admin := makeUser(t, "admin-1", "admin", domain.RoleAdmin)
	target := makeUser(t, "user-1", "alice", domain.RoleOperator)
	repo := makeStubRepo(admin, target)
	h := commands.NewUpdateUserHandler(repo)

	err := h.Handle(context.Background(), commands.UpdateUserCommand{
		ActorID: "admin-1",
		UserID:  "user-1",
		Role:    domain.Role("superuser"),
	})
	assert.ErrorIs(t, err, domain.ErrInvalidRole)
}
