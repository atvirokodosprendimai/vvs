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
	networkservices "github.com/vvs/isp/internal/modules/network/app/services"
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
	servicedomain "github.com/vvs/isp/internal/modules/service/domain"
	servicemigrations "github.com/vvs/isp/internal/modules/service/migrations"

	contacthttp "github.com/vvs/isp/internal/modules/contact/adapters/http"
	contactpersistence "github.com/vvs/isp/internal/modules/contact/adapters/persistence"
	contactcommands "github.com/vvs/isp/internal/modules/contact/app/commands"
	contactqueries "github.com/vvs/isp/internal/modules/contact/app/queries"
	contactmigrations "github.com/vvs/isp/internal/modules/contact/migrations"

	dealhttp "github.com/vvs/isp/internal/modules/deal/adapters/http"
	dealpersistence "github.com/vvs/isp/internal/modules/deal/adapters/persistence"
	dealcommands "github.com/vvs/isp/internal/modules/deal/app/commands"
	dealqueries "github.com/vvs/isp/internal/modules/deal/app/queries"
	dealmigrations "github.com/vvs/isp/internal/modules/deal/migrations"

	tickethttp "github.com/vvs/isp/internal/modules/ticket/adapters/http"
	ticketpersistence "github.com/vvs/isp/internal/modules/ticket/adapters/persistence"
	ticketcommands "github.com/vvs/isp/internal/modules/ticket/app/commands"
	ticketqueries "github.com/vvs/isp/internal/modules/ticket/app/queries"
	ticketmigrations "github.com/vvs/isp/internal/modules/ticket/migrations"

	taskhttp "github.com/vvs/isp/internal/modules/task/adapters/http"
	taskpersistence "github.com/vvs/isp/internal/modules/task/adapters/persistence"
	taskcommands "github.com/vvs/isp/internal/modules/task/app/commands"
	taskqueries "github.com/vvs/isp/internal/modules/task/app/queries"
	taskmigrations "github.com/vvs/isp/internal/modules/task/migrations"

	natsrpc "github.com/vvs/isp/internal/infrastructure/nats/rpc"

	"github.com/google/uuid"

	emailhttp "github.com/vvs/isp/internal/modules/email/adapters/http"
	emailpersistence "github.com/vvs/isp/internal/modules/email/adapters/persistence"
	smtpAdapter "github.com/vvs/isp/internal/modules/email/adapters/smtp"
	emailcommands "github.com/vvs/isp/internal/modules/email/app/commands"
	emailqueries "github.com/vvs/isp/internal/modules/email/app/queries"
	emailmigrations "github.com/vvs/isp/internal/modules/email/migrations"
	"github.com/vvs/isp/internal/modules/email/worker"
	imapAdapter "github.com/vvs/isp/internal/modules/email/adapters/imap"

	invoicedomain "github.com/vvs/isp/internal/modules/invoice/domain"
	invoicehttp "github.com/vvs/isp/internal/modules/invoice/adapters/http"
	paymenthttp "github.com/vvs/isp/internal/modules/payment/adapters/http"
	paymentcommands "github.com/vvs/isp/internal/modules/payment/app/commands"
	invoicepersistence "github.com/vvs/isp/internal/modules/invoice/adapters/persistence"
	invoicecommands "github.com/vvs/isp/internal/modules/invoice/app/commands"
	invoicequeries "github.com/vvs/isp/internal/modules/invoice/app/queries"
	invoicemigrations "github.com/vvs/isp/internal/modules/invoice/migrations"
	invoiceworkers "github.com/vvs/isp/internal/modules/invoice/app/workers"

	auditloghttp "github.com/vvs/isp/internal/modules/audit_log/adapters/http"
	auditlogpersistence "github.com/vvs/isp/internal/modules/audit_log/adapters/persistence"
	auditlogcommands "github.com/vvs/isp/internal/modules/audit_log/app/commands"
	auditlogqueries "github.com/vvs/isp/internal/modules/audit_log/app/queries"
	auditlogmigrations "github.com/vvs/isp/internal/modules/audit_log/migrations"

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

	portalhttp "github.com/vvs/isp/internal/modules/portal/adapters/http"
	portalnats "github.com/vvs/isp/internal/modules/portal/adapters/nats"
	portalpersistence "github.com/vvs/isp/internal/modules/portal/adapters/persistence"
	portalmigrations "github.com/vvs/isp/internal/modules/portal/migrations"

	iptvhttp "github.com/vvs/isp/internal/modules/iptv/adapters/http"
	iptvnats "github.com/vvs/isp/internal/modules/iptv/adapters/nats"
	iptvpersistence "github.com/vvs/isp/internal/modules/iptv/adapters/persistence"
	iptvcommands "github.com/vvs/isp/internal/modules/iptv/app/commands"
	iptvqueries "github.com/vvs/isp/internal/modules/iptv/app/queries"
	iptvmigrations "github.com/vvs/isp/internal/modules/iptv/migrations"
)

