package commands

import (
	"context"
	"encoding/json"

	"github.com/vvs/isp/internal/modules/audit_log/domain"
)

// CreateAuditLogCommand carries the data needed to record an audit event.
type CreateAuditLogCommand struct {
	ActorID    string
	ActorName  string
	Action     string
	Resource   string
	ResourceID string
	Changes    json.RawMessage
}

// CreateAuditLogHandler saves an audit log entry.
// No NATS publish — audit log is terminal; publishing an event for each audit
// would cause recursive auditing.
type CreateAuditLogHandler struct {
	repo domain.AuditLogRepository
}

func NewCreateAuditLogHandler(repo domain.AuditLogRepository) *CreateAuditLogHandler {
	return &CreateAuditLogHandler{repo: repo}
}

func (h *CreateAuditLogHandler) Handle(ctx context.Context, cmd CreateAuditLogCommand) error {
	al, err := domain.NewAuditLog(cmd.ActorID, cmd.ActorName, cmd.Action, cmd.Resource, cmd.ResourceID, cmd.Changes)
	if err != nil {
		return err
	}
	return h.repo.Save(ctx, al)
}

// Log implements audit.Logger. Convenience wrapper for injection into HTTP handlers.
func (h *CreateAuditLogHandler) Log(ctx context.Context, actorID, actorName, action, resource, resourceID string, changes json.RawMessage) error {
	return h.Handle(ctx, CreateAuditLogCommand{
		ActorID:    actorID,
		ActorName:  actorName,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Changes:    changes,
	})
}
