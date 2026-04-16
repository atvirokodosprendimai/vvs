package domain

import (
	"errors"
	"time"
)

// System tag names — always present, cannot be deleted.
const (
	TagUnread   = "unread"
	TagStarred  = "starred"
	TagArchived = "archived"
)

var ErrTagNotFound = errors.New("email: tag not found")

// EmailTag is a label applied to threads.
type EmailTag struct {
	ID        string
	AccountID string // empty = global tag
	Name      string
	Color     string // hex color, e.g. "#3b82f6"
	System    bool   // true for unread/starred/archived
	CreatedAt time.Time
}

// EmailThreadTag is the join between a thread and a tag.
type EmailThreadTag struct {
	ThreadID string
	TagID    string
}
