package queries

import (
	"context"
	"fmt"
	"time"

	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
	"github.com/vvs/isp/internal/modules/customer/domain"
	shareddomain "github.com/vvs/isp/internal/shared/domain"
)

type ListCustomersQuery struct {
	Search   string
	Status   string
	Page     int
	PageSize int
}

type ListCustomersResult struct {
	Customers  []*domain.Customer
	Total      int64
	Page       int
	PageSize   int
	TotalPages int
}

type ListCustomersHandler struct {
	db *gormsqlite.DB
}

func NewListCustomersHandler(db *gormsqlite.DB) *ListCustomersHandler {
	return &ListCustomersHandler{db: db}
}

func (h *ListCustomersHandler) Handle(ctx context.Context, q ListCustomersQuery) (ListCustomersResult, error) {
	page := shareddomain.NewPagination(q.Page, q.PageSize)

	var result ListCustomersResult
	err := h.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		query := tx.Table("customers")

		if q.Search != "" {
			search := "%" + q.Search + "%"
			query = query.Where("company_name LIKE ? OR code LIKE ? OR email LIKE ? OR contact_name LIKE ?",
				search, search, search, search)
		}
		if q.Status != "" {
			query = query.Where("status = ?", q.Status)
		}

		var total int64
		if err := query.Count(&total).Error; err != nil {
			return err
		}

		var models []CustomerReadModel
		if err := query.Order("created_at DESC").
			Offset(page.Offset()).Limit(page.PageSize).
			Find(&models).Error; err != nil {
			return err
		}

		customers := make([]*domain.Customer, len(models))
		for i, m := range models {
			customers[i] = m.ToDomain()
		}

		result = ListCustomersResult{
			Customers:  customers,
			Total:      total,
			Page:       page.Page,
			PageSize:   page.PageSize,
			TotalPages: page.TotalPages(total),
		}
		return nil
	})
	return result, err
}

type CustomerReadModel struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	Code        string    `json:"code"`
	CompanyName string    `json:"company_name"`
	ContactName string    `json:"contact_name"`
	Email       string    `json:"email"`
	Phone       string    `json:"phone"`
	Street      string    `json:"street"`
	City        string    `json:"city"`
	PostalCode  string    `json:"postal_code"`
	Country     string    `json:"country"`
	TaxID       string    `json:"tax_id"`
	Status      string    `json:"status"`
	Notes       string    `json:"notes"`
	RouterID    *string   `json:"router_id"`
	IPAddress   string    `json:"ip_address"`
	MACAddress  string    `json:"mac_address"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (CustomerReadModel) TableName() string { return "customers" }

func (m *CustomerReadModel) ToDomain() *domain.Customer {
	parts := parseCode(m.Code)
	return &domain.Customer{
		ID:          m.ID,
		Code:        parts,
		CompanyName: m.CompanyName,
		ContactName: m.ContactName,
		Email:       m.Email,
		Phone:       m.Phone,
		Street:      m.Street,
		City:        m.City,
		PostalCode:  m.PostalCode,
		Country:     m.Country,
		TaxID:       m.TaxID,
		Status:      domain.CustomerStatus(m.Status),
		Notes:       m.Notes,
		RouterID:    m.RouterID,
		IPAddress:   m.IPAddress,
		MACAddress:  m.MACAddress,
	}
}

func parseCode(code string) shareddomain.CompanyCode {
	var prefix string
	var number int
	fmt.Sscanf(code, "%3s-%05d", &prefix, &number)
	return shareddomain.NewCompanyCode(prefix, number)
}
