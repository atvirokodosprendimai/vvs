package app

import (
	"fmt"
	"log"
	"net/http"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/vvs/isp/internal/infrastructure/database"
	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
	infrahttp "github.com/vvs/isp/internal/infrastructure/http"
	infranats "github.com/vvs/isp/internal/infrastructure/nats"
	"github.com/vvs/isp/internal/infrastructure/metrics"

	authmigrations      "github.com/vvs/isp/internal/modules/auth/migrations"
	auditlogmigrations  "github.com/vvs/isp/internal/modules/audit_log/migrations"
	chatmigrations      "github.com/vvs/isp/internal/infrastructure/chat/migrations"
	contactmigrations   "github.com/vvs/isp/internal/modules/contact/migrations"
	cronmigrations      "github.com/vvs/isp/internal/modules/cron/migrations"
	customermigrations  "github.com/vvs/isp/internal/modules/customer/migrations"
	dealmigrations      "github.com/vvs/isp/internal/modules/deal/migrations"
	devicemigrations    "github.com/vvs/isp/internal/modules/device/migrations"
	emailmigrations     "github.com/vvs/isp/internal/modules/email/migrations"
	invoicemigrations   "github.com/vvs/isp/internal/modules/invoice/migrations"
	iptvmigrations      "github.com/vvs/isp/internal/modules/iptv/migrations"
	networkmigrations   "github.com/vvs/isp/internal/modules/network/migrations"
	notifmigrations     "github.com/vvs/isp/internal/infrastructure/notifications/migrations"
	portalmigrations    "github.com/vvs/isp/internal/modules/portal/migrations"
	productmigrations   "github.com/vvs/isp/internal/modules/product/migrations"
	servicemigrations   "github.com/vvs/isp/internal/modules/service/migrations"
	taskmigrations      "github.com/vvs/isp/internal/modules/task/migrations"
	ticketmigrations    "github.com/vvs/isp/internal/modules/ticket/migrations"
)

// New constructs and wires the full application.
func New(cfg Config) (*App, error) {
	// ── Prometheus metrics ────────────────────────────────────────────────────
	metrics.Register()
	if cfg.MetricsAddr != "" {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		go func() {
			log.Printf("metrics server listening on %s", cfg.MetricsAddr)
			if err := http.ListenAndServe(cfg.MetricsAddr, mux); err != nil {
				log.Printf("metrics server: %v", err)
			}
		}()
	}

	// ── Database ──────────────────────────────────────────────────────────────
	gdb, err := gormsqlite.Open(cfg.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	sqlDB, err := gdb.W.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB: %w", err)
	}
	if err := database.RunModuleMigrations(sqlDB, allMigrations()); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	// ── NATS ──────────────────────────────────────────────────────────────────
	var ns *natsserver.Server
	var nc *nats.Conn
	if cfg.NATSUrl != "" {
		nc, err = infranats.ConnectExternal(cfg.NATSUrl)
		if err != nil {
			return nil, fmt.Errorf("connect nats: %w", err)
		}
		log.Printf("NATS connected to external server: %s", cfg.NATSUrl)
	} else {
		// Prefer per-user credentials; fall back to legacy single token.
		corePass := cfg.NATSCorePassword
		if corePass == "" {
			corePass = cfg.NATSAuthToken
		}
		ns, nc, err = infranats.StartEmbedded(cfg.NATSListenAddr, corePass, cfg.NATSPortalPassword)
		if err != nil {
			return nil, fmt.Errorf("start nats: %w", err)
		}
		if cfg.NATSListenAddr != "" {
			log.Printf("NATS embedded, listening on %s", cfg.NATSListenAddr)
		}
	}
	pub := infranats.NewPublisher(nc)
	sub := infranats.NewSubscriber(nc)

	// ── Module wiring (order matters: dependencies flow downward) ─────────────
	auth  := wireAuth(gdb, cfg)
	prod  := wireProduct(gdb, pub, sub, cfg)
	svc   := wireService(gdb, pub, sub, cfg)
	cust  := wireCustomer(gdb, pub, sub, svc, cfg)
	net   := wireNetwork(gdb, pub, sub, cust, cfg)
	dev   := wireDevice(gdb, pub, sub, cfg)
	crm   := wireCRM(gdb, pub, sub, cust)
	email := wireEmail(gdb, pub, sub, cust, cfg)
	inv   := wireInvoice(gdb, pub, sub, nc, cust, svc, email, cfg)
	aud   := wireAudit(gdb, sub, cust, crm, svc, inv)
	iptv  := wireIPTV(gdb)
	infra, err := wireInfra(gdb, pub, sub, nc, auth, cust, prod, net, dev, svc, inv, iptv, crm, cfg)
	if err != nil {
		return nil, err
	}

	// ── HTTP ──────────────────────────────────────────────────────────────────
	allRoutes := collectRoutes(auth, prod, cust, svc, net, dev, crm, email, inv, aud, iptv, infra)
	router := infrahttp.NewRouter(
		gdb.R,
		auth.currentUser,
		auth.permRepo,
		infra.notifHandler,
		infra.chatHandler,
		infra.globalHandler,
		cfg.APIToken,
		infra.rpcServer,
		allRoutes...,
	)
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
		Publisher:   pub,
		Subscriber:  sub,
		HTTPServer:  httpServer,
		RPCServer:   infra.rpcServer,
		emailWorker: email.worker,
	}, nil
}

