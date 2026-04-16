package app

import (
	"context"
	"fmt"

	"github.com/vvs/isp/internal/infrastructure/arista"
	"github.com/vvs/isp/internal/infrastructure/database"
	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
	"github.com/vvs/isp/internal/infrastructure/mikrotik"
	natsrpc "github.com/vvs/isp/internal/infrastructure/nats/rpc"
	"github.com/vvs/isp/internal/shared/events"

	authpersistence "github.com/vvs/isp/internal/modules/auth/adapters/persistence"
	authcommands "github.com/vvs/isp/internal/modules/auth/app/commands"
	authqueries "github.com/vvs/isp/internal/modules/auth/app/queries"
	authmigrations "github.com/vvs/isp/internal/modules/auth/migrations"

	customerpersistence "github.com/vvs/isp/internal/modules/customer/adapters/persistence"
	customercommands "github.com/vvs/isp/internal/modules/customer/app/commands"
	customerqueries "github.com/vvs/isp/internal/modules/customer/app/queries"
	customerdomain "github.com/vvs/isp/internal/modules/customer/domain"
	customermigrations "github.com/vvs/isp/internal/modules/customer/migrations"

	networkpersistence "github.com/vvs/isp/internal/modules/network/adapters/persistence"
	networkcommands "github.com/vvs/isp/internal/modules/network/app/commands"
	networkqueries "github.com/vvs/isp/internal/modules/network/app/queries"
	networkdomain "github.com/vvs/isp/internal/modules/network/domain"
	networkmigrations "github.com/vvs/isp/internal/modules/network/migrations"

	productpersistence "github.com/vvs/isp/internal/modules/product/adapters/persistence"
	productcommands "github.com/vvs/isp/internal/modules/product/app/commands"
	productqueries "github.com/vvs/isp/internal/modules/product/app/queries"
	productmigrations "github.com/vvs/isp/internal/modules/product/migrations"

	servicepersistence "github.com/vvs/isp/internal/modules/service/adapters/persistence"
	servicecommands "github.com/vvs/isp/internal/modules/service/app/commands"
	servicequeries "github.com/vvs/isp/internal/modules/service/app/queries"
	servicemigrations "github.com/vvs/isp/internal/modules/service/migrations"

	devicepersistence "github.com/vvs/isp/internal/modules/device/adapters/persistence"
	devicecommands "github.com/vvs/isp/internal/modules/device/app/commands"
	devicequeries "github.com/vvs/isp/internal/modules/device/app/queries"
	devicemigrations "github.com/vvs/isp/internal/modules/device/migrations"

	cronpersistence "github.com/vvs/isp/internal/modules/cron/adapters/persistence"
	croncommands "github.com/vvs/isp/internal/modules/cron/app/commands"
	cronqueries "github.com/vvs/isp/internal/modules/cron/app/queries"
	cronmigrations "github.com/vvs/isp/internal/modules/cron/migrations"
)

// noopPublisher discards all events. Used by direct CLI mode where no NATS is running.
type noopPublisher struct{}

func (noopPublisher) Publish(_ context.Context, _ string, _ events.DomainEvent) error { return nil }

