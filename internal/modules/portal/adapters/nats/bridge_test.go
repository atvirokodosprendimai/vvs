package nats_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	invoicedomain "github.com/atvirokodosprendimai/vvs/internal/modules/invoice/domain"
	invoicequeries "github.com/atvirokodosprendimai/vvs/internal/modules/invoice/app/queries"
	natspkg "github.com/atvirokodosprendimai/vvs/internal/infrastructure/nats"
	portalnats "github.com/atvirokodosprendimai/vvs/internal/modules/portal/adapters/nats"
	portaldomain "github.com/atvirokodosprendimai/vvs/internal/modules/portal/domain"
)

// ── stubs ──────────────────────────────────────────────────────────────────────

type stubPortalTokenReader struct {
	tok *portaldomain.PortalToken
}

func (s *stubPortalTokenReader) FindByHash(_ context.Context, _ string) (*portaldomain.PortalToken, error) {
	return s.tok, nil
}

func (s *stubPortalTokenReader) MarkUsed(_ context.Context, _ string) error { return nil }

type stubInvoiceTokenStore struct {
	tokens map[string]*invoicedomain.InvoiceToken
}

func newStubInvoiceTokenStore() *stubInvoiceTokenStore {
	return &stubInvoiceTokenStore{tokens: make(map[string]*invoicedomain.InvoiceToken)}
}
func (s *stubInvoiceTokenStore) Save(_ context.Context, t *invoicedomain.InvoiceToken) error {
	s.tokens[t.TokenHash] = t
	return nil
}
func (s *stubInvoiceTokenStore) FindByHash(_ context.Context, hash string) (*invoicedomain.InvoiceToken, error) {
	t, ok := s.tokens[hash]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return t, nil
}

type stubCustomerReader struct {
	customer *portalnats.BridgeCustomer
}

func (s *stubCustomerReader) GetPortalCustomer(_ context.Context, _ string) (*portalnats.BridgeCustomer, error) {
	if s.customer == nil {
		return nil, fmt.Errorf("not found")
	}
	return s.customer, nil
}

// stubInvoiceGetter returns a fixed invoice by ID.
type stubInvoiceGetter struct {
	invoices map[string]*invoicequeries.InvoiceReadModel
}

func newStubInvoiceGetter() *stubInvoiceGetter {
	return &stubInvoiceGetter{invoices: make(map[string]*invoicequeries.InvoiceReadModel)}
}

func (s *stubInvoiceGetter) Handle(_ context.Context, id string) (*invoicequeries.InvoiceReadModel, error) {
	inv, ok := s.invoices[id]
	if !ok {
		return nil, fmt.Errorf("invoice not found: %s", id)
	}
	return inv, nil
}

// ── helper ─────────────────────────────────────────────────────────────────────

func startBridge(t *testing.T) (*nats.Conn, *nats.Conn, *stubPortalTokenReader, *stubInvoiceTokenStore, *stubCustomerReader) {
	t.Helper()
	ns, serverNC, err := natspkg.StartEmbedded("127.0.0.1:0", "", "") // random port for external connections
	require.NoError(t, err)
	t.Cleanup(func() { ns.Shutdown() })
	t.Cleanup(func() { serverNC.Close() })

	tokenReader := &stubPortalTokenReader{}
	invTokenStore := newStubInvoiceTokenStore()
	custReader := &stubCustomerReader{}

	bridge := portalnats.NewPortalBridge(serverNC, tokenReader, invTokenStore, nil, nil, custReader)
	require.NoError(t, bridge.Register())
	t.Cleanup(bridge.Close)

	// Client connection
	clientNC, err := nats.Connect(fmt.Sprintf("nats://%s", ns.Addr().String()))
	require.NoError(t, err)
	t.Cleanup(func() { clientNC.Close() })

	return clientNC, serverNC, tokenReader, invTokenStore, custReader
}

