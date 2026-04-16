package domain

import (
	"errors"
	"time"
)

const (
	StatusInStock        = "in_stock"
	StatusDeployed       = "deployed"
	StatusDecommissioned = "decommissioned"

	TypeModem   = "modem"
	TypeRouter  = "router"
	TypeONT     = "ont"
	TypeSwitch  = "switch"
	TypeSensor  = "sensor"
	TypeOther   = "other"
)

var (
	ErrNotFound          = errors.New("device not found")
	ErrInvalidTransition = errors.New("invalid status transition")
	ErrNameRequired      = errors.New("device name required")
)

// Device is a piece of hardware tracked in the inventory.
type Device struct {
	ID             string
	Name           string
	SerialNumber   string
	DeviceType     string
	Status         string
	CustomerID     string
	Location       string
	PurchasedAt    *time.Time
	WarrantyExpiry *time.Time
	Notes          string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func NewDevice(id, name, deviceType, serialNumber string) (*Device, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	if deviceType == "" {
		deviceType = TypeOther
	}
	now := time.Now().UTC()
	return &Device{
		ID:           id,
		Name:         name,
		SerialNumber: serialNumber,
		DeviceType:   deviceType,
		Status:       StatusInStock,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// Deploy assigns the device to a customer and location.
func (d *Device) Deploy(customerID, location string) error {
	if d.Status != StatusInStock {
		return ErrInvalidTransition
	}
	d.CustomerID = customerID
	d.Location = location
	d.Status = StatusDeployed
	d.UpdatedAt = time.Now().UTC()
	return nil
}

// Return moves a deployed device back to in_stock.
func (d *Device) Return() error {
	if d.Status != StatusDeployed {
		return ErrInvalidTransition
	}
	d.CustomerID = ""
	d.Status = StatusInStock
	d.UpdatedAt = time.Now().UTC()
	return nil
}

// Decommission marks the device as permanently retired.
func (d *Device) Decommission() error {
	if d.Status == StatusDecommissioned {
		return ErrInvalidTransition
	}
	d.CustomerID = ""
	d.Status = StatusDecommissioned
	d.UpdatedAt = time.Now().UTC()
	return nil
}

// Update applies non-nil field patches.
func (d *Device) Update(name, notes, location string, purchasedAt, warrantyExpiry *time.Time) error {
	if name != "" {
		d.Name = name
	}
	if notes != "" {
		d.Notes = notes
	}
	if location != "" {
		d.Location = location
	}
	if purchasedAt != nil {
		d.PurchasedAt = purchasedAt
	}
	if warrantyExpiry != nil {
		d.WarrantyExpiry = warrantyExpiry
	}
	d.UpdatedAt = time.Now().UTC()
	return nil
}
