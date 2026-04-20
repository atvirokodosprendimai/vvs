// Package nats provides a NATS RPC bridge for the customer portal.
// The PortalBridge runs on vvs-core and serves portal data requests from vvs-portal.
// All subjects use the isp.portal.rpc.* namespace.
package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/bot"
	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/chat"
	invoicedomain "github.com/atvirokodosprendimai/vvs/internal/modules/invoice/domain"
	invoicequeries "github.com/atvirokodosprendimai/vvs/internal/modules/invoice/app/queries"
	portaldomain "github.com/atvirokodosprendimai/vvs/internal/modules/portal/domain"
	ticketcommands "github.com/atvirokodosprendimai/vvs/internal/modules/ticket/app/commands"
	ticketdomain "github.com/atvirokodosprendimai/vvs/internal/modules/ticket/domain"
	ticketqueries "github.com/atvirokodosprendimai/vvs/internal/modules/ticket/app/queries"
)

// Subjects served by PortalBridge.
const (
	SubjectTokenValidate        = "isp.portal.rpc.token.validate"
	SubjectTokenMarkUsed        = "isp.portal.rpc.token.markused"
	SubjectInvoicesList         = "isp.portal.rpc.invoices.list"
	SubjectInvoiceGet           = "isp.portal.rpc.invoice.get"
	SubjectInvoiceGetByToken    = "isp.portal.rpc.invoice.token.get"
	SubjectInvoiceTokenValidate = "isp.portal.rpc.invoice.token.validate"
	SubjectInvoiceTokenMint     = "isp.portal.rpc.invoice.token.mint"
	SubjectCustomerGet          = "isp.portal.rpc.customer.get"

	SubjectTicketsList       = "isp.portal.rpc.tickets.list"
	SubjectTicketOpen        = "isp.portal.rpc.ticket.open"
	SubjectTicketCommentAdd  = "isp.portal.rpc.ticket.comment.add"

	SubjectServicesList = "isp.portal.rpc.services.list"

	SubjectBotMessage     = "isp.portal.rpc.bot.message"
	SubjectBotHandoff     = "isp.portal.rpc.bot.handoff"
	SubjectBotLiveMessage = "isp.portal.rpc.bot.livemessage"
	SubjectBotClose       = "isp.portal.rpc.bot.close"

	SubjectCustomerFindByEmail = "isp.portal.rpc.customer.findByEmail"
	SubjectPortalTokenCreate   = "isp.portal.rpc.token.create"
)

// portalTokenStore reads, creates, and updates portal tokens from the DB.
type portalTokenStore interface {
	FindByHash(ctx context.Context, hash string) (*portaldomain.PortalToken, error)
	MarkUsed(ctx context.Context, tokenHash string) error
	Save(ctx context.Context, token *portaldomain.PortalToken) error
}

// bridgeCustomerEmailFinder looks up a customer by email address.
type bridgeCustomerEmailFinder interface {
	FindByEmail(ctx context.Context, email string) (customerID string, err error)
}

// invoiceTokenStore persists and retrieves invoice PDF tokens.
type invoiceTokenStore interface {
	Save(ctx context.Context, t *invoicedomain.InvoiceToken) error
	FindByHash(ctx context.Context, hash string) (*invoicedomain.InvoiceToken, error)
}

// bridgeCustomerReader provides customer info for the portal header.
type bridgeCustomerReader interface {
	GetPortalCustomer(ctx context.Context, id string) (*BridgeCustomer, error)
}

// invoicesByCustomerLister lists invoices for a customer.
type invoicesByCustomerLister interface {
	Handle(ctx context.Context, q invoicequeries.ListInvoicesForCustomerQuery) ([]invoicequeries.InvoiceReadModel, error)
}

// ticketListerForCustomer lists tickets for a specific customer.
type ticketListerForCustomer interface {
	Handle(ctx context.Context, q ticketqueries.ListTicketsForCustomerQuery) ([]ticketqueries.TicketReadModel, error)
}

