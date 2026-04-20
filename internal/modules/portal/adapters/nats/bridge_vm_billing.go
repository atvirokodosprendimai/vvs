package nats

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/nats-io/nats.go"
	billingcommands "github.com/atvirokodosprendimai/vvs/internal/modules/billing/app/commands"
	billingdomain "github.com/atvirokodosprendimai/vvs/internal/modules/billing/domain"
	billingqueries "github.com/atvirokodosprendimai/vvs/internal/modules/billing/app/queries"
	proxmoxcommands "github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/app/commands"
	proxmoxdomain "github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/domain"
	proxmoxqueries "github.com/atvirokodosprendimai/vvs/internal/modules/proxmox/app/queries"
)

// ── New RPC subjects ──────────────────────────────────────────────────────────

const (
	SubjectVMPlansList          = "isp.portal.rpc.vm.plans.list"
	SubjectVMsList              = "isp.portal.rpc.vms.list"
	SubjectVMProvision          = "isp.portal.rpc.vm.provision"
	SubjectBalanceGet           = "isp.portal.rpc.balance.get"
	SubjectBalanceTopupComplete = "isp.portal.rpc.balance.topup"
	SubjectBalanceDeductVM      = "isp.portal.rpc.balance.deduct.vm"
)

// ── Interfaces ────────────────────────────────────────────────────────────────

type vmPlanLister interface {
	Handle(ctx context.Context) ([]proxmoxqueries.VMPlanReadModel, error)
}

// vmPlanGetter retrieves a single VM plan (for validation before provision).
type vmPlanGetter interface {
	Handle(ctx context.Context, id string) (*proxmoxqueries.VMPlanReadModel, error)
}

type vmListerForCustomer interface {
	Handle(ctx context.Context, customerID string) ([]proxmoxqueries.VMReadModel, error)
}

type vmCreator interface {
	Handle(ctx context.Context, cmd proxmoxcommands.CreateVMCommand) (*proxmoxdomain.VirtualMachine, error)
}

type balanceGetter interface {
	Handle(ctx context.Context, customerID string) (*billingqueries.BalanceReadModel, error)
}

type balanceCreditCmd interface {
	Handle(ctx context.Context, cmd billingcommands.TopUpBalanceCommand) error
}

type balanceDeductCmd interface {
	Handle(ctx context.Context, cmd billingcommands.DeductBalanceCommand) error
}

// ── Bridge extension fields + wiring ─────────────────────────────────────────

type vmBillingBridge struct {
	vmPlanLister    vmPlanLister
	vmPlanGetter    vmPlanGetter
	vmLister        vmListerForCustomer
	vmCreator       vmCreator
	balanceGetter   balanceGetter
	balanceCredit   balanceCreditCmd
	balanceDeduct   balanceDeductCmd
}

// WithVMAndBilling wires VM plan/provision and balance handlers into the bridge.
func (b *PortalBridge) WithVMAndBilling(
	planLister vmPlanLister,
	planGetter vmPlanGetter,
	vmLister vmListerForCustomer,
	vmCreate vmCreator,
	balGet balanceGetter,
	balCredit balanceCreditCmd,
	balDeduct balanceDeductCmd,
) *PortalBridge {
	b.vmb = &vmBillingBridge{
		vmPlanLister:  planLister,
		vmPlanGetter:  planGetter,
		vmLister:      vmLister,
		vmCreator:     vmCreate,
		balanceGetter: balGet,
		balanceCredit: balCredit,
		balanceDeduct: balDeduct,
	}
	return b
}

// vmBillingEntries returns the extra subscriptions when vmb is wired.
func (b *PortalBridge) vmBillingEntries() []struct {
	subject string
	handler nats.MsgHandler
} {
	if b.vmb == nil {
		return nil
	}
	return []struct {
		subject string
		handler nats.MsgHandler
	}{
		{SubjectVMPlansList, b.handleVMPlansList},
		{SubjectVMsList, b.handleVMsList},
		{SubjectVMProvision, b.handleVMProvision},
		{SubjectBalanceGet, b.handleBalanceGet},
		{SubjectBalanceTopupComplete, b.handleBalanceTopupComplete},
		{SubjectBalanceDeductVM, b.handleBalanceDeductVM},
	}
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (b *PortalBridge) handleVMPlansList(msg *nats.Msg) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	plans, err := b.vmb.vmPlanLister.Handle(ctx)
	if err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	// Filter: enabled only
	enabled := make([]proxmoxqueries.VMPlanReadModel, 0, len(plans))
	for _, p := range plans {
		if p.Enabled {
			enabled = append(enabled, p)
		}
	}
	bridgeReply(msg, enabled, nil)
}

