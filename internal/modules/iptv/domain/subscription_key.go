package domain

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

// SubscriptionKey is a per-subscription API credential embedded in all STB URLs.
// Token is stored PLAIN — it must be showable to the admin for copy/paste to customer.
// If compromised: revoke and issue a new one.
type SubscriptionKey struct {
	ID             string
	SubscriptionID string
	CustomerID     string
	PackageID      string
	Token          string     // 64-char hex (32 random bytes)
	CreatedAt      time.Time
	RevokedAt      *time.Time // nil = active
}

// NewSubscriptionKey generates a fresh key with a cryptographically random token.
func NewSubscriptionKey(id, subscriptionID, customerID, packageID string) (*SubscriptionKey, error) {
	if subscriptionID == "" {
		return nil, errors.New("iptv: subscription id required")
	}
	if customerID == "" {
		return nil, errors.New("iptv: customer id required")
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return &SubscriptionKey{
		ID:             id,
		SubscriptionID: subscriptionID,
		CustomerID:     customerID,
		PackageID:      packageID,
		Token:          hex.EncodeToString(b),
		CreatedAt:      time.Now().UTC(),
	}, nil
}

// IsActive returns true if the key has not been revoked.
func (k *SubscriptionKey) IsActive() bool {
	return k.RevokedAt == nil
}

// Revoke marks the key as revoked.
func (k *SubscriptionKey) Revoke() {
	now := time.Now().UTC()
	k.RevokedAt = &now
}

// SubscriptionKeyRepository is the port for subscription key persistence.
type SubscriptionKeyRepository interface {
	Save(ctx context.Context, k *SubscriptionKey) error
	FindByID(ctx context.Context, id string) (*SubscriptionKey, error)
	FindByToken(ctx context.Context, token string) (*SubscriptionKey, error)
	FindBySubscriptionID(ctx context.Context, subscriptionID string) ([]*SubscriptionKey, error)
	Delete(ctx context.Context, id string) error
}
