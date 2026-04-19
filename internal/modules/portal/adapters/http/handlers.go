package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"
	infrahttp "github.com/vvs/isp/internal/infrastructure/http"
	invoicequeries "github.com/vvs/isp/internal/modules/invoice/app/queries"
	"github.com/vvs/isp/internal/modules/portal/domain"
	ticketqueries "github.com/vvs/isp/internal/modules/ticket/app/queries"
	authhttp "github.com/vvs/isp/internal/modules/auth/adapters/http"
)

const portalCookieName = "vvs_portal"

// invoiceLister lists invoices for a customer.
// Satisfied by *invoicequeries.ListInvoicesForCustomerHandler and by the NATS portal client.
type invoiceLister interface {
	Handle(ctx context.Context, q invoicequeries.ListInvoicesForCustomerQuery) ([]invoicequeries.InvoiceReadModel, error)
}

// invoiceGetter retrieves a single invoice by ID.
// Satisfied by *invoicequeries.GetInvoiceHandler and by the NATS portal client.
type invoiceGetter interface {
	Handle(ctx context.Context, id string) (*invoicequeries.InvoiceReadModel, error)
}

// pdfTokenMinter mints a public PDF access token for an invoice.
// Returns the plain token string to embed in the URL.
// customerID is required so the bridge can verify ownership before minting.
// Satisfied by a core-side adapter wrapping invoicedomain.NewInvoiceToken+Save,
// and by the NATS portal client calling isp.portal.rpc.invoice.token.mint.
type pdfTokenMinter interface {
	MintToken(ctx context.Context, invoiceID, customerID string) (plain string, err error)
}

// customerReader fetches customer info for the portal header.
type customerReader interface {
	GetPortalCustomer(ctx context.Context, id string) (*PortalCustomer, error)
}

// portalTicketClient lists, opens, and comments on support tickets.
type portalTicketClient interface {
	ListTickets(ctx context.Context, customerID string) ([]ticketqueries.TicketReadModel, error)
	OpenTicket(ctx context.Context, customerID, subject, body string) (string, error)
	AddTicketComment(ctx context.Context, ticketID, customerID, body string) (string, error)
}

// portalServiceClient lists the customer's services.
type portalServiceClient interface {
	ListServices(ctx context.Context, customerID string) ([]*PortalService, error)
}

// portalBotClient communicates with the portal chat bot via NATS.
type portalBotClient interface {
	BotMessage(ctx context.Context, customerID, sessionID, message string) (reply, newSessionID, state string, suggestHandoff bool, err error)
	BotHandoff(ctx context.Context, customerID, sessionID string) (threadID, state string, err error)
	BotLiveMessage(ctx context.Context, customerID, sessionID, message string) (staffReply, state string, err error)
	BotClose(ctx context.Context, customerID, sessionID string, createTicket bool) (ticketID string, err error)
}

// PortalService is the service view presented in the portal.
type PortalService struct {
	ID               string
	ProductName      string
	PriceAmountCents int64
	Currency         string
	Status           string
	BillingCycle     string
	NextBillingDate  *time.Time
}

// PortalCustomer holds the customer info shown in the portal header.
type PortalCustomer struct {
	ID          string
	CompanyName string
	Email       string
	IPAddress   string
	NetworkZone string
}

// Handlers serves the customer portal — public-facing, separate from admin UI.
type Handlers struct {
	tokenRepo    domain.PortalTokenRepository
	listInvoices invoiceLister
	getInvoice   invoiceGetter
	pdfTokens    pdfTokenMinter
	custReader   customerReader
	tickets      portalTicketClient
	services     portalServiceClient
	bot          portalBotClient
	baseURL      string
	secureCookie bool
}

func NewHandlers(
	tokenRepo domain.PortalTokenRepository,
	listInvoices invoiceLister,
	getInvoice invoiceGetter,
) *Handlers {
	return &Handlers{
		tokenRepo:    tokenRepo,
		listInvoices: listInvoices,
		getInvoice:   getInvoice,
	}
}

func (h *Handlers) WithPDFTokens(m pdfTokenMinter) *Handlers {
	h.pdfTokens = m
	return h
}

func (h *Handlers) WithCustomerReader(cr customerReader) *Handlers {
	h.custReader = cr
	return h
}

func (h *Handlers) WithBaseURL(url string) *Handlers {
	h.baseURL = url
	return h
}

func (h *Handlers) WithSecureCookie(secure bool) *Handlers {
	h.secureCookie = secure
	return h
}

func (h *Handlers) WithTickets(tc portalTicketClient) *Handlers {
	h.tickets = tc
	return h
}

func (h *Handlers) WithServices(sc portalServiceClient) *Handlers {
	h.services = sc
	return h
}

