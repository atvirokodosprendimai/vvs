package persistence

import (
	"fmt"
	"time"

	"github.com/vvs/isp/internal/modules/customer/domain"
	shareddomain "github.com/vvs/isp/internal/shared/domain"
)

type CustomerModel struct {
	ID          string  `gorm:"primaryKey;type:text"`
	Code        string  `gorm:"uniqueIndex;type:text;not null"`
	CompanyName string  `gorm:"type:text;not null"`
	ContactName string  `gorm:"type:text"`
	Email       string  `gorm:"type:text"`
	Phone       string  `gorm:"type:text"`
	Street      string  `gorm:"type:text"`
	City        string  `gorm:"type:text"`
	PostalCode  string  `gorm:"type:text"`
	Country     string  `gorm:"type:text"`
	TaxID       string  `gorm:"type:text"`
	Status      string  `gorm:"type:text;not null;default:'active'"`
	Notes       string  `gorm:"type:text"`
	RouterID    *string `gorm:"type:text"`
	IPAddress   string  `gorm:"type:text"`
	MACAddress  string  `gorm:"type:text"`
	NetworkZone string  `gorm:"type:text;not null;default:''"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (CustomerModel) TableName() string { return "customers" }

func toModel(c *domain.Customer) *CustomerModel {
	return &CustomerModel{
		ID:          c.ID,
		Code:        c.Code.String(),
		CompanyName: c.CompanyName,
		ContactName: c.ContactName,
		Email:       c.Email,
		Phone:       c.Phone,
		Street:      c.Street,
		City:        c.City,
		PostalCode:  c.PostalCode,
		Country:     c.Country,
		TaxID:       c.TaxID,
		Status:      string(c.Status),
		Notes:       c.Notes,
		RouterID:    c.RouterID,
		IPAddress:   c.IPAddress,
		MACAddress:  c.MACAddress,
		NetworkZone: c.NetworkZone,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}

func toDomain(m *CustomerModel) *domain.Customer {
	return &domain.Customer{
		ID:          m.ID,
		Code:        parseCode(m.Code),
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
		NetworkZone: m.NetworkZone,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

func parseCode(code string) shareddomain.CompanyCode {
	var prefix string
	var number int
	fmt.Sscanf(code, "%3s-%05d", &prefix, &number)
	return shareddomain.NewCompanyCode(prefix, number)
}
