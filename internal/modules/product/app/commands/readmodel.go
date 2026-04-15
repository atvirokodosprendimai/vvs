package commands

import (
	"github.com/vvs/isp/internal/modules/product/app/queries"
	"github.com/vvs/isp/internal/modules/product/domain"
)

// domainToReadModel maps a domain Product to ProductReadModel for NATS event payload.
func domainToReadModel(p *domain.Product) queries.ProductReadModel {
	return queries.ProductReadModel{
		ID:            p.ID,
		Name:          p.Name,
		Description:   p.Description,
		Type:          string(p.Type),
		PriceAmount:   p.Price.Amount,
		PriceCurrency: string(p.Price.Currency),
		BillingPeriod: string(p.BillingPeriod),
		IsActive:      p.IsActive,
		CreatedAt:     p.CreatedAt,
		UpdatedAt:     p.UpdatedAt,
	}
}
