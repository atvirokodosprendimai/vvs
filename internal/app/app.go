package app

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/vvs/isp/internal/infrastructure/database"
	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
	infrahttp "github.com/vvs/isp/internal/infrastructure/http"
	infranats "github.com/vvs/isp/internal/infrastructure/nats"
	"github.com/vvs/isp/internal/shared/events"

	customerhttp "github.com/vvs/isp/internal/modules/customer/adapters/http"
	customerpersistence "github.com/vvs/isp/internal/modules/customer/adapters/persistence"
	customercommands "github.com/vvs/isp/internal/modules/customer/app/commands"
	customerqueries "github.com/vvs/isp/internal/modules/customer/app/queries"
	customerdomain "github.com/vvs/isp/internal/modules/customer/domain"
	customermigrations "github.com/vvs/isp/internal/modules/customer/migrations"

	producthttp "github.com/vvs/isp/internal/modules/product/adapters/http"
	productpersistence "github.com/vvs/isp/internal/modules/product/adapters/persistence"
	productcommands "github.com/vvs/isp/internal/modules/product/app/commands"
	productqueries "github.com/vvs/isp/internal/modules/product/app/queries"
	productmigrations "github.com/vvs/isp/internal/modules/product/migrations"

	authhttp "github.com/vvs/isp/internal/modules/auth/adapters/http"
	authpersistence "github.com/vvs/isp/internal/modules/auth/adapters/persistence"
	authcommands "github.com/vvs/isp/internal/modules/auth/app/commands"
	authqueries "github.com/vvs/isp/internal/modules/auth/app/queries"
	"github.com/vvs/isp/internal/modules/auth/domain"
	authmigrations "github.com/vvs/isp/internal/modules/auth/migrations"

	networkhttp "github.com/vvs/isp/internal/modules/network/adapters/http"
	networkpersistence "github.com/vvs/isp/internal/modules/network/adapters/persistence"
	networkcommands "github.com/vvs/isp/internal/modules/network/app/commands"
	networkqueries "github.com/vvs/isp/internal/modules/network/app/queries"
	networksubscribers "github.com/vvs/isp/internal/modules/network/app/subscribers"
	networkmigrations "github.com/vvs/isp/internal/modules/network/migrations"

	"github.com/vvs/isp/internal/infrastructure/arista"
	"github.com/vvs/isp/internal/infrastructure/chat"
	chatmigrations "github.com/vvs/isp/internal/infrastructure/chat/migrations"
	"github.com/vvs/isp/internal/infrastructure/mikrotik"
	"github.com/vvs/isp/internal/infrastructure/netbox"
	"github.com/vvs/isp/internal/infrastructure/notifications"
	notifmigrations "github.com/vvs/isp/internal/infrastructure/notifications/migrations"
	networkdomain "github.com/vvs/isp/internal/modules/network/domain"

	servicehttp "github.com/vvs/isp/internal/modules/service/adapters/http"
	servicepersistence "github.com/vvs/isp/internal/modules/service/adapters/persistence"
	servicecommands "github.com/vvs/isp/internal/modules/service/app/commands"
	servicequeries "github.com/vvs/isp/internal/modules/service/app/queries"
	servicemigrations "github.com/vvs/isp/internal/modules/service/migrations"

	natsrpc "github.com/vvs/isp/internal/infrastructure/nats/rpc"

	devicehttp "github.com/vvs/isp/internal/modules/device/adapters/http"
	devicepersistence "github.com/vvs/isp/internal/modules/device/adapters/persistence"
	devicecommands "github.com/vvs/isp/internal/modules/device/app/commands"
	devicequeries "github.com/vvs/isp/internal/modules/device/app/queries"
	devicemigrations "github.com/vvs/isp/internal/modules/device/migrations"

	cronhttp "github.com/vvs/isp/internal/modules/cron/adapters/http"
	cronpersistence "github.com/vvs/isp/internal/modules/cron/adapters/persistence"
	croncommands "github.com/vvs/isp/internal/modules/cron/app/commands"
	cronqueries "github.com/vvs/isp/internal/modules/cron/app/queries"
	cronmigrations "github.com/vvs/isp/internal/modules/cron/migrations"
)

