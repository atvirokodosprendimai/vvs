package queries

import (
	"context"
	"fmt"
	"time"

	"github.com/vvs/isp/internal/modules/customer/domain"
	shareddomain "github.com/vvs/isp/internal/shared/domain"
	"gorm.io/gorm"
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
	db *gorm.DB
}

func NewListCustomersHandler(db *gorm.DB) *ListCustomersHandler {
	return &ListCustomersHandler{db: db}
}

func (h *ListCustomersHandler) Handle(_ context.Context, q ListCustomersQuery) (ListCustomersResult, error) {
	page := shareddomain.NewPagination(q.Page, q.PageSize)

	query := h.db.Table("customers")

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
		return ListCustomersResult{}, err
	}

	var models []CustomerReadModel
	if err := query.Order("created_at DESC").
		Offset(page.Offset()).Limit(page.PageSize).
		Find(&models).Error; err != nil {
		return ListCustomersResult{}, err
	}

	customers := make([]*domain.Customer, len(models))
	for i, m := range models {
		customers[i] = m.ToDomain()
	}

	return ListCustomersResult{
		Customers:  customers,
		Total:      total,
		Page:       page.Page,
		PageSize:   page.PageSize,
		TotalPages: page.TotalPages(total),
	}, nil
}

type CustomerReadModel struct {
	ID          string `gorm:"primaryKey"`
	Code        string
	CompanyName string
	ContactName string
	Email       string
	Phone       string
	Street      string
	City        string
	PostalCode  string
	Country     string
	TaxID       string
	Status      string
	Notes       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
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
	}
}

func parseCode(code string) shareddomain.CompanyCode {
	var prefix string
	var number int
	fmt.Sscanf(code, "%3s-%05d", &prefix, &number)
	return shareddomain.NewCompanyCode(prefix, number)
}
