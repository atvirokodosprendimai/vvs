package queries

import (
	"context"

	"github.com/vvs/isp/internal/modules/email/domain"
)

type AccountLister interface {
	List(ctx context.Context) ([]*domain.EmailAccount, error)
}

type ListAccountsHandler struct {
	repo AccountLister
}

func NewListAccountsHandler(repo AccountLister) *ListAccountsHandler {
	return &ListAccountsHandler{repo: repo}
}

func (h *ListAccountsHandler) Handle(ctx context.Context) ([]AccountReadModel, error) {
	accounts, err := h.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]AccountReadModel, len(accounts))
	for i, a := range accounts {
		out[i] = AccountReadModel{
			ID:         a.ID,
			Name:       a.Name,
			Host:       a.Host,
			Port:       a.Port,
			Username:   a.Username,
			TLS:        a.TLS,
			Folder:     a.Folder,
			Status:     a.Status,
			LastError:  a.LastError,
			LastSyncAt: a.LastSyncAt,
			SMTPHost:   a.SMTPHost,
			SMTPPort:   a.SMTPPort,
			SMTPTLS:    a.SMTPTLS,
		}
	}
	return out, nil
}
