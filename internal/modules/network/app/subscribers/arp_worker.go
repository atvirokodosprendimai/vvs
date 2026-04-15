package subscribers

import (
	"context"
	"encoding/json"
	"log"

	"github.com/vvs/isp/internal/modules/network/app/commands"
	"github.com/vvs/isp/internal/shared/events"
)

// ARPWorker subscribes to customer events and isp.network.arp_requested,
// dispatching SyncCustomerARP commands within the network module.
// It replaces the runCustomerARPSubscriber function that previously lived in app.go.
type ARPWorker struct {
	cmd *commands.SyncCustomerARPHandler
}

func NewARPWorker(cmd *commands.SyncCustomerARPHandler) *ARPWorker {
	return &ARPWorker{cmd: cmd}
}

// Run blocks until ctx is cancelled. Must be called in a goroutine.
func (w *ARPWorker) Run(ctx context.Context, sub events.EventSubscriber) {
	// Two channels: auto-sync on customer status change, manual trigger from UI
	customerCh, cancelCustomer := sub.ChanSubscription("isp.customer.*")
	defer cancelCustomer()

	arpCh, cancelARP := sub.ChanSubscription("isp.network.arp_requested")
	defer cancelARP()

	for {
		select {
		case event, ok := <-customerCh:
			if !ok {
				return
			}
			w.handleCustomerEvent(ctx, event)

		case event, ok := <-arpCh:
			if !ok {
				return
			}
			w.handleARPRequested(ctx, event)

		case <-ctx.Done():
			return
		}
	}
}

// handleCustomerEvent auto-syncs ARP when a customer's status changes.
func (w *ARPWorker) handleCustomerEvent(ctx context.Context, event events.DomainEvent) {
	var payload struct {
		ID       string  `json:"id"`
		Status   string  `json:"status"`
		RouterID *string `json:"router_id"`
	}
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		return
	}
	if payload.RouterID == nil || *payload.RouterID == "" {
		return // customer has no router — nothing to provision
	}

	action := commands.ARPActionEnable
	if payload.Status == "suspended" || payload.Status == "churned" {
		action = commands.ARPActionDisable
	}

	if err := w.cmd.Handle(ctx, commands.SyncCustomerARPCommand{
		CustomerID: payload.ID,
		Action:     action,
	}); err != nil {
		log.Printf("warn: arp auto-sync for customer %s: %v", payload.ID, err)
	}
}

// handleARPRequested handles manual ARP enable/disable requests from the customer UI.
func (w *ARPWorker) handleARPRequested(ctx context.Context, event events.DomainEvent) {
	var payload struct {
		CustomerID string `json:"customer_id"`
		Action     string `json:"action"`
	}
	if err := json.Unmarshal(event.Data, &payload); err != nil {
		return
	}

	if err := w.cmd.Handle(ctx, commands.SyncCustomerARPCommand{
		CustomerID: payload.CustomerID,
		Action:     payload.Action,
	}); err != nil {
		log.Printf("warn: arp manual sync for customer %s: %v", payload.CustomerID, err)
	}
}
