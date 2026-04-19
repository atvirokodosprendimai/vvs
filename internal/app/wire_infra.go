package app

import (
	"context"
	"fmt"
	"log"

	"github.com/nats-io/nats.go"

	"github.com/vvs/isp/internal/infrastructure/chat"
	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
	infrahttp "github.com/vvs/isp/internal/infrastructure/http"
	natsrpc "github.com/vvs/isp/internal/infrastructure/nats/rpc"
	"github.com/vvs/isp/internal/infrastructure/notifications"
	"github.com/vvs/isp/internal/shared/events"

	cronhttp        "github.com/vvs/isp/internal/modules/cron/adapters/http"
	cronpersistence "github.com/vvs/isp/internal/modules/cron/adapters/persistence"
	croncommands    "github.com/vvs/isp/internal/modules/cron/app/commands"
	cronqueries     "github.com/vvs/isp/internal/modules/cron/app/queries"

	paymenthttp     "github.com/vvs/isp/internal/modules/payment/adapters/http"
	paymentcommands "github.com/vvs/isp/internal/modules/payment/app/commands"

	portalhttp        "github.com/vvs/isp/internal/modules/portal/adapters/http"
	portalnats        "github.com/vvs/isp/internal/modules/portal/adapters/nats"
	portalpersistence "github.com/vvs/isp/internal/modules/portal/adapters/persistence"

	iptvnats "github.com/vvs/isp/internal/modules/iptv/adapters/nats"
)

type infraWired struct {
	notifHandler  *infrahttp.NotifHandler
	chatHandler   *infrahttp.ChatHandler
	globalHandler *infrahttp.GlobalHandler
	rpcServer     *natsrpc.Server
	routes        []infrahttp.ModuleRoutes
}

