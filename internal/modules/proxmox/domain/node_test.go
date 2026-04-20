package domain_test

import (
	"testing"

	"github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProxmoxNode_Valid(t *testing.T) {
	n, err := domain.NewProxmoxNode("pve-01", "pve", "192.168.1.10", 8006, "root@pam", "vvs", "secret", "notes", false)
	require.NoError(t, err)
	assert.NotEmpty(t, n.ID)
	assert.Equal(t, "pve-01", n.Name)
	assert.Equal(t, "pve", n.NodeName)
	assert.Equal(t, "192.168.1.10", n.Host)
	assert.Equal(t, 8006, n.Port)
	assert.Equal(t, "root@pam", n.User)
	assert.Equal(t, "vvs", n.TokenID)
	assert.Equal(t, "secret", n.TokenSecret)
	assert.False(t, n.InsecureTLS)
}

func TestNewProxmoxNode_DefaultPort(t *testing.T) {
	n, err := domain.NewProxmoxNode("pve-01", "pve", "192.168.1.10", 0, "root@pam", "vvs", "secret", "", false)
	require.NoError(t, err)
	assert.Equal(t, 8006, n.Port)
}

func TestNewProxmoxNode_DefaultNodeName(t *testing.T) {
	n, err := domain.NewProxmoxNode("pve-01", "", "192.168.1.10", 0, "root@pam", "vvs", "secret", "", false)
	require.NoError(t, err)
	assert.Equal(t, "192.168.1.10", n.NodeName)
}

func TestNewProxmoxNode_EmptyName(t *testing.T) {
	_, err := domain.NewProxmoxNode("", "pve", "192.168.1.10", 0, "root@pam", "vvs", "secret", "", false)
	assert.ErrorIs(t, err, domain.ErrNodeNameRequired)
}

func TestNewProxmoxNode_EmptyHost(t *testing.T) {
	_, err := domain.NewProxmoxNode("pve-01", "pve", "", 0, "root@pam", "vvs", "secret", "", false)
	assert.ErrorIs(t, err, domain.ErrHostRequired)
}

func TestNewProxmoxNode_EmptyUser(t *testing.T) {
	_, err := domain.NewProxmoxNode("pve-01", "pve", "192.168.1.10", 0, "", "vvs", "secret", "", false)
	assert.ErrorIs(t, err, domain.ErrUserRequired)
}

func TestNewProxmoxNode_EmptyTokenID(t *testing.T) {
	_, err := domain.NewProxmoxNode("pve-01", "pve", "192.168.1.10", 0, "root@pam", "", "secret", "", false)
	assert.ErrorIs(t, err, domain.ErrTokenIDRequired)
}

func TestProxmoxNode_Update_PreservesSecretWhenEmpty(t *testing.T) {
	n, err := domain.NewProxmoxNode("pve-01", "pve", "192.168.1.10", 8006, "root@pam", "vvs", "original-secret", "", false)
	require.NoError(t, err)

	err = n.Update("pve-01-renamed", "pve", "192.168.1.10", 8006, "root@pam", "vvs", "", "", false)
	require.NoError(t, err)
	assert.Equal(t, "original-secret", n.TokenSecret, "empty tokenSecret must preserve existing value")
	assert.Equal(t, "pve-01-renamed", n.Name)
}

func TestProxmoxNode_ToConn(t *testing.T) {
	n, err := domain.NewProxmoxNode("pve-01", "pve", "192.168.1.10", 8006, "root@pam", "vvs", "secret", "", true)
	require.NoError(t, err)

	conn := n.ToConn()
	assert.Equal(t, "pve", conn.NodeName)
	assert.Equal(t, "192.168.1.10", conn.Host)
	assert.Equal(t, 8006, conn.Port)
	assert.Equal(t, "root@pam", conn.User)
	assert.Equal(t, "vvs", conn.TokenID)
	assert.Equal(t, "secret", conn.TokenSecret)
	assert.True(t, conn.InsecureTLS)
}
