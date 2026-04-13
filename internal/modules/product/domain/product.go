package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/vvs/isp/internal/shared/domain"
)

var (
	ErrNameRequired      = errors.New("product name is required")
	ErrInvalidType       = errors.New("invalid product type")
	ErrInvalidBilling    = errors.New("invalid billing period")
	ErrAlreadyActive     = errors.New("product is already active")
	ErrAlreadyInactive   = errors.New("product is already inactive")
	ErrProductNotFound   = errors.New("product not found")
)

type ProductType string

const (
	TypeInternet ProductType = "internet"
	TypeVoIP     ProductType = "voip"
	TypeHosting  ProductType = "hosting"
	TypeCustom   ProductType = "custom"
)

func IsValidProductType(t string) bool {
	switch ProductType(t) {
	case TypeInternet, TypeVoIP, TypeHosting, TypeCustom:
		return true
	}
	return false
}

type BillingPeriod string

const (
	BillingMonthly   BillingPeriod = "monthly"
	BillingQuarterly BillingPeriod = "quarterly"
	BillingYearly    BillingPeriod = "yearly"
)

func IsValidBillingPeriod(b string) bool {
	switch BillingPeriod(b) {
	case BillingMonthly, BillingQuarterly, BillingYearly:
		return true
	}
	return false
}

type Product struct {
	ID            string
	Name          string
	Description   string
	Type          ProductType
	Price         domain.Money
	BillingPeriod BillingPeriod
	IsActive      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func NewProduct(name, description, productType string, priceAmount int64, priceCurrency, billingPeriod string) (*Product, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrNameRequired
	}

	if !IsValidProductType(productType) {
		return nil, ErrInvalidType
	}

	if !IsValidBillingPeriod(billingPeriod) {
		return nil, ErrInvalidBilling
	}

	now := time.Now().UTC()
	return &Product{
		ID:            uuid.Must(uuid.NewV7()).String(),
		Name:          name,
		Description:   strings.TrimSpace(description),
		Type:          ProductType(productType),
		Price:         domain.NewMoney(priceAmount, priceCurrency),
		BillingPeriod: BillingPeriod(billingPeriod),
		IsActive:      true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func (p *Product) Update(name, description, productType string, priceAmount int64, priceCurrency, billingPeriod string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrNameRequired
	}

	if !IsValidProductType(productType) {
		return ErrInvalidType
	}

	if !IsValidBillingPeriod(billingPeriod) {
		return ErrInvalidBilling
	}

	p.Name = name
	p.Description = strings.TrimSpace(description)
	p.Type = ProductType(productType)
	p.Price = domain.NewMoney(priceAmount, priceCurrency)
	p.BillingPeriod = BillingPeriod(billingPeriod)
	p.UpdatedAt = time.Now().UTC()
	return nil
}

func (p *Product) Deactivate() error {
	if !p.IsActive {
		return ErrAlreadyInactive
	}
	p.IsActive = false
	p.UpdatedAt = time.Now().UTC()
	return nil
}

func (p *Product) Activate() error {
	if p.IsActive {
		return ErrAlreadyActive
	}
	p.IsActive = true
	p.UpdatedAt = time.Now().UTC()
	return nil
}
