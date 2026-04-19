package app

import (
	"context"
	"log"

	"github.com/nats-io/nats.go"

	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
	"github.com/vvs/isp/internal/shared/events"

	auditloghttp        "github.com/vvs/isp/internal/modules/audit_log/adapters/http"
	auditlogpersistence "github.com/vvs/isp/internal/modules/audit_log/adapters/persistence"
	auditlogcommands    "github.com/vvs/isp/internal/modules/audit_log/app/commands"
	auditlogqueries     "github.com/vvs/isp/internal/modules/audit_log/app/queries"

	invoicehttp        "github.com/vvs/isp/internal/modules/invoice/adapters/http"
	invoicepersistence "github.com/vvs/isp/internal/modules/invoice/adapters/persistence"
	invoicecommands    "github.com/vvs/isp/internal/modules/invoice/app/commands"
	invoicequeries     "github.com/vvs/isp/internal/modules/invoice/app/queries"
	invoiceworkers     "github.com/vvs/isp/internal/modules/invoice/app/workers"
	invoicedomain      "github.com/vvs/isp/internal/modules/invoice/domain"
)

type invoiceWired struct {
	repo            *invoicepersistence.InvoiceRepository
	tokenRepo       invoicedomain.InvoiceTokenRepository
	createCmd       *invoicecommands.CreateInvoiceHandler
	finalizeCmd     *invoicecommands.FinalizeInvoiceHandler
	markPaidCmd     *invoicecommands.MarkPaidHandler
	voidCmd         *invoicecommands.VoidInvoiceHandler
	addLineCmd      *invoicecommands.AddLineItemHandler
	updateLineCmd   *invoicecommands.UpdateLineItemHandler
	removeLineCmd   *invoicecommands.RemoveLineItemHandler
	generateCmd     *invoicecommands.GenerateFromSubscriptionsHandler
	listAll         *invoicequeries.ListAllInvoicesHandler
	get             *invoicequeries.GetInvoiceHandler
	listForCustomer *invoicequeries.ListInvoicesForCustomerHandler
	routes          *invoicehttp.Handlers
}

type auditWired struct {
	routes *auditloghttp.Handlers
}

func wireInvoice(
	gdb   *gormsqlite.DB,
	pub   events.EventPublisher,
	sub   events.EventSubscriber,
	nc    *nats.Conn,
	cust  *customerWired,
	svc   *serviceWired,
	email *emailWired,
	cfg   Config,
) *invoiceWired {
	invoiceRepo      := invoicepersistence.NewInvoiceRepository(gdb)
	invoiceTokenRepo := invoicepersistence.NewInvoiceTokenRepository(gdb)

	createInvoiceCmd   := invoicecommands.NewCreateInvoiceHandler(invoiceRepo, pub)
	finalizeInvoiceCmd := invoicecommands.NewFinalizeInvoiceHandler(invoiceRepo, pub)
	markPaidCmd        := invoicecommands.NewMarkPaidHandler(invoiceRepo, pub)
	voidInvoiceCmd     := invoicecommands.NewVoidInvoiceHandler(invoiceRepo, pub)
	addLineItemCmd     := invoicecommands.NewAddLineItemHandler(invoiceRepo, pub)
	updateLineItemCmd  := invoicecommands.NewUpdateLineItemHandler(invoiceRepo, pub)
	removeLineItemCmd  := invoicecommands.NewRemoveLineItemHandler(invoiceRepo, pub)
	generateInvoiceCmd := invoicecommands.NewGenerateFromSubscriptionsHandler(
		invoiceRepo, pub, &activeServiceBridge{repo: svc.repo},
	)

	listAllInvoicesQuery         := invoicequeries.NewListAllInvoicesHandler(gdb)
	getInvoiceQuery              := invoicequeries.NewGetInvoiceHandler(gdb)
	listInvoicesForCustomerQuery := invoicequeries.NewListInvoicesForCustomerHandler(gdb)

	vatRate := cfg.DefaultVATRate
	if vatRate <= 0 {
		vatRate = 21
	}

	routes := invoicehttp.NewHandlers(
		createInvoiceCmd, finalizeInvoiceCmd, markPaidCmd, voidInvoiceCmd,
		addLineItemCmd, updateLineItemCmd, removeLineItemCmd,
		listAllInvoicesQuery, getInvoiceQuery, listInvoicesForCustomerQuery,
		sub,
	)
	routes.WithGenerateCmd(generateInvoiceCmd)
	routes.WithCustomerSearch(&customerSearchBridge{handler: cust.listQuery})
	routes.WithTokenRepo(invoiceTokenRepo)
	routes.WithDefaultVATRate(vatRate)

	if cust.routes != nil {
		cust.routes.WithInvoicesQuery(listInvoicesForCustomerQuery)
	}

	if nc != nil {
		deliveryWorker := invoiceworkers.NewInvoiceDeliveryWorker(
			&emailAccountMailerBridge{accounts: email.accountRepo, smtp: email.smtpSender},
			&customerEmailBridge{query: cust.getQuery},
		)
		go deliveryWorker.Run(context.Background(), sub)
		log.Printf("module wired: invoice delivery worker")
	}

	log.Printf("module wired: invoice")

	return &invoiceWired{
		repo:            invoiceRepo,
		tokenRepo:       invoiceTokenRepo,
		createCmd:       createInvoiceCmd,
		finalizeCmd:     finalizeInvoiceCmd,
		markPaidCmd:     markPaidCmd,
		voidCmd:         voidInvoiceCmd,
		addLineCmd:      addLineItemCmd,
		updateLineCmd:   updateLineItemCmd,
		removeLineCmd:   removeLineItemCmd,
		generateCmd:     generateInvoiceCmd,
		listAll:         listAllInvoicesQuery,
		get:             getInvoiceQuery,
		listForCustomer: listInvoicesForCustomerQuery,
		routes:          routes,
	}
}

func wireAudit(
	gdb  *gormsqlite.DB,
	sub  events.EventSubscriber,
	cust *customerWired,
	crm  *crmWired,
	svc  *serviceWired,
	inv  *invoiceWired,
) *auditWired {
	auditLogRepo         := auditlogpersistence.NewGormAuditLogRepository(gdb)
	createAuditLogCmd    := auditlogcommands.NewCreateAuditLogHandler(auditLogRepo)
	listAuditLogsQuery   := auditlogqueries.NewListAuditLogsHandler(auditLogRepo)
	listForResourceQuery := auditlogqueries.NewListForResourceHandler(auditLogRepo)

	routes := auditloghttp.NewHandlers(listAuditLogsQuery, sub)
	routes.WithListForResource(listForResourceQuery)

	if cust.routes != nil {
		cust.routes.WithAuditLogger(createAuditLogCmd)
	}
	if crm.ticketRoutes != nil {
		crm.ticketRoutes.WithAuditLogger(createAuditLogCmd)
	}
	inv.routes.WithAuditLogger(createAuditLogCmd)
	if svc.routes != nil {
		svc.routes.WithAuditLogger(createAuditLogCmd)
	}

	log.Printf("module wired: audit_log")

	return &auditWired{routes: routes}
}
