---
tldr: Split monolith into core binary (office/intranet) and portal binary (public VPS) — customers never reach admin
status: active
---

# Plan: Core / Portal Deployment Split

## Context

- Spec: [[spec - architecture - system design and key decisions]]
- Spec: [[spec - portal - customer self-service access]]
- Current state: single binary serves everything — admin dashboard, CRUD ops, customer portal — all on one process/port
- Problem: if the binary is internet-accessible (so customers can use the portal), admin routes are also reachable
- Goal: **customers can never reach the admin dashboard** — hard network boundary, not just auth

### Architecture of the split

```
[ Office / NATed LAN ]                    [ Public VPS ]
┌─────────────────────────┐               ┌───────────────────────────────┐
│  vvs-core (cmd/server)  │◄──WireGuard──►│  vvs-portal (cmd/portal)      │
│  - admin HTTP :8080      │    NATS RPC   │  - portal HTTP :8081           │
│  - SQLite (all data)     │               │  - NO DB                       │
│  - embedded NATS :4222   │               │  - NATS client only            │
│  - billing, CRM, etc.    │               │  - /portal/* + /i/{token}      │
│  - NOT internet-facing   │               │  - Nginx TLS termination        │
└─────────────────────────┘               └───────────────────────────────┘
         ▲
         │ systemctl / SSH
     Office admins
```

### Key technical decisions

1. **No DB on portal VPS** — all reads go via NATS RPC (request/reply) to core
2. **NATS bridge** — 6 new `isp.portal.rpc.*` subjects served by core; portal client calls them
3. **WireGuard VPN** (primary) or NATS auth token (secondary) for transport security
4. **CDN-only static assets** — no file serving changes needed (Tailwind Browser + Datastar are CDN)
5. **`/i/{token}` moves to portal binary** — PDF tokens validated+invoice rendered on portal VPS
6. **Admin generate-portal-link stays on core** — core saves the portal token, URLs point to portal VPS via `VVS_BASE_URL`

### What stays on core only
- All admin CRUD (customers, invoices, products, services, billing, NATS events)
- Auth (login/logout/TOTP for admin users)
- Email/IMAP sync
- Billing cron jobs
- Dunning (sends portal links but URL = portal VPS base URL)
- InvoiceToken + PortalToken storage

### What moves to portal binary
- `GET /portal/auth` — validate portal token (calls NATS: `token.validate`)
- `POST /portal/logout`
- `GET /portal/invoices` — list (calls NATS: `invoices.list`)
- `GET /portal/invoices/{id}` — detail + PDF link (calls NATS: `invoice.get` + `invoice.token.mint`)
- `GET /i/{token}` — public PDF page (calls NATS: `invoice.token.validate` + `invoice.get`)
- Admin portal-link generation (`POST /api/customers/{id}/portal-link`) **stays on core only**

---

## Phase 1 — Spec Updates — status: open

Update the two specs to document the split before writing any code.

1. [ ] `/eidos:spec` — update `spec - architecture - system design and key decisions.md`
   - Add "Deployment modes" section describing single-binary vs split
   - Document `vvs-core` vs `vvs-portal` binary responsibilities
   - Document NATS RPC as the inter-process communication mechanism
   - Update Mapping section with new `cmd/portal/` entry

2. [ ] `/eidos:spec` — update `spec - portal - customer self-service access.md`
   - Add "Standalone deployment" section
   - Document the 6 NATS RPC subjects portal calls
   - Describe `/i/{token}` ownership in portal binary
   - Note that `generatePortalLink` admin action stays on core exclusively

---

## Phase 2 — NATS Portal Bridge (core side) — status: open

New adapter: `internal/modules/portal/adapters/nats/bridge.go`

This file runs on **core** — subscribes to `isp.portal.rpc.*` subjects and serves data from SQLite.

### 6 RPC subjects