func (h *Handlers) WithBot(bc portalBotClient) *Handlers {
	h.bot = bc
	return h
}

// RegisterRoutes registers admin routes (protected by RequireAuth in the router).
func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Post("/api/customers/{id}/portal-link", h.generatePortalLink)
}

// authLimiter allows 10 magic-link auth attempts per IP per 15 minutes.
var authLimiter = infrahttp.NewIPRateLimiter(10, 15*time.Minute)

// RegisterPublicRoutes registers portal routes before the auth middleware.
// Implements infrastructure/http.PublicModuleRoutes.
func (h *Handlers) RegisterPublicRoutes(r chi.Router) {
	r.With(authLimiter.Middleware()).Get("/portal/auth", h.portalAuth)
	r.Post("/portal/logout", h.portalLogout)
	r.Group(func(r chi.Router) {
		r.Use(h.requirePortalAuth)
		r.Get("/portal", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/portal/invoices", http.StatusFound)
		})
		r.Get("/portal/invoices", h.invoiceList)
		r.Get("/portal/invoices/{id}", h.invoiceDetail)
		r.Get("/portal/tickets", h.ticketList)
		r.Get("/portal/services", h.serviceList)
		r.Get("/portal/tickets/new", h.ticketNew)
		r.Post("/portal/tickets", h.ticketCreate)
		r.Get("/portal/tickets/{id}", h.ticketDetail)
		r.Post("/portal/tickets/{id}/comments", h.ticketCommentAdd)
		r.Post("/portal/bot/message", h.botMessage)
		r.Post("/portal/bot/handoff", h.botHandoff)
		r.Post("/portal/bot/livemessage", h.botLiveMessage)
		r.Post("/portal/bot/close", h.botClose)
	})
}

// requirePortalAuth validates the portal cookie and injects customerID into context.
func (h *Handlers) requirePortalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(portalCookieName)
		if err != nil {
			http.Redirect(w, r, "/portal/auth?expired=1", http.StatusFound)
			return
		}
		tok, err := h.tokenRepo.FindByHash(r.Context(), domain.HashOf(cookie.Value))
		if err != nil || tok == nil || tok.IsExpired() {
			http.Redirect(w, r, "/portal/auth?expired=1", http.StatusFound)
			return
		}
		ctx := context.WithValue(r.Context(), portalCustomerKey{}, tok.CustomerID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type portalCustomerKey struct{}

// PortalCustomerIDFromContext returns the authenticated customer ID stored in ctx
// by requirePortalAuth. Returns "" if not set.
func PortalCustomerIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(portalCustomerKey{}).(string)
	return v
}

// portalAuth validates a token from the URL, sets the portal cookie, and redirects.
func (h *Handlers) portalAuth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	plain := r.URL.Query().Get("token")
	if plain == "" {
		PortalExpiredPage().Render(r.Context(), w)
		return
	}

	tok, err := h.tokenRepo.FindByHash(r.Context(), domain.HashOf(plain))
	if err != nil || tok == nil || tok.IsExpired() || tok.IsUsed() {
		PortalExpiredPage().Render(r.Context(), w)
		return
	}

	// Mark token as consumed before issuing the cookie — single-use enforcement.
	if err := h.tokenRepo.MarkUsed(r.Context(), domain.HashOf(plain)); err != nil {
		log.Printf("portal auth: mark token used: %v", err)
		// Non-fatal: proceed to issue cookie (MarkUsed failure is better than locking the user out).
	}

	http.SetCookie(w, &http.Cookie{
		Name:     portalCookieName,
		Value:    plain,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.secureCookie,
		MaxAge:   int(time.Until(tok.ExpiresAt).Seconds()),
	})
	http.Redirect(w, r, "/portal/invoices", http.StatusFound)
}

// portalLogout clears the portal cookie.
func (h *Handlers) portalLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     portalCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.secureCookie,
		MaxAge:   -1,
	})
	http.Redirect(w, r, "/portal/auth?expired=1", http.StatusFound)
}

// invoiceList renders the customer's invoice list.
func (h *Handlers) invoiceList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	customerID := PortalCustomerIDFromContext(r.Context())

	invoices, err := h.listInvoices.Handle(r.Context(), invoicequeries.ListInvoicesForCustomerQuery{
		CustomerID: customerID,
	})
	if err != nil {
		log.Printf("portal invoiceList: %v", err)
		http.Error(w, "error loading invoices", http.StatusInternalServerError)
		return
	}

	cust := h.resolveCustomer(r.Context(), customerID)
	PortalInvoiceListPage(cust, invoices).Render(r.Context(), w)
}

