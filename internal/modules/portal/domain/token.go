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

// PortalToken grants a specific customer authenticated access to the customer portal.
// The plaintext token is sent to the customer; only its SHA-256 hash is persisted.
type PortalToken struct {
	ID         string
	CustomerID string
	TokenHash  string // SHA-256(plaintext), only this is stored
	ExpiresAt  time.Time
	CreatedAt  time.Time
}

// NewPortalToken creates a portal token for the given customer with the specified TTL.
// Returns the token record and the plaintext token to embed in the URL.
// Returns an error if the OS random source fails.
func NewPortalToken(customerID string, ttl time.Duration) (*PortalToken, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, "", fmt.Errorf("portal token: read random: %w", err)
	}
	plain := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(plain))
	now := time.Now().UTC()
	return &PortalToken{
		ID:         uuid.Must(uuid.NewV7()).String(),
		CustomerID: customerID,
		TokenHash:  hex.EncodeToString(sum[:]),
		ExpiresAt:  now.Add(ttl),
		CreatedAt:  now,
	}, plain, nil
}

// IsExpired reports whether the token has passed its expiry time.
func (t *PortalToken) IsExpired() bool {
	return time.Now().UTC().After(t.ExpiresAt)
}

// HashOf returns the SHA-256 hex hash of a plaintext token for lookup.
func HashOf(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// PortalTokenRepository is the persistence port for portal tokens.
type PortalTokenRepository interface {
	Save(ctx context.Context, t *PortalToken) error
	FindByHash(ctx context.Context, hash string) (*PortalToken, error)
	DeleteByCustomerID(ctx context.Context, customerID string) error
	PruneExpired(ctx context.Context) error
}
