package nats

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/vvs/isp/internal/infrastructure/metrics"
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
		metrics.NATSReceived.WithLabelValues(subject).Inc()
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

// ChanSubscription subscribes to subject and returns a channel of events plus
// a cancel func. The caller MUST call cancel when done to unsubscribe and
// prevent leaking the NATS subscription (e.g. defer cancel() in SSE handlers).
func (s *Subscriber) ChanSubscription(subject string) (<-chan events.DomainEvent, func()) {
	ch := make(chan events.DomainEvent, 16)
	sub, _ := s.nc.Subscribe(subject, func(msg *nats.Msg) {
		var event events.DomainEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			return
		}
		select {
		case ch <- event:
		default: // drop if consumer is slow
		}
	})
	cancel := func() {
		_ = sub.Unsubscribe()
		close(ch)
	}
	return ch, cancel
}

// ChanSubscriptionOf subscribes to subject and returns a typed channel of T plus
// a cancel func. event.Data (not the DomainEvent envelope) is unmarshalled into T.
// The caller MUST call cancel when done (e.g. defer cancel() in SSE handlers).
// Wildcard subjects (e.g. "isp.invoice.*") are supported.
func ChanSubscriptionOf[T any](s *Subscriber, subject string) (<-chan T, func()) {
	ch := make(chan T, 16)
	sub, _ := s.nc.Subscribe(subject, func(msg *nats.Msg) {
		var event events.DomainEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			return
		}
		var item T
		if err := json.Unmarshal(event.Data, &item); err != nil {
			return
		}
		select {
		case ch <- item:
		default: // drop if consumer is slow
		}
	})
	cancel := func() {
		_ = sub.Unsubscribe()
		close(ch)
	}
	return ch, cancel
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
