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
		out[i] = toChannelReadModel(ch)
	}
	return out, nil
}

// ── GetChannelHandler ─────────────────────────────────────────────────────────

type GetChannelHandler struct{ repo domain.ChannelRepository }

func NewGetChannelHandler(repo domain.ChannelRepository) *GetChannelHandler {
	return &GetChannelHandler{repo: repo}
}

func (h *GetChannelHandler) Handle(ctx context.Context, id string) (*ChannelReadModel, error) {
	ch, err := h.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	m := toChannelReadModel(ch)
	return &m, nil
}

func toChannelReadModel(ch *domain.Channel) ChannelReadModel {
	slug := ch.Slug
	if slug == "" {
		slug = domain.Slugify(ch.Name)
	}
	return ChannelReadModel{
		ID:        ch.ID,
		Name:      ch.Name,
		Slug:      slug,
		LogoURL:   ch.LogoURL,
		StreamURL: ch.StreamURL,
		Category:  ch.Category,
		EPGSource: ch.EPGSource,
		Active:    ch.Active,
		CreatedAt: ch.CreatedAt,
	}
}