// invoiceDetail renders a single invoice detail with a PDF link.
func (h *Handlers) invoiceDetail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	customerID := PortalCustomerIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	inv, err := h.getInvoice.Handle(r.Context(), id)
	if err != nil || inv == nil {
		http.Error(w, "invoice not found", http.StatusNotFound)
		return
	}
	// Ownership check — customer can only view their own invoices.
	if inv.CustomerID != customerID {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	pdfURL := ""
	if h.pdfTokens != nil {
		if plain, err := h.pdfTokens.MintToken(r.Context(), inv.ID, customerID); err == nil {
			pdfURL = fmt.Sprintf("/i/%s", plain)
		}
	}

	cust := h.resolveCustomer(r.Context(), customerID)
	PortalInvoiceDetailPage(cust, inv, pdfURL).Render(r.Context(), w)
}

// generatePortalLink is the admin action to create a portal access link for a customer.
func (h *Handlers) generatePortalLink(w http.ResponseWriter, r *http.Request) {
	actor := authhttp.UserFromContext(r.Context())
	if actor == nil || !actor.IsAdmin() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	customerID := chi.URLParam(r, "id")
	tok, plain, err := domain.NewPortalToken(customerID, 15*time.Minute)
	if err != nil {
		log.Printf("portal generatePortalLink: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := h.tokenRepo.Save(r.Context(), tok); err != nil {
		log.Printf("portal generatePortalLink: save: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	baseURL := h.baseURL
	if baseURL == "" {
		baseURL = "http://" + r.Host
	}
	portalURL := fmt.Sprintf("%s/portal/auth?token=%s", baseURL, plain)

	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(PortalLinkFragment(portalURL))
}

// serviceList renders the customer's active services.
func (h *Handlers) serviceList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	customerID := PortalCustomerIDFromContext(r.Context())
	if h.services == nil {
		http.Error(w, "service information not available", http.StatusServiceUnavailable)
		return
	}

	services, err := h.services.ListServices(r.Context(), customerID)
	if err != nil {
		log.Printf("portal serviceList: %v", err)
		http.Error(w, "error loading services", http.StatusInternalServerError)
		return
	}

	cust := h.resolveCustomer(r.Context(), customerID)
	PortalServiceListPage(cust, services).Render(r.Context(), w)
}

// ticketList renders the customer's support ticket list.
func (h *Handlers) ticketList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	customerID := PortalCustomerIDFromContext(r.Context())
	if h.tickets == nil {
		http.Error(w, "support tickets not available", http.StatusServiceUnavailable)
		return
	}

	tickets, err := h.tickets.ListTickets(r.Context(), customerID)
	if err != nil {
		log.Printf("portal ticketList: %v", err)
		http.Error(w, "error loading tickets", http.StatusInternalServerError)
		return
	}

	cust := h.resolveCustomer(r.Context(), customerID)
	PortalTicketListPage(cust, tickets).Render(r.Context(), w)
}

// ticketNew renders the new ticket form.
func (h *Handlers) ticketNew(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	customerID := PortalCustomerIDFromContext(r.Context())
	cust := h.resolveCustomer(r.Context(), customerID)
	PortalTicketNewPage(cust, "").Render(r.Context(), w)
}

// ticketCreate opens a new support ticket.
func (h *Handlers) ticketCreate(w http.ResponseWriter, r *http.Request) {
	if h.tickets == nil {
		http.Error(w, "support tickets not available", http.StatusServiceUnavailable)
		return
	}
	customerID := PortalCustomerIDFromContext(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	subject := r.FormValue("subject")
	body := r.FormValue("body")
	if subject == "" || body == "" {
		cust := h.resolveCustomer(r.Context(), customerID)
		PortalTicketNewPage(cust, "Subject and message are required.").Render(r.Context(), w)
		return
	}

	ticketID, err := h.tickets.OpenTicket(r.Context(), customerID, subject, body)
	if err != nil {
		log.Printf("portal ticketCreate: %v", err)
		cust := h.resolveCustomer(r.Context(), customerID)
		PortalTicketNewPage(cust, "Failed to open ticket. Please try again.").Render(r.Context(), w)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/portal/tickets/%s", ticketID), http.StatusSeeOther)
}

// ticketDetail renders a single support ticket with comments.
func (h *Handlers) ticketDetail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	customerID := PortalCustomerIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	if h.tickets == nil {
		http.Error(w, "support tickets not available", http.StatusServiceUnavailable)
		return
	}

	tickets, err := h.tickets.ListTickets(r.Context(), customerID)
	if err != nil {
		log.Printf("portal ticketDetail: %v", err)
		http.Error(w, "error loading ticket", http.StatusInternalServerError)
		return
	}
	var found *ticketqueries.TicketReadModel
	for i := range tickets {
		if tickets[i].ID == id {
			found = &tickets[i]
			break
		}
	}
	if found == nil {
		http.Error(w, "ticket not found", http.StatusNotFound)
		return
	}

	cust := h.resolveCustomer(r.Context(), customerID)
	PortalTicketDetailPage(cust, found, "").Render(r.Context(), w)
}

// ticketCommentAdd adds a comment to an existing support ticket.
func (h *Handlers) ticketCommentAdd(w http.ResponseWriter, r *http.Request) {
	if h.tickets == nil {
		http.Error(w, "support tickets not available", http.StatusServiceUnavailable)
		return
	}
	customerID := PortalCustomerIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	body := r.FormValue("body")
	if body == "" {
		http.Redirect(w, r, fmt.Sprintf("/portal/tickets/%s", id), http.StatusSeeOther)
		return
	}

	if _, err := h.tickets.AddTicketComment(r.Context(), id, customerID, body); err != nil {
		log.Printf("portal ticketCommentAdd: %v", err)
		// Re-render ticket detail with error so the customer knows the comment failed.
		tickets, _ := h.tickets.ListTickets(r.Context(), customerID)
		var found *ticketqueries.TicketReadModel
		for i := range tickets {
			if tickets[i].ID == id {
				found = &tickets[i]
				break
			}
		}
		cust := h.resolveCustomer(r.Context(), customerID)
		PortalTicketDetailPage(cust, found, "Failed to post comment. Please try again.").Render(r.Context(), w)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/portal/tickets/%s", id), http.StatusSeeOther)
}

// botMessage handles POST /portal/bot/message.
func (h *Handlers) botMessage(w http.ResponseWriter, r *http.Request) {
	if h.bot == nil {
		http.Error(w, "bot not available", http.StatusServiceUnavailable)
		return
	}
	customerID := PortalCustomerIDFromContext(r.Context())
	var req struct {
		SessionID string `json:"sessionID"`
		Message   string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	reply, sid, state, suggest, err := h.bot.BotMessage(r.Context(), customerID, req.SessionID, req.Message)
	if err != nil {
		log.Printf("portal botMessage: %v", err)
		http.Error(w, "bot error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
		"reply":          reply,
		"sessionID":      sid,
		"state":          state,
		"suggestHandoff": suggest,
	})
}

// botHandoff handles POST /portal/bot/handoff.
func (h *Handlers) botHandoff(w http.ResponseWriter, r *http.Request) {
	if h.bot == nil {
		http.Error(w, "bot not available", http.StatusServiceUnavailable)
		return
	}
	customerID := PortalCustomerIDFromContext(r.Context())
	var req struct {
		SessionID string `json:"sessionID"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	_, state, err := h.bot.BotHandoff(r.Context(), customerID, req.SessionID)
	if err != nil {
		log.Printf("portal botHandoff: %v", err)
		http.Error(w, "handoff error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"state": state}) //nolint:errcheck
}

// botLiveMessage handles POST /portal/bot/livemessage.
func (h *Handlers) botLiveMessage(w http.ResponseWriter, r *http.Request) {
	if h.bot == nil {
		http.Error(w, "bot not available", http.StatusServiceUnavailable)
		return
	}
	customerID := PortalCustomerIDFromContext(r.Context())
	var req struct {
		SessionID string `json:"sessionID"`
		Message   string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	staffReply, state, err := h.bot.BotLiveMessage(r.Context(), customerID, req.SessionID, req.Message)
	if err != nil {
		log.Printf("portal botLiveMessage: %v", err)
		http.Error(w, "live message error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
		"staffReply": staffReply,
		"state":      state,
	})
}

// botClose handles POST /portal/bot/close.
func (h *Handlers) botClose(w http.ResponseWriter, r *http.Request) {
	if h.bot == nil {
		http.Error(w, "bot not available", http.StatusServiceUnavailable)
		return
	}
	customerID := PortalCustomerIDFromContext(r.Context())
	var req struct {
		SessionID    string `json:"sessionID"`
		CreateTicket bool   `json:"createTicket"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	ticketID, err := h.bot.BotClose(r.Context(), customerID, req.SessionID, req.CreateTicket)
	if err != nil {
		log.Printf("portal botClose: %v", err)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ticketID": ticketID}) //nolint:errcheck
}

// resolveCustomer fetches customer info for the portal header, returning a fallback on error.
func (h *Handlers) resolveCustomer(ctx context.Context, customerID string) *PortalCustomer {
	if h.custReader == nil {
		return &PortalCustomer{ID: customerID}
	}
	c, err := h.custReader.GetPortalCustomer(ctx, customerID)
	if err != nil || c == nil {
		return &PortalCustomer{ID: customerID}
	}
	return c
}
