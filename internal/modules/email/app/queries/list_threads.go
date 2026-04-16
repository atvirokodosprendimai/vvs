package queries

import (
	"context"
	"strings"

	"github.com/vvs/isp/internal/modules/email/domain"
)

type ListThreadsQuery struct {
	AccountID string // empty = all accounts
	TagFilter string // tag name to filter by; empty = no filter
}

type ListThreadsHandler struct {
	threads domain.EmailThreadRepository
	tags    domain.EmailTagRepository
}

func NewListThreadsHandler(threads domain.EmailThreadRepository, tags domain.EmailTagRepository) *ListThreadsHandler {
	return &ListThreadsHandler{threads: threads, tags: tags}
}

func (h *ListThreadsHandler) Handle(ctx context.Context, q ListThreadsQuery) ([]ThreadReadModel, error) {
	var threadList []*domain.EmailThread
	var err error

	if q.AccountID != "" {
		threadList, err = h.threads.ListForAccount(ctx, q.AccountID)
	} else {
		// No cross-account list method yet — return empty for now.
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	out := make([]ThreadReadModel, 0, len(threadList))
	for _, t := range threadList {
		tags, _ := h.tags.ListForThread(ctx, t.ID)
		trm := threadToReadModel(t, tags)

		if q.TagFilter != "" && !hasTagByName(trm.Tags, q.TagFilter) {
			continue
		}
		out = append(out, trm)
	}
	return out, nil
}

// ListThreadsForCustomerHandler returns threads linked to a customer.
type ListThreadsForCustomerHandler struct {
	threads domain.EmailThreadRepository
	tags    domain.EmailTagRepository
}

func NewListThreadsForCustomerHandler(threads domain.EmailThreadRepository, tags domain.EmailTagRepository) *ListThreadsForCustomerHandler {
	return &ListThreadsForCustomerHandler{threads: threads, tags: tags}
}

func (h *ListThreadsForCustomerHandler) Handle(ctx context.Context, customerID string) ([]ThreadReadModel, error) {
	threadList, err := h.threads.ListForCustomer(ctx, customerID)
	if err != nil {
		return nil, err
	}
	out := make([]ThreadReadModel, len(threadList))
	for i, t := range threadList {
		tags, _ := h.tags.ListForThread(ctx, t.ID)
		out[i] = threadToReadModel(t, tags)
	}
	return out, nil
}

func threadToReadModel(t *domain.EmailThread, tags []*domain.EmailTag) ThreadReadModel {
	trm := ThreadReadModel{
		ID:                   t.ID,
		AccountID:            t.AccountID,
		Subject:              t.Subject,
		ParticipantAddresses: t.ParticipantAddresses,
		CustomerID:           t.CustomerID,
		MessageCount:         t.MessageCount,
		LastMessageAt:        t.LastMessageAt,
		Tags:                 make([]TagReadModel, len(tags)),
	}
	for i, tag := range tags {
		trm.Tags[i] = TagReadModel{ID: tag.ID, Name: tag.Name, Color: tag.Color, System: tag.System}
		if tag.Name == domain.TagUnread {
			trm.Unread = true
		}
	}
	return trm
}

func hasTagByName(tags []TagReadModel, name string) bool {
	for _, t := range tags {
		if strings.EqualFold(t.Name, name) {
			return true
		}
	}
	return false
}