// NewDirect opens the database, runs migrations, and wires all command/query handlers
// without starting any servers. Returns an RPCDispatcher for use by the CLI.
func NewDirect(dbPath string) (*natsrpc.Server, func(), error) {
	gdb, err := gormsqlite.Open(dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open database: %w", err)
	}
	cleanup := func() { _ = gdb.Close() }

	sqlDB, err := gdb.W.DB()
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("get sql.DB: %w", err)
	}
	if err := database.RunModuleMigrations(sqlDB, []database.ModuleMigration{
		{Name: "auth", FS: authmigrations.FS, TableName: "goose_auth"},
		{Name: "customer", FS: customermigrations.FS, TableName: "goose_customer"},
		{Name: "product", FS: productmigrations.FS, TableName: "goose_product"},
		{Name: "network", FS: networkmigrations.FS, TableName: "goose_network"},
		{Name: "service", FS: servicemigrations.FS, TableName: "goose_service"},
		{Name: "device", FS: devicemigrations.FS, TableName: "goose_device"},
		{Name: "cron", FS: cronmigrations.FS, TableName: "goose_cron"},
	}); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("run migrations: %w", err)
	}

	pub := noopPublisher{}

	// auth
	userRepo := authpersistence.NewGormUserRepository(gdb)
	sessionRepo := authpersistence.NewGormSessionRepository(gdb)
	createUserCmd := authcommands.NewCreateUserHandler(userRepo)
	deleteUserCmd := authcommands.NewDeleteUserHandler(userRepo, sessionRepo)
	listUsersQuery := authqueries.NewListUsersHandler(userRepo)

	// customer
	customerRepo := customerpersistence.NewGormCustomerRepository(gdb)
	createCustomerCmd := customercommands.NewCreateCustomerHandler(customerRepo, pub)
	updateCustomerCmd := customercommands.NewUpdateCustomerHandler(customerRepo, pub)
	deleteCustomerCmd := customercommands.NewDeleteCustomerHandler(customerRepo, pub)
	listCustomersQuery := customerqueries.NewListCustomersHandler(gdb)
	getCustomerQuery := customerqueries.NewGetCustomerHandler(gdb)

	// product
	productRepo := productpersistence.NewGormProductRepository(gdb)
	createProductCmd := productcommands.NewCreateProductHandler(productRepo, pub)
	updateProductCmd := productcommands.NewUpdateProductHandler(productRepo, pub)
	deleteProductCmd := productcommands.NewDeleteProductHandler(productRepo, pub)
	listProductsQuery := productqueries.NewListProductsHandler(gdb)
	getProductQuery := productqueries.NewGetProductHandler(gdb)

	// network
	routerRepo := networkpersistence.NewGormRouterRepository(gdb)
	createRouterCmd := networkcommands.NewCreateRouterHandler(routerRepo, pub)
	updateRouterCmd := networkcommands.NewUpdateRouterHandler(routerRepo, pub)
	deleteRouterCmd := networkcommands.NewDeleteRouterHandler(routerRepo, pub)
	listRoutersQuery := networkqueries.NewListRoutersHandler(routerRepo)
	getRouterQuery := networkqueries.NewGetRouterHandler(routerRepo)
	syncARPCmd := networkcommands.NewSyncCustomerARPHandler(
		&customerARPBridge{repo: customerRepo},
		routerRepo,
		&provisionerDispatcher{mikrotik: mikrotik.New(), arista: arista.New()},
		nil, // no IPAM in direct mode
		pub,
	)

	// service
	serviceRepo := servicepersistence.NewGormServiceRepository(gdb)
	assignServiceCmd := servicecommands.NewAssignServiceHandler(serviceRepo, pub)
	suspendServiceCmd := servicecommands.NewSuspendServiceHandler(serviceRepo, pub)
	reactivateServiceCmd := servicecommands.NewReactivateServiceHandler(serviceRepo, pub)
	cancelServiceCmd := servicecommands.NewCancelServiceHandler(serviceRepo, pub)
	listServicesQuery := servicequeries.NewListServicesForCustomerHandler(serviceRepo)

	// device
	deviceRepo := devicepersistence.NewGormDeviceRepository(gdb)
	registerDeviceCmd := devicecommands.NewRegisterDeviceHandler(deviceRepo, pub)
	deployDeviceCmd := devicecommands.NewDeployDeviceHandler(deviceRepo, pub)
	returnDeviceCmd := devicecommands.NewReturnDeviceHandler(deviceRepo, pub)
	decommissionDeviceCmd := devicecommands.NewDecommissionDeviceHandler(deviceRepo, pub)
	updateDeviceCmd := devicecommands.NewUpdateDeviceHandler(deviceRepo, pub)
	listDevicesQuery := devicequeries.NewListDevicesHandler(gdb)
	getDeviceQuery := devicequeries.NewGetDeviceHandler(gdb)

	// cron
	cronRepo := cronpersistence.NewGormJobRepository(gdb)
	addJobCmd := croncommands.NewAddJobHandler(cronRepo)
	pauseJobCmd := croncommands.NewPauseJobHandler(cronRepo)
	resumeJobCmd := croncommands.NewResumeJobHandler(cronRepo)
	deleteJobCmd := croncommands.NewDeleteJobHandler(cronRepo)
	listJobsQuery := cronqueries.NewListJobsHandler(cronRepo)
	getJobQuery := cronqueries.NewGetJobHandler(cronRepo)

	rpcServer := natsrpc.New(nil, natsrpc.Config{
		ListUsers:  listUsersQuery,
		CreateUser: createUserCmd,
		DeleteUser: deleteUserCmd,

		ListCustomers:  listCustomersQuery,
		GetCustomer:    getCustomerQuery,
		CreateCustomer: createCustomerCmd,
		UpdateCustomer: updateCustomerCmd,
		DeleteCustomer: deleteCustomerCmd,

		ListProducts:  listProductsQuery,
		GetProduct:    getProductQuery,
		CreateProduct: createProductCmd,
		UpdateProduct: updateProductCmd,
		DeleteProduct: deleteProductCmd,

		ListRouters:  listRoutersQuery,
		GetRouter:    getRouterQuery,
		CreateRouter: createRouterCmd,
		UpdateRouter: updateRouterCmd,
		DeleteRouter: deleteRouterCmd,
		SyncARP:      syncARPCmd,

		ListServices:      listServicesQuery,
		AssignService:     assignServiceCmd,
		SuspendService:    suspendServiceCmd,
		ReactivateService: reactivateServiceCmd,
		CancelService:     cancelServiceCmd,

		ListDevices:        listDevicesQuery,
		GetDevice:          getDeviceQuery,
		RegisterDevice:     registerDeviceCmd,
		DeployDevice:       deployDeviceCmd,
		ReturnDevice:       returnDeviceCmd,
		DecommissionDevice: decommissionDeviceCmd,
		UpdateDevice:       updateDeviceCmd,

		ListJobs:  listJobsQuery,
		GetJob:    getJobQuery,
		AddJob:    addJobCmd,
		PauseJob:  pauseJobCmd,
		ResumeJob: resumeJobCmd,
		DeleteJob: deleteJobCmd,
	})

	// Note: Register() (NATS subscriptions) is intentionally not called —
	// Dispatch() works without NATS and is all the CLI needs.
	return rpcServer, cleanup, nil
}

// Ensure customerdomain import is used (bridge struct references it).
var _ customerdomain.CustomerRepository = (*customerpersistence.GormCustomerRepository)(nil)
var _ networkdomain.RouterProvisioner = (*mikrotik.Client)(nil)
