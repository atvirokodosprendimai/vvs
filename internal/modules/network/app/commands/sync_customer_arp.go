package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	customerdomain "github.com/vvs/isp/internal/modules/customer/domain"
	"github.com/vvs/isp/internal/modules/network/domain"
	"github.com/vvs/isp/internal/shared/events"
)

const (
	ARPActionEnable  = "enable"
	ARPActionDisable = "disable"
)

type SyncCustomerARPCommand struct {
	CustomerID string
	Action     string // ARPActionEnable | ARPActionDisable
}

// ARPChangedEvent is the payload for isp.network.arp_changed.
type ARPChangedEvent struct {
	CustomerID string `json:"customer_id"`
	IP         string `json:"ip"`
	Action     string `json:"action"` // "enable" | "disable"
	Status     string `json:"status"` // "active" | "disabled"
}

type SyncCustomerARPHandler struct {
	customers   customerdomain.CustomerRepository
	routers     domain.RouterRepository
	provisioner domain.RouterProvisioner
	ipam        domain.IPAMProvider // may be nil if not configured
	publisher   events.EventPublisher
}

func NewSyncCustomerARPHandler(
	customers customerdomain.CustomerRepository,
	routers domain.RouterRepository,
	provisioner domain.RouterProvisioner,
	ipam domain.IPAMProvider,
	publisher events.EventPublisher,
) *SyncCustomerARPHandler {
	return &SyncCustomerARPHandler{
		customers:   customers,
		routers:     routers,
		provisioner: provisioner,
		ipam:        ipam,
		publisher:   publisher,
	}
}

func (h *SyncCustomerARPHandler) Handle(ctx context.Context, cmd SyncCustomerARPCommand) error {
	customer, err := h.customers.FindByID(ctx, cmd.CustomerID)
	if err != nil {
		return fmt.Errorf("sync arp: load customer: %w", err)
	}

	if !customer.HasNetworkProvisioning() {
		return nil // no router assigned — nothing to do
	}

	// If IP unknown and IPAM configured, resolve from NetBox
	if customer.IPAddress == "" && h.ipam != nil {
		ip, mac, _, err := h.ipam.GetIPByCustomerCode(ctx, customer.Code.String())
		if err != nil {
			return fmt.Errorf("sync arp: resolve IP: %w", err)
		}
		customer.SetNetworkInfo(*customer.RouterID, ip, mac)
		if err := h.customers.Save(ctx, customer); err != nil {
			log.Printf("warn: sync arp: save customer after IP resolve: %v", err)
		}
	}

	if customer.IPAddress == "" {
		return fmt.Errorf("sync arp: customer %s has no IP address", cmd.CustomerID)
	}

	router, err := h.routers.FindByID(ctx, *customer.RouterID)
	if err != nil {
		return fmt.Errorf("sync arp: load router: %w", err)
	}

	conn := router.ToConn()

	var arpStatus string
	switch cmd.Action {
	case ARPActionEnable:
		if customer.MACAddress == "" {
			return fmt.Errorf("sync arp: customer %s has no MAC address", cmd.CustomerID)
		}
		if err := h.provisioner.SetARPStatic(ctx, conn, customer.IPAddress, customer.MACAddress, customer.ID); err != nil {
			return fmt.Errorf("sync arp: set static: %w", err)
		}
		arpStatus = "active"
	case ARPActionDisable:
		if err := h.provisioner.DisableARP(ctx, conn, customer.IPAddress); err != nil {
			return fmt.Errorf("sync arp: disable: %w", err)
		}
		arpStatus = "disabled"
	default:
		return fmt.Errorf("sync arp: unknown action %q", cmd.Action)
	}

	// Write ARP status back to NetBox (best-effort)
	if h.ipam != nil {
		_, _, ipID, err := h.ipam.GetIPByCustomerCode(ctx, customer.Code.String())
		if err == nil && ipID > 0 {
			if err := h.ipam.UpdateARPStatus(ctx, ipID, arpStatus); err != nil {
				log.Printf("warn: sync arp: update netbox arp_status: %v", err)
			}
		}
	}

	data, _ := json.Marshal(ARPChangedEvent{
		CustomerID: customer.ID,
		IP:         customer.IPAddress,
		Action:     cmd.Action,
		Status:     arpStatus,
	})
	h.publisher.Publish(ctx, "isp.network.arp_changed", events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "network.arp_changed",
		AggregateID: customer.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})

	return nil
}
