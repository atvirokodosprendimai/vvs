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
	ErrAlreadyChurned      = errors.New("customer is already churned")
	ErrInvalidTransition   = errors.New("invalid status transition")
	ErrCustomerNotFound    = errors.New("customer not found")
)

type CustomerStatus string

const (
	StatusLead      CustomerStatus = "lead"
	StatusProspect  CustomerStatus = "prospect"
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
	// Network provisioning fields — set when customer has a managed network connection
	RouterID     *string // FK to routers table; nil = no provisioning
	IPAddress    string  // e.g. "10.0.1.55"
	MACAddress   string  // e.g. "AA:BB:CC:DD:EE:FF"
	NetworkZone  string  // matches netbox_prefixes.location for IP allocation, e.g. "Kaunas"
	CreatedAt  time.Time
	UpdatedAt  time.Time
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

// SetNetworkInfo assigns or clears the customer's network provisioning details.
// Pass empty routerID to remove provisioning.
func (c *Customer) SetNetworkInfo(routerID, ipAddress, macAddress string) {
	if routerID == "" {
		c.RouterID = nil
	} else {
		c.RouterID = &routerID
	}
	c.IPAddress = strings.TrimSpace(ipAddress)
	c.MACAddress = strings.TrimSpace(macAddress)
	c.UpdatedAt = time.Now().UTC()
}

// SetNetworkZone sets the zone used for IP allocation.
func (c *Customer) SetNetworkZone(zone string) {
	c.NetworkZone = strings.TrimSpace(zone)
	c.UpdatedAt = time.Now().UTC()
}

// HasNetworkProvisioning reports whether this customer has a router assigned.
func (c *Customer) HasNetworkProvisioning() bool {
	return c.RouterID != nil && *c.RouterID != ""
}

func (c *Customer) Qualify() error {
	if c.Status != StatusLead {
		return ErrInvalidTransition
	}
	c.Status = StatusProspect
	c.UpdatedAt = time.Now().UTC()
	return nil
}

func (c *Customer) Convert() error {
	if c.Status != StatusProspect {
		return ErrInvalidTransition
	}
	c.Status = StatusActive
	c.UpdatedAt = time.Now().UTC()
	return nil
}

func (c *Customer) Activate() error {
	if c.Status == StatusActive {
		return ErrAlreadyActive
	}
	if c.Status == StatusChurned {
		return ErrInvalidTransition
	}
	c.Status = StatusActive
	c.UpdatedAt = time.Now().UTC()
	return nil
}

func (c *Customer) Churn() error {
	if c.Status == StatusChurned {
		return ErrAlreadyChurned
	}
	c.Status = StatusChurned
	c.UpdatedAt = time.Now().UTC()
	return nil
}
