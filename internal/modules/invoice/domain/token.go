package domain

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
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
// Returns an error if the OS random source fails (rare but must not silently produce a weak token).
func NewInvoiceToken(invoiceID string, ttl time.Duration) (*InvoiceToken, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, "", fmt.Errorf("invoice token: read random: %w", err)
	}
	plain := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(plain))
	now := time.Now().UTC()
	return &InvoiceToken{
		ID:        uuid.Must(uuid.NewV7()).String(),
		InvoiceID: invoiceID,
		TokenHash: hex.EncodeToString(sum[:]),
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
	}, plain, nil
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