| Subject | Request | Reply |
|---------|---------|-------|
| `isp.portal.rpc.token.validate` | `{hash: string}` | `{customerID, expiresAt}` |
| `isp.portal.rpc.invoices.list` | `{customerID: string}` | `{invoices: []InvoiceReadModel}` |
| `isp.portal.rpc.invoice.get` | `{invoiceID, customerID: string}` | `{invoice: InvoiceReadModel}` (ownership check included) |
| `isp.portal.rpc.invoice.token.validate` | `{tokenHash: string}` | `{invoiceID: string}` |
| `isp.portal.rpc.invoice.token.mint` | `{invoiceID: string}` | `{plain: string}` (48h TTL) |
| `isp.portal.rpc.customer.get` | `{customerID: string}` | `{id, name, email: string}` |

### Bridge struct

```go
package nats

type PortalBridge struct {
    nc           *nats.Conn
    tokenRepo    portalTokenReader       // portal/domain.PortalTokenRepository
    invoiceToken invoiceTokenStore       // invoice/domain.InvoiceTokenRepository
    listInvoices *invoicequeries.ListInvoicesForCustomerHandler
    getInvoice   *invoicequeries.GetInvoiceHandler
    custReader   portalCustomerReader    // returns id/name/email
    subs         []*nats.Subscription
}

type portalTokenReader interface {
    FindByHash(ctx context.Context, hash string) (*portaldomain.PortalToken, error)
}

type invoiceTokenStore interface {
    Save(ctx context.Context, t *invoicedomain.InvoiceToken) error
    FindByHash(ctx context.Context, hash string) (*invoicedomain.InvoiceToken, error)
}

type portalCustomerReader interface {
    GetPortalCustomer(ctx context.Context, id string) (*portalhttptype, error)
}
```

### Pattern (same as existing rpc/server.go)

```go
func (b *PortalBridge) Register() error {
    subjects := map[string]nats.MsgHandler{
        "isp.portal.rpc.token.validate":         b.handleTokenValidate,
        "isp.portal.rpc.invoices.list":           b.handleInvoicesList,
        "isp.portal.rpc.invoice.get":             b.handleInvoiceGet,
        "isp.portal.rpc.invoice.token.validate":  b.handleInvoiceTokenValidate,
        "isp.portal.rpc.invoice.token.mint":      b.handleInvoiceTokenMint,
        "isp.portal.rpc.customer.get":            b.handleCustomerGet,
    }
    for subject, handler := range subjects {
        sub, err := b.nc.Subscribe(subject, handler)
        // ...
    }
}

// helpers
func reply(msg *nats.Msg, data any, err error) { /* same pattern as rpc/server.go */ }
```

### Actions

3. [ ] Write `internal/modules/portal/adapters/nats/bridge.go`
   - Use exact same `reply(msg, data, err)` helper pattern as `rpc/server.go:358`
   - `handleInvoiceTokenMint`: calls `invoicedomain.NewInvoiceToken(invoiceID, 48h)` then saves
   - `handleInvoiceGet`: performs ownership check `inv.CustomerID != req.CustomerID` before returning

4. [ ] Write `internal/modules/portal/adapters/nats/bridge_test.go`
   - Test each handler with a stub that returns canned data
   - Verify error reply on unknown customerID
   - Verify ownership enforcement in `invoice.get`

5. [ ] Wire bridge in `internal/app/app.go`
   - Only when NATS conn is exposed (`natsListenAddr != ""`) — bridge is only useful when portal binary will connect
   - OR: always wire it (zero cost when no subscriber), simpler — prefer this
   - `portalBridge := natsbridge.NewPortalBridge(nc, portalTokenRepo, invoiceTokenRepo, listInvoicesCmd, getInvoiceCmd, custReader)`
   - `portalBridge.Register()`

---

## Phase 3 — Interface Extraction in Portal HTTP Handler — status: open

Currently `internal/modules/portal/adapters/http/handlers.go` holds concrete query types:

```go
listInvoices  *invoicequeries.ListInvoicesForCustomerHandler   // ← concrete
getInvoice    *invoicequeries.GetInvoiceHandler                 // ← concrete
```

