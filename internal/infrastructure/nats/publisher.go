package nats

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/vvs/isp/internal/shared/events"
)

type Publisher struct {
	nc *nats.Conn
}

func NewPublisher(nc *nats.Conn) *Publisher {
	return &Publisher{nc: nc}
}

func (p *Publisher) Publish(_ context.Context, subject string, event events.DomainEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	return p.nc.Publish(subject, data)
}
