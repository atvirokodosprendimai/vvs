package queries

import (
	"context"
	"time"

	"github.com/vvs/isp/internal/modules/payment/domain"
	shareddomain "github.com/vvs/isp/internal/shared/domain"
	"gorm.io/gorm"
)

type ListPaymentsQuery struct {
	Search   string
	Status   string
	Page     int
	PageSize int
}

type ListPaymentsResult struct {
	Payments   []*domain.Payment
	Total      int64
	Page       int
	PageSize   int
	TotalPages int
}

type ListPaymentsHandler struct {
	db *gorm.DB
}

func NewListPaymentsHandler(db *gorm.DB) *ListPaymentsHandler {
	return &ListPaymentsHandler{db: db}
}

func (h *ListPaymentsHandler) Handle(_ context.Context, q ListPaymentsQuery) (ListPaymentsResult, error) {
	page := shareddomain.NewPagination(q.Page, q.PageSize)

	query := h.db.Table("payments")

	if q.Search != "" {
		search := "%" + q.Search + "%"
		query = query.Where("reference LIKE ? OR payer_name LIKE ? OR payer_iban LIKE ?",
			search, search, search)
	}

	if q.Status != "" {
		query = query.Where("status = ?", q.Status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return ListPaymentsResult{}, err
	}

	var models []PaymentReadModel
	if err := query.Order("booking_date DESC, created_at DESC").
		Offset(page.Offset()).Limit(page.PageSize).
		Find(&models).Error; err != nil {
		return ListPaymentsResult{}, err
	}

	payments := make([]*domain.Payment, len(models))
	for i, m := range models {
		payments[i] = m.ToDomain()
	}

	return ListPaymentsResult{
		Payments:   payments,
		Total:      total,
		Page:       page.Page,
		PageSize:   page.PageSize,
		TotalPages: page.TotalPages(total),
	}, nil
}

type PaymentReadModel struct {
	ID             string  `gorm:"primaryKey"`
	AmountCents    int64
	AmountCurrency string
	Reference      string
	PayerName      string
	PayerIBAN      string
	BookingDate    time.Time
	InvoiceID      *string
	CustomerID     *string
	Status         string
	ImportBatchID  string
	CreatedAt      time.Time
}

func (PaymentReadModel) TableName() string { return "payments" }

func (m *PaymentReadModel) ToDomain() *domain.Payment {
	return &domain.Payment{
		ID:            m.ID,
		Amount:        shareddomain.NewMoney(m.AmountCents, m.AmountCurrency),
		Reference:     m.Reference,
		PayerName:     m.PayerName,
		PayerIBAN:     m.PayerIBAN,
		BookingDate:   m.BookingDate,
		InvoiceID:     m.InvoiceID,
		CustomerID:    m.CustomerID,
		Status:        domain.PaymentStatus(m.Status),
		ImportBatchID: m.ImportBatchID,
		CreatedAt:     m.CreatedAt,
	}
}