These need to become interfaces so the NATS client can satisfy them.

6. [ ] Extract interfaces in `internal/modules/portal/adapters/http/handlers.go`

```go
// invoiceLister lists invoices for a customer.
type invoiceLister interface {
    Handle(ctx context.Context, q invoicequeries.ListInvoicesForCustomerQuery) ([]invoicequeries.InvoiceReadModel, error)
}

// invoiceGetter gets a single invoice by ID.
type invoiceGetter interface {
    Handle(ctx context.Context, id string) (*invoicequeries.InvoiceReadModel, error)
}
```

Change struct fields to interfaces:
```go
listInvoices  invoiceLister
getInvoice    invoiceGetter
```

Change `NewHandlers` signature to accept these interfaces.

No behaviour change — existing code works identically since concrete handlers satisfy the new interfaces. Run `go build ./...` to verify.

---

## Phase 4 — NATS Portal Client (portal side) — status: open

New file: `internal/modules/portal/adapters/nats/client.go`

`PortalNATSClient` implements all interfaces the portal HTTP handler needs, backed by NATS RPC calls to core.

```go
type PortalNATSClient struct {
    nc      *nats.Conn
    timeout time.Duration  // default 5s
}

func NewPortalNATSClient(nc *nats.Conn) *PortalNATSClient

// Implements domain.PortalTokenRepository.FindByHash
func (c *PortalNATSClient) FindByHash(ctx, hash) (*domain.PortalToken, error)
    // → request to "isp.portal.rpc.token.validate"
    // → reconstruct PortalToken{CustomerID, ExpiresAt} from reply

// Implements domain.PortalTokenRepository.Save  (never called by portal binary — returns ErrNotSupported)
func (c *PortalNATSClient) Save(ctx, token) error

// Implements invoiceTokenSaver.Save
func (c *PortalNATSClient) SaveInvoiceToken(ctx, invoiceID) (plain string, err error)
    // → request to "isp.portal.rpc.invoice.token.mint"

// Implements invoiceLister
func (c *PortalNATSClient) Handle(ctx, q ListInvoicesForCustomerQuery) ([]InvoiceReadModel, error)
    // → request to "isp.portal.rpc.invoices.list"

// Implements invoiceGetter
func (c *PortalNATSClient) Handle(ctx, id) (*InvoiceReadModel, error)
    // → request to "isp.portal.rpc.invoice.get" (passes empty customerID, ownership checked by bridge for admin access; portal passes customerID)

// GetPortalCustomer implements customerReader
func (c *PortalNATSClient) GetPortalCustomer(ctx, id) (*portalhttptype.PortalCustomer, error)
    // → request to "isp.portal.rpc.customer.get"

// ValidateInvoiceToken validates /i/{token} tokens
func (c *PortalNATSClient) ValidateInvoiceToken(ctx, tokenHash string) (invoiceID string, err error)
    // → request to "isp.portal.rpc.invoice.token.validate"
```

Note on `invoiceTokenSaver`: current interface is `Save(ctx, *invoicedomain.InvoiceToken) error`. But portal doesn't build an InvoiceToken object — it calls mint and gets a plain token back. Two options:
- a. Change interface to accept `invoiceID string, ttl time.Duration` — cleaner for portal use
- b. Keep existing interface; portal calls NATS to mint and gets plain token, then constructs InvoiceToken for the Save call... awkward
- **Use option a**: Add a new narrow interface in portal http package

```go
// pdfTokenMinter mints a public PDF access token for an invoice, returns the plain token string.
type pdfTokenMinter interface {
    MintToken(ctx context.Context, invoiceID string) (plain string, err error)
}
```

Rename `pdfTokens invoiceTokenSaver` → `pdfTokens pdfTokenMinter` in Handlers struct. Update `invoiceDetail` to call `h.pdfTokens.MintToken(ctx, inv.ID)`.

