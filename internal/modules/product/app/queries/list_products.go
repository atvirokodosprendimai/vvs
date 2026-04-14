package queries

import (
	"context"
	"time"

	"github.com/vvs/isp/internal/modules/product/domain"
	shareddomain "github.com/vvs/isp/internal/shared/domain"
	"gorm.io/gorm"
)

type ListProductsQuery struct {
	Search   string
	Type     string
	Page     int
	PageSize int
}

type ListProductsResult struct {
	Products   []*domain.Product
	Total      int64
	Page       int
	PageSize   int
	TotalPages int
}

type ListProductsHandler struct {
	db *gorm.DB
}

func NewListProductsHandler(db *gorm.DB) *ListProductsHandler {
	return &ListProductsHandler{db: db}
}

func (h *ListProductsHandler) Handle(_ context.Context, q ListProductsQuery) (ListProductsResult, error) {
	page := shareddomain.NewPagination(q.Page, q.PageSize)

	query := h.db.Table("products")

	if q.Search != "" {
		search := "%" + q.Search + "%"
		query = query.Where("name LIKE ? OR description LIKE ?", search, search)
	}

	if q.Type != "" {
		query = query.Where("type = ?", q.Type)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return ListProductsResult{}, err
	}

	var models []ProductReadModel
	if err := query.Order("created_at DESC").
		Offset(page.Offset()).Limit(page.PageSize).
		Find(&models).Error; err != nil {
		return ListProductsResult{}, err
	}

	products := make([]*domain.Product, len(models))
	for i, m := range models {
		products[i] = m.ToDomain()
	}

	return ListProductsResult{
		Products:   products,
		Total:      total,
		Page:       page.Page,
		PageSize:   page.PageSize,
		TotalPages: page.TotalPages(total),
	}, nil
}

type ProductReadModel struct {
	ID            string `gorm:"primaryKey"`
	Name          string
	Description   string
	Type          string
	PriceAmount   int64
	PriceCurrency string
	BillingPeriod string
	IsActive      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (ProductReadModel) TableName() string { return "products" }

func (m *ProductReadModel) ToDomain() *domain.Product {
	return &domain.Product{
		ID:            m.ID,
		Name:          m.Name,
		Description:   m.Description,
		Type:          domain.ProductType(m.Type),
		Price:         shareddomain.NewMoney(m.PriceAmount, m.PriceCurrency),
		BillingPeriod: domain.BillingPeriod(m.BillingPeriod),
		IsActive:      m.IsActive,
	}
}
