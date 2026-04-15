package queries

import (
	"context"
	"time"

	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
	"github.com/vvs/isp/internal/modules/product/domain"
	shareddomain "github.com/vvs/isp/internal/shared/domain"
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
	db *gormsqlite.DB
}

func NewListProductsHandler(db *gormsqlite.DB) *ListProductsHandler {
	return &ListProductsHandler{db: db}
}

func (h *ListProductsHandler) Handle(ctx context.Context, q ListProductsQuery) (ListProductsResult, error) {
	page := shareddomain.NewPagination(q.Page, q.PageSize)

	var result ListProductsResult
	err := h.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		query := tx.Table("products")

		if q.Search != "" {
			search := "%" + q.Search + "%"
			query = query.Where("name LIKE ? OR description LIKE ?", search, search)
		}
		if q.Type != "" {
			query = query.Where("type = ?", q.Type)
		}

		var total int64
		if err := query.Count(&total).Error; err != nil {
			return err
		}

		var models []ProductReadModel
		if err := query.Order("created_at DESC").
			Offset(page.Offset()).Limit(page.PageSize).
			Find(&models).Error; err != nil {
			return err
		}

		products := make([]*domain.Product, len(models))
		for i, m := range models {
			products[i] = m.ToDomain()
		}

		result = ListProductsResult{
			Products:   products,
			Total:      total,
			Page:       page.Page,
			PageSize:   page.PageSize,
			TotalPages: page.TotalPages(total),
		}
		return nil
	})
	return result, err
}

type ProductReadModel struct {
	ID            string    `gorm:"primaryKey" json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Type          string    `json:"type"`
	PriceAmount   int64     `json:"price_amount"`
	PriceCurrency string    `json:"price_currency"`
	BillingPeriod string    `json:"billing_period"`
	IsActive      bool      `json:"is_active"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
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