The core-side adapter wraps `invoicedomain.NewInvoiceToken + pdfTokens.Save` into one `MintToken` call.
The NATS-side adapter calls `isp.portal.rpc.invoice.token.mint` in `MintToken`.

7. [ ] Write `internal/modules/portal/adapters/nats/client.go`
   - Use `nc.RequestWithContext(ctx, subject, jsonBytes)` for all calls
   - Deserialize from `envelope{Data, Error}` — same pattern as `rpc/server.go`
   - Default timeout 5s (configurable via `WithTimeout`)

8. [ ] Write `internal/modules/portal/adapters/nats/client_test.go`
   - Use real embedded NATS (`natsinfra.StartEmbedded("")`)
   - Spin up a stub handler that replies with canned JSON
   - Verify serialization round-trips, error propagation, timeout behaviour

9. [ ] Update `internal/modules/portal/adapters/http/handlers.go`
   - Change `pdfTokens invoiceTokenSaver` → `pdfTokens pdfTokenMinter` with `MintToken(ctx, invoiceID) (string, error)`
   - Introduce `invoiceDetail` calling `MintToken` instead of building InvoiceToken manually

10. [ ] Add core-side `MintToken` wrapper in `internal/app/app.go` or a small adapter struct
    - Wraps `invoicedomain.NewInvoiceToken` + `invoiceTokenRepo.Save` into a `MintToken` method
    - Satisfies `pdfTokenMinter` for the existing monolith wiring

---

## Phase 5 — cmd/portal Binary — status: open

New entry point: `cmd/portal/main.go`

This binary has **zero** imports of:
- `internal/infrastructure/http/router.go` (admin router)
- `internal/app/app.go` (monolith composer)
- Any module's admin handlers

It only imports:
- `internal/modules/portal/adapters/http` (portal HTTP handlers)
- `internal/modules/portal/adapters/nats` (portal NATS client)
- `internal/modules/invoice/adapters/http` (for `InvoicePrintPage` template — read-only templ component)
- `internal/modules/invoice/app/queries` (for `InvoiceReadModel` type)
- `github.com/go-chi/chi/v5`
- `github.com/nats-io/nats.go`

### CLI flags

```
--addr             string    HTTP listen address (default ":8081", env: PORTAL_ADDR)
--nats-url         string    NATS server URL, e.g. nats://10.8.0.1:4222 (required, env: NATS_URL)
--base-url         string    Public base URL for /i/{token} links (env: VVS_BASE_URL)
--secure-cookie    bool      Set Secure flag on portal cookies (env: PORTAL_SECURE_COOKIE)
--nats-token       string    NATS auth token (optional, env: NATS_AUTH_TOKEN)
--nats-creds       string    Path to NATS credentials file (optional, env: NATS_CREDS_FILE)
```

### Startup sequence

```go
func main() {
    // 1. Parse flags / env
    // 2. Connect to NATS
    nc, err := nats.Connect(natsURL,
        nats.Token(natsToken),           // optional
        nats.UserCredentials(natsCreds), // optional
        nats.MaxReconnects(-1),
        nats.ReconnectWait(3 * time.Second),
    )
    // 3. Build NATS client
    client := natsclient.NewPortalNATSClient(nc)
    // 4. Build portal HTTP handler
    h := portalhttp.NewHandlers(client, client, client).
        WithPDFTokens(client).
        WithCustomerReader(client).
        WithBaseURL(baseURL).
        WithSecureCookie(secureCookie)
    // 5. Build router — ONLY public portal routes + /i/{token}
    r := chi.NewRouter()
    r.Use(middleware.RealIP, middleware.Recoverer)
    h.RegisterPublicRoutes(r)
    r.Get("/i/{token}", publicInvoiceByToken(client))  // new handler in cmd/portal
    // 6. Serve HTTP
    srv := &http.Server{Addr: addr, Handler: r}
    // graceful shutdown on SIGINT/SIGTERM
}
```

### `/i/{token}` handler (inline in cmd/portal or small portal package)

