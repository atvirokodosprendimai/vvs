// Package nats provides a NATS RPC bridge for the customer portal.
// The PortalBridge runs on vvs-core and serves portal data requests from vvs-portal.
// All subjects use the isp.portal.rpc.* namespace.
package nats

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/nats-io/nats.go"
	invoicedomain "github.com/vvs/isp/internal/modules/invoice/domain"
	invoicequeries "github.com/vvs/isp/internal/modules/invoice/app/queries"
	portaldomain "github.com/vvs/isp/internal/modules/portal/domain"
)

// Subjects served by PortalBridge.
const (
	SubjectTokenValidate        = "isp.portal.rpc.token.validate"
	SubjectInvoicesList         = "isp.portal.rpc.invoices.list"
	SubjectInvoiceGet           = "isp.portal.rpc.invoice.get"
	SubjectInvoiceTokenValidate = "isp.portal.rpc.invoice.token.validate"
	SubjectInvoiceTokenMint     = "isp.portal.rpc.invoice.token.mint"
	SubjectCustomerGet          = "isp.portal.rpc.customer.get"
)

// portalTokenReader reads portal tokens from the DB.
type portalTokenReader interface {
	FindByHash(ctx context.Context, hash string) (*portaldomain.PortalToken, error)
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

// BridgeCustomer is the minimal customer data exposed over NATS.
type BridgeCustomer struct {
	ID          string
	CompanyName string
	Email       string
}

// PortalBridge subscribes to isp.portal.rpc.* subjects and serves portal data.
// Runs on vvs-core — has direct access to SQLite via the injected handlers/repos.
type PortalBridge struct {
	nc           *nats.Conn
	tokenRepo    portalTokenReader
	invoiceToken invoiceTokenStore
	listInvoices *invoicequeries.ListInvoicesForCustomerHandler
	getInvoice   *invoicequeries.GetInvoiceHandler
	custReader   bridgeCustomerReader
	subs         []*nats.Subscription
}

// NewPortalBridge creates a bridge. Call Register() to start serving.
func NewPortalBridge(
	nc *nats.Conn,
	tokenRepo portalTokenReader,
	invoiceToken invoiceTokenStore,
	listInvoices *invoicequeries.ListInvoicesForCustomerHandler,
	getInvoice *invoicequeries.GetInvoiceHandler,
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

// Register subscribes to all portal RPC subjects.
func (b *PortalBridge) Register() error {
	type entry struct {
		subject string
		handler nats.MsgHandler
	}
	entries := []entry{
		{SubjectTokenValidate, b.handleTokenValidate},
		{SubjectInvoicesList, b.handleInvoicesList},
		{SubjectInvoiceGet, b.handleInvoiceGet},
		{SubjectInvoiceTokenValidate, b.handleInvoiceTokenValidate},
		{SubjectInvoiceTokenMint, b.handleInvoiceTokenMint},
		{SubjectCustomerGet, b.handleCustomerGet},
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
		CustomerID string    `json:"customerID"`
		ExpiresAt  time.Time `json:"expiresAt"`
	}{tok.CustomerID, tok.ExpiresAt}, nil)
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	inv, err := b.getInvoice.Handle(ctx, req.InvoiceID)
	if err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	// Ownership check: portal can only fetch its customer's invoices.
	if req.CustomerID != "" && inv.CustomerID != req.CustomerID {
		bridgeReply(msg, nil, errForbidden)
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

func (b *PortalBridge) handleInvoiceTokenMint(msg *nats.Msg) {
	var req struct {
		InvoiceID string `json:"invoiceID"`
	}
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		bridgeReply(msg, nil, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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
