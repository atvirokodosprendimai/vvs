package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/bot"
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
	invoicequeries "github.com/atvirokodosprendimai/vvs/internal/modules/invoice/app/queries"

	tickethttp "github.com/atvirokodosprendimai/vvs/internal/modules/ticket/adapters/http"
	ticketcommands "github.com/atvirokodosprendimai/vvs/internal/modules/ticket/app/commands"
	ticketqueries "github.com/atvirokodosprendimai/vvs/internal/modules/ticket/app/queries"

	portalhttp "github.com/atvirokodosprendimai/vvs/internal/modules/portal/adapters/http"
	portaldomain "github.com/atvirokodosprendimai/vvs/internal/modules/portal/domain"
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

// ── Core in-process portal bridges ──────────────────────────────────────────

// corePortalTicketBridge implements portalhttp.portalTicketClient in-process.
type corePortalTicketBridge struct {
	list    *ticketqueries.ListTicketsForCustomerHandler
	open    *ticketcommands.OpenTicketHandler
	comment *ticketcommands.AddCommentHandler
}

func (b *corePortalTicketBridge) ListTickets(ctx context.Context, customerID string) ([]ticketqueries.TicketReadModel, error) {
	return b.list.Handle(ctx, ticketqueries.ListTicketsForCustomerQuery{CustomerID: customerID})
}

func (b *corePortalTicketBridge) OpenTicket(ctx context.Context, customerID, subject, body string) (string, error) {
	tk, err := b.open.Handle(ctx, ticketcommands.OpenTicketCommand{
		CustomerID: customerID,
		Subject:    subject,
		Body:       body,
		Priority:   "normal",
	})
	if err != nil {
		return "", err
	}
	return tk.ID, nil
}

func (b *corePortalTicketBridge) AddTicketComment(ctx context.Context, ticketID, customerID, body string) (string, error) {
	c, err := b.comment.Handle(ctx, ticketcommands.AddCommentCommand{
		TicketID: ticketID,
		Body:     body,
		AuthorID: customerID,
	})
	if err != nil {
		return "", err
	}
	return c.ID, nil
}

// corePortalHTTPServiceBridge implements portalhttp.portalServiceClient in-process.
type corePortalHTTPServiceBridge struct {
	repo servicedomain.ServiceRepository
}

