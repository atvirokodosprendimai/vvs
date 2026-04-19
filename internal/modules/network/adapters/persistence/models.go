package persistence

import (
	"time"

	"github.com/vvs/isp/internal/modules/network/domain"
)

type RouterModel struct {
	ID          string `gorm:"primaryKey;type:text"`
	Name        string `gorm:"type:text;not null"`
	RouterType  string `gorm:"type:text;not null;default:mikrotik"`
	Host        string `gorm:"type:text;not null"`
	Port        int    `gorm:"not null;default:8728"`
	Username    string `gorm:"type:text"`
	Password    string `gorm:"type:text"`
	PasswordEnc []byte `gorm:"column:password_enc"`
	Notes       string `gorm:"type:text"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (RouterModel) TableName() string { return "routers" }

func toModel(r *domain.Router) *RouterModel {
	return &RouterModel{
		ID:         r.ID,
		Name:       r.Name,
		RouterType: r.RouterType,
		Host:       r.Host,
		Port:       r.Port,
		Username:   r.Username,
		Password:   r.Password,
		Notes:      r.Notes,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
}

func toDomain(m *RouterModel) *domain.Router {
	return &domain.Router{
		ID:         m.ID,
		Name:       m.Name,
		RouterType: m.RouterType,
		Host:       m.Host,
		Port:       m.Port,
		Username:   m.Username,
		Password:   m.Password,
		Notes:      m.Notes,
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
	}
}
