package app

import (
	"context"
	"fmt"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/chat"
	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/gormsqlite"
	infrahttp "github.com/atvirokodosprendimai/vvs/internal/infrastructure/http"
	natsrpc "github.com/atvirokodosprendimai/vvs/internal/infrastructure/nats/rpc"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"

	"github.com/atvirokodosprendimai/vvs/internal/modules/auth/domain"

	customerqueries "github.com/atvirokodosprendimai/vvs/internal/modules/customer/app/queries"
	customerdomain "github.com/atvirokodosprendimai/vvs/internal/modules/customer/domain"

	networkdomain "github.com/atvirokodosprendimai/vvs/internal/modules/network/domain"

	servicedomain "github.com/atvirokodosprendimai/vvs/internal/modules/service/domain"

	emailpersistence "github.com/atvirokodosprendimai/vvs/internal/modules/email/adapters/persistence"
	emaildomain "github.com/atvirokodosprendimai/vvs/internal/modules/email/domain"
	"github.com/atvirokodosprendimai/vvs/internal/modules/email/worker"

	invoicecommands "github.com/atvirokodosprendimai/vvs/internal/modules/invoice/app/commands"
	invoicehttp "github.com/atvirokodosprendimai/vvs/internal/modules/invoice/adapters/http"
	invoicedomain "github.com/atvirokodosprendimai/vvs/internal/modules/invoice/domain"

	tickethttp "github.com/atvirokodosprendimai/vvs/internal/modules/ticket/adapters/http"

	portalhttp "github.com/atvirokodosprendimai/vvs/internal/modules/portal/adapters/http"
	portalnats "github.com/atvirokodosprendimai/vvs/internal/modules/portal/adapters/nats"
)

// App is the fully assembled application.
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

// ── Cross-module bridges ──────────────────────────────────────────────────────
// All bridges live here (composition root) so modules do not import each other.

// customerARPBridge adapts the customer repo to networkdomain.CustomerARPProvider.
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

// activeServiceBridge adapts the service repo to invoicecommands.ActiveServiceLister.
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

// customerSearchBridge adapts the customer query to invoicehttp.CustomerSearcher.
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

// ticketCustomerNameBridge resolves customer names for standalone ticket pages.
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

// ticketCustomerSearchBridge adapts the customer query to tickethttp.CustomerSearcher.
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

// provisionerDispatcher picks the right RouterProvisioner based on RouterType.
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
	smtp     emaildomain.EmailSender
}

func (b *emailAccountMailerBridge) Send(ctx context.Context, to, subject, body string) error {
	accounts, err := b.accounts.ListActive(ctx)
	if err != nil || len(accounts) == 0 {
		return fmt.Errorf("invoice delivery: no active email account")
	}
	return b.smtp.Send(ctx, accounts[0], to, subject, body, "", "")
}

// customerEmailBridge implements invoiceworkers.CustomerEmailGetter via GetCustomer query.
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

// portalCustomerBridge implements portalhttp.customerReader.
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
		IPAddress:   c.IPAddress,
		NetworkZone: c.NetworkZone,
	}, nil
}

// invoiceTokenMinter implements portalhttp.pdfTokenMinter.
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

// natsPortalCustomerBridge adapts GetCustomer to portalnats.bridgeCustomerReader.
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
		IPAddress:   c.IPAddress,
		NetworkZone: c.NetworkZone,
	}, nil
}

// portalEmailFinderBridge adapts CustomerRepository.FindByEmail to portalnats.bridgeCustomerEmailFinder.
type portalEmailFinderBridge struct {
	repo customerdomain.CustomerRepository
}

func (b *portalEmailFinderBridge) FindByEmail(ctx context.Context, email string) (string, error) {
	c, err := b.repo.FindByEmail(ctx, email)
	if err != nil {
		return "", err
	}
	return c.ID, nil
}

// portalServiceBridge adapts servicedomain.ServiceRepository to portalnats.bridgeServiceLister.
type portalServiceBridge struct {
	repo servicedomain.ServiceRepository
}

func (b *portalServiceBridge) ListForCustomer(ctx context.Context, customerID string) ([]*portalnats.BridgeService, error) {
	svcs, err := b.repo.ListForCustomer(ctx, customerID)
	if err != nil {
		return nil, err
	}
	out := make([]*portalnats.BridgeService, len(svcs))
	for i, s := range svcs {
		out[i] = &portalnats.BridgeService{
			ID:               s.ID,
			ProductName:      s.ProductName,
			PriceAmountCents: s.PriceAmount,
			Currency:         s.Currency,
			Status:           s.Status,
			BillingCycle:     s.BillingCycle,
			NextBillingDate:  s.NextBillingDate,
		}
	}
	return out, nil
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