func startBridgeWithInvoices(t *testing.T) (*nats.Conn, *nats.Conn, *stubInvoiceTokenStore, *stubInvoiceGetter) {
	t.Helper()
	ns, serverNC, err := natspkg.StartEmbedded("127.0.0.1:0", "", "")
	require.NoError(t, err)
	t.Cleanup(func() { ns.Shutdown() })
	t.Cleanup(func() { serverNC.Close() })

	invTokenStore := newStubInvoiceTokenStore()
	invGetter := newStubInvoiceGetter()

	bridge := portalnats.NewPortalBridge(serverNC, &stubPortalTokenReader{}, invTokenStore, nil, invGetter, &stubCustomerReader{})
	require.NoError(t, bridge.Register())
	t.Cleanup(bridge.Close)

	clientNC, err := nats.Connect(fmt.Sprintf("nats://%s", ns.Addr().String()))
	require.NoError(t, err)
	t.Cleanup(func() { clientNC.Close() })

	return clientNC, serverNC, invTokenStore, invGetter
}

func rpcRequest(t *testing.T, nc *nats.Conn, subject string, req any) map[string]any {
	t.Helper()
	b, _ := json.Marshal(req)
	msg, err := nc.Request(subject, b, 2*time.Second)
	require.NoError(t, err)
	var env map[string]any
	require.NoError(t, json.Unmarshal(msg.Data, &env))
	return env
}

// ── tests ──────────────────────────────────────────────────────────────────────

func TestBridge_TokenValidate_ValidToken(t *testing.T) {
	clientNC, _, tokenReader, _, _ := startBridge(t)

	tokenReader.tok = &portaldomain.PortalToken{
		ID:         "tok-1",
		CustomerID: "cust-123",
		TokenHash:  "abc",
		ExpiresAt:  time.Now().Add(time.Hour),
	}

	env := rpcRequest(t, clientNC, portalnats.SubjectTokenValidate, map[string]string{"hash": "abc"})
	assert.Empty(t, env["error"])
	data := env["data"].(map[string]any)
	assert.Equal(t, "cust-123", data["customerID"])
}

func TestBridge_TokenValidate_ExpiredToken(t *testing.T) {
	clientNC, _, tokenReader, _, _ := startBridge(t)

	tokenReader.tok = &portaldomain.PortalToken{
		CustomerID: "cust-123",
		ExpiresAt:  time.Now().Add(-time.Hour), // expired
	}

	env := rpcRequest(t, clientNC, portalnats.SubjectTokenValidate, map[string]string{"hash": "abc"})
	assert.NotEmpty(t, env["error"])
}

func TestBridge_TokenValidate_NotFound(t *testing.T) {
	clientNC, _, _, _, _ := startBridge(t)
	// tokenReader.tok is nil by default

	env := rpcRequest(t, clientNC, portalnats.SubjectTokenValidate, map[string]string{"hash": "missing"})
	assert.NotEmpty(t, env["error"])
}

func TestBridge_InvoiceTokenMint_StoresAndReturnsPlain(t *testing.T) {
	clientNC, _, invTokenStore, invGetter := startBridgeWithInvoices(t)

	invGetter.invoices["inv-42"] = &invoicequeries.InvoiceReadModel{
		ID:         "inv-42",
		CustomerID: "cust-1",
	}

	env := rpcRequest(t, clientNC, portalnats.SubjectInvoiceTokenMint, map[string]string{
		"invoiceID":  "inv-42",
		"customerID": "cust-1",
	})
	assert.Empty(t, env["error"])
	data := env["data"].(map[string]any)
	plain, ok := data["plain"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, plain)

	// Token should be persisted in the store
	assert.Len(t, invTokenStore.tokens, 1)
}

func TestBridge_InvoiceTokenMint_MissingCustomerID_Forbidden(t *testing.T) {
	clientNC, _, _, invGetter := startBridgeWithInvoices(t)
	invGetter.invoices["inv-42"] = &invoicequeries.InvoiceReadModel{ID: "inv-42", CustomerID: "cust-1"}

	env := rpcRequest(t, clientNC, portalnats.SubjectInvoiceTokenMint, map[string]string{"invoiceID": "inv-42"})
	assert.NotEmpty(t, env["error"])
}

