package domain_test

import (
	"testing"
	"time"

	"github.com/vvs/isp/internal/modules/portal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPortalToken(t *testing.T) {
	tok, plain, err := domain.NewPortalToken("cust-123", 24*time.Hour)
	require.NoError(t, err)

	assert.NotEmpty(t, tok.ID)
	assert.Equal(t, "cust-123", tok.CustomerID)
	assert.NotEmpty(t, tok.TokenHash)
	assert.NotEmpty(t, plain)
	assert.NotEqual(t, plain, tok.TokenHash, "plaintext must not equal stored hash")
	assert.False(t, tok.IsExpired())
	assert.WithinDuration(t, time.Now().Add(24*time.Hour), tok.ExpiresAt, 5*time.Second)
}

func TestNewPortalToken_TwoCallsProduceDifferentTokens(t *testing.T) {
	tok1, plain1, err1 := domain.NewPortalToken("cust-1", time.Hour)
	tok2, plain2, err2 := domain.NewPortalToken("cust-1", time.Hour)

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.NotEqual(t, plain1, plain2)
	assert.NotEqual(t, tok1.TokenHash, tok2.TokenHash)
	assert.NotEqual(t, tok1.ID, tok2.ID)
}

func TestHashOf_MatchesNewTokenHash(t *testing.T) {
	tok, plain, err := domain.NewPortalToken("cust-abc", time.Hour)
	require.NoError(t, err)

	assert.Equal(t, tok.TokenHash, domain.HashOf(plain))
}

func TestHashOf_DeterministicForSameInput(t *testing.T) {
	h1 := domain.HashOf("same-token")
	h2 := domain.HashOf("same-token")
	assert.Equal(t, h1, h2)
}

func TestPortalToken_IsExpired(t *testing.T) {
	tok, _, err := domain.NewPortalToken("c", time.Millisecond)
	require.NoError(t, err)

	// Newly created — not expired yet (or just barely)
	// Wait until expiry
	time.Sleep(5 * time.Millisecond)
	assert.True(t, tok.IsExpired())
}

func TestPortalToken_IsNotExpired_FutureTTL(t *testing.T) {
	tok, _, err := domain.NewPortalToken("c", 24*time.Hour)
	require.NoError(t, err)
	assert.False(t, tok.IsExpired())
}