// ticketOpener opens a new support ticket.
type ticketOpener interface {
	Handle(ctx context.Context, cmd ticketcommands.OpenTicketCommand) (*ticketdomain.Ticket, error)
}

// ticketCommenter adds a comment to an existing ticket.
type ticketCommenter interface {
	Handle(ctx context.Context, cmd ticketcommands.AddCommentCommand) (*ticketdomain.TicketComment, error)
}

// invoiceByIDGetter retrieves a single invoice by its ID.
type invoiceByIDGetter interface {
	Handle(ctx context.Context, id string) (*invoicequeries.InvoiceReadModel, error)
}

// bridgeServiceLister lists a customer's services.
type bridgeServiceLister interface {
	ListForCustomer(ctx context.Context, customerID string) ([]*BridgeService, error)
}

// BridgeService is the minimal service data sent over NATS to the portal.
type BridgeService struct {
	ID              string
	ProductName     string
	PriceAmountCents int64
	Currency        string
	Status          string
	BillingCycle    string
	NextBillingDate *time.Time
}

// BridgeCustomer is the minimal customer data exposed over NATS.
type BridgeCustomer struct {
	ID          string
	CompanyName string
	Email       string
	IPAddress   string
	NetworkZone string
}

// PortalBridge subscribes to isp.portal.rpc.* subjects and serves portal data.
// Runs on vvs-core — has direct access to SQLite via the injected handlers/repos.
type PortalBridge struct {
	nc             *nats.Conn
	tokenRepo      portalTokenStore
	invoiceToken   invoiceTokenStore
	listInvoices   invoicesByCustomerLister
	getInvoice     invoiceByIDGetter
	custReader     bridgeCustomerReader
	emailFinder    bridgeCustomerEmailFinder
	ticketLister   ticketListerForCustomer
	openTicket     ticketOpener
	addComment     ticketCommenter
	serviceLister  bridgeServiceLister
	// VM + billing bridge (optional — wired via WithVMAndBilling)
	vmb *vmBillingBridge
	// bot components
	botSessions    *bot.Sessions
	botOllama      *bot.OllamaClient
	chatStore      *chat.Store
	subs           []*nats.Subscription
}

// NewPortalBridge creates a bridge. Call Register() to start serving.
func NewPortalBridge(
	nc *nats.Conn,
	tokenRepo portalTokenStore,
	invoiceToken invoiceTokenStore,
	listInvoices invoicesByCustomerLister,
	getInvoice invoiceByIDGetter,
	custReader bridgeCustomerReader,
) *PortalBridge {
	return &PortalBridge{
		nc:           nc,
		tokenRepo:    tokenRepo,
		invoiceToken: invoiceToken,
		listInvoices: listInvoices,
		getInvoice:   getInvoice,
		custReader:   custReader,
	}
}

// WithBot wires the bot session store, Ollama client, and chat store into the bridge.
// Call before Register() to enable the bot RPC subjects.
func (b *PortalBridge) WithBot(sessions *bot.Sessions, ollama *bot.OllamaClient, cs *chat.Store) *PortalBridge {
	b.botSessions = sessions
	b.botOllama   = ollama
	b.chatStore    = cs
	return b
}

// WithEmailFinder wires the customer email lookup into the bridge.
// Call before Register() to enable the findByEmail and token.create RPC subjects.
func (b *PortalBridge) WithEmailFinder(finder bridgeCustomerEmailFinder) *PortalBridge {
	b.emailFinder = finder
	return b
}

// WithServices wires the service lister into the bridge.
// Call before Register() to enable the services list RPC subject.
func (b *PortalBridge) WithServices(lister bridgeServiceLister) *PortalBridge {
	b.serviceLister = lister
	return b
}

// WithTickets wires the ticket sub-handlers into the bridge.
// Call before Register() to enable ticket RPC subjects.
func (b *PortalBridge) WithTickets(lister ticketListerForCustomer, opener ticketOpener, commenter ticketCommenter) *PortalBridge {
	b.ticketLister = lister
	b.openTicket   = opener
	b.addComment   = commenter
	return b
}

