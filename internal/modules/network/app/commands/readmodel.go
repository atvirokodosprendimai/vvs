package commands

import (
	"github.com/atvirokodosprendimai/vvs/internal/modules/network/app/queries"
	"github.com/atvirokodosprendimai/vvs/internal/modules/network/domain"
)

func toReadModel(r *domain.Router) queries.RouterReadModel {
	return queries.RouterReadModel{
		ID:         r.ID,
		Name:       r.Name,
		RouterType: r.RouterType,
		Host:       r.Host,
		Port:       r.Port,
		Username:   r.Username,
		Notes:      r.Notes,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
}
