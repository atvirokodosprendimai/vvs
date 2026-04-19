package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vvs/isp/internal/modules/auth/domain"
)

func TestNewUser(t *testing.T) {
	t.Run("creates user with hashed password", func(t *testing.T) {
		u, err := domain.NewUser("alice", "secret123", domain.RoleOperator)
		require.NoError(t, err)
		assert.Equal(t, "alice", u.Username)
		assert.Equal(t, domain.RoleOperator, u.Role)
		assert.NotEmpty(t, u.ID)
		assert.NotEqual(t, "secret123", u.PasswordHash)
		assert.NotEmpty(t, u.CreatedAt)
	})

	t.Run("rejects empty username", func(t *testing.T) {
		_, err := domain.NewUser("", "secret", domain.RoleAdmin)
		assert.ErrorIs(t, err, domain.ErrUsernameRequired)
	})

	t.Run("rejects empty password", func(t *testing.T) {
		_, err := domain.NewUser("bob", "", domain.RoleAdmin)
		assert.ErrorIs(t, err, domain.ErrPasswordRequired)
	})

	t.Run("rejects invalid role", func(t *testing.T) {
		_, err := domain.NewUser("bob", "secret", domain.Role("superuser"))
		assert.ErrorIs(t, err, domain.ErrInvalidRole)
	})
}

func TestVerifyPassword(t *testing.T) {
	u, _ := domain.NewUser("alice", "correct", domain.RoleAdmin)

	assert.True(t, u.VerifyPassword("correct"))
	assert.False(t, u.VerifyPassword("wrong"))
	assert.False(t, u.VerifyPassword(""))
}

func TestChangePassword(t *testing.T) {
	u, _ := domain.NewUser("alice", "old", domain.RoleAdmin)
	oldHash := u.PasswordHash

	require.NoError(t, u.ChangePassword("new"))
	assert.NotEqual(t, oldHash, u.PasswordHash)
	assert.True(t, u.VerifyPassword("new"))
	assert.False(t, u.VerifyPassword("old"))
}

func TestChangePassword_RejectsEmpty(t *testing.T) {
	u, _ := domain.NewUser("alice", "pass", domain.RoleAdmin)
	assert.ErrorIs(t, u.ChangePassword(""), domain.ErrPasswordRequired)
}

func TestValidRole(t *testing.T) {
	assert.True(t, domain.ValidRole(domain.RoleAdmin))
	assert.True(t, domain.ValidRole(domain.RoleOperator))
	assert.True(t, domain.ValidRole(domain.RoleViewer))
	assert.False(t, domain.ValidRole(domain.Role("superuser")))
}

func TestCanWrite(t *testing.T) {
	admin, _ := domain.NewUser("a", "pass", domain.RoleAdmin)
	op, _ := domain.NewUser("b", "pass", domain.RoleOperator)
	viewer, _ := domain.NewUser("c", "pass", domain.RoleViewer)

	assert.True(t, admin.CanWrite())
	assert.True(t, op.CanWrite())
	assert.False(t, viewer.CanWrite())
}

// ── TOTP tests ────────────────────────────────────────────────────────────────

func TestEnableTOTP_SetsSecretAndFlag(t *testing.T) {
	u, err := domain.NewUser("alice", "Password1!", domain.RoleAdmin)
	require.NoError(t, err)

	assert.False(t, u.TOTPEnabled)
	assert.Empty(t, u.TOTPSecret)

	u.EnableTOTP("BASE32SECRET")
	assert.True(t, u.TOTPEnabled)
	assert.Equal(t, "BASE32SECRET", u.TOTPSecret)
}

func TestDisableTOTP_ClearsSecretAndFlag(t *testing.T) {
	u, err := domain.NewUser("alice", "Password1!", domain.RoleAdmin)
	require.NoError(t, err)
	u.EnableTOTP("BASE32SECRET")

	u.DisableTOTP()
	assert.False(t, u.TOTPEnabled)
	assert.Empty(t, u.TOTPSecret)
}

func TestVerifyTOTP_NotEnabled_ReturnsFalse(t *testing.T) {
	u, err := domain.NewUser("alice", "Password1!", domain.RoleAdmin)
	require.NoError(t, err)

	assert.False(t, u.VerifyTOTP("123456"))
}

func TestVerifyTOTP_WrongCode_ReturnsFalse(t *testing.T) {
	u, err := domain.NewUser("alice", "Password1!", domain.RoleAdmin)
	require.NoError(t, err)
	u.EnableTOTP("JBSWY3DPEHPK3PXP") // standard test secret

	assert.False(t, u.VerifyTOTP("000000"))
}
