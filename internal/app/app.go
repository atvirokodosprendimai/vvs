package app

import (
	"context"
	"fmt"
	"log"

	"github.com/nats-io/nats.go"
	"gorm.io/gorm"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/vvs/isp/internal/infrastructure/database"
	infrahttp "github.com/vvs/isp/internal/infrastructure/http"
	infranats "github.com/vvs/isp/internal/infrastructure/nats"
	"github.com/vvs/isp/internal/shared/events"

	customerhttp "github.com/vvs/isp/internal/modules/customer/adapters/http"
	customerpersistence "github.com/vvs/isp/internal/modules/customer/adapters/persistence"
	customercommands "github.com/vvs/isp/internal/modules/customer/app/commands"
	customerqueries "github.com/vvs/isp/internal/modules/customer/app/queries"
	customermigrations "github.com/vvs/isp/internal/modules/customer/migrations"

	producthttp "github.com/vvs/isp/internal/modules/product/adapters/http"
	productpersistence "github.com/vvs/isp/internal/modules/product/adapters/persistence"
	productcommands "github.com/vvs/isp/internal/modules/product/app/commands"
	productqueries "github.com/vvs/isp/internal/modules/product/app/queries"
	productmigrations "github.com/vvs/isp/internal/modules/product/migrations"

	invoicehttp "github.com/vvs/isp/internal/modules/invoice/adapters/http"
	invoicepersistence "github.com/vvs/isp/internal/modules/invoice/adapters/persistence"
	invoicecommands "github.com/vvs/isp/internal/modules/invoice/app/commands"
	invoicequeries "github.com/vvs/isp/internal/modules/invoice/app/queries"
	invoicemigrations "github.com/vvs/isp/internal/modules/invoice/migrations"

	recurringhttp "github.com/vvs/isp/internal/modules/recurring/adapters/http"
	recurringpersistence "github.com/vvs/isp/internal/modules/recurring/adapters/persistence"
	recurringcommands "github.com/vvs/isp/internal/modules/recurring/app/commands"
	recurringqueries "github.com/vvs/isp/internal/modules/recurring/app/queries"
	recurringmigrations "github.com/vvs/isp/internal/modules/recurring/migrations"

	paymenthttp "github.com/vvs/isp/internal/modules/payment/adapters/http"
	paymentpersistence "github.com/vvs/isp/internal/modules/payment/adapters/persistence"
	paymentcommands "github.com/vvs/isp/internal/modules/payment/app/commands"
	paymentqueries "github.com/vvs/isp/internal/modules/payment/app/queries"
	paymentmigrations "github.com/vvs/isp/internal/modules/payment/migrations"
)

type App struct {
	Writer     *database.WriteSerializer
	NATSServer *natsserver.Server
	NATSConn   *nats.Conn
	Publisher  events.EventPublisher
	Subscriber events.EventSubscriber
	HTTPServer *infrahttp.Server
	DB         *gorm.DB
	Reader     *gorm.DB
}

