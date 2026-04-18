package audit

import (
	"context"
	"encoding/json"
)

// Logger records an audit event for a mutation.
// Implementations must be non-blocking (fire-and-forget acceptable).
type Logger interface {
	Log(ctx context.Context, actorID, actorName, action, resource, resourceID string, changes json.RawMessage) error
}

// NoopLogger discards all audit events. Use in tests or when audit module is disabled.
type NoopLogger struct{}

func (NoopLogger) Log(_ context.Context, _, _, _, _, _ string, _ json.RawMessage) error {
	return nil
}