func TestBridge_InvoiceTokenMint_WrongCustomerID_Forbidden(t *testing.T) {
	clientNC, _, _, invGetter := startBridgeWithInvoices(t)
	invGetter.invoices["inv-42"] = &invoicequeries.InvoiceReadModel{ID: "inv-42", CustomerID: "cust-1"}

	env := rpcRequest(t, clientNC, portalnats.SubjectInvoiceTokenMint, map[string]string{
		"invoiceID":  "inv-42",
		"customerID": "cust-evil",
	})
	assert.NotEmpty(t, env["error"])
}

func TestBridge_InvoiceTokenValidate_Valid(t *testing.T) {
	clientNC, _, invTokenStore, invGetter := startBridgeWithInvoices(t)

	invGetter.invoices["inv-99"] = &invoicequeries.InvoiceReadModel{ID: "inv-99", CustomerID: "cust-1"}

	// Mint a token via the bridge
	env := rpcRequest(t, clientNC, portalnats.SubjectInvoiceTokenMint, map[string]string{
		"invoiceID":  "inv-99",
		"customerID": "cust-1",
	})
	plain := env["data"].(map[string]any)["plain"].(string)
	_ = invTokenStore

	// Validate it
	raw := sha256.Sum256([]byte(plain))
	sum := hex.EncodeToString(raw[:])
	env2 := rpcRequest(t, clientNC, portalnats.SubjectInvoiceTokenValidate, map[string]string{"tokenHash": sum})
	assert.Empty(t, env2["error"])
	data := env2["data"].(map[string]any)
	assert.Equal(t, "inv-99", data["invoiceID"])
}

func TestBridge_InvoiceGetByToken_Valid(t *testing.T) {
	clientNC, _, invTokenStore, invGetter := startBridgeWithInvoices(t)

	invGetter.invoices["inv-77"] = &invoicequeries.InvoiceReadModel{ID: "inv-77", CustomerID: "cust-1"}

	// Mint a token first
	env := rpcRequest(t, clientNC, portalnats.SubjectInvoiceTokenMint, map[string]string{
		"invoiceID":  "inv-77",
		"customerID": "cust-1",
	})
	plain := env["data"].(map[string]any)["plain"].(string)
	_ = invTokenStore

	// Now use SubjectInvoiceGetByToken — validates token + returns invoice atomically
	raw := sha256.Sum256([]byte(plain))
	tokenHash := hex.EncodeToString(raw[:])
	env2 := rpcRequest(t, clientNC, portalnats.SubjectInvoiceGetByToken, map[string]string{"tokenHash": tokenHash})
	assert.Empty(t, env2["error"])
	data := env2["data"].(map[string]any)
	invoice := data["invoice"].(map[string]any)
	assert.Equal(t, "inv-77", invoice["ID"])
}

