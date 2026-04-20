package commands

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/atvirokodosprendimai/vvs/internal/modules/billing/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
)

// ── TopUpBalance ──────────────────────────────────────────────────────────────

type TopUpBalanceCommand struct {
	CustomerID      string
	AmountCents     int64
	Description     string
	StripeSessionID string // empty for manual admin top-ups
}

type TopUpBalanceHandler struct {
	balanceRepo domain.BalanceRepository
	publisher   events.EventPublisher
}

func NewTopUpBalanceHandler(balanceRepo domain.BalanceRepository, pub events.EventPublisher) *TopUpBalanceHandler {
	return &TopUpBalanceHandler{balanceRepo: balanceRepo, publisher: pub}
}

func (h *TopUpBalanceHandler) Handle(ctx context.Context, cmd TopUpBalanceCommand) error {
	desc := cmd.Description
	if desc == "" {
		desc = "Balance top-up"
	}
	if err := h.balanceRepo.Credit(ctx, cmd.CustomerID, cmd.AmountCents, domain.EntryTypeTopUp, desc, cmd.StripeSessionID); err != nil {
		return err
	}
	data, _ := json.Marshal(map[string]any{
		"customerId":  cmd.CustomerID,
		"amountCents": cmd.AmountCents,
	})
	h.publisher.Publish(ctx, events.BillingBalanceCredited.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "billing.balance.credited",
		AggregateID: cmd.CustomerID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})
	return nil
}

// ── DeductBalance ─────────────────────────────────────────────────────────────

type DeductBalanceCommand struct {
	CustomerID  string
	AmountCents int64
	EntryType   domain.EntryType
	Description string
}

type DeductBalanceHandler struct {
	balanceRepo domain.BalanceRepository
	publisher   events.EventPublisher
}

func NewDeductBalanceHandler(balanceRepo domain.BalanceRepository, pub events.EventPublisher) *DeductBalanceHandler {
	return &DeductBalanceHandler{balanceRepo: balanceRepo, publisher: pub}
}

func (h *DeductBalanceHandler) Handle(ctx context.Context, cmd DeductBalanceCommand) error {
	entryType := cmd.EntryType
	if entryType == "" {
		entryType = domain.EntryTypeVMPurchase
	}
	if err := h.balanceRepo.Deduct(ctx, cmd.CustomerID, cmd.AmountCents, entryType, cmd.Description); err != nil {
		return err
	}
	data, _ := json.Marshal(map[string]any{
		"customerId":  cmd.CustomerID,
		"amountCents": cmd.AmountCents,
	})
	h.publisher.Publish(ctx, events.BillingBalanceDebited.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "billing.balance.debited",
		AggregateID: cmd.CustomerID,
		OccurredAt:  time.Now().UTC(),
		Data:        data,
	})
	return nil
}

// ── AdjustBalance (admin manual credit/debit) ─────────────────────────────────

type AdjustBalanceCommand struct {
	CustomerID  string
	AmountCents int64 // positive = credit, negative = deduct
	Description string
}

type AdjustBalanceHandler struct {
	topUp  *TopUpBalanceHandler
	deduct *DeductBalanceHandler
}

func NewAdjustBalanceHandler(topUp *TopUpBalanceHandler, deduct *DeductBalanceHandler) *AdjustBalanceHandler {
	return &AdjustBalanceHandler{topUp: topUp, deduct: deduct}
}

func (h *AdjustBalanceHandler) Handle(ctx context.Context, cmd AdjustBalanceCommand) error {
	if cmd.AmountCents >= 0 {
		return h.topUp.Handle(ctx, TopUpBalanceCommand{
			CustomerID:  cmd.CustomerID,
			AmountCents: cmd.AmountCents,
			Description: cmd.Description,
		})
	}
	return h.deduct.Handle(ctx, DeductBalanceCommand{
		CustomerID:  cmd.CustomerID,
		AmountCents: -cmd.AmountCents,
		EntryType:   domain.EntryTypeAdjustment,
		Description: cmd.Description,
	})
}
