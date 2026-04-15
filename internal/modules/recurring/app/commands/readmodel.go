package commands

import (
	"github.com/vvs/isp/internal/modules/recurring/app/queries"
	"github.com/vvs/isp/internal/modules/recurring/domain"
)

// domainToReadModel maps a domain RecurringInvoice to RecurringFullReadModel for NATS event payload.
// Includes lines so Total() can be computed in the SSE handler without a DB re-query.
func domainToReadModel(inv *domain.RecurringInvoice) queries.RecurringFullReadModel {
	lines := make([]queries.RecurringLineReadModel, len(inv.Lines))
	for i, l := range inv.Lines {
		lines[i] = queries.RecurringLineReadModel{
			ID:                l.ID,
			RecurringID:       inv.ID,
			ProductID:         l.ProductID,
			ProductName:       l.ProductName,
			Description:       l.Description,
			Quantity:          l.Quantity,
			UnitPriceAmount:   l.UnitPrice.Amount,
			UnitPriceCurrency: string(l.UnitPrice.Currency),
			SortOrder:         l.SortOrder,
		}
	}
	return queries.RecurringFullReadModel{
		RecurringReadModel: queries.RecurringReadModel{
			ID:           inv.ID,
			CustomerID:   inv.CustomerID,
			CustomerName: inv.CustomerName,
			Frequency:    string(inv.Schedule.Frequency),
			DayOfMonth:   inv.Schedule.DayOfMonth,
			NextRunDate:  inv.NextRunDate,
			LastRunDate:  inv.LastRunDate,
			Status:       string(inv.Status),
			CreatedAt:    inv.CreatedAt,
			UpdatedAt:    inv.UpdatedAt,
		},
		Lines: lines,
	}
}