// collectRoutes gathers all module route handlers in registration order.
// Concrete-pointer routes that may be nil (cust, svc) use explicit nil guards
// to avoid the non-nil-interface-wrapping-nil-pointer trap.
func collectRoutes(
	auth  *authWired,
	prod  *productWired,
	cust  *customerWired,
	svc   *serviceWired,
	net   *networkWired,
	dev   *deviceWired,
	crm   *crmWired,
	email *emailWired,
	inv   *invoiceWired,
	aud   *auditWired,
	iptv  *iptvWired,
	infra *infraWired,
) []infrahttp.ModuleRoutes {
	var routes []infrahttp.ModuleRoutes
	add := func(r infrahttp.ModuleRoutes) {
		if r != nil {
			routes = append(routes, r)
		}
	}
	add(auth.routes)
	add(prod.routes)
	// Concrete-pointer fields: explicit nil check avoids non-nil interface wrapping nil ptr.
	if cust.routes != nil {
		routes = append(routes, cust.routes)
	}
	if svc.routes != nil {
		routes = append(routes, svc.routes)
	}
	add(net.routes)
	add(dev.routes)
	for _, r := range crm.routes {
		add(r)
	}
	add(email.routes)
	add(inv.routes)
	add(aud.routes)
	add(iptv.routes)
	for _, r := range infra.routes {
		add(r)
	}
	return routes
}

// allMigrations returns the ordered list of module migrations to run at startup.
func allMigrations() []database.ModuleMigration {
	return []database.ModuleMigration{
		{Name: "auth",          FS: authmigrations.FS,      TableName: "goose_auth"},
		{Name: "customer",      FS: customermigrations.FS,  TableName: "goose_customer"},
		{Name: "product",       FS: productmigrations.FS,   TableName: "goose_product"},
		{Name: "network",       FS: networkmigrations.FS,   TableName: "goose_network"},
		{Name: "notifications", FS: notifmigrations.FS,     TableName: "goose_notifications"},
		{Name: "chat",          FS: chatmigrations.FS,      TableName: "goose_chat"},
		{Name: "service",       FS: servicemigrations.FS,   TableName: "goose_service"},
		{Name: "device",        FS: devicemigrations.FS,    TableName: "goose_device"},
		{Name: "cron",          FS: cronmigrations.FS,      TableName: "goose_cron"},
		{Name: "contact",       FS: contactmigrations.FS,   TableName: "goose_contact"},
		{Name: "deal",          FS: dealmigrations.FS,      TableName: "goose_deal"},
		{Name: "ticket",        FS: ticketmigrations.FS,    TableName: "goose_ticket"},
		{Name: "task",          FS: taskmigrations.FS,      TableName: "goose_task"},
		{Name: "email",         FS: emailmigrations.FS,     TableName: "goose_email"},
		{Name: "invoice",       FS: invoicemigrations.FS,   TableName: "goose_invoice"},
		{Name: "audit_log",     FS: auditlogmigrations.FS,  TableName: "goose_audit_log"},
		{Name: "portal",        FS: portalmigrations.FS,    TableName: "goose_portal"},
		{Name: "iptv",          FS: iptvmigrations.FS,      TableName: "goose_iptv"},
	}
}
