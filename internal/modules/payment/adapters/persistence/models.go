package persistence

import (
	"time"

	"github.com/vvs/isp/internal/modules/payment/domain"
	shareddomain "github.com/vvs/isp/internal/shared/domain"
)

type PaymentModel struct {
	ID             string     `gorm:"primaryKey;type:text"`
	AmountCents    int64      `gorm:"column:amount_cents;not null;default:0"`
	AmountCurrency string     `gorm:"column:amount_currency;type:text;not null;default:'EUR'"`
	Reference      string     `gorm:"type:text"`
	PayerName      string     `gorm:"type:text"`
	PayerIBAN      string     `gorm:"type:text"`
	BookingDate    time.Time  `gorm:"not null"`
	InvoiceID      *string    `gorm:"type:text"`
	CustomerID     *string    `gorm:"type:text"`
	Status         string     `gorm:"type:text;not null;default:'imported'"`
	ImportBatchID  string     `gorm:"type:text"`
	CreatedAt      time.Time
}

func (PaymentModel) TableName() string { return "payments" }

func toModel(p *domain.Payment) *PaymentModel {
	return &PaymentModel{
		ID:             p.ID,
		AmountCents:    p.Amount.Amount,
		AmountCurrency: p.Amount.Currency,
		Reference:      p.Reference,
		PayerName:      p.PayerName,
		PayerIBAN:      p.PayerIBAN,
		BookingDate:    p.BookingDate,
		InvoiceID:      p.InvoiceID,
		CustomerID:     p.CustomerID,
		Status:         string(p.Status),
		ImportBatchID:  p.ImportBatchID,
		CreatedAt:      p.CreatedAt,
	}
}

func toDomain(m *PaymentModel) *domain.Payment {
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
