package commands

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/modules/iptv/domain"
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

type DeleteSTBCommand struct{ ID string }

type DeleteSTBHandler struct{ repo domain.STBRepository }

func NewDeleteSTBHandler(repo domain.STBRepository) *DeleteSTBHandler {
	return &DeleteSTBHandler{repo: repo}
}

func (h *DeleteSTBHandler) Handle(ctx context.Context, cmd DeleteSTBCommand) error {
	return h.repo.Delete(ctx, cmd.ID)
}
