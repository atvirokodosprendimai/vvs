package commands

import (
	"context"

	"github.com/google/uuid"
	"github.com/atvirokodosprendimai/vvs/internal/modules/email/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
)

type PauseAccountHandler struct {
	repo      domain.EmailAccountRepository
	publisher events.EventPublisher
}

func NewPauseAccountHandler(repo domain.EmailAccountRepository, pub events.EventPublisher) *PauseAccountHandler {
	return &PauseAccountHandler{repo: repo, publisher: pub}
}

func (h *PauseAccountHandler) Handle(ctx context.Context, id string) error {
	a, err := h.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	a.Pause()
	if err := h.repo.Save(ctx, a); err != nil {
		return err
	}
	h.publisher.Publish(ctx, events.EmailAccountPaused.String(), events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: "email.account_paused", AggregateID: id,
	})
	return nil
}

type ResumeAccountHandler struct {
	repo      domain.EmailAccountRepository
	publisher events.EventPublisher
}

func NewResumeAccountHandler(repo domain.EmailAccountRepository, pub events.EventPublisher) *ResumeAccountHandler {
	return &ResumeAccountHandler{repo: repo, publisher: pub}
}

func (h *ResumeAccountHandler) Handle(ctx context.Context, id string) error {
	a, err := h.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	a.Resume()
	if err := h.repo.Save(ctx, a); err != nil {
		return err
	}
	h.publisher.Publish(ctx, events.EmailAccountResumed.String(), events.DomainEvent{
		ID: uuid.Must(uuid.NewV7()).String(), Type: "email.account_resumed", AggregateID: id,
	})
	return nil
}
