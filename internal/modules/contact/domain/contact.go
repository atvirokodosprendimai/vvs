package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrNotFound          = errors.New("contact not found")
	ErrFirstNameRequired = errors.New("first name required")
)

type Contact struct {
	ID         string
	CustomerID string
	FirstName  string
	LastName   string
	Email      string
	Phone      string
	Role       string
	Notes      string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func NewContact(id, customerID, firstName, lastName, email, phone, role string) (*Contact, error) {
	firstName = strings.TrimSpace(firstName)
	if firstName == "" {
		return nil, ErrFirstNameRequired
	}
	now := time.Now().UTC()
	return &Contact{
		ID:         id,
		CustomerID: customerID,
		FirstName:  strings.TrimSpace(firstName),
		LastName:   strings.TrimSpace(lastName),
		Email:      strings.TrimSpace(email),
		Phone:      strings.TrimSpace(phone),
		Role:       strings.TrimSpace(role),
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

func (c *Contact) Update(firstName, lastName, email, phone, role, notes string) error {
	firstName = strings.TrimSpace(firstName)
	if firstName == "" {
		return ErrFirstNameRequired
	}
	c.FirstName = firstName
	c.LastName = strings.TrimSpace(lastName)
	c.Email = strings.TrimSpace(email)
	c.Phone = strings.TrimSpace(phone)
	c.Role = strings.TrimSpace(role)
	c.Notes = strings.TrimSpace(notes)
	c.UpdatedAt = time.Now().UTC()
	return nil
}

func (c *Contact) FullName() string {
	if c.LastName == "" {
		return c.FirstName
	}
	return c.FirstName + " " + c.LastName
}