// Register subscribes to all portal RPC subjects.
func (b *PortalBridge) Register() error {
	type entry struct {
		subject string
		handler nats.MsgHandler
	}
	entries := []entry{
		{SubjectTokenValidate, b.handleTokenValidate},
		{SubjectTokenMarkUsed, b.handleTokenMarkUsed},
		{SubjectInvoicesList, b.handleInvoicesList},
		{SubjectInvoiceGet, b.handleInvoiceGet},
		{SubjectInvoiceGetByToken, b.handleInvoiceGetByToken},
		{SubjectInvoiceTokenValidate, b.handleInvoiceTokenValidate},
		{SubjectInvoiceTokenMint, b.handleInvoiceTokenMint},
		{SubjectCustomerGet, b.handleCustomerGet},
		{SubjectTicketsList, b.handleTicketsList},
		{SubjectTicketOpen, b.handleTicketOpen},
		{SubjectTicketCommentAdd, b.handleTicketCommentAdd},
		{SubjectServicesList, b.handleServicesList},
		{SubjectBotMessage, b.handleBotMessage},
		{SubjectBotHandoff, b.handleBotHandoff},
		{SubjectBotLiveMessage, b.handleBotLiveMessage},
		{SubjectBotClose, b.handleBotClose},
		{SubjectCustomerFindByEmail, b.handleCustomerFindByEmail},
		{SubjectPortalTokenCreate, b.handlePortalTokenCreate},
	}
	for _, extra := range b.vmBillingEntries() {
		entries = append(entries, extra)
	}
	for _, e := range entries {
		sub, err := b.nc.Subscribe(e.subject, e.handler)
		if err != nil {
			return err
		}
		b.subs = append(b.subs, sub)
	}
	return nil
}

// Close unsubscribes all handlers.
func (b *PortalBridge) Close() {
	for _, s := range b.subs {
		_ = s.Unsubscribe()
	}
	b.subs = nil
}

// ── handlers ──────────────────────────────────────────────────────────────────

