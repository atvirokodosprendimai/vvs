package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/atvirokodosprendimai/vvs/internal/modules/deal/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
)

// AdvanceDealCommand advances a deal stage using one of:
// qualify / propose / negotiate / win / lose
type AdvanceDealCommand struct {
	ID     string
	Action string // qualify | propose | negotiate | win | lose
}

type AdvanceDealHandler struct {
	repo      domain.DealRepository
	publisher events.EventPublisher
}

func NewAdvanceDealHandler(repo domain.DealRepository, pub events.EventPublisher) *AdvanceDealHandler {
	return &AdvanceDealHandler{repo: repo, publisher: pub}
}

func (h *AdvanceDealHandler) Handle(ctx context.Context, cmd AdvanceDealCommand) error {
	deal, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}

	switch cmd.Action {
	case "qualify":
		err = deal.Qualify()
	case "propose":
		err = deal.Propose()
	case "negotiate":
		err = deal.Negotiate()
	case "win":
		err = deal.Win()
	case "lose":
		err = deal.Lose()
	default:
		return fmt.Errorf("unknown action %q: must be qualify, propose, negotiate, win, or lose", cmd.Action)
	}
	if err != nil {
		return err
	}

	if err := h.repo.Save(ctx, deal); err != nil {
		return err
	}
	h.publisher.Publish(ctx, events.DealAdvanced.String(), events.DomainEvent{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Type:        "deal.advanced",
		AggregateID: deal.ID,
		OccurredAt:  time.Now().UTC(),
	})
	return nil
}
