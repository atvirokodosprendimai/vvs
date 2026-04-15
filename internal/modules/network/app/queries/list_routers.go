package queries

import (
	"context"
	"time"

	"github.com/vvs/isp/internal/modules/network/domain"
)

// RouterReadModel is the flat read representation for SSE/HTTP responses.
type RouterReadModel struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	Username  string    `json:"username"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// Password intentionally omitted from read model
}

type ListRoutersHandler struct {
	repo domain.RouterRepository
}

func NewListRoutersHandler(repo domain.RouterRepository) *ListRoutersHandler {
	return &ListRoutersHandler{repo: repo}
}

func (h *ListRoutersHandler) Handle(ctx context.Context) ([]RouterReadModel, error) {
	routers, err := h.repo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]RouterReadModel, len(routers))
	for i, r := range routers {
		result[i] = domainToReadModel(r)
	}
	return result, nil
}

func domainToReadModel(r *domain.Router) RouterReadModel {
	return RouterReadModel{
		ID:        r.ID,
		Name:      r.Name,
		Host:      r.Host,
		Port:      r.Port,
		Username:  r.Username,
		Notes:     r.Notes,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}
