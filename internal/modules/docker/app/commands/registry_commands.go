package commands

import (
	"context"

	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/domain"
)

// ── CreateContainerRegistry ───────────────────────────────────────────────────

type CreateRegistryCommand struct {
	Name     string
	URL      string
	Username string
	Password string
}

type CreateRegistryHandler struct {
	repo domain.ContainerRegistryRepository
}

func NewCreateRegistryHandler(repo domain.ContainerRegistryRepository) *CreateRegistryHandler {
	return &CreateRegistryHandler{repo: repo}
}

func (h *CreateRegistryHandler) Handle(ctx context.Context, cmd CreateRegistryCommand) (*domain.ContainerRegistry, error) {
	reg, err := domain.NewContainerRegistry(cmd.Name, cmd.URL, cmd.Username, cmd.Password)
	if err != nil {
		return nil, err
	}
	if err := h.repo.Save(ctx, reg); err != nil {
		return nil, err
	}
	return reg, nil
}

// ── UpdateContainerRegistry ───────────────────────────────────────────────────

type UpdateRegistryCommand struct {
	ID       string
	Name     string
	URL      string
	Username string
	Password string // empty = keep existing password
}

type UpdateRegistryHandler struct {
	repo domain.ContainerRegistryRepository
}

func NewUpdateRegistryHandler(repo domain.ContainerRegistryRepository) *UpdateRegistryHandler {
	return &UpdateRegistryHandler{repo: repo}
}

func (h *UpdateRegistryHandler) Handle(ctx context.Context, cmd UpdateRegistryCommand) error {
	reg, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}
	reg.Update(cmd.Name, cmd.URL, cmd.Username, cmd.Password)
	return h.repo.Save(ctx, reg)
}

// ── DeleteContainerRegistry ───────────────────────────────────────────────────

type DeleteRegistryHandler struct {
	repo domain.ContainerRegistryRepository
}

func NewDeleteRegistryHandler(repo domain.ContainerRegistryRepository) *DeleteRegistryHandler {
	return &DeleteRegistryHandler{repo: repo}
}

func (h *DeleteRegistryHandler) Handle(ctx context.Context, id string) error {
	return h.repo.Delete(ctx, id)
}
