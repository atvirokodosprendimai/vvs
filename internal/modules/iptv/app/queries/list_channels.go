package queries

import (
	"context"

	"github.com/atvirokodosprendimai/vvs/internal/modules/iptv/domain"
)

type ListChannelsHandler struct{ repo domain.ChannelRepository }

func NewListChannelsHandler(repo domain.ChannelRepository) *ListChannelsHandler {
	return &ListChannelsHandler{repo: repo}
}

func (h *ListChannelsHandler) Handle(ctx context.Context) ([]ChannelReadModel, error) {
	channels, err := h.repo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ChannelReadModel, len(channels))
	for i, ch := range channels {
		out[i] = ChannelReadModel{
			ID:        ch.ID,
			Name:      ch.Name,
			LogoURL:   ch.LogoURL,
			StreamURL: ch.StreamURL,
			Category:  ch.Category,
			EPGSource: ch.EPGSource,
			Active:    ch.Active,
			CreatedAt: ch.CreatedAt,
		}
	}
	return out, nil
}