type App struct {
	DB          *gormsqlite.DB
	NATSServer  *natsserver.Server
	NATSConn    *nats.Conn
	Publisher   events.EventPublisher
	Subscriber  events.EventSubscriber
	HTTPServer  *infrahttp.Server
	RPCServer   *natsrpc.Server
	emailWorker *worker.SyncWorker
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
		{Name: "contact", FS: contactmigrations.FS, TableName: "goose_contact"},
		{Name: "deal", FS: dealmigrations.FS, TableName: "goose_deal"},
		{Name: "ticket", FS: ticketmigrations.FS, TableName: "goose_ticket"},
		{Name: "task", FS: taskmigrations.FS, TableName: "goose_task"},
		{Name: "email", FS: emailmigrations.FS, TableName: "goose_email"},
		{Name: "invoice", FS: invoicemigrations.FS, TableName: "goose_invoice"},
		{Name: "audit_log", FS: auditlogmigrations.FS, TableName: "goose_audit_log"},
		{Name: "portal", FS: portalmigrations.FS, TableName: "goose_portal"},
		{Name: "iptv", FS: iptvmigrations.FS, TableName: "goose_iptv"},
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
		ns, nc, err = infranats.StartEmbedded(cfg.NATSListenAddr, cfg.NATSAuthToken)
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
	permRepo := authpersistence.NewGormRolePermissionsRepository(gdb)

	// Prune expired sessions on startup to avoid unbounded growth.
	if err := sessionRepo.PruneExpired(context.Background()); err != nil {
		log.Printf("warn: prune sessions on startup: %v", err)
	}

	loginCmd := authcommands.NewLoginHandler(userRepo, sessionRepo)
	logoutCmd := authcommands.NewLogoutHandler(sessionRepo)
	createUserCmd := authcommands.NewCreateUserHandler(userRepo)
	deleteUserCmd := authcommands.NewDeleteUserHandler(userRepo, sessionRepo)
	changeSelfPasswordCmd := authcommands.NewChangeSelfPasswordHandler(userRepo)
	updateUserCmd := authcommands.NewUpdateUserHandler(userRepo)
	createSessionCmd := authcommands.NewCreateSessionHandler(sessionRepo)
	listUsersQuery := authqueries.NewListUsersHandler(userRepo)
	getCurrentUserQuery := authqueries.NewGetCurrentUserHandler(userRepo, sessionRepo)
	authRoutes := authhttp.NewHandlers(loginCmd, logoutCmd, createUserCmd, deleteUserCmd, changeSelfPasswordCmd, updateUserCmd, listUsersQuery, getCurrentUserQuery).
		WithPermRepo(permRepo).
		WithMaxAge(cfg.SessionLifetime()).
		WithSecureCookie(cfg.SecureCookie).
		WithTOTPUsers(userRepo).
		WithCreateSession(createSessionCmd)

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
	noteRepo := customerpersistence.NewGormNoteRepository(gdb)
	updateCustomerCmd := customercommands.NewUpdateCustomerHandler(customerRepo, publisher)
	deleteCustomerCmd := customercommands.NewDeleteCustomerHandler(customerRepo, publisher)
	changeStatusCmd := customercommands.NewChangeCustomerStatusHandler(customerRepo, publisher)
	addNoteCmd := customercommands.NewAddNoteHandler(noteRepo)
	listCustomersQuery := customerqueries.NewListCustomersHandler(gdb)
	getCustomerQuery := customerqueries.NewGetCustomerHandler(gdb)
	listNotesQuery := customerqueries.NewListNotesHandler(gdb)

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
	routerRepo := networkpersistence.NewGormRouterRepository(gdb, []byte(cfg.RouterEncKey))
	createRouterCmd := networkcommands.NewCreateRouterHandler(routerRepo, publisher)
	updateRouterCmd := networkcommands.NewUpdateRouterHandler(routerRepo, publisher)
	deleteRouterCmd := networkcommands.NewDeleteRouterHandler(routerRepo, publisher)
	listRoutersQuery := networkqueries.NewListRoutersHandler(routerRepo)
	getRouterQuery := networkqueries.NewGetRouterHandler(routerRepo)

	provisioner := &provisionerDispatcher{
		mikrotik: mikrotik.New(),
		arista:   arista.New(),
	}
	prefixRepo := networkpersistence.NewGormPrefixRepository(gdb)
	var ipamProvider networkdomain.IPAMProvider
	if cfg.NetBoxURL != "" && cfg.NetBoxToken != "" {
		nbClient := netbox.New(cfg.NetBoxURL, cfg.NetBoxToken)
		allocator := networkservices.NewIPAllocatorService(prefixRepo, nbClient)
		ipamProvider = allocator
		log.Printf("NetBox IPAM configured: %s", cfg.NetBoxURL)
	}
	createCustomerCmd := customercommands.NewCreateCustomerHandler(customerRepo, publisher, ipamProvider)
	syncARPCmd := networkcommands.NewSyncCustomerARPHandler(
		&customerARPBridge{repo: customerRepo}, routerRepo, provisioner, ipamProvider, publisher,
	)

	var customerRoutes *customerhttp.Handlers
	if cfg.IsEnabled("customer") {
		customerRoutes = customerhttp.NewHandlers(
			createCustomerCmd, updateCustomerCmd, deleteCustomerCmd,
			changeStatusCmd, addNoteCmd,
			listCustomersQuery, getCustomerQuery, listNotesQuery,
			subscriber, publisher,
			listServicesQuery,
		)
		moduleRoutes = append(moduleRoutes, customerRoutes)
		log.Printf("module enabled: customer")
	}

	if cfg.IsEnabled("network") {
		if customerRoutes != nil {
			customerRoutes.WithReader(gdb.R)
		}

		networkRoutes := networkhttp.NewHandlers(
			createRouterCmd, updateRouterCmd, deleteRouterCmd,
			listRoutersQuery, getRouterQuery, syncARPCmd, prefixRepo, subscriber,
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

	// 9. Contact module
	contactRepo := contactpersistence.NewGormContactRepository(gdb)
	addContactCmd := contactcommands.NewAddContactHandler(contactRepo, publisher)
	updateContactCmd := contactcommands.NewUpdateContactHandler(contactRepo, publisher)
	deleteContactCmd := contactcommands.NewDeleteContactHandler(contactRepo, publisher)
	listContactsQuery := contactqueries.NewListContactsForCustomerHandler(gdb)

	contactRoutes := contacthttp.NewHandlers(addContactCmd, updateContactCmd, deleteContactCmd, listContactsQuery, subscriber)
	moduleRoutes = append(moduleRoutes, contactRoutes)
	if customerRoutes != nil {
		customerRoutes.WithContactsQuery(listContactsQuery)
	}
	log.Printf("module wired: contact")

	// Deal module
	dealRepo := dealpersistence.NewGormDealRepository(gdb)
	addDealCmd := dealcommands.NewAddDealHandler(dealRepo, publisher)
	updateDealCmd := dealcommands.NewUpdateDealHandler(dealRepo, publisher)
	deleteDealCmd := dealcommands.NewDeleteDealHandler(dealRepo, publisher)
	advanceDealCmd := dealcommands.NewAdvanceDealHandler(dealRepo, publisher)
	listDealsQuery := dealqueries.NewListDealsForCustomerHandler(dealRepo)

	listAllDealsQuery := dealqueries.NewListAllDealsHandler(dealRepo)
	dealRoutes := dealhttp.NewHandlers(addDealCmd, updateDealCmd, deleteDealCmd, advanceDealCmd, listDealsQuery, subscriber)
	dealRoutes.WithListAll(listAllDealsQuery)
	dealRoutes.WithCustomerNames(&dealCustomerNameBridge{repo: customerRepo})
	moduleRoutes = append(moduleRoutes, dealRoutes)
	if customerRoutes != nil {
		customerRoutes.WithDealsQuery(listDealsQuery)
	}
	log.Printf("module wired: deal")

	// Ticket module
	ticketRepo := ticketpersistence.NewGormTicketRepository(gdb)
	openTicketCmd := ticketcommands.NewOpenTicketHandler(ticketRepo, publisher)
	updateTicketCmd := ticketcommands.NewUpdateTicketHandler(ticketRepo, publisher)
	deleteTicketCmd := ticketcommands.NewDeleteTicketHandler(ticketRepo, publisher)
	changeTicketStatusCmd := ticketcommands.NewChangeTicketStatusHandler(ticketRepo, publisher)
	addCommentCmd := ticketcommands.NewAddCommentHandler(ticketRepo, publisher)
	listTicketsQuery := ticketqueries.NewListTicketsForCustomerHandler(ticketRepo)
	listCommentsQuery := ticketqueries.NewListCommentsHandler(ticketRepo)

	ticketNameResolver := &ticketCustomerNameBridge{repo: customerRepo}
	listAllTicketsQuery := ticketqueries.NewListAllTicketsHandler(ticketRepo, ticketNameResolver)
	getTicketQuery := ticketqueries.NewGetTicketHandler(ticketRepo, ticketNameResolver)

	ticketRoutes := tickethttp.NewHandlers(
		openTicketCmd, updateTicketCmd, deleteTicketCmd,
		changeTicketStatusCmd, addCommentCmd,
		listTicketsQuery, listCommentsQuery,
		subscriber, publisher,
	)
	ticketRoutes.WithListAll(listAllTicketsQuery)
	ticketRoutes.WithGetTicket(getTicketQuery)
	ticketRoutes.WithCustomerSearch(&ticketCustomerSearchBridge{handler: listCustomersQuery})
	moduleRoutes = append(moduleRoutes, ticketRoutes)
	if customerRoutes != nil {
		customerRoutes.WithTicketsQuery(listTicketsQuery)
	}
	log.Printf("module wired: ticket")

	// Task module
	taskRepo := taskpersistence.NewGormTaskRepository(gdb)
	createTaskCmd := taskcommands.NewCreateTaskHandler(taskRepo, publisher)
	updateTaskCmd := taskcommands.NewUpdateTaskHandler(taskRepo, publisher)
	deleteTaskCmd := taskcommands.NewDeleteTaskHandler(taskRepo, publisher)
	changeTaskStatusCmd := taskcommands.NewChangeTaskStatusHandler(taskRepo, publisher)
	listTasksForCustomerQuery := taskqueries.NewListTasksForCustomerHandler(taskRepo)
	listAllTasksQuery := taskqueries.NewListAllTasksHandler(taskRepo)

	taskRoutes := taskhttp.NewHandlers(
		createTaskCmd, updateTaskCmd, deleteTaskCmd, changeTaskStatusCmd,
		listTasksForCustomerQuery, listAllTasksQuery,
		subscriber, publisher,
	)
	moduleRoutes = append(moduleRoutes, taskRoutes)
	if customerRoutes != nil {
		customerRoutes.WithTasksQuery(listTasksForCustomerQuery)
	}
	log.Printf("module wired: task")

	// Email module
	emailAccountRepo := emailpersistence.NewGormEmailAccountRepository(gdb)
	emailThreadRepo := emailpersistence.NewGormEmailThreadRepository(gdb)
	emailMessageRepo := emailpersistence.NewGormEmailMessageRepository(gdb)
	emailAttachmentRepo := emailpersistence.NewGormEmailAttachmentRepository(gdb)
	emailTagRepo := emailpersistence.NewGormEmailTagRepository(gdb)
	emailFolderRepo := emailpersistence.NewGormEmailFolderRepository(gdb)

	emailEncKey := []byte(cfg.EmailEncKey) // 32-byte AES key from config; empty = dev mode (no encryption)

	configureAccountCmd := emailcommands.NewConfigureAccountHandler(emailAccountRepo, publisher, emailEncKey)
	deleteAccountCmd := emailcommands.NewDeleteAccountHandler(emailAccountRepo, publisher)
	pauseAccountCmd := emailcommands.NewPauseAccountHandler(emailAccountRepo, publisher)
	resumeAccountCmd := emailcommands.NewResumeAccountHandler(emailAccountRepo, publisher)
	applyTagCmd := emailcommands.NewApplyTagHandler(emailThreadRepo, emailTagRepo, publisher)
	removeTagCmd := emailcommands.NewRemoveTagHandler(emailTagRepo, publisher)
	markReadCmd := emailcommands.NewMarkReadHandler(emailTagRepo, publisher)
	linkCustomerCmd := emailcommands.NewLinkCustomerHandler(emailThreadRepo, publisher)
	smtpSender := smtpAdapter.NewSender(emailEncKey)
	imapAppender := imapAdapter.NewAppender(emailEncKey)
	sendReplyCmd := emailcommands.NewSendReplyHandler(emailThreadRepo, emailMessageRepo, emailAccountRepo, smtpSender, publisher)
	composeEmailCmd := emailcommands.NewComposeEmailHandler(emailAccountRepo, emailThreadRepo, emailMessageRepo, smtpSender, imapAppender, publisher)
	listEmailThreadsQuery := emailqueries.NewListThreadsHandler(emailThreadRepo, emailTagRepo).
		WithFolderRepo(emailFolderRepo)
	getEmailThreadQuery := emailqueries.NewGetThreadHandler(emailThreadRepo, emailMessageRepo, emailAttachmentRepo, emailTagRepo)
	listEmailForCustomerQuery := emailqueries.NewListThreadsForCustomerHandler(emailThreadRepo, emailTagRepo)
	listEmailAccountsQuery := emailqueries.NewListAccountsHandler(emailAccountRepo)
	listFoldersQuery := emailqueries.NewListFoldersHandler(emailFolderRepo)
	searchAttachmentsQuery := emailqueries.NewSearchAttachmentsHandler(emailAttachmentRepo)

	emailRepos := imapAdapter.Repos{
		DB:          gdb,
		Accounts:    emailAccountRepo,
		Folders:     emailFolderRepo,
		Threads:     emailThreadRepo,
		Messages:    emailMessageRepo,
		Attachments: emailAttachmentRepo,
		Tags:        emailTagRepo,
		EncKey:      emailEncKey,
	}

	discoverFn := func(ctx context.Context, accountID string) ([]emailqueries.FolderReadModel, error) {
		acc, err := emailAccountRepo.FindByID(ctx, accountID)
		if err != nil {
			return nil, err
		}
		newID := func() string { return uuid.Must(uuid.NewV7()).String() }
		if _, err := imapAdapter.DiscoverFolders(ctx, acc, emailRepos, newID); err != nil {
			return nil, err
		}
		return listFoldersQuery.Handle(ctx, accountID)
	}

	toggleStarCmd := emailcommands.NewToggleStarHandler(emailTagRepo, publisher)

	emailRoutes := emailhttp.NewHandlers(
		configureAccountCmd, deleteAccountCmd, pauseAccountCmd, resumeAccountCmd,
		applyTagCmd, removeTagCmd, markReadCmd, linkCustomerCmd, sendReplyCmd,
		listEmailThreadsQuery, getEmailThreadQuery, listEmailForCustomerQuery,
		listEmailAccountsQuery, listFoldersQuery, emailFolderRepo, discoverFn,
		emailAttachmentRepo,
		subscriber, publisher,
	).WithPageSize(cfg.EmailPageSize).
		WithSearchAttachments(searchAttachmentsQuery).
		WithComposeCmd(composeEmailCmd).
		WithCustomerInfo(&emailCustomerInfoBridge{repo: customerRepo}).
		WithContactEmailLookup(&emailContactLookupBridge{db: gdb}).
		WithStarToggler(toggleStarCmd)
	moduleRoutes = append(moduleRoutes, emailRoutes)
	if customerRoutes != nil {
		customerRoutes.WithEmailThreadsQuery(listEmailForCustomerQuery)
	}

	emailSyncInterval := time.Duration(cfg.EmailSyncIntervalSecs) * time.Second
	emailWorker := worker.NewSyncWorker(emailRepos, publisher, subscriber, emailSyncInterval)
	emailWorker.Start()
	log.Printf("module wired: email")

	// Invoice module
	invoiceRepo := invoicepersistence.NewInvoiceRepository(gdb)
	invoiceTokenRepo := invoicepersistence.NewInvoiceTokenRepository(gdb)

	createInvoiceCmd := invoicecommands.NewCreateInvoiceHandler(invoiceRepo, publisher)
	finalizeInvoiceCmd := invoicecommands.NewFinalizeInvoiceHandler(invoiceRepo, publisher)
	markPaidCmd := invoicecommands.NewMarkPaidHandler(invoiceRepo, publisher)
	voidInvoiceCmd := invoicecommands.NewVoidInvoiceHandler(invoiceRepo, publisher)
	addLineItemCmd := invoicecommands.NewAddLineItemHandler(invoiceRepo, publisher)
	updateLineItemCmd := invoicecommands.NewUpdateLineItemHandler(invoiceRepo, publisher)
	removeLineItemCmd := invoicecommands.NewRemoveLineItemHandler(invoiceRepo, publisher)
	generateInvoiceCmd := invoicecommands.NewGenerateFromSubscriptionsHandler(
		invoiceRepo, publisher, &activeServiceBridge{repo: serviceRepo},
	)

	listAllInvoicesQuery := invoicequeries.NewListAllInvoicesHandler(gdb)
	getInvoiceQuery := invoicequeries.NewGetInvoiceHandler(gdb)
	listInvoicesForCustomerQuery := invoicequeries.NewListInvoicesForCustomerHandler(gdb)

	invoiceRoutes := invoicehttp.NewHandlers(
		createInvoiceCmd, finalizeInvoiceCmd, markPaidCmd, voidInvoiceCmd,
		addLineItemCmd, updateLineItemCmd, removeLineItemCmd,
		listAllInvoicesQuery, getInvoiceQuery, listInvoicesForCustomerQuery,
		subscriber,
	)
	invoiceRoutes.WithGenerateCmd(generateInvoiceCmd)
	invoiceRoutes.WithCustomerSearch(&customerSearchBridge{handler: listCustomersQuery})
	invoiceRoutes.WithTokenRepo(invoiceTokenRepo)
	vatRate := cfg.DefaultVATRate
	if vatRate <= 0 {
		vatRate = 21
	}
	invoiceRoutes.WithDefaultVATRate(vatRate)
	moduleRoutes = append(moduleRoutes, invoiceRoutes)
	if customerRoutes != nil {
		customerRoutes.WithInvoicesQuery(listInvoicesForCustomerQuery)
	}
	log.Printf("module wired: invoice")

	// Invoice delivery worker — sends invoice email on isp.invoice.finalized
	if nc != nil {
		deliveryWorker := invoiceworkers.NewInvoiceDeliveryWorker(
			&emailAccountMailerBridge{accounts: emailAccountRepo, smtp: smtpSender},
			&customerEmailBridge{query: getCustomerQuery},
		)
		go deliveryWorker.Run(context.Background(), subscriber)
		log.Printf("module wired: invoice delivery worker")
	}

	// Audit Log module
	auditLogRepo := auditlogpersistence.NewGormAuditLogRepository(gdb)
	createAuditLogCmd := auditlogcommands.NewCreateAuditLogHandler(auditLogRepo)
	listAuditLogsQuery := auditlogqueries.NewListAuditLogsHandler(auditLogRepo)
	listForResourceQuery := auditlogqueries.NewListForResourceHandler(auditLogRepo)
	auditRoutes := auditloghttp.NewHandlers(listAuditLogsQuery, subscriber)
	auditRoutes.WithListForResource(listForResourceQuery)
	moduleRoutes = append(moduleRoutes, auditRoutes)
	if customerRoutes != nil {
		customerRoutes.WithAuditLogger(createAuditLogCmd)
	}
	if ticketRoutes != nil {
		ticketRoutes.WithAuditLogger(createAuditLogCmd)
	}
	invoiceRoutes.WithAuditLogger(createAuditLogCmd)
	log.Printf("module wired: audit_log")

	// 10. Service module — commands + route registration
	assignServiceCmd := servicecommands.NewAssignServiceHandler(serviceRepo, publisher)
	suspendServiceCmd := servicecommands.NewSuspendServiceHandler(serviceRepo, publisher)
	reactivateServiceCmd := servicecommands.NewReactivateServiceHandler(serviceRepo, publisher)
	cancelServiceCmd := servicecommands.NewCancelServiceHandler(serviceRepo, publisher)

	if cfg.IsEnabled("service") {
		serviceRoutes := servicehttp.NewServiceHandlers(
			assignServiceCmd, suspendServiceCmd, reactivateServiceCmd, cancelServiceCmd,
			listServicesQuery, subscriber, publisher,
		)
		serviceRoutes.WithAuditLogger(createAuditLogCmd)
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

		ListAllInvoices:     listAllInvoicesQuery,
		GetInvoice:          getInvoiceQuery,
		ListInvoicesForCust: listInvoicesForCustomerQuery,
		CreateInvoice:       createInvoiceCmd,
		FinalizeInvoice:     finalizeInvoiceCmd,
		MarkPaidInvoice:     markPaidCmd,
		VoidInvoice:         voidInvoiceCmd,
		GenerateInvoice:     generateInvoiceCmd,
		AddInvoiceLine:      addLineItemCmd,
		UpdateInvoiceLine:   updateLineItemCmd,
		RemoveInvoiceLine:   removeLineItemCmd,
	})

	// Cron web UI (always enabled)
	cronRoutes := cronhttp.NewCronHandlers(
		cronqueries.NewListJobsHandler(cronRepo),
		cronqueries.NewGetJobHandler(cronRepo),
		croncommands.NewAddJobHandler(cronRepo),
		croncommands.NewUpdateJobHandler(cronRepo),
		croncommands.NewPauseJobHandler(cronRepo),
		croncommands.NewResumeJobHandler(cronRepo),
		croncommands.NewDeleteJobHandler(cronRepo),
	)
	moduleRoutes = append(moduleRoutes, cronRoutes)

	// Payment import module
	paymentRoutes := paymenthttp.NewHandlers(
		paymentcommands.NewPreviewImportHandler(invoiceRepo),
		paymentcommands.NewConfirmImportHandler(markPaidCmd),
	)
	moduleRoutes = append(moduleRoutes, paymentRoutes)
	log.Printf("module wired: payment import")

	// Portal module — customer self-service invoice access
	portalTokenRepo := portalpersistence.NewGormPortalTokenRepository(gdb)
	portalRoutes := portalhttp.NewHandlers(portalTokenRepo, listInvoicesForCustomerQuery, getInvoiceQuery).
		WithPDFTokens(&invoiceTokenMinter{tokenRepo: invoiceTokenRepo}).
		WithCustomerReader(&portalCustomerBridge{query: getCustomerQuery}).
		WithBaseURL(cfg.BaseURL).
		WithSecureCookie(cfg.SecureCookie)
	moduleRoutes = append(moduleRoutes, portalRoutes)
	log.Printf("module wired: portal")

	// Portal NATS bridge — serves isp.portal.rpc.* for vvs-portal binary
	portalBridge := portalnats.NewPortalBridge(
		nc, portalTokenRepo, invoiceTokenRepo,
		listInvoicesForCustomerQuery, getInvoiceQuery,
		&natsPortalCustomerBridge{query: getCustomerQuery},
	)
	if err := portalBridge.Register(); err != nil {
		return nil, fmt.Errorf("portal nats bridge: %w", err)
	}
	log.Printf("portal NATS bridge registered")

	// IPTV module
	iptvChannelRepo := iptvpersistence.NewChannelRepository(gdb)
	iptvPackageRepo := iptvpersistence.NewPackageRepository(gdb)
	iptvSubRepo := iptvpersistence.NewSubscriptionRepository(gdb)
	iptvSTBRepo := iptvpersistence.NewSTBRepository(gdb)
	iptvKeyRepo := iptvpersistence.NewSubscriptionKeyRepository(gdb)
	iptvEPGRepo := iptvpersistence.NewEPGProgrammeRepository(gdb)
	iptvRoutes := iptvhttp.NewIPTVHandlers(
		iptvcommands.NewCreateChannelHandler(iptvChannelRepo),
		iptvcommands.NewUpdateChannelHandler(iptvChannelRepo),
		iptvcommands.NewDeleteChannelHandler(iptvChannelRepo),
		iptvcommands.NewCreatePackageHandler(iptvPackageRepo),
		iptvcommands.NewUpdatePackageHandler(iptvPackageRepo),
		iptvcommands.NewDeletePackageHandler(iptvPackageRepo),
		iptvcommands.NewAddChannelToPackageHandler(iptvPackageRepo),
		iptvcommands.NewRemoveChannelFromPackageHandler(iptvPackageRepo),
		iptvcommands.NewCreateSubscriptionHandler(iptvSubRepo, iptvKeyRepo, iptvPackageRepo),
		iptvcommands.NewSuspendSubscriptionHandler(iptvSubRepo),
		iptvcommands.NewReactivateSubscriptionHandler(iptvSubRepo),
		iptvcommands.NewCancelSubscriptionHandler(iptvSubRepo),
		iptvcommands.NewRevokeSubscriptionKeyHandler(iptvKeyRepo),
		iptvcommands.NewReissueSubscriptionKeyHandler(iptvKeyRepo),
		iptvcommands.NewAssignSTBHandler(iptvSTBRepo),
		iptvcommands.NewDeleteSTBHandler(iptvSTBRepo),
		iptvqueries.NewListChannelsHandler(iptvChannelRepo),
		iptvqueries.NewListPackagesHandler(iptvPackageRepo),
		iptvqueries.NewListSubscriptionsHandler(iptvSubRepo, iptvPackageRepo),
		iptvqueries.NewListSTBsHandler(iptvSTBRepo),
		iptvcommands.NewImportEPGHandler(iptvEPGRepo),
	)
	moduleRoutes = append(moduleRoutes, iptvRoutes)
	log.Printf("module wired: iptv")

	// IPTV STB NATS bridge — serves isp.stb.rpc.* for vvs-stb binary
	stbBridge := iptvnats.NewSTBBridge(nc, iptvKeyRepo, iptvSubRepo, iptvChannelRepo, iptvEPGRepo,
		iptvSTBRepo, iptvSubRepo, iptvKeyRepo)
	if err := stbBridge.Register(); err != nil {
		return nil, fmt.Errorf("stb nats bridge: %w", err)
	}
	log.Printf("STB NATS bridge registered")

	if err := rpcServer.Register(); err != nil {
		return nil, fmt.Errorf("nats rpc: %w", err)
	}

	// 13. HTTP router — pass gdb.R to dashboard handler
	router := infrahttp.NewRouter(gdb.R, getCurrentUserQuery, permRepo, notifHandler, chatHandler, globalHandler, cfg.APIToken, rpcServer, moduleRoutes...)
	httpServer := infrahttp.NewServer(cfg.ListenAddr, router)

	enabled := cfg.EnabledModules
	if len(enabled) == 0 {
		enabled = []string{"all"}
	}
	log.Printf("VVS ISP Manager initialized (db: %s, modules: %v)", cfg.DatabasePath, enabled)

	return &App{
		DB:          gdb,
		NATSServer:  ns,
		NATSConn:    nc,
		Publisher:   publisher,
		Subscriber:  subscriber,
		HTTPServer:  httpServer,
		RPCServer:   rpcServer,
		emailWorker: emailWorker,
	}, nil
}

func (a *App) Start() error {
	return a.HTTPServer.Start()
}

func (a *App) Shutdown(ctx context.Context) error {
	if a.emailWorker != nil {
		a.emailWorker.Stop()
	}
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
		ID:          c.ID,
		Code:        c.Code.String(),
		RouterID:    c.RouterID,
		IPAddress:   c.IPAddress,
		MACAddress:  c.MACAddress,
		NetworkZone: c.NetworkZone,
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

// activeServiceBridge adapts the service repository to the invoice module's
// ActiveServiceLister interface. Lives here (composition root) so the invoice
// module does not import the service domain package.
type activeServiceBridge struct {
	repo servicedomain.ServiceRepository
}

func (b *activeServiceBridge) ListActiveForCustomer(ctx context.Context, customerID string) ([]invoicecommands.ServiceInfo, error) {
	svcs, err := b.repo.ListForCustomer(ctx, customerID)
	if err != nil {
		return nil, err
	}
	var active []invoicecommands.ServiceInfo
	for _, s := range svcs {
		if s.Status != servicedomain.StatusActive {
			continue
		}
		active = append(active, invoicecommands.ServiceInfo{
			ID:          s.ID,
			ProductID:   s.ProductID,
			ProductName: s.ProductName,
			PriceAmount: s.PriceAmount,
		})
	}
	return active, nil
}

// customerSearchBridge adapts the customer query handler to the invoice module's
// CustomerSearcher interface. Lives here (composition root) so invoice
// module does not import the customer domain package.
type customerSearchBridge struct {
	handler *customerqueries.ListCustomersHandler
}

func (b *customerSearchBridge) SearchCustomers(ctx context.Context, query string, limit int) ([]invoicehttp.CustomerSearchResult, error) {
	result, err := b.handler.Handle(ctx, customerqueries.ListCustomersQuery{
		Search:   query,
		PageSize: limit,
		Page:     1,
	})
	if err != nil {
		return nil, err
	}
	out := make([]invoicehttp.CustomerSearchResult, len(result.Customers))
	for i, c := range result.Customers {
		out[i] = invoicehttp.CustomerSearchResult{
			ID:          c.ID,
			Code:        c.Code.String(),
			CompanyName: c.CompanyName,
		}
	}
	return out, nil
}

// ticketCustomerNameBridge resolves customer names for the ticket module's
// standalone pages. Lives here (composition root) so the ticket module
// does not import the customer domain package.
type ticketCustomerNameBridge struct {
	repo customerdomain.CustomerRepository
}

func (b *ticketCustomerNameBridge) CustomerName(ctx context.Context, id string) string {
	c, err := b.repo.FindByID(ctx, id)
	if err != nil {
		return ""
	}
	return c.CompanyName
}

// ticketCustomerSearchBridge adapts the customer query handler to the ticket module's
// CustomerSearcher interface.
type ticketCustomerSearchBridge struct {
	handler *customerqueries.ListCustomersHandler
}

func (b *ticketCustomerSearchBridge) SearchCustomers(ctx context.Context, query string, limit int) ([]tickethttp.CustomerSearchResult, error) {
	result, err := b.handler.Handle(ctx, customerqueries.ListCustomersQuery{
		Search:   query,
		PageSize: limit,
		Page:     1,
	})
	if err != nil {
		return nil, err
	}
	out := make([]tickethttp.CustomerSearchResult, len(result.Customers))
	for i, c := range result.Customers {
		out[i] = tickethttp.CustomerSearchResult{
			ID:          c.ID,
			Code:        c.Code.String(),
			CompanyName: c.CompanyName,
		}
	}
	return out, nil
}

// dealCustomerNameBridge adapts the customer repo to the deal module's CustomerNameResolver.
type dealCustomerNameBridge struct {
	repo customerdomain.CustomerRepository
}

func (b *dealCustomerNameBridge) ResolveCustomerName(ctx context.Context, id string) string {
	c, err := b.repo.FindByID(ctx, id)
	if err != nil {
		return ""
	}
	return c.CompanyName
}

// emailCustomerInfoBridge adapts the customer repo to the email module's customerInfoResolver.
type emailCustomerInfoBridge struct {
	repo customerdomain.CustomerRepository
}

func (b *emailCustomerInfoBridge) ResolveCustomerName(ctx context.Context, id string) string {
	c, err := b.repo.FindByID(ctx, id)
	if err != nil {
		return ""
	}
	return c.CompanyName
}

func (b *emailCustomerInfoBridge) ResolveCustomerCode(ctx context.Context, id string) string {
	c, err := b.repo.FindByID(ctx, id)
	if err != nil {
		return ""
	}
	return c.Code.String()
}

// emailContactLookupBridge finds a customer ID from a contact email address.
type emailContactLookupBridge struct {
	db *gormsqlite.DB
}

func (b *emailContactLookupBridge) FindCustomerByContactEmail(ctx context.Context, email string) (customerID, customerName, customerCode string, err error) {
	var row struct {
		CustomerID  string
		CompanyName string
		Code        string
	}
	result := b.db.R.WithContext(ctx).Raw(
		`SELECT c.id AS customer_id, c.company_name, c.code
		 FROM contacts ct
		 JOIN customers c ON c.id = ct.customer_id
		 WHERE ct.email = ?
		 LIMIT 1`,
		email,
	).Scan(&row)
	if result.Error != nil {
		return "", "", "", result.Error
	}
	return row.CustomerID, row.CompanyName, row.Code, nil
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

// emailAccountMailerBridge implements invoiceworkers.Mailer using the first active email account.
type emailAccountMailerBridge struct {
	accounts *emailpersistence.GormEmailAccountRepository
	smtp     *smtpAdapter.Sender
}

func (b *emailAccountMailerBridge) Send(ctx context.Context, to, subject, body string) error {
	accounts, err := b.accounts.ListActive(ctx)
	if err != nil || len(accounts) == 0 {
		return fmt.Errorf("invoice delivery: no active email account")
	}
	return b.smtp.Send(ctx, accounts[0], to, subject, body, "", "")
}

// customerEmailBridge implements invoiceworkers.CustomerEmailGetter via the GetCustomer query.
type customerEmailBridge struct {
	query *customerqueries.GetCustomerHandler
}

func (b *customerEmailBridge) GetCustomerEmail(ctx context.Context, customerID string) (string, error) {
	c, err := b.query.Handle(ctx, customerqueries.GetCustomerQuery{ID: customerID})
	if err != nil {
		return "", err
	}
	return c.Email, nil
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

// portalCustomerBridge implements portalhttp.customerReader using the GetCustomer query.
type portalCustomerBridge struct {
	query *customerqueries.GetCustomerHandler
}

func (b *portalCustomerBridge) GetPortalCustomer(ctx context.Context, id string) (*portalhttp.PortalCustomer, error) {
	c, err := b.query.Handle(ctx, customerqueries.GetCustomerQuery{ID: id})
	if err != nil {
		return nil, err
	}
	return &portalhttp.PortalCustomer{
		ID:          c.ID,
		CompanyName: c.CompanyName,
		Email:       c.Email,
	}, nil
}

// invoiceTokenMinter implements portalhttp.pdfTokenMinter using the invoice token repository.
type invoiceTokenMinter struct {
	tokenRepo invoicedomain.InvoiceTokenRepository
}

func (m *invoiceTokenMinter) MintToken(ctx context.Context, invoiceID, _ string) (string, error) {
	tok, plain, err := invoicedomain.NewInvoiceToken(invoiceID, 48*time.Hour)
	if err != nil {
		return "", err
	}
	return plain, m.tokenRepo.Save(ctx, tok)
}

// natsPortalCustomerBridge adapts portalCustomerBridge to portalnats.bridgeCustomerReader.
type natsPortalCustomerBridge struct {
	query *customerqueries.GetCustomerHandler
}

func (b *natsPortalCustomerBridge) GetPortalCustomer(ctx context.Context, id string) (*portalnats.BridgeCustomer, error) {
	c, err := b.query.Handle(ctx, customerqueries.GetCustomerQuery{ID: id})
	if err != nil {
		return nil, err
	}
	return &portalnats.BridgeCustomer{
		ID:          c.ID,
		CompanyName: c.CompanyName,
		Email:       c.Email,
	}, nil
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
