package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/shared/domain"
)

var (
	ErrCompanyNameRequired = errors.New("company name is required")
	ErrAlreadySuspended    = errors.New("customer is already suspended")
	ErrAlreadyActive       = errors.New("customer is already active")
	ErrCustomerNotFound    = errors.New("customer not found")
)

type CustomerStatus string

const (
	StatusActive    CustomerStatus = "active"
	StatusSuspended CustomerStatus = "suspended"
	StatusChurned   CustomerStatus = "churned"
)

type Customer struct {
	ID          string
	Code        domain.CompanyCode
	CompanyName string
	ContactName string
	Email       string
	Phone       string
	Street      string
	City        string
	PostalCode  string
	Country     string
	TaxID       string
	Status      CustomerStatus
	Notes       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewCustomer(code domain.CompanyCode, companyName, contactName, email, phone string) (*Customer, error) {
	companyName = strings.TrimSpace(companyName)
	if companyName == "" {
		return nil, ErrCompanyNameRequired
	}

	now := time.Now().UTC()
	return &Customer{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Code:        code,
		CompanyName: companyName,
		ContactName: strings.TrimSpace(contactName),
		Email:       strings.TrimSpace(email),
		Phone:       strings.TrimSpace(phone),
		Status:      StatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func (c *Customer) Update(companyName, contactName, email, phone, street, city, postalCode, country, taxID, notes string) error {
	companyName = strings.TrimSpace(companyName)
	if companyName == "" {
		return ErrCompanyNameRequired
	}

	c.CompanyName = companyName
	c.ContactName = strings.TrimSpace(contactName)
	c.Email = strings.TrimSpace(email)
	c.Phone = strings.TrimSpace(phone)
	c.Street = strings.TrimSpace(street)
	c.City = strings.TrimSpace(city)
	c.PostalCode = strings.TrimSpace(postalCode)
	c.Country = strings.TrimSpace(country)
	c.TaxID = strings.TrimSpace(taxID)
	c.Notes = strings.TrimSpace(notes)
	c.UpdatedAt = time.Now().UTC()
	return nil
}

func (c *Customer) Suspend() error {
	if c.Status == StatusSuspended {
		return ErrAlreadySuspended
	}
	c.Status = StatusSuspended
	c.UpdatedAt = time.Now().UTC()
	return nil
}

func (c *Customer) Activate() error {
	if c.Status == StatusActive {
		return ErrAlreadyActive
	}
	c.Status = StatusActive
	c.UpdatedAt = time.Now().UTC()
	return nil
}
