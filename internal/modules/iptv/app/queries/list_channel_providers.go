package queries

import (
	"context"

	"github.com/atvirokodosprendimai/vvs/internal/modules/iptv/domain"
)

type channelProviderLister interface {
	FindByChannelID(ctx context.Context, channelID string) ([]*domain.ChannelProvider, error)
}

type ListChannelProvidersHandler struct {
	repo channelProviderLister
}

func NewListChannelProvidersHandler(repo channelProviderLister) *ListChannelProvidersHandler {
	return &ListChannelProvidersHandler{repo: repo}
}

func (h *ListChannelProvidersHandler) Handle(ctx context.Context, channelID string) ([]ChannelProviderReadModel, error) {
	providers, err := h.repo.FindByChannelID(ctx, channelID)
	if err != nil {
		return nil, err
	}
	out := make([]ChannelProviderReadModel, len(providers))
	for i, p := range providers {
		out[i] = ChannelProviderReadModel{
			ID:          p.ID,
			ChannelID:   p.ChannelID,
			Name:        p.Name,
			URLTemplate: p.URLTemplate,
			Token:       p.Token,
			Type:        string(p.Type),
			Priority:    p.Priority,
			Active:      p.Active,
			CreatedAt:   p.CreatedAt,
		}
	}
	return out, nil
}
