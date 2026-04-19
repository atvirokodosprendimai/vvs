package queries

import (
	"context"

	"github.com/vvs/isp/internal/modules/iptv/domain"
)

type ListPackagesHandler struct{ repo domain.PackageRepository }

func NewListPackagesHandler(repo domain.PackageRepository) *ListPackagesHandler {
	return &ListPackagesHandler{repo: repo}
}

func (h *ListPackagesHandler) Handle(ctx context.Context) ([]PackageReadModel, error) {
	packages, err := h.repo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PackageReadModel, len(packages))
	for i, p := range packages {
		channelIDs, _ := h.repo.ListChannelIDs(ctx, p.ID)
		out[i] = PackageReadModel{
			ID:          p.ID,
			Name:        p.Name,
			PriceCents:  p.PriceCents,
			Description: p.Description,
			ChannelCount: len(channelIDs),
			CreatedAt:   p.CreatedAt,
		}
	}
	return out, nil
}

type GetPackageChannelsHandler struct {
	pkgRepo     domain.PackageRepository
	channelRepo domain.ChannelRepository
}

func NewGetPackageChannelsHandler(pkgRepo domain.PackageRepository, channelRepo domain.ChannelRepository) *GetPackageChannelsHandler {
	return &GetPackageChannelsHandler{pkgRepo: pkgRepo, channelRepo: channelRepo}
}

func (h *GetPackageChannelsHandler) Handle(ctx context.Context, packageID string) ([]ChannelReadModel, error) {
	channels, err := h.channelRepo.FindByPackage(ctx, packageID)
	if err != nil {
		return nil, err
	}
	out := make([]ChannelReadModel, len(channels))
	for i, ch := range channels {
		out[i] = ChannelReadModel{
			ID: ch.ID, Name: ch.Name, LogoURL: ch.LogoURL,
			StreamURL: ch.StreamURL, Category: ch.Category,
			EPGSource: ch.EPGSource, Active: ch.Active, CreatedAt: ch.CreatedAt,
		}
	}
	return out, nil
}
