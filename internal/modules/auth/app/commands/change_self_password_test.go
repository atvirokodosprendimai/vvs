package commands_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/atvirokodosprendimai/vvs/internal/modules/auth/app/commands"
	"github.com/atvirokodosprendimai/vvs/internal/modules/auth/domain"
)

type stubUserRepo struct {
	users map[string]*domain.User
	saved []*domain.User
}

func (r *stubUserRepo) Save(_ context.Context, u *domain.User) error {
	r.saved = append(r.saved, u)
	r.users[u.ID] = u
	return nil
}

func (r *stubUserRepo) FindByID(_ context.Context, id string) (*domain.User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return u, nil
}

func (r *stubUserRepo) FindByUsername(_ context.Context, _ string) (*domain.User, error) {
	return nil, domain.ErrUserNotFound
}

func (r *stubUserRepo) ListAll(_ context.Context) ([]*domain.User, error) { return nil, nil }
func (r *stubUserRepo) Delete(_ context.Context, _ string) error          { return nil }

func newStubUserRepo(u *domain.User) *stubUserRepo {
	return &stubUserRepo{users: map[string]*domain.User{u.ID: u}}
}

func TestChangeSelfPassword_CorrectCurrent_Succeeds(t *testing.T) {
	u, _ := domain.NewUser("alice", "oldpass", domain.RoleOperator)
	repo := newStubUserRepo(u)
	h := commands.NewChangeSelfPasswordHandler(repo)

	err := h.Handle(context.Background(), commands.ChangeSelfPasswordCommand{
		UserID:          u.ID,
		CurrentPassword: "oldpass",
		NewPassword:     "newpass123",
	})

	require.NoError(t, err)
	assert.True(t, u.VerifyPassword("newpass123"))
	assert.Len(t, repo.saved, 1)
}

func TestChangeSelfPassword_WrongCurrent_Rejected(t *testing.T) {
	u, _ := domain.NewUser("alice", "correctpass", domain.RoleOperator)
	repo := newStubUserRepo(u)
	h := commands.NewChangeSelfPasswordHandler(repo)

	err := h.Handle(context.Background(), commands.ChangeSelfPasswordCommand{
		UserID:          u.ID,
		CurrentPassword: "wrongpass",
		NewPassword:     "newpass123",
	})

	assert.ErrorIs(t, err, domain.ErrInvalidPassword)
	assert.Empty(t, repo.saved)
}

func TestChangeSelfPassword_EmptyNew_Rejected(t *testing.T) {
	u, _ := domain.NewUser("alice", "correctpass", domain.RoleOperator)
	repo := newStubUserRepo(u)
	h := commands.NewChangeSelfPasswordHandler(repo)

	err := h.Handle(context.Background(), commands.ChangeSelfPasswordCommand{
		UserID:          u.ID,
		CurrentPassword: "correctpass",
		NewPassword:     "",
	})

	assert.ErrorIs(t, err, domain.ErrPasswordRequired)
	assert.Empty(t, repo.saved)
}

func TestChangeSelfPassword_UserNotFound(t *testing.T) {
	repo := &stubUserRepo{users: map[string]*domain.User{}}
	h := commands.NewChangeSelfPasswordHandler(repo)

	err := h.Handle(context.Background(), commands.ChangeSelfPasswordCommand{
		UserID:          "nonexistent",
		CurrentPassword: "pass",
		NewPassword:     "newpass",
	})

	assert.ErrorIs(t, err, domain.ErrUserNotFound)
}
