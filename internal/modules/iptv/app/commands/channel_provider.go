package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/modules/iptv/domain"
	"github.com/google/uuid"
)

// ── CreateChannelProvider ─────────────────────────────────────────────────────

type CreateChannelProviderCommand struct {
	ChannelID   string
	Name        string
	URLTemplate string
	Token       string
	Type        string // "internal" | "external"
	Priority    int
}

type CreateChannelProviderHandler struct {
	repo domain.ChannelProviderRepository
}

func NewCreateChannelProviderHandler(repo domain.ChannelProviderRepository) *CreateChannelProviderHandler {
	return &CreateChannelProviderHandler{repo: repo}
}

func (h *CreateChannelProviderHandler) Handle(ctx context.Context, cmd CreateChannelProviderCommand) (*domain.ChannelProvider, error) {
	pt := domain.ProviderType(cmd.Type)
	if pt != domain.ProviderInternal && pt != domain.ProviderExternal {
		return nil, fmt.Errorf("invalid provider type: %s", cmd.Type)
	}
	now := time.Now().UTC()
	p := &domain.ChannelProvider{
		ID:          uuid.Must(uuid.NewV7()).String(),
		ChannelID:   cmd.ChannelID,
		Name:        cmd.Name,
		URLTemplate: cmd.URLTemplate,
		Token:       cmd.Token,
		Type:        pt,
		Priority:    cmd.Priority,
		Active:      true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := h.repo.Save(ctx, p); err != nil {
		return nil, fmt.Errorf("save provider: %w", err)
	}
	return p, nil
}

// ── UpdateChannelProvider ─────────────────────────────────────────────────────

type UpdateChannelProviderCommand struct {
	ID          string
	Name        string
	URLTemplate string
	Token       string
	Type        string
	Priority    int
	Active      bool
}

type UpdateChannelProviderHandler struct {
	repo domain.ChannelProviderRepository
}

func NewUpdateChannelProviderHandler(repo domain.ChannelProviderRepository) *UpdateChannelProviderHandler {
	return &UpdateChannelProviderHandler{repo: repo}
}

func (h *UpdateChannelProviderHandler) Handle(ctx context.Context, cmd UpdateChannelProviderCommand) error {
	p, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}
	pt := domain.ProviderType(cmd.Type)
	if pt != domain.ProviderInternal && pt != domain.ProviderExternal {
		return fmt.Errorf("invalid provider type: %s", cmd.Type)
	}
	p.Name = cmd.Name
	p.URLTemplate = cmd.URLTemplate
	p.Token = cmd.Token
	p.Type = pt
	p.Priority = cmd.Priority
	p.Active = cmd.Active
	p.UpdatedAt = time.Now().UTC()
	return h.repo.Save(ctx, p)
}

// ── DeleteChannelProvider ─────────────────────────────────────────────────────

type DeleteChannelProviderCommand struct{ ID string }

type DeleteChannelProviderHandler struct {
	repo domain.ChannelProviderRepository
}

func NewDeleteChannelProviderHandler(repo domain.ChannelProviderRepository) *DeleteChannelProviderHandler {
	return &DeleteChannelProviderHandler{repo: repo}
}

func (h *DeleteChannelProviderHandler) Handle(ctx context.Context, cmd DeleteChannelProviderCommand) error {
	return h.repo.Delete(ctx, cmd.ID)
}