type App struct {
	DB         *gormsqlite.DB
	NATSServer *natsserver.Server
	NATSConn   *nats.Conn
	Publisher  events.EventPublisher
	Subscriber events.EventSubscriber
	HTTPServer *infrahttp.Server
	RPCServer  *natsrpc.Server
}

func New(cfg Config) (*App, error) {
	// 1. Database — single gormsqlite.DB for the whole app
	gdb, err := gormsqlite.Open(cfg.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// 2. Run migrations for ALL modules regardless of enabled flags
	sqlDB, err := gdb.W.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB: %w", err)
	}
	if err := database.RunModuleMigrations(sqlDB, []database.ModuleMigration{
		{Name: "auth", FS: authmigrations.FS, TableName: "goose_auth"},
		{Name: "customer", FS: customermigrations.FS, TableName: "goose_customer"},
		{Name: "product", FS: productmigrations.FS, TableName: "goose_product"},
		{Name: "network", FS: networkmigrations.FS, TableName: "goose_network"},
		{Name: "notifications", FS: notifmigrations.FS, TableName: "goose_notifications"},
		{Name: "chat", FS: chatmigrations.FS, TableName: "goose_chat"},
		{Name: "service", FS: servicemigrations.FS, TableName: "goose_service"},
		{Name: "device", FS: devicemigrations.FS, TableName: "goose_device"},
		{Name: "cron", FS: cronmigrations.FS, TableName: "goose_cron"},
	}); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	// 3. NATS — embedded or external
	var ns *natsserver.Server
	var nc *nats.Conn
	if cfg.NATSUrl != "" {
		nc, err = infranats.ConnectExternal(cfg.NATSUrl)
		if err != nil {
			return nil, fmt.Errorf("connect nats: %w", err)
		}
		log.Printf("NATS connected to external server: %s", cfg.NATSUrl)
	} else {
		ns, nc, err = infranats.StartEmbedded(cfg.NATSListenAddr)
		if err != nil {
			return nil, fmt.Errorf("start nats: %w", err)
		}
		if cfg.NATSListenAddr != "" {
			log.Printf("NATS embedded, listening on %s", cfg.NATSListenAddr)
		}
	}

	publisher := infranats.NewPublisher(nc)
	subscriber := infranats.NewSubscriber(nc)

	// 4. Auth module — always wired (session middleware depends on it)
	userRepo := authpersistence.NewGormUserRepository(gdb)
	sessionRepo := authpersistence.NewGormSessionRepository(gdb)
	loginCmd := authcommands.NewLoginHandler(userRepo, sessionRepo)
	logoutCmd := authcommands.NewLogoutHandler(sessionRepo)
	createUserCmd := authcommands.NewCreateUserHandler(userRepo)
	deleteUserCmd := authcommands.NewDeleteUserHandler(userRepo, sessionRepo)
	listUsersQuery := authqueries.NewListUsersHandler(userRepo)
	getCurrentUserQuery := authqueries.NewGetCurrentUserHandler(userRepo, sessionRepo)
	authRoutes := authhttp.NewHandlers(loginCmd, logoutCmd, createUserCmd, deleteUserCmd, listUsersQuery, getCurrentUserQuery)

	if cfg.AdminUser != "" && cfg.AdminPassword != "" {
		if err := seedAdmin(context.Background(), userRepo, cfg.AdminUser, cfg.AdminPassword); err != nil {
			log.Printf("warn: seed admin: %v", err)
		}
	}

	var moduleRoutes []infrahttp.ModuleRoutes
	moduleRoutes = append(moduleRoutes, authRoutes)

	// 5. Service repository — wired early so customer detail page can list services
	serviceRepo := servicepersistence.NewGormServiceRepository(gdb)
	listServicesQuery := servicequeries.NewListServicesForCustomerHandler(serviceRepo)

	// 6. Customer module
	customerRepo := customerpersistence.NewGormCustomerRepository(gdb)
	createCustomerCmd := customercommands.NewCreateCustomerHandler(customerRepo, publisher)
	updateCustomerCmd := customercommands.NewUpdateCustomerHandler(customerRepo, publisher)
	deleteCustomerCmd := customercommands.NewDeleteCustomerHandler(customerRepo, publisher)
	listCustomersQuery := customerqueries.NewListCustomersHandler(gdb)
	getCustomerQuery := customerqueries.NewGetCustomerHandler(gdb)

	var customerRoutes *customerhttp.Handlers
	if cfg.IsEnabled("customer") {
		customerRoutes = customerhttp.NewHandlers(
			createCustomerCmd, updateCustomerCmd, deleteCustomerCmd,
			listCustomersQuery, getCustomerQuery, subscriber, publisher,
			listServicesQuery,
		)
		moduleRoutes = append(moduleRoutes, customerRoutes)
		log.Printf("module enabled: customer")
	}

	// 6. Product module
	productRepo := productpersistence.NewGormProductRepository(gdb)
	createProductCmd := productcommands.NewCreateProductHandler(productRepo, publisher)
	updateProductCmd := productcommands.NewUpdateProductHandler(productRepo, publisher)
	deleteProductCmd := productcommands.NewDeleteProductHandler(productRepo, publisher)
	listProductsQuery := productqueries.NewListProductsHandler(gdb)
	getProductQuery := productqueries.NewGetProductHandler(gdb)

	if cfg.IsEnabled("product") {
		productRoutes := producthttp.NewHandlers(
			createProductCmd, updateProductCmd, deleteProductCmd,
			listProductsQuery, getProductQuery, subscriber,
		)
		moduleRoutes = append(moduleRoutes, productRoutes)
		log.Printf("module enabled: product")
	}

	// 7. Network module
	routerRepo := networkpersistence.NewGormRouterRepository(gdb)
	createRouterCmd := networkcommands.NewCreateRouterHandler(routerRepo, publisher)
	updateRouterCmd := networkcommands.NewUpdateRouterHandler(routerRepo, publisher)
	deleteRouterCmd := networkcommands.NewDeleteRouterHandler(routerRepo, publisher)
	listRoutersQuery := networkqueries.NewListRoutersHandler(routerRepo)
	getRouterQuery := networkqueries.NewGetRouterHandler(routerRepo)

	provisioner := &provisionerDispatcher{
		mikrotik: mikrotik.New(),
		arista:   arista.New(),
	}
	var ipamProvider networkdomain.IPAMProvider
	if cfg.NetBoxURL != "" && cfg.NetBoxToken != "" {
		ipamProvider = netbox.New(cfg.NetBoxURL, cfg.NetBoxToken)
		log.Printf("NetBox IPAM configured: %s", cfg.NetBoxURL)
	}
	syncARPCmd := networkcommands.NewSyncCustomerARPHandler(
		&customerARPBridge{repo: customerRepo}, routerRepo, provisioner, ipamProvider, publisher,
	)

	if cfg.IsEnabled("network") {
		if customerRoutes != nil {
			customerRoutes.WithReader(gdb.R)
		}

		networkRoutes := networkhttp.NewHandlers(
			createRouterCmd, updateRouterCmd, deleteRouterCmd,
			listRoutersQuery, getRouterQuery, syncARPCmd, subscriber,
		)
		moduleRoutes = append(moduleRoutes, networkRoutes)

		arpWorker := networksubscribers.NewARPWorker(syncARPCmd)
		go arpWorker.Run(context.Background(), subscriber)

		log.Printf("module enabled: network")
	}

	// 8. Device module
	cronRepo := cronpersistence.NewGormJobRepository(gdb)
	deviceRepo := devicepersistence.NewGormDeviceRepository(gdb)
	registerDeviceCmd := devicecommands.NewRegisterDeviceHandler(deviceRepo, publisher)
	deployDeviceCmd := devicecommands.NewDeployDeviceHandler(deviceRepo, publisher)
	returnDeviceCmd := devicecommands.NewReturnDeviceHandler(deviceRepo, publisher)
	decommissionDeviceCmd := devicecommands.NewDecommissionDeviceHandler(deviceRepo, publisher)
	updateDeviceCmd := devicecommands.NewUpdateDeviceHandler(deviceRepo, publisher)
	listDevicesQuery := devicequeries.NewListDevicesHandler(gdb)
	getDeviceQuery := devicequeries.NewGetDeviceHandler(gdb)

	if cfg.IsEnabled("device") {
		deviceRoutes := devicehttp.NewDeviceHandlers(
			registerDeviceCmd, deployDeviceCmd, returnDeviceCmd,
			decommissionDeviceCmd, updateDeviceCmd,
			listDevicesQuery, getDeviceQuery,
			subscriber, publisher,
		)
		moduleRoutes = append(moduleRoutes, deviceRoutes)
		log.Printf("module enabled: device")
	}

	// 9. Service module — commands + route registration
	assignServiceCmd := servicecommands.NewAssignServiceHandler(serviceRepo, publisher)
	suspendServiceCmd := servicecommands.NewSuspendServiceHandler(serviceRepo, publisher)
	reactivateServiceCmd := servicecommands.NewReactivateServiceHandler(serviceRepo, publisher)
	cancelServiceCmd := servicecommands.NewCancelServiceHandler(serviceRepo, publisher)

	if cfg.IsEnabled("service") {
		serviceRoutes := servicehttp.NewServiceHandlers(
			assignServiceCmd, suspendServiceCmd, reactivateServiceCmd, cancelServiceCmd,
			listServicesQuery, subscriber, publisher,
		)
		moduleRoutes = append(moduleRoutes, serviceRoutes)
		log.Printf("module enabled: service")
	}

	// 10. Notifications — global cross-cutting concern
	notifStore := notifications.NewStore(gdb)
	notifWorker := notifications.NewWorker(notifStore, publisher)
	go notifWorker.Run(context.Background(), subscriber)
	notifHandler := infrahttp.NewNotifHandler(notifStore, subscriber)

	// 11. Chat
	chatStore := chat.NewStore(gdb)
	if err := seedGeneralChannel(context.Background(), chatStore); err != nil {
		log.Printf("warn: seed #general channel: %v", err)
	}
	chatHandler := infrahttp.NewChatHandler(chatStore, subscriber, publisher)
	globalHandler := infrahttp.NewGlobalHandler(notifStore, chatStore, subscriber)

	// 12. NATS RPC server — request/reply for all core functions
	rpcServer := natsrpc.New(nc, natsrpc.Config{
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

		ListJobs:  cronqueries.NewListJobsHandler(cronRepo),
		GetJob:    cronqueries.NewGetJobHandler(cronRepo),
		AddJob:    croncommands.NewAddJobHandler(cronRepo),
		PauseJob:  croncommands.NewPauseJobHandler(cronRepo),
		ResumeJob: croncommands.NewResumeJobHandler(cronRepo),
		DeleteJob: croncommands.NewDeleteJobHandler(cronRepo),
	})

	// Cron web UI (always enabled)
	cronRoutes := cronhttp.NewCronHandlers(
		cronqueries.NewListJobsHandler(cronRepo),
		croncommands.NewAddJobHandler(cronRepo),
		croncommands.NewPauseJobHandler(cronRepo),
		croncommands.NewResumeJobHandler(cronRepo),
		croncommands.NewDeleteJobHandler(cronRepo),
	)
	moduleRoutes = append(moduleRoutes, cronRoutes)
	if err := rpcServer.Register(); err != nil {
		return nil, fmt.Errorf("nats rpc: %w", err)
	}

	// 13. HTTP router — pass gdb.R to dashboard handler
	router := infrahttp.NewRouter(gdb.R, getCurrentUserQuery, notifHandler, chatHandler, globalHandler, cfg.APIToken, rpcServer, moduleRoutes...)
	httpServer := infrahttp.NewServer(cfg.ListenAddr, router)

	enabled := cfg.EnabledModules
	if len(enabled) == 0 {
		enabled = []string{"all"}
	}
	log.Printf("VVS ISP Manager initialized (db: %s, modules: %v)", cfg.DatabasePath, enabled)

	return &App{
		DB:         gdb,
		NATSServer: ns,
		NATSConn:   nc,
		Publisher:  publisher,
		Subscriber: subscriber,
		HTTPServer: httpServer,
		RPCServer:  rpcServer,
	}, nil
}

