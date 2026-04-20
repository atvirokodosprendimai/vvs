package queries

import (
	"context"

	"github.com/atvirokodosprendimai/vvs/internal/modules/iptv/domain"
)

type iptvStackLister interface {
	FindAll(ctx context.Context) ([]*domain.IPTVStack, error)
}

type iptvStackChannelLister interface {
	FindByStackID(ctx context.Context, stackID string) ([]*domain.IPTVStackChannel, error)
}

type iptvStackChannelDetailReader interface {
	FindByID(ctx context.Context, id string) (*domain.Channel, error)
}

type iptvProviderDetailReader interface {
	FindByID(ctx context.Context, id string) (*domain.ChannelProvider, error)
}

// ── ListIPTVStacksHandler ─────────────────────────────────────────────────────

type ListIPTVStacksHandler struct {
	stackRepo   iptvStackLister
	channelRepo iptvStackChannelLister
}

func NewListIPTVStacksHandler(stackRepo iptvStackLister, channelRepo iptvStackChannelLister) *ListIPTVStacksHandler {
	return &ListIPTVStacksHandler{stackRepo: stackRepo, channelRepo: channelRepo}
}

func (h *ListIPTVStacksHandler) Handle(ctx context.Context) ([]IPTVStackReadModel, error) {
	stacks, err := h.stackRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]IPTVStackReadModel, len(stacks))
	for i, s := range stacks {
		chs, _ := h.channelRepo.FindByStackID(ctx, s.ID)
		out[i] = IPTVStackReadModel{
			ID:                 s.ID,
			Name:               s.Name,
			ClusterID:          s.ClusterID,
			NodeID:             s.NodeID,
			WANNetworkID:       s.WANNetworkID,
			OverlayNetworkID:   s.OverlayNetworkID,
			WANNetworkName:     s.WANNetworkName,
			OverlayNetworkName: s.OverlayNetworkName,
			WanIP:              s.WanIP,
			WANInterface:       s.WANInterface,
			Status:             string(s.Status),
			LastDeployedAt:     s.LastDeployedAt,
			ChannelCount:       len(chs),
			CreatedAt:          s.CreatedAt,
		}
	}
	return out, nil
}

// ── GetIPTVStackChannelsHandler ───────────────────────────────────────────────

type GetIPTVStackChannelsHandler struct {
	stackChannelRepo iptvStackChannelLister
	channelRepo      iptvStackChannelDetailReader
	providerRepo     iptvProviderDetailReader
}

func NewGetIPTVStackChannelsHandler(
	stackChannelRepo iptvStackChannelLister,
	channelRepo iptvStackChannelDetailReader,
	providerRepo iptvProviderDetailReader,
) *GetIPTVStackChannelsHandler {
	return &GetIPTVStackChannelsHandler{
		stackChannelRepo: stackChannelRepo,
		channelRepo:      channelRepo,
		providerRepo:     providerRepo,
	}
}

func (h *GetIPTVStackChannelsHandler) Handle(ctx context.Context, stackID string) ([]IPTVStackChannelReadModel, error) {
	assignments, err := h.stackChannelRepo.FindByStackID(ctx, stackID)
	if err != nil {
		return nil, err
	}
	out := make([]IPTVStackChannelReadModel, 0, len(assignments))
	for _, sc := range assignments {
		ch, err := h.channelRepo.FindByID(ctx, sc.ChannelID)
		if err != nil {
			continue
		}
		slug := ch.Slug
		if slug == "" {
			slug = domain.Slugify(ch.Name)
		}
		rm := IPTVStackChannelReadModel{
			ID:          sc.ID,
			StackID:     sc.StackID,
			ChannelID:   sc.ChannelID,
			ChannelName: ch.Name,
			ChannelSlug: slug,
		}
		if sc.ProviderID != "" {
			if p, err := h.providerRepo.FindByID(ctx, sc.ProviderID); err == nil {
				rm.ProviderID   = p.ID
				rm.ProviderName = p.Name
				rm.ProviderType = string(p.Type)
			}
		}
		out = append(out, rm)
	}
	return out, nil
}
