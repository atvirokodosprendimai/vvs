package nats

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/shared/events"
)

func TestChanSubscriptionOf_ReceivesTypedPayload(t *testing.T) {
	ns, nc, err := StartEmbedded("")
	if err != nil {
		t.Fatalf("start embedded nats: %v", err)
	}
	defer ns.Shutdown()
	defer nc.Close()

	sub := NewSubscriber(nc)
	pub := NewPublisher(nc)

	type item struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	ch, cancel := ChanSubscriptionOf[item](sub, "test.item.created")
	defer cancel()

	payload, _ := json.Marshal(item{Name: "hello", Value: 42})
	pub.Publish(context.Background(), "test.item.created", events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "item.created",
		AggregateID: "x",
		OccurredAt:  time.Now().UTC(),
		Data:        payload,
	})

	select {
	case got := <-ch:
		if got.Name != "hello" {
			t.Errorf("expected Name=hello; got %q", got.Name)
		}
		if got.Value != 42 {
			t.Errorf("expected Value=42; got %d", got.Value)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for typed item")
	}
}

func TestChanSubscriptionOf_WildcardSubject(t *testing.T) {
	ns, nc, err := StartEmbedded("")
	if err != nil {
		t.Fatalf("start embedded nats: %v", err)
	}
	defer ns.Shutdown()
	defer nc.Close()

	sub := NewSubscriber(nc)
	pub := NewPublisher(nc)

	type item struct {
		ID string `json:"id"`
	}

	ch, cancel := ChanSubscriptionOf[item](sub, "test.item.*")
	defer cancel()

	for _, subj := range []string{"test.item.created", "test.item.updated", "test.item.voided"} {
		payload, _ := json.Marshal(item{ID: subj})
		pub.Publish(context.Background(), subj, events.DomainEvent{
			ID:          uuid.Must(uuid.NewV7()).String(),
			Type:        subj,
			AggregateID: subj,
			OccurredAt:  time.Now().UTC(),
			Data:        payload,
		})
	}

	received := map[string]bool{}
	timeout := time.After(2 * time.Second)
	for len(received) < 3 {
		select {
		case got := <-ch:
			received[got.ID] = true
		case <-timeout:
			t.Fatalf("timeout; only received %d/3 items: %v", len(received), received)
		}
	}
}

func TestChanSubscriptionOf_CancelUnsubscribes(t *testing.T) {
	ns, nc, err := StartEmbedded("")
	if err != nil {
		t.Fatalf("start embedded nats: %v", err)
	}
	defer ns.Shutdown()
	defer nc.Close()

	sub := NewSubscriber(nc)

	type item struct{ ID string }
	ch, cancel := ChanSubscriptionOf[item](sub, "test.cancel.*")
	cancel() // cancel immediately

	// channel should be closed
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected channel closed after cancel")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("channel not closed after cancel")
	}
}