func wireInfra(
	gdb  *gormsqlite.DB,
	pub  events.EventPublisher,
	sub  events.EventSubscriber,
	nc   *nats.Conn,
	auth *authWired,
	cust *customerWired,
	prod *productWired,
	net  *networkWired,
	dev  *deviceWired,
	svc  *serviceWired,
	inv  *invoiceWired,
	iptv *iptvWired,
	cfg  Config,
) (*infraWired, error) {
	// ── Notifications ─────────────────────────────────────────────────────────
	notifStore  := notifications.NewStore(gdb)
	notifWorker := notifications.NewWorker(notifStore, pub)
	go notifWorker.Run(context.Background(), sub)
	notifHandler := infrahttp.NewNotifHandler(notifStore, sub)

	// ── Chat ──────────────────────────────────────────────────────────────────
	chatStore := chat.NewStore(gdb)
	if err := seedGeneralChannel(context.Background(), chatStore); err != nil {
		log.Printf("warn: seed #general channel: %v", err)
	}
	chatHandler   := infrahttp.NewChatHandler(chatStore, sub, pub)
	globalHandler := infrahttp.NewGlobalHandler(notifStore, chatStore, sub)

	// ── Cron ──────────────────────────────────────────────────────────────────
	cronRepo := cronpersistence.NewGormJobRepository(gdb)

	// ── NATS RPC ──────────────────────────────────────────────────────────────
	rpcServer := natsrpc.New(nc, natsrpc.Config{
		ListUsers:  auth.listUsers,
		CreateUser: auth.createUser,
		DeleteUser: auth.deleteUser,

		ListCustomers:  cust.listQuery,
		GetCustomer:    cust.getQuery,
		CreateCustomer: net.createCustomer,
		UpdateCustomer: cust.updateCmd,
		DeleteCustomer: cust.deleteCmd,

		ListProducts:  prod.listQuery,
		GetProduct:    prod.getQuery,
		CreateProduct: prod.createCmd,
		UpdateProduct: prod.updateCmd,
		DeleteProduct: prod.deleteCmd,

		ListRouters:  net.listRouters,
		GetRouter:    net.getRouter,
		CreateRouter: net.createRouter,
		UpdateRouter: net.updateRouter,
		DeleteRouter: net.deleteRouter,
		SyncARP:      net.syncARP,

		ListServices:      svc.listServices,
		AssignService:     svc.assignCmd,
		SuspendService:    svc.suspendCmd,
		ReactivateService: svc.reactivateCmd,
		CancelService:     svc.cancelCmd,

		ListDevices:        dev.listQuery,
		GetDevice:          dev.getQuery,
		RegisterDevice:     dev.registerCmd,
		DeployDevice:       dev.deployCmd,
		ReturnDevice:       dev.returnCmd,
		DecommissionDevice: dev.decommCmd,
		UpdateDevice:       dev.updateCmd,

		ListJobs:  cronqueries.NewListJobsHandler(cronRepo),
		GetJob:    cronqueries.NewGetJobHandler(cronRepo),
		AddJob:    croncommands.NewAddJobHandler(cronRepo),
		PauseJob:  croncommands.NewPauseJobHandler(cronRepo),
		ResumeJob: croncommands.NewResumeJobHandler(cronRepo),
		DeleteJob: croncommands.NewDeleteJobHandler(cronRepo),

		ListAllInvoices:     inv.listAll,
		GetInvoice:          inv.get,
		ListInvoicesForCust: inv.listForCustomer,
		CreateInvoice:       inv.createCmd,
		FinalizeInvoice:     inv.finalizeCmd,
		MarkPaidInvoice:     inv.markPaidCmd,
		VoidInvoice:         inv.voidCmd,
		GenerateInvoice:     inv.generateCmd,
		AddInvoiceLine:      inv.addLineCmd,
		UpdateInvoiceLine:   inv.updateLineCmd,
		RemoveInvoiceLine:   inv.removeLineCmd,
	})
	if err := rpcServer.Register(); err != nil {
		return nil, fmt.Errorf("nats rpc: %w", err)
	}

	// ── Cron web UI ───────────────────────────────────────────────────────────
	cronRoutes := cronhttp.NewCronHandlers(
		cronqueries.NewListJobsHandler(cronRepo),
		cronqueries.NewGetJobHandler(cronRepo),
		croncommands.NewAddJobHandler(cronRepo),
		croncommands.NewUpdateJobHandler(cronRepo),
		croncommands.NewPauseJobHandler(cronRepo),
		croncommands.NewResumeJobHandler(cronRepo),
		croncommands.NewDeleteJobHandler(cronRepo),
	)

	// ── Payment import ────────────────────────────────────────────────────────
	paymentRoutes := paymenthttp.NewHandlers(
		paymentcommands.NewPreviewImportHandler(inv.repo),
		paymentcommands.NewConfirmImportHandler(inv.markPaidCmd),
	)
	log.Printf("module wired: payment import")

	// ── Portal — customer self-service invoice access ─────────────────────────
	portalTokenRepo := portalpersistence.NewGormPortalTokenRepository(gdb)
	portalRoutes := portalhttp.NewHandlers(portalTokenRepo, inv.listForCustomer, inv.get).
		WithPDFTokens(&invoiceTokenMinter{tokenRepo: inv.tokenRepo}).
		WithCustomerReader(&portalCustomerBridge{query: cust.getQuery}).
		WithBaseURL(cfg.BaseURL).
		WithSecureCookie(cfg.SecureCookie)
	log.Printf("module wired: portal")

	// ── Portal NATS bridge (serves isp.portal.rpc.* for vvs-portal binary) ───
	portalBridge := portalnats.NewPortalBridge(
		nc, portalTokenRepo, inv.tokenRepo,
		inv.listForCustomer, inv.get,
		&natsPortalCustomerBridge{query: cust.getQuery},
	)
	if err := portalBridge.Register(); err != nil {
		return nil, fmt.Errorf("portal nats bridge: %w", err)
	}
	log.Printf("portal NATS bridge registered")

	// ── STB NATS bridge (serves isp.stb.rpc.* for vvs-stb binary) ───────────
	stbBridge := iptvnats.NewSTBBridge(nc,
		iptv.keyRepo, iptv.subRepo, iptv.channelRepo, iptv.epgRepo,
		iptv.stbRepo, iptv.subRepo, iptv.keyRepo,
	)
	if err := stbBridge.Register(); err != nil {
		return nil, fmt.Errorf("stb nats bridge: %w", err)
	}
	log.Printf("STB NATS bridge registered")

	return &infraWired{
		notifHandler:  notifHandler,
		chatHandler:   chatHandler,
		globalHandler: globalHandler,
		rpcServer:     rpcServer,
		routes:        []infrahttp.ModuleRoutes{cronRoutes, paymentRoutes, portalRoutes},
	}, nil
}
