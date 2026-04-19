package queries

import (
	"encoding/json"
	"time"
)

// AuditLogReadModel is the DTO returned by audit log queries.
type AuditLogReadModel struct {
	ID         string          `json:"id"`
	ActorID    string          `json:"actor_id"`
	ActorName  string          `json:"actor_name"`
	Action     string          `json:"action"`
	Resource   string          `json:"resource"`
	ResourceID string          `json:"resource_id"`
	Changes    json.RawMessage `json:"changes,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}
