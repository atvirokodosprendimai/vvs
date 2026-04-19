package app

import (
	"log"

	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
	infrahttp "github.com/vvs/isp/internal/infrastructure/http"
	"github.com/vvs/isp/internal/shared/events"

	customerhttp "github.com/vvs/isp/internal/modules/customer/adapters/http"
	customerpersistence "github.com/vvs/isp/internal/modules/customer/adapters/persistence"
	customercommands "github.com/vvs/isp/internal/modules/customer/app/commands"
	customerqueries "github.com/vvs/isp/internal/modules/customer/app/queries"

	producthttp "github.com/vvs/isp/internal/modules/product/adapters/http"
	productpersistence "github.com/vvs/isp/internal/modules/product/adapters/persistence"
	productcommands "github.com/vvs/isp/internal/modules/product/app/commands"
	productqueries "github.com/vvs/isp/internal/modules/product/app/queries"
)

type customerWired struct {
	repo        *customerpersistence.GormCustomerRepository
	updateCmd   *customercommands.UpdateCustomerHandler
	deleteCmd   *customercommands.DeleteCustomerHandler
	listQuery   *customerqueries.ListCustomersHandler
	getQuery    *customerqueries.GetCustomerHandler
	routes      *customerhttp.Handlers // nil when module disabled
}

func wireCustomer(
	gdb *gormsqlite.DB,
	pub events.EventPublisher,
	sub events.EventSubscriber,
	svc *serviceWired,
	cfg Config,
) *customerWired {
	customerRepo := customerpersistence.NewGormCustomerRepository(gdb)
	noteRepo     := customerpersistence.NewGormNoteRepository(gdb)

	updateCustomerCmd  := customercommands.NewUpdateCustomerHandler(customerRepo, pub)
	deleteCustomerCmd  := customercommands.NewDeleteCustomerHandler(customerRepo, pub)
	changeStatusCmd    := customercommands.NewChangeCustomerStatusHandler(customerRepo, pub)
	addNoteCmd         := customercommands.NewAddNoteHandler(noteRepo)
	listCustomersQuery := customerqueries.NewListCustomersHandler(gdb)
	getCustomerQuery   := customerqueries.NewGetCustomerHandler(gdb)
	listNotesQuery     := customerqueries.NewListNotesHandler(gdb)

	var routes *customerhttp.Handlers
	if cfg.IsEnabled("customer") {
		routes = customerhttp.NewHandlers(
			// createCustomerCmd wired in wireNetwork (needs ipamProvider)
			// pass nil for now; wireNetwork will set it via WithCreateCmd
			nil, updateCustomerCmd, deleteCustomerCmd,
			changeStatusCmd, addNoteCmd,
			listCustomersQuery, getCustomerQuery, listNotesQuery,
			sub, pub,
			svc.listServices,
		)
		log.Printf("module enabled: customer")
	}

	return &customerWired{
		repo:      customerRepo,
		updateCmd: updateCustomerCmd,
		deleteCmd: deleteCustomerCmd,
		listQuery: listCustomersQuery,
		getQuery:  getCustomerQuery,
		routes:    routes,
	}
}

// ── Product ───────────────────────────────────────────────────────────────────

type productWired struct {
	createCmd *productcommands.CreateProductHandler
	updateCmd *productcommands.UpdateProductHandler
	deleteCmd *productcommands.DeleteProductHandler
	listQuery *productqueries.ListProductsHandler
	getQuery  *productqueries.GetProductHandler
	routes    infrahttp.ModuleRoutes
}

func wireProduct(gdb *gormsqlite.DB, pub events.EventPublisher, sub events.EventSubscriber, cfg Config) *productWired {
	productRepo := productpersistence.NewGormProductRepository(gdb)

	createProductCmd := productcommands.NewCreateProductHandler(productRepo, pub)
	updateProductCmd := productcommands.NewUpdateProductHandler(productRepo, pub)
	deleteProductCmd := productcommands.NewDeleteProductHandler(productRepo, pub)
	listProductsQuery := productqueries.NewListProductsHandler(gdb)
	getProductQuery   := productqueries.NewGetProductHandler(gdb)

	var routes infrahttp.ModuleRoutes
	if cfg.IsEnabled("product") {
		routes = producthttp.NewHandlers(
			createProductCmd, updateProductCmd, deleteProductCmd,
			listProductsQuery, getProductQuery, sub,
		)
		log.Printf("module enabled: product")
	}

	return &productWired{
		createCmd: createProductCmd,
		updateCmd: updateProductCmd,
		deleteCmd: deleteProductCmd,
		listQuery: listProductsQuery,
		getQuery:  getProductQuery,
		routes:    routes,
	}
}
