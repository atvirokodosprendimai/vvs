package domain

import (
	"context"
	"time"
)

// Filter controls which audit log entries are returned.
type Filter struct {
	ActorID  string
	Resource string
	From     *time.Time
	To       *time.Time
	Limit    int // 0 = use default (100)
}

// AuditLogRepository is the persistence port for audit logs.
// Only Save is a write; all reads are query-only.
type AuditLogRepository interface {
	Save(ctx context.Context, al *AuditLog) error
	ListAll(ctx context.Context, filter Filter) ([]*AuditLog, error)
	ListForResource(ctx context.Context, resource, resourceID string) ([]*AuditLog, error)
}