func (b *PortalBridge) handleVMsList(msg *nats.Msg) {
	var req struct {
		CustomerID string `json:"customerID"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	vms, err := b.vmb.vmLister.Handle(ctx, req.CustomerID)
	if err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	bridgeReply(msg, vms, nil)
}

func (b *PortalBridge) handleVMProvision(msg *nats.Msg) {
	var req struct {
		CustomerID      string `json:"customerID"`
		PlanID          string `json:"planID"`
		StripeSessionID string `json:"stripeSessionID"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Validate plan exists and is enabled
	plan, err := b.vmb.vmPlanGetter.Handle(ctx, req.PlanID)
	if err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	if !plan.Enabled {
		bridgeReply(msg, nil, errors.New("plan not available"))
		return
	}

	// Create VM from plan specs
	vm, err := b.vmb.vmCreator.Handle(ctx, proxmoxcommands.CreateVMCommand{
		NodeID:       plan.NodeID,
		CustomerID:   req.CustomerID,
		Name:         "", // will be auto-named by command
		TemplateVMID: plan.TemplateVMID,
		Storage:      plan.Storage,
		Cores:        plan.Cores,
		MemoryMB:     plan.MemoryMB,
		DiskGB:       plan.DiskGB,
		FullClone:    true,
	})
	if err != nil {
		log.Printf("portal bridge: provision VM: %v", err)
		bridgeReply(msg, nil, err)
		return
	}
	bridgeReply(msg, map[string]string{"vmID": vm.ID}, nil)
}

func (b *PortalBridge) handleBalanceGet(msg *nats.Msg) {
	var req struct {
		CustomerID string `json:"customerID"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rm, err := b.vmb.balanceGetter.Handle(ctx, req.CustomerID)
	if err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	bridgeReply(msg, rm, nil)
}

func (b *PortalBridge) handleBalanceTopupComplete(msg *nats.Msg) {
	var req struct {
		CustomerID      string `json:"customerID"`
		AmountCents     int64  `json:"amountCents"`
		StripeSessionID string `json:"stripeSessionID"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := b.vmb.balanceCredit.Handle(ctx, billingcommands.TopUpBalanceCommand{
		CustomerID:      req.CustomerID,
		AmountCents:     req.AmountCents,
		Description:     "Stripe top-up",
		StripeSessionID: req.StripeSessionID,
	})
	if err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	bridgeReply(msg, map[string]bool{"ok": true}, nil)
}

func (b *PortalBridge) handleBalanceDeductVM(msg *nats.Msg) {
	var req struct {
		CustomerID string `json:"customerID"`
		PlanID     string `json:"planID"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Validate plan
	plan, err := b.vmb.vmPlanGetter.Handle(ctx, req.PlanID)
	if err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	if !plan.Enabled {
		bridgeReply(msg, nil, errors.New("plan not available"))
		return
	}

	// Deduct balance
	if err := b.vmb.balanceDeduct.Handle(ctx, billingcommands.DeductBalanceCommand{
		CustomerID:  req.CustomerID,
		AmountCents: plan.PriceMonthlyEuroCents,
		EntryType:   billingdomain.EntryTypeVMPurchase,
		Description: "VM plan: " + plan.Name,
	}); err != nil {
		bridgeReply(msg, nil, err)
		return
	}

	// Provision VM
	vm, err := b.vmb.vmCreator.Handle(ctx, proxmoxcommands.CreateVMCommand{
		NodeID:       plan.NodeID,
		CustomerID:   req.CustomerID,
		TemplateVMID: plan.TemplateVMID,
		Storage:      plan.Storage,
		Cores:        plan.Cores,
		MemoryMB:     plan.MemoryMB,
		DiskGB:       plan.DiskGB,
		FullClone:    true,
	})
	if err != nil {
		// Refund the balance — best effort
		_ = b.vmb.balanceCredit.Handle(ctx, billingcommands.TopUpBalanceCommand{
			CustomerID:  req.CustomerID,
			AmountCents: plan.PriceMonthlyEuroCents,
			Description: "Refund: VM provision failed (" + err.Error() + ")",
		})
		log.Printf("portal bridge: provision VM (balance path): %v", err)
		bridgeReply(msg, nil, err)
		return
	}
	bridgeReply(msg, map[string]string{"vmID": vm.ID}, nil)
}
