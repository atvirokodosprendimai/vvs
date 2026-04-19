package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vvs/isp/internal/modules/invoice/domain"
)

func TestNewInvoiceToken_GeneratesUniqueTokens(t *testing.T) {
	tok1, plain1, err1 := domain.NewInvoiceToken("inv-1", 48*time.Hour)
	tok2, plain2, err2 := domain.NewInvoiceToken("inv-1", 48*time.Hour)

	require.NoError(t, err1)
	require.NoError(t, err2)
	require.NotEmpty(t, plain1)
	require.NotEmpty(t, plain2)
	assert.NotEqual(t, plain1, plain2, "each token must be unique")
	assert.NotEqual(t, tok1.TokenHash, tok2.TokenHash)
}

func TestNewInvoiceToken_HashIsNotPlaintext(t *testing.T) {
	tok, plain, err := domain.NewInvoiceToken("inv-1", 48*time.Hour)
	require.NoError(t, err)
	assert.NotEqual(t, tok.TokenHash, plain, "stored hash must differ from plain token")
	assert.NotEmpty(t, tok.ID)
	assert.Equal(t, "inv-1", tok.InvoiceID)
}

func TestNewInvoiceToken_ExpiryRespectsTTL(t *testing.T) {
	before := time.Now()
	tok, _, err := domain.NewInvoiceToken("inv-1", 48*time.Hour)
	after := time.Now()

	require.NoError(t, err)
	assert.True(t, tok.ExpiresAt.After(before.Add(47*time.Hour)))
	assert.True(t, tok.ExpiresAt.Before(after.Add(49*time.Hour)))
}

func TestInvoiceToken_IsExpired(t *testing.T) {
	tok, _, err := domain.NewInvoiceToken("inv-1", -1*time.Second)
	require.NoError(t, err)
	assert.True(t, tok.IsExpired())

	tok2, _, err2 := domain.NewInvoiceToken("inv-1", 1*time.Hour)
	require.NoError(t, err2)
	assert.False(t, tok2.IsExpired())
}
