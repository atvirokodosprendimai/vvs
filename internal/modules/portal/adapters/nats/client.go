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
)

// PortalNATSClient satisfies the interfaces expected by portal/adapters/http.Handlers:
//   - domain.PortalTokenRepository  (FindByHash only — write ops return ErrNotSupported)
//   - invoiceLister
//   - invoiceGetter
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
func (c *PortalNATSClient) FindByHash(_ context.Context, hash string) (*portaldomain.PortalToken, error) {
	var resp struct {
		CustomerID string    `json:"customerID"`
		ExpiresAt  time.Time `json:"expiresAt"`
	}
	if err := c.rpc(SubjectTokenValidate, map[string]string{"hash": hash}, &resp); err != nil {
		return nil, err
	}
	return &portaldomain.PortalToken{
		CustomerID: resp.CustomerID,
		ExpiresAt:  resp.ExpiresAt,
	}, nil
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
func (c *PortalNATSClient) Handle(_ context.Context, q invoicequeries.ListInvoicesForCustomerQuery) ([]invoicequeries.InvoiceReadModel, error) {
	var resp struct {
		Invoices []invoicequeries.InvoiceReadModel `json:"invoices"`
	}
	if err := c.rpc(SubjectInvoicesList, map[string]string{"customerID": q.CustomerID}, &resp); err != nil {
		return nil, err
	}
	return resp.Invoices, nil
}

// ── invoiceGetter ─────────────────────────────────────────────────────────────

// GetInvoice retrieves a single invoice by ID via NATS.
// Named differently from Handle to avoid method set collision — use InvoiceGetterAdapter.
func (c *PortalNATSClient) GetInvoice(_ context.Context, id string) (*invoicequeries.InvoiceReadModel, error) {
	var resp struct {
		Invoice *invoicequeries.InvoiceReadModel `json:"invoice"`
	}
	if err := c.rpc(SubjectInvoiceGet, map[string]string{"invoiceID": id}, &resp); err != nil {
		return nil, err
	}
	return resp.Invoice, nil
}

// ── pdfTokenMinter ───────────────────────────────────────────────────────────

// MintToken mints a new invoice PDF token via NATS.
// Satisfies portal/adapters/http.pdfTokenMinter.
func (c *PortalNATSClient) MintToken(_ context.Context, invoiceID string) (string, error) {
	var resp struct {
		Plain string `json:"plain"`
	}
	if err := c.rpc(SubjectInvoiceTokenMint, map[string]string{"invoiceID": invoiceID}, &resp); err != nil {
		return "", err
	}
	return resp.Plain, nil
}

// ── customer ─────────────────────────────────────────────────────────────────

// GetCustomer fetches customer info for the portal header via NATS.
func (c *PortalNATSClient) GetCustomer(_ context.Context, id string) (*BridgeCustomer, error) {
	var cust BridgeCustomer
	if err := c.rpc(SubjectCustomerGet, map[string]string{"customerID": id}, &cust); err != nil {
		return nil, err
	}
	return &cust, nil
}

// ValidateInvoiceToken validates a PDF token hash via NATS.
// Returns the InvoiceID the token grants access to.
func (c *PortalNATSClient) ValidateInvoiceToken(_ context.Context, tokenHash string) (string, error) {
	var resp struct {
		InvoiceID string `json:"invoiceID"`
	}
	if err := c.rpc(SubjectInvoiceTokenValidate, map[string]string{"tokenHash": tokenHash}, &resp); err != nil {
		return "", err
	}
	return resp.InvoiceID, nil
}

// ── InvoiceGetterAdapter ──────────────────────────────────────────────────────

// InvoiceGetterAdapter wraps PortalNATSClient so it satisfies the invoiceGetter interface
// (Handle(ctx, id string) (*InvoiceReadModel, error)) used by portal/adapters/http.
type InvoiceGetterAdapter struct {
	C *PortalNATSClient
}

func (a *InvoiceGetterAdapter) Handle(ctx context.Context, id string) (*invoicequeries.InvoiceReadModel, error) {
	return a.C.GetInvoice(ctx, id)
}

// ── helpers ───────────────────────────────────────────────────────────────────

// ErrNotSupported is returned by write operations not available on the portal side.
var ErrNotSupported = errors.New("operation not supported on portal binary")

func (c *PortalNATSClient) rpc(subject string, req any, out any) error {
	b, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("portal rpc: marshal: %w", err)
	}
	msg, err := c.nc.Request(subject, b, c.timeout)
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
