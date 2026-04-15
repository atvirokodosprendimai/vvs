package events

import (
	"context"
	"time"
)

type DomainEvent struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	AggregateID string    `json:"aggregate_id"`
	OccurredAt  time.Time `json:"occurred_at"`
	Data        []byte    `json:"data"`
}

type EventPublisher interface {
	Publish(ctx context.Context, subject string, event DomainEvent) error
}

type EventHandler func(event DomainEvent) error

type EventSubscriber interface {
	Subscribe(subject string, handler EventHandler) error
	// ChanSubscription subscribes to subject and returns a channel of events plus
	// a cancel func. Caller MUST call cancel (e.g. defer cancel()) to unsubscribe.
	// Supports wildcard subjects (e.g. "isp.invoice.*").
	ChanSubscription(subject string) (<-chan DomainEvent, func())
	Close() error
}
