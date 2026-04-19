package domain

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
)

// InvoiceToken is a short-lived signed token granting public access to one invoice PDF.
type InvoiceToken struct {
	ID        string
	InvoiceID string
	TokenHash string // SHA-256(plain token), stored; plain token is sent to customer
	ExpiresAt time.Time
	CreatedAt time.Time
}

// NewInvoiceToken creates a token and returns it along with the plaintext token to embed in the URL.
// The plaintext token is never stored — only its SHA-256 hash is persisted.
func NewInvoiceToken(invoiceID string, ttl time.Duration) (*InvoiceToken, string) {
	raw := make([]byte, 32)
	_, _ = rand.Read(raw)
	plain := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(plain))
	now := time.Now().UTC()
	return &InvoiceToken{
		ID:        uuid.Must(uuid.NewV7()).String(),
		InvoiceID: invoiceID,
		TokenHash: hex.EncodeToString(sum[:]),
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
	}, plain
}

// IsExpired reports whether the token has passed its expiry time.
func (t *InvoiceToken) IsExpired() bool {
	return time.Now().UTC().After(t.ExpiresAt)
}

// InvoiceTokenRepository is the port for token persistence.
type InvoiceTokenRepository interface {
	Save(ctx context.Context, t *InvoiceToken) error
	FindByHash(ctx context.Context, hash string) (*InvoiceToken, error)
}
