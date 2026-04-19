package queries

import (
	"context"

	"github.com/atvirokodosprendimai/vvs/internal/modules/email/domain"
)

type ListFoldersHandler struct {
	folders domain.EmailFolderRepository
}

func NewListFoldersHandler(folders domain.EmailFolderRepository) *ListFoldersHandler {
	return &ListFoldersHandler{folders: folders}
}

func (h *ListFoldersHandler) Handle(ctx context.Context, accountID string) ([]FolderReadModel, error) {
	list, err := h.folders.ListForAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]FolderReadModel, len(list))
	for i, f := range list {
		out[i] = FolderReadModel{
			ID:      f.ID,
			Name:    f.Name,
			LastUID: f.LastUID,
			Enabled: f.Enabled,
		}
	}
	return out, nil
}
