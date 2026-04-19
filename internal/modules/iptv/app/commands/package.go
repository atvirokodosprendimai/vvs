package commands

import (
	"context"

	"github.com/google/uuid"
	"github.com/atvirokodosprendimai/vvs/internal/modules/iptv/domain"
)

type CreatePackageCommand struct {
	Name        string
	PriceCents  int64
	Description string
}

type CreatePackageHandler struct{ repo domain.PackageRepository }

func NewCreatePackageHandler(repo domain.PackageRepository) *CreatePackageHandler {
	return &CreatePackageHandler{repo: repo}
}

func (h *CreatePackageHandler) Handle(ctx context.Context, cmd CreatePackageCommand) (*domain.Package, error) {
	pkg := &domain.Package{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Name:        cmd.Name,
		PriceCents:  cmd.PriceCents,
		Description: cmd.Description,
	}
	if err := h.repo.Save(ctx, pkg); err != nil {
		return nil, err
	}
	return pkg, nil
}

type UpdatePackageCommand struct {
	ID          string
	Name        string
	PriceCents  int64
	Description string
}

type UpdatePackageHandler struct{ repo domain.PackageRepository }

func NewUpdatePackageHandler(repo domain.PackageRepository) *UpdatePackageHandler {
	return &UpdatePackageHandler{repo: repo}
}

func (h *UpdatePackageHandler) Handle(ctx context.Context, cmd UpdatePackageCommand) (*domain.Package, error) {
	pkg, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}
	pkg.Name = cmd.Name
	pkg.PriceCents = cmd.PriceCents
	pkg.Description = cmd.Description
	if err := h.repo.Save(ctx, pkg); err != nil {
		return nil, err
	}
	return pkg, nil
}

type DeletePackageCommand struct{ ID string }

type DeletePackageHandler struct{ repo domain.PackageRepository }

func NewDeletePackageHandler(repo domain.PackageRepository) *DeletePackageHandler {
	return &DeletePackageHandler{repo: repo}
}

func (h *DeletePackageHandler) Handle(ctx context.Context, cmd DeletePackageCommand) error {
	return h.repo.Delete(ctx, cmd.ID)
}

type AddChannelToPackageCommand struct {
	PackageID string
	ChannelID string
}

type AddChannelToPackageHandler struct{ repo domain.PackageRepository }

func NewAddChannelToPackageHandler(repo domain.PackageRepository) *AddChannelToPackageHandler {
	return &AddChannelToPackageHandler{repo: repo}
}

func (h *AddChannelToPackageHandler) Handle(ctx context.Context, cmd AddChannelToPackageCommand) error {
	return h.repo.AddChannel(ctx, cmd.PackageID, cmd.ChannelID)
}

type RemoveChannelFromPackageCommand struct {
	PackageID string
	ChannelID string
}

type RemoveChannelFromPackageHandler struct{ repo domain.PackageRepository }

func NewRemoveChannelFromPackageHandler(repo domain.PackageRepository) *RemoveChannelFromPackageHandler {
	return &RemoveChannelFromPackageHandler{repo: repo}
}

func (h *RemoveChannelFromPackageHandler) Handle(ctx context.Context, cmd RemoveChannelFromPackageCommand) error {
	return h.repo.RemoveChannel(ctx, cmd.PackageID, cmd.ChannelID)
}
