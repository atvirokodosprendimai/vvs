package commands

import (
	"github.com/atvirokodosprendimai/vvs/internal/modules/customer/app/queries"
	"github.com/atvirokodosprendimai/vvs/internal/modules/customer/domain"
)

// domainToReadModel maps a domain Customer to CustomerReadModel for NATS event payload.
func domainToReadModel(c *domain.Customer) queries.CustomerReadModel {
	return queries.CustomerReadModel{
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
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}
