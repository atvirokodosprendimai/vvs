package notifications

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
)

// Worker subscribes to all ISP domain events (isp.>) and creates notification
// rows for events worth surfacing. After each insert it publishes
// isp.notifications to wake connected SSE clients.
type Worker struct {
	store     *Store
	publisher events.EventPublisher
}

// NewWorker creates a Worker.
func NewWorker(store *Store, pub events.EventPublisher) *Worker {
	return &Worker{store: store, publisher: pub}
}

// Run blocks until ctx is cancelled. Call it in a goroutine.
func (w *Worker) Run(ctx context.Context, sub events.EventSubscriber) {
	ch, cancel := sub.ChanSubscription(events.Everything.String())
	defer cancel()

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return
			}
			w.handle(ctx, event)
		case <-ctx.Done():
			return
		}
	}
}

func (w *Worker) handle(ctx context.Context, event events.DomainEvent) {
	title, url := titleFor(event)
	if title == "" {
		return // event not worth notifying
	}

	id := uuid.Must(uuid.NewV7()).String()
	if err := w.store.Create(ctx, id, title, url); err != nil {
		log.Printf("warn: notifications: create: %v", err)
		return
	}

	// Wake all SSE clients so they re-query their unread state.
	data, _ := json.Marshal(map[string]string{"id": id})
	w.publisher.Publish(ctx, events.Notifications.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "notification.created",
		AggregateID: id,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})
}

// titleFor maps a DomainEvent to a human-readable notification title and an
// optional URL. Returns ("", "") to skip events that need no notification.
func titleFor(event events.DomainEvent) (title, url string) {
	switch event.Type {
	case "customer.created":
		var p struct {
			CompanyName string `json:"company_name"`
			Code        string `json:"code"`
		}
		if err := json.Unmarshal(event.Data, &p); err == nil && p.CompanyName != "" {
			return "New customer: " + p.CompanyName, "/customers/" + event.AggregateID
		}
		return "New customer created", "/customers/" + event.AggregateID

	case "customer.deleted":
		var p struct{ Code string `json:"code"` }
		if err := json.Unmarshal(event.Data, &p); err == nil && p.Code != "" {
			return "Customer " + p.Code + " deleted", ""
		}
		return "Customer deleted", ""

	case "network.arp_changed":
		var p struct {
			IP     string `json:"ip"`
			Action string `json:"action"`
			Status string `json:"status"`
		}
		if err := json.Unmarshal(event.Data, &p); err == nil && p.IP != "" {
			label := "enabled"
			if p.Status == "disabled" {
				label = "disabled"
			}
			return "ARP " + label + ": " + p.IP, ""
		}

	case "network.router.created":
		var p struct{ Name string `json:"name"` }
		if err := json.Unmarshal(event.Data, &p); err == nil && p.Name != "" {
			return "New router: " + p.Name, "/routers/" + event.AggregateID
		}

	case "network.router.deleted":
		var p struct{ Name string `json:"name"` }
		if err := json.Unmarshal(event.Data, &p); err == nil && p.Name != "" {
			return "Router " + p.Name + " deleted", ""
		}
	}
	return "", ""
}
