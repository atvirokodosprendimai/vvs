// Package nats — client side of the portal NATS RPC bridge.
// PortalNATSClient runs in vvs-portal and delegates all data access to vvs-core
// via the isp.portal.rpc.* subjects served by PortalBridge.
package nats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	invoicequeries "github.com/vvs/isp/internal/modules/invoice/app/queries"
	portaldomain "github.com/vvs/isp/internal/modules/portal/domain"
	ticketqueries "github.com/vvs/isp/internal/modules/ticket/app/queries"
)

// PortalNATSClient satisfies the interfaces expected by portal/adapters/http.Handlers:
//   - domain.PortalTokenRepository  (FindByHash only — write ops return ErrNotSupported)
//   - invoiceLister
//   - invoiceGetter (via InvoiceGetterAdapter)
//   - pdfTokenMinter
//   - NATSCustomerReader (use NATSCustomerAdapter to satisfy customerReader)
type PortalNATSClient struct {
	nc      *nats.Conn
	timeout time.Duration
}

// NewPortalNATSClient creates a client. timeout controls per-RPC deadline (0 → 5s default).
func NewPortalNATSClient(nc *nats.Conn, timeout time.Duration) *PortalNATSClient {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &PortalNATSClient{nc: nc, timeout: timeout}
}

// ── domain.PortalTokenRepository ─────────────────────────────────────────────

// FindByHash validates a portal session token via NATS.
// Satisfies domain.PortalTokenRepository.FindByHash.
func (c *PortalNATSClient) FindByHash(ctx context.Context, hash string) (*portaldomain.PortalToken, error) {
	var resp struct {
		CustomerID string     `json:"customerID"`
		ExpiresAt  time.Time  `json:"expiresAt"`
		UsedAt     *time.Time `json:"usedAt,omitempty"`
	}
	if err := c.rpc(ctx, SubjectTokenValidate, map[string]string{"hash": hash}, &resp); err != nil {
		return nil, err
	}
	return &portaldomain.PortalToken{
		CustomerID: resp.CustomerID,
		ExpiresAt:  resp.ExpiresAt,
		UsedAt:     resp.UsedAt,
	}, nil
}

// MarkUsed marks a portal magic-link token as consumed via NATS.
// Satisfies domain.PortalTokenRepository.MarkUsed.
func (c *PortalNATSClient) MarkUsed(ctx context.Context, tokenHash string) error {
	var resp struct{ OK bool }
	return c.rpc(ctx, SubjectTokenMarkUsed, map[string]string{"hash": tokenHash}, &resp)
}

// Save is not used on the portal side.
func (c *PortalNATSClient) Save(_ context.Context, _ *portaldomain.PortalToken) error {
	return ErrNotSupported
}

// DeleteByCustomerID is not used on the portal side.
func (c *PortalNATSClient) DeleteByCustomerID(_ context.Context, _ string) error {
	return ErrNotSupported
}

// PruneExpired is not used on the portal side.
func (c *PortalNATSClient) PruneExpired(_ context.Context) error {
	return ErrNotSupported
}

// ── invoiceLister ─────────────────────────────────────────────────────────────

// Handle lists invoices for a customer via NATS.
// Satisfies portal/adapters/http.invoiceLister.
func (c *PortalNATSClient) Handle(ctx context.Context, q invoicequeries.ListInvoicesForCustomerQuery) ([]invoicequeries.InvoiceReadModel, error) {
	var resp struct {
		Invoices []invoicequeries.InvoiceReadModel `json:"invoices"`
	}
	if err := c.rpc(ctx, SubjectInvoicesList, map[string]string{"customerID": q.CustomerID}, &resp); err != nil {
		return nil, err
	}
	return resp.Invoices, nil
}

// ── invoiceGetter ─────────────────────────────────────────────────────────────

// GetInvoice retrieves a single invoice by ID for a specific customer via NATS.
// customerID is mandatory — the bridge enforces ownership.
// Use InvoiceGetterAdapter to satisfy the invoiceGetter interface.
func (c *PortalNATSClient) GetInvoice(ctx context.Context, id, customerID string) (*invoicequeries.InvoiceReadModel, error) {
	var resp struct {
		Invoice *invoicequeries.InvoiceReadModel `json:"invoice"`
	}
	if err := c.rpc(ctx, SubjectInvoiceGet, map[string]string{"invoiceID": id, "customerID": customerID}, &resp); err != nil {
		return nil, err
	}
	return resp.Invoice, nil
}

// GetInvoiceByTokenHash validates a PDF token and returns the invoice in one call.
// Used by GET /i/{token} — no customer session required; the token proves access.
func (c *PortalNATSClient) GetInvoiceByTokenHash(ctx context.Context, tokenHash string) (*invoicequeries.InvoiceReadModel, error) {
	var resp struct {
		Invoice *invoicequeries.InvoiceReadModel `json:"invoice"`
	}
	if err := c.rpc(ctx, SubjectInvoiceGetByToken, map[string]string{"tokenHash": tokenHash}, &resp); err != nil {
		return nil, err
	}
	return resp.Invoice, nil
}

// ── pdfTokenMinter ───────────────────────────────────────────────────────────

// MintToken mints a new invoice PDF token via NATS.
// customerID is required; the bridge verifies ownership before minting.
// Satisfies portal/adapters/http.pdfTokenMinter.
func (c *PortalNATSClient) MintToken(ctx context.Context, invoiceID, customerID string) (string, error) {
	var resp struct {
		Plain string `json:"plain"`
	}
	if err := c.rpc(ctx, SubjectInvoiceTokenMint, map[string]string{"invoiceID": invoiceID, "customerID": customerID}, &resp); err != nil {
		return "", err
	}
	return resp.Plain, nil
}