func (b *corePortalHTTPServiceBridge) ListServices(ctx context.Context, customerID string) ([]*portalhttp.PortalService, error) {
	svcs, err := b.repo.ListForCustomer(ctx, customerID)
	if err != nil {
		return nil, err
	}
	out := make([]*portalhttp.PortalService, len(svcs))
	for i, s := range svcs {
		out[i] = &portalhttp.PortalService{
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

// corePortalLoginBridge implements portalhttp.portalLoginClient in-process.
type corePortalLoginBridge struct {
	custRepo  customerdomain.CustomerRepository
	tokenRepo portaldomain.PortalTokenRepository
}

func (b *corePortalLoginBridge) FindCustomerByEmail(ctx context.Context, email string) (string, error) {
	c, err := b.custRepo.FindByEmail(ctx, email)
	if err != nil {
		return "", err
	}
	return c.ID, nil
}

func (b *corePortalLoginBridge) CreatePortalToken(ctx context.Context, customerID string, ttl time.Duration) (string, time.Time, error) {
	tok, plain, err := portaldomain.NewPortalToken(customerID, ttl)
	if err != nil {
		return "", time.Time{}, err
	}
	if err := b.tokenRepo.Save(ctx, tok); err != nil {
		return "", time.Time{}, err
	}
	return plain, tok.ExpiresAt, nil
}

// corePortalBotBridge implements portalhttp.portalBotClient in-process.
// Mirrors the logic of portalnats.PortalBridge bot handlers.
type corePortalBotBridge struct {
	sessions    *bot.Sessions
	ollama      *bot.OllamaClient
	chatStore   *chat.Store
	openTicket  *ticketcommands.OpenTicketHandler
	custQuery   *customerqueries.GetCustomerHandler
	svcRepo     servicedomain.ServiceRepository
	listInvoices *invoicequeries.ListInvoicesForCustomerHandler
}

func (b *corePortalBotBridge) buildRuleContext(ctx context.Context, customerID string) *bot.RuleContext {
	rc := &bot.RuleContext{CustomerID: customerID}
	if b.custQuery != nil {
		if c, err := b.custQuery.Handle(ctx, customerqueries.GetCustomerQuery{ID: customerID}); err == nil {
			rc.Customer = &bot.CustomerInfo{
				CompanyName: c.CompanyName,
				Email:       c.Email,
				IPAddress:   c.IPAddress,
			}
		}
	}
	if b.svcRepo != nil {
		if svcs, err := b.svcRepo.ListForCustomer(ctx, customerID); err == nil {
			for _, s := range svcs {
				rc.Services = append(rc.Services, &bot.ServiceInfo{
					ID:               s.ID,
					ProductName:      s.ProductName,
					PriceAmountCents: s.PriceAmount,
					Status:           s.Status,
					NextBillingDate:  s.NextBillingDate,
				})
			}
		}
	}
	if b.listInvoices != nil {
		if invoices, err := b.listInvoices.Handle(ctx, invoicequeries.ListInvoicesForCustomerQuery{CustomerID: customerID}); err == nil {
			for _, inv := range invoices {
				info := bot.InvoiceInfo{
					Code:        inv.Code,
					Status:      inv.Status,
					TotalAmount: inv.TotalAmount,
					IssueDate:   inv.IssueDate,
				}
				if inv.PaidAt != nil {
					info.PaidAt = inv.PaidAt
				}
				rc.Invoices = append(rc.Invoices, info)
				if inv.Status == "finalized" {
					rc.OverdueCount++
					rc.OverdueTotal += inv.TotalAmount
				}
			}
		}
	}
	return rc
}

func (b *corePortalBotBridge) ollamaFallback(ctx context.Context, history []bot.BotMessage, userMsg string, rc *bot.RuleContext) string {
	sysPrompt := bot.BuildSystemPrompt(rc.Customer, rc.Services, rc.Invoices)
	msgs := []bot.OllamaMessage{{Role: "system", Content: sysPrompt}}
	for _, m := range history {
		msgs = append(msgs, bot.OllamaMessage{Role: m.Role, Content: m.Content})
	}
	msgs = append(msgs, bot.OllamaMessage{Role: "user", Content: userMsg})
	reply, err := b.ollama.Chat(ctx, msgs)
	if err != nil {
		return "I'm not sure how to answer that. Would you like to speak with a staff member?"
	}
	return reply
}

func (b *corePortalBotBridge) BotMessage(ctx context.Context, customerID, sessionID, message string) (reply, newSessionID, state string, suggestHandoff bool, err error) {
	if sessionID == "" {
		sessionID = uuid.Must(uuid.NewV7()).String()
	}
	sess := b.sessions.GetOrCreate(sessionID, customerID)
	if sess.CustomerID != customerID {
		return "", sessionID, "", false, fmt.Errorf("forbidden")
	}
	if sess.State == bot.StateClosed {
		return "This conversation has ended.", sessionID, sess.State, false, nil
	}
	if sess.State == bot.StateLive {
		return "You are connected to a staff member. Use the live chat to send messages.", sessionID, sess.State, false, nil
	}
	rc := b.buildRuleContext(ctx, customerID)
	var matched bool
	reply, matched, suggestHandoff = bot.MatchRules(ctx, message, rc)
	if !matched {
		if b.ollama != nil {
			history := b.sessions.MessagesSnapshot(sessionID)
			reply = b.ollamaFallback(ctx, history, message, rc)
		} else {
			reply = "I'm not sure how to answer that. Would you like to speak with a staff member?"
			suggestHandoff = true
		}
	}
	now := time.Now()
	b.sessions.AppendMessages(sessionID,
		bot.BotMessage{Role: "user", Content: message, At: now},
		bot.BotMessage{Role: "assistant", Content: reply, At: now},
	)
	return reply, sessionID, sess.State, suggestHandoff, nil
}

func (b *corePortalBotBridge) BotHandoff(ctx context.Context, customerID, sessionID string) (threadID, state string, err error) {
	if b.chatStore == nil {
		return "", "", fmt.Errorf("chat not available")
	}
	sess := b.sessions.Get(sessionID)
	if sess == nil || sess.CustomerID != customerID {
		return "", "", fmt.Errorf("forbidden")
	}
	threadID = fmt.Sprintf("portal-%s", sessionID)
	exists, err := b.chatStore.ThreadExists(ctx, threadID)
	if err != nil {
		return "", "", err
	}
	customerName := customerID
	if b.custQuery != nil {
		if c, err := b.custQuery.Handle(ctx, customerqueries.GetCustomerQuery{ID: customerID}); err == nil {
			customerName = c.CompanyName
		}
	}
	if !exists {
		if err := b.chatStore.CreateThread(ctx, chat.Thread{
			ID:        threadID,
			Type:      "portal-support",
			Name:      fmt.Sprintf("Portal: %s", customerName),
			CreatedBy: "system",
			CreatedAt: time.Now().UTC(),
		}); err != nil {
			return "", "", err
		}
		history := buildBotHistoryText(b.sessions.MessagesSnapshot(sessionID))
		_ = b.chatStore.Save(ctx, chat.Message{
			ID:        uuid.Must(uuid.NewV7()).String(),
			ThreadID:  threadID,
			UserID:    "system",
			Username:  "Bot",
			Body:      fmt.Sprintf("Customer %s connected via portal chat.\n\n%s", customerName, history),
			CreatedAt: time.Now().UTC(),
		})
	}
	b.sessions.UpdateState(sessionID, bot.StateHandoff, threadID)
	return threadID, bot.StateHandoff, nil
}

func (b *corePortalBotBridge) BotLiveMessage(ctx context.Context, customerID, sessionID, message string) (staffReply, state string, err error) {
	if b.chatStore == nil {
		return "", "", fmt.Errorf("chat not available")
	}
	sess := b.sessions.Get(sessionID)
	if sess == nil || sess.CustomerID != customerID || sess.ThreadID == "" {
		return "", "", fmt.Errorf("forbidden")
	}
	if message != "" {
		_ = b.chatStore.Save(ctx, chat.Message{
			ID:        uuid.Must(uuid.NewV7()).String(),
			ThreadID:  sess.ThreadID,
			UserID:    customerID,
			Username:  "Customer",
			Body:      message,
			CreatedAt: time.Now().UTC(),
		})
		b.sessions.UpdateState(sessionID, bot.StateLive, "")
	}
	msgs, err := b.chatStore.Recent(ctx, sess.ThreadID, 1)
	if err != nil {
		return "", sess.State, nil
	}
	for _, m := range msgs {
		if m.UserID != customerID && m.UserID != "system" {
			staffReply = m.Body
			break
		}
	}
	return staffReply, sess.State, nil
}

func (b *corePortalBotBridge) BotClose(ctx context.Context, customerID, sessionID string, createTicket bool) (ticketID string, err error) {
	sess := b.sessions.Get(sessionID)
	if sess != nil && sess.CustomerID != customerID {
		return "", fmt.Errorf("forbidden")
	}
	if createTicket && b.openTicket != nil && sess != nil {
		subject := "Portal chat — unresolved question"
		body := buildBotHistoryText(b.sessions.MessagesSnapshot(sessionID))
		if tk, err := b.openTicket.Handle(ctx, ticketcommands.OpenTicketCommand{
			CustomerID: customerID,
			Subject:    subject,
			Body:       body,
			Priority:   "normal",
		}); err == nil {
			ticketID = tk.ID
		}
	}
	b.sessions.UpdateState(sessionID, bot.StateClosed, "")
	return ticketID, nil
}

// buildBotHistoryText formats bot session history for display in chat threads.
func buildBotHistoryText(messages []bot.BotMessage) string {
	var sb strings.Builder
	for _, m := range messages {
		sb.WriteString(fmt.Sprintf("[%s] %s: %s\n", m.At.Format("15:04"), m.Role, m.Content))
	}
	return sb.String()
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
