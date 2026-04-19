package domain

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrMissingAction     = errors.New("audit: action is required")
	ErrMissingResource   = errors.New("audit: resource is required")
	ErrMissingResourceID = errors.New("audit: resource_id is required")
)

// AuditLog is an immutable record of a domain mutation.
type AuditLog struct {
	ID         string
	ActorID    string
	ActorName  string
	Action     string          // e.g. "customer.created", "invoice.finalized"
	Resource   string          // e.g. "customer", "ticket", "invoice"
	ResourceID string
	Changes    json.RawMessage // JSON snapshot or diff — may be nil
	CreatedAt  time.Time
}

// NewAuditLog creates a new audit log entry. actor fields may be empty (system actions).
func NewAuditLog(actorID, actorName, action, resource, resourceID string, changes json.RawMessage) (*AuditLog, error) {
	if action == "" {
		return nil, ErrMissingAction
	}
	if resource == "" {
		return nil, ErrMissingResource
	}
	if resourceID == "" {
		return nil, ErrMissingResourceID
	}
	return &AuditLog{
		ID:         uuid.Must(uuid.NewV7()).String(),
		ActorID:    actorID,
		ActorName:  actorName,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Changes:    changes,
		CreatedAt:  time.Now().UTC(),
	}, nil
}