func TestBridge_InvoiceGetByToken_Expired(t *testing.T) {
	clientNC, _, invTokenStore, _ := startBridgeWithInvoices(t)

	// Insert an expired token directly
	expiredTok := &invoicedomain.InvoiceToken{
		ID:        "tok-exp",
		InvoiceID: "inv-x",
		TokenHash: "expiredhash",
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	_ = invTokenStore.Save(context.Background(), expiredTok)

	env := rpcRequest(t, clientNC, portalnats.SubjectInvoiceGetByToken, map[string]string{"tokenHash": "expiredhash"})
	assert.NotEmpty(t, env["error"])
}

func TestBridge_CustomerGet_Found(t *testing.T) {
	clientNC, _, _, _, custReader := startBridge(t)

	custReader.customer = &portalnats.BridgeCustomer{
		ID:          "cust-1",
		CompanyName: "UAB Test",
		Email:       "test@example.com",
	}

	env := rpcRequest(t, clientNC, portalnats.SubjectCustomerGet, map[string]string{"customerID": "cust-1"})
	assert.Empty(t, env["error"])
	data := env["data"].(map[string]any)
	assert.Equal(t, "UAB Test", data["CompanyName"])
}

func TestBridge_CustomerGet_NotFound(t *testing.T) {
	clientNC, _, _, _, _ := startBridge(t)
	// custReader.customer is nil

	env := rpcRequest(t, clientNC, portalnats.SubjectCustomerGet, map[string]string{"customerID": "missing"})
	assert.NotEmpty(t, env["error"])
}

// listInvoices handler requires DB-backed handler — nil returns error, not panic.
func TestBridge_InvoicesList_NilHandler_ReturnsError(t *testing.T) {
	ns, serverNC, err := natspkg.StartEmbedded("127.0.0.1:0", "", "")
	require.NoError(t, err)
	t.Cleanup(func() { ns.Shutdown() })
	defer serverNC.Close()

	bridge := portalnats.NewPortalBridge(serverNC, &stubPortalTokenReader{}, newStubInvoiceTokenStore(), nil, nil, &stubCustomerReader{})
	require.NoError(t, bridge.Register())
	defer bridge.Close()

	clientNC, err := nats.Connect(fmt.Sprintf("nats://%s", ns.Addr().String()))
	require.NoError(t, err)
	defer clientNC.Close()

	env := rpcRequest(t, clientNC, portalnats.SubjectInvoicesList, map[string]string{"customerID": "x"})
	assert.NotEmpty(t, env["error"])
}

func TestBridge_InvoiceGet_EmptyCustomerID_Forbidden(t *testing.T) {
	// customerID is now mandatory — empty value must be rejected before any DB access.
	ns, serverNC, err := natspkg.StartEmbedded("127.0.0.1:0", "", "")
	require.NoError(t, err)
	t.Cleanup(func() { ns.Shutdown() })
	defer serverNC.Close()

	invGetter := newStubInvoiceGetter()
	invGetter.invoices["inv-1"] = &invoicequeries.InvoiceReadModel{ID: "inv-1", CustomerID: "cust-1"}

	bridge := portalnats.NewPortalBridge(serverNC, &stubPortalTokenReader{}, newStubInvoiceTokenStore(), nil, invGetter, &stubCustomerReader{})
	require.NoError(t, bridge.Register())
	defer bridge.Close()

	clientNC, err := nats.Connect(fmt.Sprintf("nats://%s", ns.Addr().String()))
	require.NoError(t, err)
	defer clientNC.Close()

	// Empty customerID → forbidden
	env := rpcRequest(t, clientNC, portalnats.SubjectInvoiceGet, map[string]any{
		"invoiceID":  "inv-1",
		"customerID": "",
	})
	assert.NotEmpty(t, env["error"])
}

func TestBridge_InvoiceGet_WrongCustomerID_Forbidden(t *testing.T) {
	ns, serverNC, err := natspkg.StartEmbedded("127.0.0.1:0", "", "")
	require.NoError(t, err)
	t.Cleanup(func() { ns.Shutdown() })
	defer serverNC.Close()

	invGetter := newStubInvoiceGetter()
	invGetter.invoices["inv-1"] = &invoicequeries.InvoiceReadModel{ID: "inv-1", CustomerID: "cust-owner"}

	bridge := portalnats.NewPortalBridge(serverNC, &stubPortalTokenReader{}, newStubInvoiceTokenStore(), nil, invGetter, &stubCustomerReader{})
	require.NoError(t, bridge.Register())
	defer bridge.Close()

	clientNC, err := nats.Connect(fmt.Sprintf("nats://%s", ns.Addr().String()))
	require.NoError(t, err)
	defer clientNC.Close()

	// Wrong customerID → forbidden
	env := rpcRequest(t, clientNC, portalnats.SubjectInvoiceGet, map[string]any{
		"invoiceID":  "inv-1",
		"customerID": "cust-attacker",
	})
	assert.NotEmpty(t, env["error"])
}