func New(cfg Config) (*App, error) {
	// 1. Database
	db, err := database.OpenSQLite(cfg.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	reader, err := database.OpenSQLiteReader(cfg.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("open reader database: %w", err)
	}

	// 2. Run migrations
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB: %w", err)
	}

	if err := database.RunModuleMigrations(sqlDB, []database.ModuleMigration{
		{Name: "customer", FS: customermigrations.FS, TableName: "goose_customer"},
		{Name: "product", FS: productmigrations.FS, TableName: "goose_product"},
		{Name: "invoice", FS: invoicemigrations.FS, TableName: "goose_invoice"},
		{Name: "recurring", FS: recurringmigrations.FS, TableName: "goose_recurring"},
		{Name: "payment", FS: paymentmigrations.FS, TableName: "goose_payment"},
	}); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	// 3. Write serializer
	writer := database.NewWriteSerializer(db)

	// 4. Embedded NATS
	ns, nc, err := infranats.StartEmbedded()
	if err != nil {
		return nil, fmt.Errorf("start nats: %w", err)
	}

	publisher := infranats.NewPublisher(nc)
	subscriber := infranats.NewSubscriber(nc)

	// 5. Wire Customer module
	customerRepo := customerpersistence.NewGormCustomerRepository(writer, reader)
	createCustomerCmd := customercommands.NewCreateCustomerHandler(customerRepo, publisher)
	updateCustomerCmd := customercommands.NewUpdateCustomerHandler(customerRepo, publisher)
	deleteCustomerCmd := customercommands.NewDeleteCustomerHandler(customerRepo, publisher)
	listCustomersQuery := customerqueries.NewListCustomersHandler(reader)
	getCustomerQuery := customerqueries.NewGetCustomerHandler(reader)
	customerRoutes := customerhttp.NewHandlers(
		createCustomerCmd, updateCustomerCmd, deleteCustomerCmd,
		listCustomersQuery, getCustomerQuery, subscriber,
	)

	// 6. Wire Product module
	productRepo := productpersistence.NewGormProductRepository(writer, reader)
	createProductCmd := productcommands.NewCreateProductHandler(productRepo, publisher)
	updateProductCmd := productcommands.NewUpdateProductHandler(productRepo, publisher)
	deleteProductCmd := productcommands.NewDeleteProductHandler(productRepo, publisher)
	listProductsQuery := productqueries.NewListProductsHandler(reader)
	getProductQuery := productqueries.NewGetProductHandler(reader)
	productRoutes := producthttp.NewHandlers(
		createProductCmd, updateProductCmd, deleteProductCmd,
		listProductsQuery, getProductQuery, subscriber,
	)

	// 7. Wire Invoice module
	invoiceRepo := invoicepersistence.NewGormInvoiceRepository(writer, reader)
	createInvoiceCmd := invoicecommands.NewCreateInvoiceHandler(invoiceRepo, publisher)
	finalizeInvoiceCmd := invoicecommands.NewFinalizeInvoiceHandler(invoiceRepo, publisher)
	voidInvoiceCmd := invoicecommands.NewVoidInvoiceHandler(invoiceRepo, publisher)
	listInvoicesQuery := invoicequeries.NewListInvoicesHandler(reader)
	getInvoiceQuery := invoicequeries.NewGetInvoiceHandler(reader)
	invoiceRoutes := invoicehttp.NewHandlers(
		createInvoiceCmd, finalizeInvoiceCmd, voidInvoiceCmd,
		listInvoicesQuery, getInvoiceQuery, subscriber,
	)

	// 8. Wire Recurring module
	recurringRepo := recurringpersistence.NewGormRecurringRepository(writer, reader)
	createRecurringCmd := recurringcommands.NewCreateRecurringHandler(recurringRepo, publisher)
	updateRecurringCmd := recurringcommands.NewUpdateRecurringHandler(recurringRepo, publisher)
	toggleRecurringCmd := recurringcommands.NewToggleRecurringHandler(recurringRepo, publisher)
	listRecurringQuery := recurringqueries.NewListRecurringHandler(reader)
	getRecurringQuery := recurringqueries.NewGetRecurringHandler(reader)
	recurringRoutes := recurringhttp.NewHandlers(
		createRecurringCmd, updateRecurringCmd, toggleRecurringCmd,
		listRecurringQuery, getRecurringQuery, subscriber,
	)

	// 9. Wire Payment module
	paymentRepo := paymentpersistence.NewGormPaymentRepository(writer, reader)
	recordPaymentCmd := paymentcommands.NewRecordPaymentHandler(paymentRepo, publisher)
	importPaymentsCmd := paymentcommands.NewImportPaymentsHandler(paymentRepo, publisher)
	matchPaymentCmd := paymentcommands.NewMatchPaymentHandler(paymentRepo, publisher)
	listPaymentsQuery := paymentqueries.NewListPaymentsHandler(reader)
	getPaymentQuery := paymentqueries.NewGetPaymentHandler(reader)
	unmatchedPaymentsQuery := paymentqueries.NewUnmatchedPaymentsHandler(reader)
	paymentRoutes := paymenthttp.NewHandlers(
		recordPaymentCmd, importPaymentsCmd, matchPaymentCmd,
		listPaymentsQuery, getPaymentQuery, unmatchedPaymentsQuery, subscriber,
	)

	// 10. Router
	router := infrahttp.NewRouter(reader,
		customerRoutes, productRoutes, invoiceRoutes,
		recurringRoutes, paymentRoutes,
	)

	httpServer := infrahttp.NewServer(cfg.ListenAddr, router)

	log.Printf("VVS ISP Manager initialized (db: %s)", cfg.DatabasePath)

	return &App{
		Writer:     writer,
		NATSServer: ns,
		NATSConn:   nc,
		Publisher:  publisher,
		Subscriber: subscriber,
		HTTPServer: httpServer,
		DB:         db,
		Reader:     reader,
	}, nil
}

func (a *App) Start() error {
	return a.HTTPServer.Start()
}

func (a *App) Shutdown(ctx context.Context) error {
	err := a.HTTPServer.Shutdown(ctx)
	a.Writer.Close()
	if a.Subscriber != nil {
		a.Subscriber.Close()
	}
	a.NATSConn.Close()
	a.NATSServer.WaitForShutdown()
	return err
}
