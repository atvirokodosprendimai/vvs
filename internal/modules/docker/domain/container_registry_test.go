package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/domain"
)

func TestNewContainerRegistry_Valid(t *testing.T) {
	reg, err := domain.NewContainerRegistry("My Registry", "registry.example.com", "user", "secret")
	require.NoError(t, err)
	assert.NotEmpty(t, reg.ID)
	assert.Equal(t, "My Registry", reg.Name)
	assert.Equal(t, "registry.example.com", reg.URL)
	assert.Equal(t, "user", reg.Username)
	assert.Equal(t, "secret", reg.Password)
	assert.False(t, reg.CreatedAt.IsZero())
}

func TestNewContainerRegistry_MissingName(t *testing.T) {
	_, err := domain.NewContainerRegistry("", "registry.example.com", "u", "p")
	assert.ErrorIs(t, err, domain.ErrRegistryNameRequired)
}

func TestNewContainerRegistry_MissingURL(t *testing.T) {
	_, err := domain.NewContainerRegistry("My Reg", "", "u", "p")
	assert.ErrorIs(t, err, domain.ErrRegistryURLRequired)
}

func TestContainerRegistry_Update_KeepsPasswordIfEmpty(t *testing.T) {
	reg, _ := domain.NewContainerRegistry("Reg", "reg.io", "user", "original")
	before := reg.UpdatedAt

	reg.Update("Reg2", "reg2.io", "user2", "")

	assert.Equal(t, "Reg2", reg.Name)
	assert.Equal(t, "reg2.io", reg.URL)
	assert.Equal(t, "user2", reg.Username)
	assert.Equal(t, "original", reg.Password) // unchanged
	assert.True(t, reg.UpdatedAt.After(before) || reg.UpdatedAt.Equal(before))
}

func TestContainerRegistry_Update_ReplacesPassword(t *testing.T) {
	reg, _ := domain.NewContainerRegistry("Reg", "reg.io", "user", "original")
	reg.Update("Reg", "reg.io", "user", "newpass")
	assert.Equal(t, "newpass", reg.Password)
}
