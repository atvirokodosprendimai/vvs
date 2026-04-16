package queries

import (
	"context"
	"time"

	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
	"github.com/vvs/isp/internal/shared/domain"
)

type ListDevicesQuery struct {
	Status     string
	CustomerID string
	DeviceType string
	Search     string
	Page       int
	PageSize   int
}

type ListDevicesResult struct {
	Devices    []DeviceReadModel
	Total      int64
	Page       int
	PageSize   int
	TotalPages int
}

type ListDevicesHandler struct {
	db *gormsqlite.DB
}

func NewListDevicesHandler(db *gormsqlite.DB) *ListDevicesHandler {
	return &ListDevicesHandler{db: db}
}

func (h *ListDevicesHandler) Handle(ctx context.Context, q ListDevicesQuery) (ListDevicesResult, error) {
	page := domain.NewPagination(q.Page, q.PageSize)

	var result ListDevicesResult
	err := h.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		query := tx.Table("devices")

		if q.Status != "" {
			query = query.Where("status = ?", q.Status)
		}
		if q.CustomerID != "" {
			query = query.Where("customer_id = ?", q.CustomerID)
		}
		if q.DeviceType != "" {
			query = query.Where("device_type = ?", q.DeviceType)
		}
		if q.Search != "" {
			s := "%" + q.Search + "%"
			query = query.Where("name LIKE ? OR serial_number LIKE ? OR location LIKE ?", s, s, s)
		}

		var total int64
		if err := query.Count(&total).Error; err != nil {
			return err
		}

		var models []DeviceReadModel
		if err := query.Order("created_at DESC").
			Offset(page.Offset()).Limit(page.PageSize).
			Find(&models).Error; err != nil {
			return err
		}

		result = ListDevicesResult{
			Devices:    models,
			Total:      total,
			Page:       page.Page,
			PageSize:   page.PageSize,
			TotalPages: page.TotalPages(total),
		}
		return nil
	})
	return result, err
}

// DeviceReadModel is the flat read model returned by list and get queries.
type DeviceReadModel struct {
	ID             string     `gorm:"primaryKey" json:"id"`
	Name           string     `json:"name"`
	SerialNumber   string     `json:"serial_number"`
	DeviceType     string     `json:"device_type"`
	Status         string     `json:"status"`
	CustomerID     string     `json:"customer_id"`
	Location       string     `json:"location"`
	PurchasedAt    *time.Time `json:"purchased_at"`
	WarrantyExpiry *time.Time `json:"warranty_expiry"`
	Notes          string     `json:"notes"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (DeviceReadModel) TableName() string { return "devices" }
