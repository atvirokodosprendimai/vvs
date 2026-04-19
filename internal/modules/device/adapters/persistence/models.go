package persistence

import (
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/modules/device/domain"
)

// DeviceModel is the GORM model mapping to the devices table.
type DeviceModel struct {
	ID             string     `gorm:"primaryKey;type:text"`
	Name           string     `gorm:"type:text;not null"`
	SerialNumber   string     `gorm:"type:text"`
	DeviceType     string     `gorm:"type:text;not null;default:'other'"`
	Status         string     `gorm:"type:text;not null;default:'in_stock'"`
	CustomerID     string     `gorm:"type:text"`
	Location       string     `gorm:"type:text"`
	PurchasedAt    *time.Time
	WarrantyExpiry *time.Time
	Notes          string    `gorm:"type:text"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (DeviceModel) TableName() string { return "devices" }

func toModel(d *domain.Device) DeviceModel {
	return DeviceModel{
		ID:             d.ID,
		Name:           d.Name,
		SerialNumber:   d.SerialNumber,
		DeviceType:     d.DeviceType,
		Status:         d.Status,
		CustomerID:     d.CustomerID,
		Location:       d.Location,
		PurchasedAt:    d.PurchasedAt,
		WarrantyExpiry: d.WarrantyExpiry,
		Notes:          d.Notes,
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
	}
}

func (m *DeviceModel) toDomain() *domain.Device {
	return &domain.Device{
		ID:             m.ID,
		Name:           m.Name,
		SerialNumber:   m.SerialNumber,
		DeviceType:     m.DeviceType,
		Status:         m.Status,
		CustomerID:     m.CustomerID,
		Location:       m.Location,
		PurchasedAt:    m.PurchasedAt,
		WarrantyExpiry: m.WarrantyExpiry,
		Notes:          m.Notes,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}
