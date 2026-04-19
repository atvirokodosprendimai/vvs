package commands

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/atvirokodosprendimai/vvs/internal/modules/customer/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
)

type CreateCustomerCommand struct {
	CompanyName string
	ContactName string
	Email       string
	Phone       string
	NetworkZone string // zone for IP allocation (e.g. "Kaunas")
}

// IPAllocator is a minimal port used only for IP allocation on customer create.
// Satisfied by *services.IPAllocatorService.
type IPAllocator interface {
	AllocateIP(ctx context.Context, customerCode, zone string) (ip string, id int, err error)
}

type CreateCustomerHandler struct {
	repo      domain.CustomerRepository
	publisher events.EventPublisher
	ipam      IPAllocator // optional; nil if NetBox not configured or no prefix set
}

func NewCreateCustomerHandler(repo domain.CustomerRepository, pub events.EventPublisher, ipam IPAllocator) *CreateCustomerHandler {
	return &CreateCustomerHandler{repo: repo, publisher: pub, ipam: ipam}
}

func (h *CreateCustomerHandler) Handle(ctx context.Context, cmd CreateCustomerCommand) (*domain.Customer, error) {
	code, err := h.repo.NextCode(ctx)
	if err != nil {
		return nil, err
	}

	customer, err := domain.NewCustomer(code, cmd.CompanyName, cmd.ContactName, cmd.Email, cmd.Phone)
	if err != nil {
		return nil, err
	}
	customer.SetNetworkZone(cmd.NetworkZone)

	if err := h.repo.Save(ctx, customer); err != nil {
		return nil, err
	}

	// Auto-allocate IP from NetBox if configured (best-effort — never blocks create)
	if h.ipam != nil {
		if ip, _, err := h.ipam.AllocateIP(ctx, customer.Code.String(), customer.NetworkZone); err != nil {
			log.Printf("warn: create customer %s: netbox ip allocation: %v", customer.Code, err)
		} else if ip != "" {
			customer.SetNetworkInfo("", ip, "")
			if err := h.repo.Save(ctx, customer); err != nil {
				log.Printf("warn: create customer %s: save allocated ip: %v", customer.Code, err)
			}
		}
	}

	data, _ := json.Marshal(domainToReadModel(customer))

	h.publisher.Publish(ctx, events.CustomerCreated.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "customer.created",
		AggregateID: customer.ID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})

	return customer, nil
}
