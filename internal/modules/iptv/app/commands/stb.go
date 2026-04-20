package commands

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/atvirokodosprendimai/vvs/internal/modules/iptv/domain"
)

type AssignSTBCommand struct {
	MAC        string
	Model      string
	Firmware   string
	Serial     string
	CustomerID string
	Notes      string
}

type AssignSTBHandler struct{ repo domain.STBRepository }

func NewAssignSTBHandler(repo domain.STBRepository) *AssignSTBHandler {
	return &AssignSTBHandler{repo: repo}
}

func (h *AssignSTBHandler) Handle(ctx context.Context, cmd AssignSTBCommand) (*domain.STB, error) {
	stb := &domain.STB{
		ID:         uuid.Must(uuid.NewV7()).String(),
		MAC:        strings.ToUpper(cmd.MAC),
		Model:      cmd.Model,
		Firmware:   cmd.Firmware,
		Serial:     cmd.Serial,
		CustomerID: cmd.CustomerID,
		AssignedAt: time.Now().UTC(),
		Notes:      cmd.Notes,
	}
	if err := h.repo.Save(ctx, stb); err != nil {
		return nil, err
	}
	return stb, nil
}

type UpdateSTBCommand struct {
	ID    string
	Model string
	Notes string
}

type UpdateSTBHandler struct{ repo domain.STBRepository }

func NewUpdateSTBHandler(repo domain.STBRepository) *UpdateSTBHandler {
	return &UpdateSTBHandler{repo: repo}
}

func (h *UpdateSTBHandler) Handle(ctx context.Context, cmd UpdateSTBCommand) error {
	stb, err := h.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return err
	}
	stb.Model = cmd.Model
	stb.Notes = cmd.Notes
	stb.UpdatedAt = time.Now().UTC()
	return h.repo.Save(ctx, stb)
}

type DeleteSTBCommand struct{ ID string }

type DeleteSTBHandler struct{ repo domain.STBRepository }

func NewDeleteSTBHandler(repo domain.STBRepository) *DeleteSTBHandler {
	return &DeleteSTBHandler{repo: repo}
}

func (h *DeleteSTBHandler) Handle(ctx context.Context, cmd DeleteSTBCommand) error {
	return h.repo.Delete(ctx, cmd.ID)
}