```go
func publicInvoiceByToken(client *natsclient.PortalNATSClient) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        plain := chi.URLParam(r, "token")
        hash := sha256hex(plain)

        invoiceID, err := client.ValidateInvoiceToken(r.Context(), hash)
        if err != nil {
            http.Error(w, "Link expired or not found", http.StatusNotFound)
            return
        }

        inv, err := client.Handle(r.Context(), invoiceID) // invoiceGetter — no ownership check needed (token established it)
        if err != nil {
            http.Error(w, "Invoice not found", http.StatusNotFound)
            return
        }

        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        w.Header().Set("Cache-Control", "no-store")
        invoicehttp.InvoicePrintPage(*inv).Render(r.Context(), w)
    }
}
```

### Build targets

Add to `Makefile` (or document in README):
```makefile
build-core:
    go build -o bin/vvs-core ./cmd/server

build-portal:
    go build -o bin/vvs-portal ./cmd/portal

build-all: build-core build-portal
```

11. [ ] Create `cmd/portal/main.go`
    - No DB, no migrations, no WriteSerializer, no embedded NATS server
    - NATS client reconnect loop with `-1` (infinite) reconnects

12. [ ] Create `cmd/portal/invoice_handler.go`
    - `publicInvoiceByToken(client)` handler as described above
    - Imports `internal/modules/invoice/adapters/http.InvoicePrintPage` — only the template

13. [ ] Verify `go build ./cmd/portal` compiles
14. [ ] Verify `go build ./cmd/server` still compiles (no regressions)

---

## Phase 6 — NATS Security — status: open

When NATS is exposed on a network (via `--nats-listen`), it needs protection.

### Option A — WireGuard VPN (recommended)

WireGuard creates an encrypted tunnel between office server and VPS. NATS binds to WireGuard interface only:
```
# Core startup (office server with WireGuard IP 10.8.0.1)
VVS_NATS_LISTEN_ADDR=10.8.0.1:4222

# Portal startup (VPS with WireGuard peer 10.8.0.2)
NATS_URL=nats://10.8.0.1:4222
```
NATS is never reachable from the public internet — only from WireGuard peers.

### Option B — NATS Auth Token

Quick protection without VPN. Less secure (NATS reachable on public internet, protected only by token):
```
# Core startup
VVS_NATS_AUTH_TOKEN=very-long-random-secret

# Portal startup
NATS_AUTH_TOKEN=very-long-random-secret  → --nats-token flag
```

### Implementation

15. [ ] Add `--nats-auth-token` flag to `cmd/server/main.go` (env: `VVS_NATS_AUTH_TOKEN`)
    - Pass to `StartEmbedded`: add `authToken string` param
    - In `embedded.go`: `if authToken != "" { opts.Authorization = authToken }`

16. [ ] Add TLS support to NATS in `embedded.go` (optional, for Option C)
    - `--nats-tls-cert`, `--nats-tls-key` flags
    - `opts.TLSConfig = &tls.Config{...}` when cert+key provided

17. [ ] Write `deploy/wireguard-setup.md`
    - Step-by-step WireGuard config for office server + VPS
    - wg0.conf examples for both peers
    - systemd wg-quick@wg0 service
    - Firewall rules: allow port 4222 only from WireGuard IP range

---

## Phase 7 — Deployment Artifacts — status: open

18. [ ] `deploy/core.env.example`
    ```env
    VVS_DB_PATH=/var/lib/vvs/vvs.db
    VVS_ADDR=127.0.0.1:8080          # only localhost — Nginx proxies
    VVS_ADMIN_USER=admin
    VVS_ADMIN_PASSWORD=changeme
    VVS_NATS_LISTEN_ADDR=10.8.0.1:4222  # WireGuard interface
    VVS_NATS_AUTH_TOKEN=              # optional backup auth
    VVS_BASE_URL=https://portal.example.com  # portal VPS URL for generated links
    VVS_EMAIL_ENC_KEY=
    VVS_ROUTER_ENC_KEY=
    ```

