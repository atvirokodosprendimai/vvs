package main

import (
	"context"
	"log"
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"

	customerqueries "github.com/atvirokodosprendimai/vvs/internal/modules/customer/app/queries"
	invoicecommands "github.com/atvirokodosprendimai/vvs/internal/modules/invoice/app/commands"
	invoicepersistence "github.com/atvirokodosprendimai/vvs/internal/modules/invoice/adapters/persistence"
	servicedomain "github.com/atvirokodosprendimai/vvs/internal/modules/service/domain"
	servicepersistence "github.com/atvirokodosprendimai/vvs/internal/modules/service/adapters/persistence"
)

// RegisterBillingActions wires the billing run dependencies and registers the
// "generate-due-invoices" cron action. Call before RunDueJobs.
func RegisterBillingActions(gdb *gormsqlite.DB, pub events.EventPublisher) {
	serviceRepo := servicepersistence.NewGormServiceRepository(gdb)
	customerQuery := customerqueries.NewGetCustomerHandler(gdb)
	invoiceRepo := invoicepersistence.NewInvoiceRepository(gdb)
	handler := invoicecommands.NewGenerateFromServicesHandler(invoiceRepo, pub)

	RegisterAction("generate-due-invoices", func(ctx context.Context) error {
		now := time.Now().UTC()

		due, err := serviceRepo.ListDueForBilling(ctx, now)
		if err != nil {
			return err
		}
		if len(due) == 0 {
			return nil
		}

		// Group due services by customer.
		byCustomer := map[string][]*servicedomain.Service{}
		for _, svc := range due {
			byCustomer[svc.CustomerID] = append(byCustomer[svc.CustomerID], svc)
		}

		for customerID, services := range byCustomer {
			cust, err := customerQuery.Handle(ctx, customerqueries.GetCustomerQuery{ID: customerID})
			if err != nil {
				log.Printf("billing: customer %s not found: %v", customerID, err)
				continue
			}

			items := make([]invoicecommands.ServiceInfo, len(services))
			for i, svc := range services {
				items[i] = invoicecommands.ServiceInfo{
					ID:          svc.ID,
					ProductID:   svc.ProductID,
					ProductName: svc.ProductName,
					PriceAmount: svc.PriceAmount,
				}
			}

			inv, err := handler.Handle(ctx, invoicecommands.GenerateFromServicesCommand{
				CustomerID:     customerID,
				CustomerName:   cust.CompanyName,
				CustomerCode:   cust.Code.String(),
				DefaultVATRate: 21,
				Services:       items,
			})
			if err != nil {
				log.Printf("billing: generate invoice for customer %s: %v", customerID, err)
				continue
			}
			if inv == nil {
				continue
			}
			log.Printf("billing: generated invoice %s for customer %s (%d services)", inv.ID, customerID, len(services))

			// Advance next billing date for each billed service.
			for _, svc := range services {
				svc.AdvanceNextBillingDate()
				if err := serviceRepo.Save(ctx, svc); err != nil {
					log.Printf("billing: advance next billing date for service %s: %v", svc.ID, err)
				}
			}
		}

		return nil
	})
}
