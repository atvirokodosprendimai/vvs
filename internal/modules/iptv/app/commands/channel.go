package commands

import (
	"context"

	"github.com/google/uuid"
	"github.com/atvirokodosprendimai/vvs/internal/modules/iptv/domain"
)

// ── Create ────────────────────────────────────────────────────────────────────

type CreateChannelCommand struct {
	Name      string
	LogoURL   string
	StreamURL string
	Category  string
	EPGSource string
}

type CreateChannelHandler struct{ repo domain.ChannelRepository }

func NewCreateChannelHandler(repo domain.ChannelRepository) *CreateChannelHandler {
	return &CreateChannelHandler{repo: repo}
}

func (h *CreateChannelHandler) Handle(ctx context.Context, cmd CreateChannelCommand) (*domain.Channel, error) {
	ch := &domain.Channel{
		ID:        uuid.Must(uuid.NewV7()).String(),
		Name:      cmd.Name,
		LogoURL:   cmd.LogoURL,
		StreamURL: cmd.StreamURL,
		Category:  cmd.Category,
		EPGSource: cmd.EPGSource,
		Active:    true,
	}
	if err := h.repo.Save(ctx, ch); err != nil {
		return nil, err
	}
	return ch, nil
}

// ── Update ────────────────────────────────────────────────────────────────────

type UpdateChannelCommand struct {
	ID        string
	Name      string
	LogoURL   string
	StreamURL string
	Category  string
	EPGSource string
	Active    bool
}

type UpdateChannelHandler struct{ repo domain.ChannelRepository }

func NewUpdateChannelHandler(repo domain.ChannelRepository) *UpdateChannelHandler {
	return &UpdateChannelHandler{repo: repo}
}

func (h *UpdateChannelHandler) Handle(ctx context.Context, cmd UpdateChannelCommand) (*domain.Channel, error) {
	ch, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}
	ch.Name = cmd.Name
	ch.LogoURL = cmd.LogoURL
	ch.StreamURL = cmd.StreamURL
	ch.Category = cmd.Category
	ch.EPGSource = cmd.EPGSource
	ch.Active = cmd.Active
	if err := h.repo.Save(ctx, ch); err != nil {
		return nil, err
	}
	return ch, nil
}

// ── Delete ────────────────────────────────────────────────────────────────────

type DeleteChannelCommand struct{ ID string }

type DeleteChannelHandler struct{ repo domain.ChannelRepository }

func NewDeleteChannelHandler(repo domain.ChannelRepository) *DeleteChannelHandler {
	return &DeleteChannelHandler{repo: repo}
}

func (h *DeleteChannelHandler) Handle(ctx context.Context, cmd DeleteChannelCommand) error {
	return h.repo.Delete(ctx, cmd.ID)
}