// ── customer ─────────────────────────────────────────────────────────────────

// GetCustomer fetches customer info for the portal header via NATS.
func (c *PortalNATSClient) GetCustomer(ctx context.Context, id string) (*BridgeCustomer, error) {
	var cust BridgeCustomer
	if err := c.rpc(ctx, SubjectCustomerGet, map[string]string{"customerID": id}, &cust); err != nil {
		return nil, err
	}
	return &cust, nil
}

// ListServices returns the customer's active and suspended services via NATS.
func (c *PortalNATSClient) ListServices(ctx context.Context, customerID string) ([]*BridgeService, error) {
	var resp struct {
		Services []*BridgeService `json:"services"`
	}
	if err := c.rpc(ctx, SubjectServicesList, map[string]string{"customerID": customerID}, &resp); err != nil {
		return nil, err
	}
	return resp.Services, nil
}

// ValidateInvoiceToken validates a PDF token hash via NATS.
// Returns the InvoiceID the token grants access to.
func (c *PortalNATSClient) ValidateInvoiceToken(ctx context.Context, tokenHash string) (string, error) {
	var resp struct {
		InvoiceID string `json:"invoiceID"`
	}
	if err := c.rpc(ctx, SubjectInvoiceTokenValidate, map[string]string{"tokenHash": tokenHash}, &resp); err != nil {
		return "", err
	}
	return resp.InvoiceID, nil
}

// ── tickets ───────────────────────────────────────────────────────────────────

// ListTickets returns all tickets for the authenticated portal customer.
func (c *PortalNATSClient) ListTickets(ctx context.Context, customerID string) ([]ticketqueries.TicketReadModel, error) {
	var resp struct {
		Tickets []ticketqueries.TicketReadModel `json:"tickets"`
	}
	if err := c.rpc(ctx, SubjectTicketsList, map[string]string{"customerID": customerID}, &resp); err != nil {
		return nil, err
	}
	return resp.Tickets, nil
}

// OpenTicket opens a new support ticket on behalf of the portal customer.
// Returns the new ticket ID.
func (c *PortalNATSClient) OpenTicket(ctx context.Context, customerID, subject, body string) (string, error) {
	var resp struct {
		TicketID string `json:"ticketID"`
	}
	if err := c.rpc(ctx, SubjectTicketOpen, map[string]string{
		"customerID": customerID,
		"subject":    subject,
		"body":       body,
	}, &resp); err != nil {
		return "", err
	}
	return resp.TicketID, nil
}

// AddTicketComment adds a customer comment to an existing ticket.
// Returns the new comment ID.
func (c *PortalNATSClient) AddTicketComment(ctx context.Context, ticketID, customerID, body string) (string, error) {
	var resp struct {
		CommentID string `json:"commentID"`
	}
	if err := c.rpc(ctx, SubjectTicketCommentAdd, map[string]string{
		"ticketID":   ticketID,
		"customerID": customerID,
		"body":       body,
	}, &resp); err != nil {
		return "", err
	}
	return resp.CommentID, nil
}

// ── InvoiceGetterAdapter ──────────────────────────────────────────────────────

// InvoiceGetterAdapter wraps PortalNATSClient so it satisfies the invoiceGetter interface
// (Handle(ctx, id string) (*InvoiceReadModel, error)) used by portal/adapters/http.
// CustomerIDFromCtx extracts the authenticated customer ID from the request context —
// inject portalhttp.PortalCustomerIDFromContext when constructing this adapter.
type InvoiceGetterAdapter struct {
	C                 *PortalNATSClient
	CustomerIDFromCtx func(context.Context) string
}

func (a *InvoiceGetterAdapter) Handle(ctx context.Context, id string) (*invoicequeries.InvoiceReadModel, error) {
	customerID := ""
	if a.CustomerIDFromCtx != nil {
		customerID = a.CustomerIDFromCtx(ctx)
	}
	return a.C.GetInvoice(ctx, id, customerID)
}

// ── helpers ───────────────────────────────────────────────────────────────────

// ErrNotSupported is returned by write operations not available on the portal side.
var ErrNotSupported = errors.New("operation not supported on portal binary")

// rpc marshals req, sends to subject with context-aware timeout, and unmarshals the reply into out.
// ctx cancellation propagates; c.timeout acts as a maximum deadline.
func (c *PortalNATSClient) rpc(ctx context.Context, subject string, req any, out any) error {
	b, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("portal rpc: marshal: %w", err)
	}
	tctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	msg, err := c.nc.RequestMsgWithContext(tctx, &nats.Msg{Subject: subject, Data: b})
	if err != nil {
		return fmt.Errorf("portal rpc %s: %w", subject, err)
	}
	var env struct {
		Data  json.RawMessage `json:"data,omitempty"`
		Error string          `json:"error,omitempty"`
	}
	if err := json.Unmarshal(msg.Data, &env); err != nil {
		return fmt.Errorf("portal rpc %s: unmarshal envelope: %w", subject, err)
	}
	if env.Error != "" {
		return fmt.Errorf("portal rpc %s: %s", subject, env.Error)
	}
	if out != nil {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return fmt.Errorf("portal rpc %s: unmarshal data: %w", subject, err)
		}
	}
	return nil
}
