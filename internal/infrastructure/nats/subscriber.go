package nats

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/vvs/isp/internal/shared/events"
)

type Subscriber struct {
	nc   *nats.Conn
	subs []*nats.Subscription
	mu   sync.Mutex
}

func NewSubscriber(nc *nats.Conn) *Subscriber {
	return &Subscriber{nc: nc}
}

func (s *Subscriber) Subscribe(subject string, handler events.EventHandler) error {
	sub, err := s.nc.Subscribe(subject, func(msg *nats.Msg) {
		var event events.DomainEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			fmt.Printf("unmarshal event error: %v\n", err)
			return
		}
		if err := handler(event); err != nil {
			fmt.Printf("handle event error: %v\n", err)
		}
	})
	if err != nil {
		return fmt.Errorf("subscribe %s: %w", subject, err)
	}

	s.mu.Lock()
	s.subs = append(s.subs, sub)
	s.mu.Unlock()

	return nil
}

func (s *Subscriber) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, sub := range s.subs {
		if err := sub.Unsubscribe(); err != nil {
			return err
		}
	}
	s.subs = nil
	return nil
}