func (b *PortalBridge) handleTokenValidate(msg *nats.Msg) {
	var req struct {
		Hash string `json:"hash"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tok, err := b.tokenRepo.FindByHash(ctx, req.Hash)
	if err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	if tok == nil || tok.IsExpired() {
		bridgeReply(msg, nil, errExpired)
		return
	}
	bridgeReply(msg, struct {
		CustomerID string     `json:"customerID"`
		ExpiresAt  time.Time  `json:"expiresAt"`
		UsedAt     *time.Time `json:"usedAt,omitempty"`
	}{tok.CustomerID, tok.ExpiresAt, tok.UsedAt}, nil)
}

func (b *PortalBridge) handleTokenMarkUsed(msg *nats.Msg) {
	var req struct {
		Hash string `json:"hash"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	if req.Hash == "" {
		bridgeReply(msg, nil, errForbidden)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := b.tokenRepo.MarkUsed(ctx, req.Hash); err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	bridgeReply(msg, struct{ OK bool }{true}, nil)
}

func (b *PortalBridge) handleInvoicesList(msg *nats.Msg) {
	if b.listInvoices == nil {
		bridgeReply(msg, nil, &bridgeError{"invoice list handler not configured"})
		return
	}
	var req struct {
		CustomerID string `json:"customerID"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	// customerID is mandatory — reject to prevent potential wildcard queries.
	if req.CustomerID == "" {
		bridgeReply(msg, nil, errForbidden)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	invoices, err := b.listInvoices.Handle(ctx, invoicequeries.ListInvoicesForCustomerQuery{CustomerID: req.CustomerID})
	if err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	bridgeReply(msg, struct {
		Invoices []invoicequeries.InvoiceReadModel `json:"invoices"`
	}{invoices}, nil)
}

// handleInvoiceGet fetches an invoice by ID. customerID is mandatory — the bridge
// enforces ownership so a portal session can only access its own customer's invoices.
func (b *PortalBridge) handleInvoiceGet(msg *nats.Msg) {
	if b.getInvoice == nil {
		bridgeReply(msg, nil, &bridgeError{"invoice get handler not configured"})
		return
	}
	var req struct {
		InvoiceID  string `json:"invoiceID"`
		CustomerID string `json:"customerID"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	// customerID is mandatory — reject unauthenticated callers.
	if req.CustomerID == "" {
		bridgeReply(msg, nil, errForbidden)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	inv, err := b.getInvoice.Handle(ctx, req.InvoiceID)
	if err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	if inv == nil || inv.CustomerID != req.CustomerID {
		bridgeReply(msg, nil, errForbidden)
		return
	}
	bridgeReply(msg, struct {
		Invoice *invoicequeries.InvoiceReadModel `json:"invoice"`
	}{inv}, nil)
}

// handleInvoiceGetByToken serves GET /i/{token} — validates the PDF token and
// returns the invoice in a single atomic call. No customerID required; the token
// itself is the proof of access.
func (b *PortalBridge) handleInvoiceGetByToken(msg *nats.Msg) {
	if b.getInvoice == nil {
		bridgeReply(msg, nil, &bridgeError{"invoice get handler not configured"})
		return
	}
	var req struct {
		TokenHash string `json:"tokenHash"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tok, err := b.invoiceToken.FindByHash(ctx, req.TokenHash)
	if err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	if tok == nil || tok.IsExpired() {
		bridgeReply(msg, nil, errExpired)
		return
	}
	inv, err := b.getInvoice.Handle(ctx, tok.InvoiceID)
	if err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	if inv == nil {
		bridgeReply(msg, nil, errExpired)
		return
	}
	bridgeReply(msg, struct {
		Invoice *invoicequeries.InvoiceReadModel `json:"invoice"`
	}{inv}, nil)
}

func (b *PortalBridge) handleInvoiceTokenValidate(msg *nats.Msg) {
	var req struct {
		TokenHash string `json:"tokenHash"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tok, err := b.invoiceToken.FindByHash(ctx, req.TokenHash)
	if err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	if tok == nil || tok.IsExpired() {
		bridgeReply(msg, nil, errExpired)
		return
	}
	bridgeReply(msg, struct {
		InvoiceID string `json:"invoiceID"`
	}{tok.InvoiceID}, nil)
}

// handleInvoiceTokenMint mints a public PDF token. customerID is mandatory and
// verified against the invoice to prevent a portal session from minting tokens
// for invoices belonging to other customers.
func (b *PortalBridge) handleInvoiceTokenMint(msg *nats.Msg) {
	if b.getInvoice == nil {
		bridgeReply(msg, nil, &bridgeError{"invoice get handler not configured"})
		return
	}
	var req struct {
		InvoiceID  string `json:"invoiceID"`
		CustomerID string `json:"customerID"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	if req.CustomerID == "" {
		bridgeReply(msg, nil, errForbidden)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	inv, err := b.getInvoice.Handle(ctx, req.InvoiceID)
	if err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	if inv == nil || inv.CustomerID != req.CustomerID {
		bridgeReply(msg, nil, errForbidden)
		return
	}
	tok, plain, err := invoicedomain.NewInvoiceToken(req.InvoiceID, 48*time.Hour)
	if err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	if err := b.invoiceToken.Save(ctx, tok); err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	bridgeReply(msg, struct {
		Plain string `json:"plain"`
	}{plain}, nil)
}

// ── bot handlers ──────────────────────────────────────────────────────────────

// handleBotMessage processes a customer message: rule-based FAQ → AI fallback.
func (b *PortalBridge) handleBotMessage(msg *nats.Msg) {
	if b.botSessions == nil {
		bridgeReply(msg, nil, &bridgeError{"bot not configured"})
		return
	}
	var req struct {
		CustomerID string `json:"customerID"`
		SessionID  string `json:"sessionID"`
		Message    string `json:"message"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	if req.CustomerID == "" {
		bridgeReply(msg, nil, errForbidden)
		return
	}
	if req.SessionID == "" {
		req.SessionID = uuid.Must(uuid.NewV7()).String()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	sess := b.botSessions.GetOrCreate(req.SessionID, req.CustomerID)
	// Ownership: session belongs to the authenticated customer only.
	if sess.CustomerID != req.CustomerID {
		bridgeReply(msg, nil, errForbidden)
		return
	}
	if sess.State == bot.StateClosed {
		bridgeReply(msg, map[string]any{"reply": "This conversation has ended.", "sessionID": req.SessionID, "state": sess.State}, nil)
		return
	}
	if sess.State == bot.StateLive {
		bridgeReply(msg, map[string]any{"reply": "You are connected to a staff member. Use /portal/bot/livemessage to send messages.", "sessionID": req.SessionID, "state": sess.State}, nil)
		return
	}

	// Build rule context.
	rc := b.buildRuleContext(ctx, req.CustomerID)

	// Try rule-based FAQ.
	reply, matched, suggestHandoff := bot.MatchRules(ctx, req.Message, rc)
	if !matched {
		// AI fallback via Ollama — snapshot history before releasing context.
		if b.botOllama != nil {
			history := b.botSessions.MessagesSnapshot(req.SessionID)
			reply = b.ollamaFallback(ctx, history, req.Message, rc)
		} else {
			reply = "I'm not sure how to answer that. Would you like to speak with a staff member?"
			suggestHandoff = true
		}
	}

	// Append to session history under lock.
	now := time.Now()
	b.botSessions.AppendMessages(req.SessionID,
		bot.BotMessage{Role: "user", Content: req.Message, At: now},
		bot.BotMessage{Role: "assistant", Content: reply, At: now},
	)

	bridgeReply(msg, map[string]any{
		"reply":          reply,
		"sessionID":      req.SessionID,
		"state":          sess.State,
		"suggestHandoff": suggestHandoff,
	}, nil)
}

// handleBotHandoff initiates a live staff handoff.
func (b *PortalBridge) handleBotHandoff(msg *nats.Msg) {
	if b.botSessions == nil || b.chatStore == nil {
		bridgeReply(msg, nil, &bridgeError{"bot not configured"})
		return
	}
	var req struct {
		CustomerID string `json:"customerID"`
		SessionID  string `json:"sessionID"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	if req.CustomerID == "" || req.SessionID == "" {
		bridgeReply(msg, nil, errForbidden)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess := b.botSessions.Get(req.SessionID)
	if sess == nil || sess.CustomerID != req.CustomerID {
		bridgeReply(msg, nil, errForbidden)
		return
	}

	threadID := fmt.Sprintf("portal-%s", req.SessionID)
	exists, err := b.chatStore.ThreadExists(ctx, threadID)
	if err != nil {
		bridgeReply(msg, nil, err)
		return
	}

	customerName := req.CustomerID
	if b.custReader != nil {
		if c, err := b.custReader.GetPortalCustomer(ctx, req.CustomerID); err == nil && c != nil {
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
			bridgeReply(msg, nil, err)
			return
		}

		// Post conversation history as first message (snapshot under lock).
		history := buildHistoryText(b.botSessions.MessagesSnapshot(req.SessionID))
		_ = b.chatStore.Save(ctx, chat.Message{
			ID:        uuid.Must(uuid.NewV7()).String(),
			ThreadID:  threadID,
			UserID:    "system",
			Username:  "Bot",
			Body:      fmt.Sprintf("Customer %s connected via portal chat.\n\n%s", customerName, history),
			CreatedAt: time.Now().UTC(),
		})
	}

	// Update state under lock.
	b.botSessions.UpdateState(req.SessionID, bot.StateHandoff, threadID)

	bridgeReply(msg, map[string]any{
		"threadID": threadID,
		"state":    bot.StateHandoff,
	}, nil)
}

// handleBotLiveMessage routes a message in a live handoff session.
func (b *PortalBridge) handleBotLiveMessage(msg *nats.Msg) {
	if b.botSessions == nil || b.chatStore == nil {
		bridgeReply(msg, nil, &bridgeError{"bot not configured"})
		return
	}
	var req struct {
		CustomerID string `json:"customerID"`
		SessionID  string `json:"sessionID"`
		Message    string `json:"message"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	if req.CustomerID == "" || req.SessionID == "" {
		bridgeReply(msg, nil, errForbidden)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess := b.botSessions.Get(req.SessionID)
	if sess == nil || sess.CustomerID != req.CustomerID || sess.ThreadID == "" {
		bridgeReply(msg, nil, errForbidden)
		return
	}

	// Store customer message.
	if req.Message != "" {
		_ = b.chatStore.Save(ctx, chat.Message{
			ID:        uuid.Must(uuid.NewV7()).String(),
			ThreadID:  sess.ThreadID,
			UserID:    req.CustomerID,
			Username:  "Customer",
			Body:      req.Message,
			CreatedAt: time.Now().UTC(),
		})
		b.botSessions.UpdateState(req.SessionID, bot.StateLive, "")
	}

	// Get latest staff reply.
	msgs, err := b.chatStore.Recent(ctx, sess.ThreadID, 1)
	if err != nil {
		bridgeReply(msg, map[string]any{"state": sess.State}, nil)
		return
	}
	var staffReply string
	for _, m := range msgs {
		if m.UserID != req.CustomerID && m.UserID != "system" {
			staffReply = m.Body
			break
		}
	}
	bridgeReply(msg, map[string]any{
		"staffReply": staffReply,
		"state":      sess.State,
	}, nil)
}

// handleBotClose closes a bot session and optionally creates a support ticket.
func (b *PortalBridge) handleBotClose(msg *nats.Msg) {
	if b.botSessions == nil {
		bridgeReply(msg, nil, &bridgeError{"bot not configured"})
		return
	}
	var req struct {
		CustomerID   string `json:"customerID"`
		SessionID    string `json:"sessionID"`
		CreateTicket bool   `json:"createTicket"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	if req.CustomerID == "" || req.SessionID == "" {
		bridgeReply(msg, nil, errForbidden)
		return
	}

	sess := b.botSessions.Get(req.SessionID)
	// Ownership check: only the session owner may close it.
	if sess != nil && sess.CustomerID != req.CustomerID {
		bridgeReply(msg, nil, errForbidden)
		return
	}
	ticketID := ""
	if req.CreateTicket && b.openTicket != nil && sess != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		subject := "Portal chat — unresolved question"
		body := buildHistoryText(b.botSessions.MessagesSnapshot(req.SessionID))
		if tk, err := b.openTicket.Handle(ctx, ticketcommands.OpenTicketCommand{
			CustomerID: req.CustomerID,
			Subject:    subject,
			Body:       body,
			Priority:   "normal",
		}); err == nil {
			ticketID = tk.ID
		}
	}
	b.botSessions.UpdateState(req.SessionID, bot.StateClosed, "")
	b.botSessions.Delete(req.SessionID)

	bridgeReply(msg, map[string]any{
		"ticketID": ticketID,
		"state":    bot.StateClosed,
	}, nil)
}

// buildRuleContext fetches data needed for rule evaluation.
func (b *PortalBridge) buildRuleContext(ctx context.Context, customerID string) *bot.RuleContext {
	rc := &bot.RuleContext{CustomerID: customerID}

	if b.custReader != nil {
		if c, err := b.custReader.GetPortalCustomer(ctx, customerID); err == nil && c != nil {
			rc.Customer = &bot.CustomerInfo{
				CompanyName: c.CompanyName,
				Email:       c.Email,
				IPAddress:   c.IPAddress,
			}
		}
	}

	if b.serviceLister != nil {
		if svcs, err := b.serviceLister.ListForCustomer(ctx, customerID); err == nil {
			for _, s := range svcs {
				rc.Services = append(rc.Services, &bot.ServiceInfo{
					ID:               s.ID,
					ProductName:      s.ProductName,
					PriceAmountCents: s.PriceAmountCents,
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

// ollamaFallback calls Ollama with session history + system prompt.
// history is a snapshot taken before the mutex was released.
func (b *PortalBridge) ollamaFallback(ctx context.Context, history []bot.BotMessage, userMsg string, rc *bot.RuleContext) string {
	sysPrompt := bot.BuildSystemPrompt(rc.Customer, rc.Services, rc.Invoices)
	msgs := []bot.OllamaMessage{{Role: "system", Content: sysPrompt}}
	for _, m := range history {
		msgs = append(msgs, bot.OllamaMessage{Role: m.Role, Content: m.Content})
	}
	msgs = append(msgs, bot.OllamaMessage{Role: "user", Content: userMsg})

	reply, err := b.botOllama.Chat(ctx, msgs)
	if err != nil {
		log.Printf("portal bot: ollama: %v", err)
		return "I'm having trouble answering that right now. Would you like to speak with a staff member?"
	}
	return strings.TrimSpace(reply)
}

// buildHistoryText formats the bot session messages as plain text for handoff/ticket.
func buildHistoryText(messages []bot.BotMessage) string {
	if len(messages) == 0 {
		return "(no prior messages)"
	}
	var sb strings.Builder
	for _, m := range messages {
		role := "Customer"
		if m.Role == "assistant" {
			role = "Bot"
		}
		sb.WriteString(fmt.Sprintf("[%s] %s\n", role, m.Content))
	}
	return sb.String()
}

func (b *PortalBridge) handleServicesList(msg *nats.Msg) {
	if b.serviceLister == nil {
		bridgeReply(msg, nil, &bridgeError{"service list handler not configured"})
		return
	}
	var req struct {
		CustomerID string `json:"customerID"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	if req.CustomerID == "" {
		bridgeReply(msg, nil, errForbidden)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	services, err := b.serviceLister.ListForCustomer(ctx, req.CustomerID)
	if err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	bridgeReply(msg, struct {
		Services []*BridgeService `json:"services"`
	}{services}, nil)
}

func (b *PortalBridge) handleCustomerGet(msg *nats.Msg) {
	var req struct {
		CustomerID string `json:"customerID"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, err := b.custReader.GetPortalCustomer(ctx, req.CustomerID)
	if err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	bridgeReply(msg, c, nil)
}

func (b *PortalBridge) handleTicketsList(msg *nats.Msg) {
	if b.ticketLister == nil {
		bridgeReply(msg, nil, &bridgeError{"ticket list handler not configured"})
		return
	}
	var req struct {
		CustomerID string `json:"customerID"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	if req.CustomerID == "" {
		bridgeReply(msg, nil, errForbidden)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tickets, err := b.ticketLister.Handle(ctx, ticketqueries.ListTicketsForCustomerQuery{CustomerID: req.CustomerID})
	if err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	bridgeReply(msg, struct {
		Tickets []ticketqueries.TicketReadModel `json:"tickets"`
	}{tickets}, nil)
}

// handleTicketOpen opens a support ticket on behalf of the authenticated portal customer.
// customerID is mandatory — enforces that the ticket is owned by the caller.
func (b *PortalBridge) handleTicketOpen(msg *nats.Msg) {
	if b.openTicket == nil {
		bridgeReply(msg, nil, &bridgeError{"ticket open handler not configured"})
		return
	}
	var req struct {
		CustomerID string `json:"customerID"`
		Subject    string `json:"subject"`
		Body       string `json:"body"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	if req.CustomerID == "" {
		bridgeReply(msg, nil, errForbidden)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tk, err := b.openTicket.Handle(ctx, ticketcommands.OpenTicketCommand{
		CustomerID: req.CustomerID,
		Subject:    req.Subject,
		Body:       req.Body,
		Priority:   "normal",
	})
	if err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	bridgeReply(msg, struct {
		TicketID string `json:"ticketID"`
	}{tk.ID}, nil)
}

// handleTicketCommentAdd adds a customer comment to a ticket.
// customerID is mandatory and verified against the ticket owner.
func (b *PortalBridge) handleTicketCommentAdd(msg *nats.Msg) {
	if b.ticketLister == nil || b.addComment == nil {
		bridgeReply(msg, nil, &bridgeError{"ticket comment handler not configured"})
		return
	}
	var req struct {
		TicketID   string `json:"ticketID"`
		CustomerID string `json:"customerID"`
		Body       string `json:"body"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	if req.CustomerID == "" {
		bridgeReply(msg, nil, errForbidden)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Verify ownership — list tickets and check the target ticket belongs to this customer.
	tickets, err := b.ticketLister.Handle(ctx, ticketqueries.ListTicketsForCustomerQuery{CustomerID: req.CustomerID})
	if err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	owned := false
	for _, t := range tickets {
		if t.ID == req.TicketID {
			owned = true
			break
		}
	}
	if !owned {
		bridgeReply(msg, nil, errForbidden)
		return
	}

	comment, err := b.addComment.Handle(ctx, ticketcommands.AddCommentCommand{
		TicketID: req.TicketID,
		Body:     req.Body,
		AuthorID: req.CustomerID,
	})
	if err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	bridgeReply(msg, struct {
		CommentID string `json:"commentID"`
	}{comment.ID}, nil)
}

// ── helpers ───────────────────────────────────────────────────────────────────

type envelope struct {
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

var (
	errExpired   = &bridgeError{"token expired or not found"}
	errForbidden = &bridgeError{"forbidden"}
)

type bridgeError struct{ msg string }

func (e *bridgeError) Error() string { return e.msg }

func bridgeReply(msg *nats.Msg, data any, err error) {
	var env envelope
	if err != nil {
		env = envelope{Error: err.Error()}
	} else {
		env = envelope{Data: data}
	}
	b, merr := json.Marshal(env)
	if merr != nil {
		log.Printf("portal bridge: marshal reply: %v", merr)
		return
	}
	if err := msg.Respond(b); err != nil {
		log.Printf("portal bridge: respond: %v", err)
	}
}

// ── Self-service login subjects ───────────────────────────────────────────────

func (b *PortalBridge) handleCustomerFindByEmail(msg *nats.Msg) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var req struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil || req.Email == "" {
		bridgeReply(msg, nil, fmt.Errorf("invalid request"))
		return
	}
	if b.emailFinder == nil {
		bridgeReply(msg, nil, fmt.Errorf("email lookup not configured"))
		return
	}
	customerID, err := b.emailFinder.FindByEmail(ctx, req.Email)
	if err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	bridgeReply(msg, map[string]string{"customerID": customerID}, nil)
}

func (b *PortalBridge) handlePortalTokenCreate(msg *nats.Msg) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var req struct {
		CustomerID string `json:"customerID"`
		TTLSeconds int    `json:"ttlSeconds"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil || req.CustomerID == "" {
		bridgeReply(msg, nil, fmt.Errorf("invalid request"))
		return
	}
	ttl := time.Duration(req.TTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	tok, plain, err := portaldomain.NewPortalToken(req.CustomerID, ttl)
	if err != nil {
		bridgeReply(msg, nil, fmt.Errorf("generate token: %w", err))
		return
	}
	if err := b.tokenRepo.Save(ctx, tok); err != nil {
		bridgeReply(msg, nil, fmt.Errorf("save token: %w", err))
		return
	}
	bridgeReply(msg, map[string]any{
		"plain":     plain,
		"expiresAt": tok.ExpiresAt,
	}, nil)
}