func (a *App) Start() error {
	return a.HTTPServer.Start()
}

func (a *App) Shutdown(ctx context.Context) error {
	err := a.HTTPServer.Shutdown(ctx)
	if a.RPCServer != nil {
		a.RPCServer.Close()
	}
	if a.Subscriber != nil {
		a.Subscriber.Close()
	}
	a.NATSConn.Close()
	if a.NATSServer != nil {
		a.NATSServer.WaitForShutdown()
	}
	_ = a.DB.Close()
	return err
}

// customerARPBridge adapts the customer repository to the network module's
// CustomerARPProvider interface. Lives here (composition root) so the network
// module does not import the customer domain package.
type customerARPBridge struct {
	repo customerdomain.CustomerRepository
}

func (b *customerARPBridge) FindARPData(ctx context.Context, id string) (networkdomain.CustomerARPData, error) {
	c, err := b.repo.FindByID(ctx, id)
	if err != nil {
		return networkdomain.CustomerARPData{}, err
	}
	return networkdomain.CustomerARPData{
		ID:         c.ID,
		Code:       c.Code.String(),
		RouterID:   c.RouterID,
		IPAddress:  c.IPAddress,
		MACAddress: c.MACAddress,
	}, nil
}

func (b *customerARPBridge) UpdateNetworkInfo(ctx context.Context, id, routerID, ip, mac string) error {
	c, err := b.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	c.SetNetworkInfo(routerID, ip, mac)
	return b.repo.Save(ctx, c)
}