19. [ ] `deploy/portal.env.example`
    ```env
    PORTAL_ADDR=127.0.0.1:8081       # only localhost — Nginx proxies
    NATS_URL=nats://10.8.0.1:4222    # core's NATS over WireGuard
    NATS_AUTH_TOKEN=                  # if using token auth
    VVS_BASE_URL=https://portal.example.com  # own public URL for link construction
    PORTAL_SECURE_COOKIE=true
    ```

20. [ ] `deploy/vvs-core.service` (systemd)
    ```ini
    [Unit]
    Description=VVS Core Business Management
    After=network.target

    [Service]
    Type=simple
    User=vvs
    WorkingDirectory=/opt/vvs
    EnvironmentFile=/etc/vvs/core.env
    ExecStart=/opt/vvs/bin/vvs-core serve
    Restart=always
    RestartSec=5

    [Install]
    WantedBy=multi-user.target
    ```

21. [ ] `deploy/vvs-portal.service` (systemd)
    - Same pattern, `ExecStart=/opt/vvs/bin/vvs-portal`

22. [ ] `deploy/nginx-portal.conf`
    - TLS termination, reverse proxy to `:8081`
    - Strong TLS: TLS 1.2+ only, HSTS, OCSP stapling
    - Rate limiting: `limit_req_zone` on `/portal/auth` (brute-force protection on token endpoint)
    ```nginx
    limit_req_zone $binary_remote_addr zone=portal_auth:10m rate=5r/m;

    server {
        listen 443 ssl http2;
        server_name portal.example.com;
        # ... TLS config

        location /portal/auth {
            limit_req zone=portal_auth burst=3 nodelay;
            proxy_pass http://127.0.0.1:8081;
        }

        location / {
            proxy_pass http://127.0.0.1:8081;
            proxy_set_header X-Forwarded-Proto https;
        }
    }
    ```

---

## Phase 8 — Admin Isolation Verification — status: open

Confirm the portal binary truly has zero admin routes.

23. [ ] Smoke test `cmd/portal_isolation_test.go`
    ```go
    func TestPortalBinary_AdminRoutesReturn404(t *testing.T) {
        // Start portal binary with embedded NATS stub
        // Hit: GET /login, GET /customers, GET /invoices, GET /settings/permissions
        // All should return 404 (route not registered), NOT 302 to /login
        // 302 to /login would mean admin RequireAuth middleware is present
    }
    ```

24. [ ] Integration smoke test: `go test ./cmd/portal/... -v`
    - Connect to real embedded NATS
    - Portal binary serves requests
    - Verify `/portal/auth?token=invalid` → 200 expired page (not 500)
    - Verify `/customers` → 404 (not 302)

---

## Verification

```bash
# Build both binaries
go build ./cmd/server && go build ./cmd/portal

# Phase 2 bridge tests
go test ./internal/modules/portal/adapters/nats/... -v

# Phase 4 NATS client tests  
go test ./internal/modules/portal/adapters/nats/... -v

# Phase 5 portal binary smoke tests
go test ./cmd/portal/... -v

# Manual integration test
# Terminal 1: start core
./bin/vvs-core serve --nats-listen 127.0.0.1:4222 --admin-user admin --admin-password test

# Terminal 2: start portal
./bin/vvs-portal --nats-url nats://127.0.0.1:4222 --addr :8081

# Terminal 3: verify isolation
curl -I http://localhost:8081/customers   # must be 404
curl -I http://localhost:8081/login       # must be 404
curl -I http://localhost:8081/portal/auth # must be 200

# Verify admin still works
curl -I http://localhost:8080/login       # must be 200 (admin login page)

# Generate a portal link on core, use it on portal VPS
# Admin: POST /api/customers/{id}/portal-link → get URL
# Open URL on port 8081 — portal should work
```

## Adjustments

<!-- Plans evolve. Document changes with timestamps. -->

## Progress Log

- 2026-04-19: Plan created — architecture designed, 6 NATS RPC subjects defined, 8 phases, 24 actions
