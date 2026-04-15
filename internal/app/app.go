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

	"github.com/vvs/isp/internal/infrastructure/mikrotik"
	"github.com/vvs/isp/internal/infrastructure/netbox"
	networkdomain "github.com/vvs/isp/internal/modules/network/domain"
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

	// 2. Run migrations for ALL modules regardless of enabled flags
	//    (schema must stay consistent even if a module is not serving routes)
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB: %w", err)
	}

	if err := database.RunModuleMigrations(sqlDB, []database.ModuleMigration{
		{Name: "auth", FS: authmigrations.FS, TableName: "goose_auth"},
		{Name: "customer", FS: customermigrations.FS, TableName: "goose_customer"},
		{Name: "product", FS: productmigrations.FS, TableName: "goose_product"},
		{Name: "network", FS: networkmigrations.FS, TableName: "goose_network"},
	}); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	// 3. Write serializer
	writer := database.NewWriteSerializer(db)

	// 4. NATS — embedded or external
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

	// 5. Auth module — always wired (session middleware depends on it)
	userRepo := authpersistence.NewGormUserRepository(writer, reader)
	sessionRepo := authpersistence.NewGormSessionRepository(writer, reader)
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

	// Module routes collected here; only enabled modules are added
	var moduleRoutes []infrahttp.ModuleRoutes
	moduleRoutes = append(moduleRoutes, authRoutes) // auth always mounted

	// 6. Customer module
	customerRepo := customerpersistence.NewGormCustomerRepository(writer, reader)
	createCustomerCmd := customercommands.NewCreateCustomerHandler(customerRepo, publisher)
	updateCustomerCmd := customercommands.NewUpdateCustomerHandler(customerRepo, publisher)
	deleteCustomerCmd := customercommands.NewDeleteCustomerHandler(customerRepo, publisher)
	listCustomersQuery := customerqueries.NewListCustomersHandler(reader)
	getCustomerQuery := customerqueries.NewGetCustomerHandler(reader)

	var customerRoutes *customerhttp.Handlers
	if cfg.IsEnabled("customer") {
		customerRoutes = customerhttp.NewHandlers(
			createCustomerCmd, updateCustomerCmd, deleteCustomerCmd,
			listCustomersQuery, getCustomerQuery, subscriber, publisher,
		)
		moduleRoutes = append(moduleRoutes, customerRoutes)
		log.Printf("module enabled: customer")
	}

	// 7. Product module
	productRepo := productpersistence.NewGormProductRepository(writer, reader)
	createProductCmd := productcommands.NewCreateProductHandler(productRepo, publisher)
	updateProductCmd := productcommands.NewUpdateProductHandler(productRepo, publisher)
	deleteProductCmd := productcommands.NewDeleteProductHandler(productRepo, publisher)
	listProductsQuery := productqueries.NewListProductsHandler(reader)
	getProductQuery := productqueries.NewGetProductHandler(reader)

	if cfg.IsEnabled("product") {
		productRoutes := producthttp.NewHandlers(
			createProductCmd, updateProductCmd, deleteProductCmd,
			listProductsQuery, getProductQuery, subscriber,
		)
		moduleRoutes = append(moduleRoutes, productRoutes)
		log.Printf("module enabled: product")
	}

	// 8. Network module
	routerRepo := networkpersistence.NewGormRouterRepository(writer, reader)
	createRouterCmd := networkcommands.NewCreateRouterHandler(routerRepo, publisher)
	updateRouterCmd := networkcommands.NewUpdateRouterHandler(routerRepo, publisher)
	deleteRouterCmd := networkcommands.NewDeleteRouterHandler(routerRepo)
	listRoutersQuery := networkqueries.NewListRoutersHandler(routerRepo)
	getRouterQuery := networkqueries.NewGetRouterHandler(routerRepo)

	if cfg.IsEnabled("network") {
		networkRoutes := networkhttp.NewHandlers(
			createRouterCmd, updateRouterCmd, deleteRouterCmd,
			listRoutersQuery, getRouterQuery, subscriber,
		)
		moduleRoutes = append(moduleRoutes, networkRoutes)

		// Customer form router dropdown (only when both modules are active)
		if customerRoutes != nil {
			customerRoutes.WithReader(reader)
		}

		mikrotikClient := mikrotik.New()

		var ipamProvider networkdomain.IPAMProvider
		if cfg.NetBoxURL != "" && cfg.NetBoxToken != "" {
			ipamProvider = netbox.New(cfg.NetBoxURL, cfg.NetBoxToken)
			log.Printf("NetBox IPAM configured: %s", cfg.NetBoxURL)
		}

		syncARPCmd := networkcommands.NewSyncCustomerARPHandler(
			customerRepo, routerRepo, mikrotikClient, ipamProvider, publisher,
		)

		arpWorker := networksubscribers.NewARPWorker(syncARPCmd)
		go arpWorker.Run(context.Background(), subscriber)

		log.Printf("module enabled: network")
	}

	// 9. HTTP router
	router := infrahttp.NewRouter(reader, getCurrentUserQuery, moduleRoutes...)
	httpServer := infrahttp.NewServer(cfg.ListenAddr, router)

	enabled := cfg.EnabledModules
	if len(enabled) == 0 {
		enabled = []string{"all"}
	}
	log.Printf("VVS ISP Manager initialized (db: %s, modules: %v)", cfg.DatabasePath, enabled)

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
	if a.NATSServer != nil {
		a.NATSServer.WaitForShutdown()
	}
	return err
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