// provisionerDispatcher picks the right RouterProvisioner based on conn.RouterType.
// Lives here (composition root) so neither infrastructure package imports the other.
type provisionerDispatcher struct {
	mikrotik networkdomain.RouterProvisioner
	arista   networkdomain.RouterProvisioner
}

func (d *provisionerDispatcher) SetARPStatic(ctx context.Context, conn networkdomain.RouterConn, ip, mac, customerID string) error {
	return d.pick(conn).SetARPStatic(ctx, conn, ip, mac, customerID)
}

func (d *provisionerDispatcher) DisableARP(ctx context.Context, conn networkdomain.RouterConn, ip string) error {
	return d.pick(conn).DisableARP(ctx, conn, ip)
}

func (d *provisionerDispatcher) GetARPEntry(ctx context.Context, conn networkdomain.RouterConn, ip string) (*networkdomain.ARPEntry, error) {
	return d.pick(conn).GetARPEntry(ctx, conn, ip)
}

func (d *provisionerDispatcher) pick(conn networkdomain.RouterConn) networkdomain.RouterProvisioner {
	if conn.RouterType == networkdomain.RouterTypeArista {
		return d.arista
	}
	return d.mikrotik
}

// seedGeneralChannel ensures the #general channel exists.
func seedGeneralChannel(ctx context.Context, store *chat.Store) error {
	exists, err := store.ThreadExists(ctx, "general")
	if err != nil || exists {
		return err
	}
	return store.CreateThread(ctx, chat.Thread{
		ID:        "general",
		Type:      "channel",
		Name:      "#general",
		IsPrivate: false,
		CreatedBy: "system",
		CreatedAt: time.Now().UTC(),
	})
}

// seedAdmin creates or updates the admin user on startup.
func seedAdmin(ctx context.Context, users domain.UserRepository, username, password string) error {
	existing, err := users.FindByUsername(ctx, username)
	if err == nil {
		if err := existing.ChangePassword(password); err != nil {
			return err
		}
		return users.Save(ctx, existing)
	}
	u, err := domain.NewUser(username, password, domain.RoleAdmin)
	if err != nil {
		return err
	}
	return users.Save(ctx, u)
}
