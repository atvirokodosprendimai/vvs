package queries

import (
	"context"
	"strings"

	"github.com/vvs/isp/internal/modules/email/domain"
)

type ListThreadsQuery struct {
	AccountID    string // empty = all accounts
	TagFilter    string // tag name to filter by; empty = no filter
	Search       string // filter by subject or participant address (case-insensitive)
	FolderFilter string // IMAP folder name to filter by; empty = no filter
	Page         int    // 0-based page number
	PageSize     int    // 0 → DefaultPageSize
}

type ListThreadsHandler struct {
	threads domain.EmailThreadRepository
	tags    domain.EmailTagRepository
	folders domain.EmailFolderRepository // optional; enables FolderFilter
}

func NewListThreadsHandler(threads domain.EmailThreadRepository, tags domain.EmailTagRepository) *ListThreadsHandler {
	return &ListThreadsHandler{threads: threads, tags: tags}
}

// WithFolderRepo attaches a folder repo so FolderFilter is applied.
func (h *ListThreadsHandler) WithFolderRepo(folders domain.EmailFolderRepository) *ListThreadsHandler {
	h.folders = folders
	return h
}

func (h *ListThreadsHandler) Handle(ctx context.Context, q ListThreadsQuery) (ThreadListResult, error) {
	pageSize := q.PageSize
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}

	var threadList []*domain.EmailThread
	var err error
	if q.AccountID != "" {
		threadList, err = h.threads.ListForAccount(ctx, q.AccountID)
	} else {
		threadList, err = h.threads.ListAll(ctx)
	}
	if err != nil {
		return ThreadListResult{}, err
	}

	search := strings.ToLower(strings.TrimSpace(q.Search))

	// Build folder-thread set if FolderFilter is active.
	var folderThreadSet map[string]struct{}
	if q.FolderFilter != "" && q.AccountID != "" && h.folders != nil {
		ids, _ := h.folders.ListThreadIDsWithFolder(ctx, q.AccountID, q.FolderFilter)
		folderThreadSet = make(map[string]struct{}, len(ids))
		for _, id := range ids {
			folderThreadSet[id] = struct{}{}
		}
	}

	all := make([]ThreadReadModel, 0, len(threadList))
	for _, t := range threadList {
		if folderThreadSet != nil {
			if _, ok := folderThreadSet[t.ID]; !ok {
				continue
			}
		}
		tags, _ := h.tags.ListForThread(ctx, t.ID)
		trm := threadToReadModel(t, tags)
		if q.TagFilter != "" && !hasTagByName(trm.Tags, q.TagFilter) {
			continue
		}
		if search != "" &&
			!strings.Contains(strings.ToLower(trm.Subject), search) &&
			!strings.Contains(strings.ToLower(trm.ParticipantAddresses), search) {
			continue
		}
		all = append(all, trm)
	}

	total := len(all)
	page := q.Page
	start := page * pageSize
	if start >= total {
		start = 0
		page = 0
	}
	end := min(start+pageSize, total)

	return ThreadListResult{
		Threads:  all[start:end],
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
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
