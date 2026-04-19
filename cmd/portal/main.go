// vvs-portal — public-facing customer portal binary.
//
// Runs on an internet-facing VPS. Has zero direct DB access.
// All data is fetched from vvs-core via NATS RPC over a WireGuard VPN tunnel
// (or an authenticated NATS connection).
//
// Routes served:
//   GET/POST /portal/auth         — magic-link token → session cookie
//   POST     /portal/logout       — clear session cookie
//   GET      /portal              → redirect /portal/invoices
//   GET      /portal/invoices     — customer invoice list
//   GET      /portal/invoices/{id} — invoice detail + PDF link
//   GET      /i/{token}           — public printable invoice PDF
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	"github.com/nats-io/nats.go"
	"github.com/urfave/cli/v3"
	invoicehttp "github.com/atvirokodosprendimai/vvs/internal/modules/invoice/adapters/http"
	invoicequeries "github.com/atvirokodosprendimai/vvs/internal/modules/invoice/app/queries"
	portalhttp "github.com/atvirokodosprendimai/vvs/internal/modules/portal/adapters/http"
	portalnats "github.com/atvirokodosprendimai/vvs/internal/modules/portal/adapters/nats"
)

func main() {
	if err := godotenv.Load(); err == nil {
		log.Println("Loaded config from .env")
	}

	cmd := &cli.Command{
		Name:  "vvs-portal",
		Usage: "VVS Customer Portal (public-facing, no DB)",
		Commands: []*cli.Command{
			serveCommand(),
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

func serveCommand() *cli.Command {
	return &cli.Command{
		Name:  "serve",
		Usage: "Start the customer portal HTTP server",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "addr",
				Usage:   "HTTP listen address",
				Value:   ":8081",
				Sources: cli.EnvVars("PORTAL_ADDR"),
			},
			&cli.StringFlag{
				Name:     "nats-url",
				Usage:    "NATS URL for vvs-core bridge (e.g. nats://10.0.0.1:4222)",
				Sources:  cli.EnvVars("NATS_URL"),
				Required: true,
			},
			&cli.StringFlag{
				Name:    "nats-portal-password",
				Usage:   "Password for the 'portal' NATS user (required when vvs-core uses per-user auth)",
				Sources: cli.EnvVars("NATS_PORTAL_PASSWORD"),
			},
			&cli.StringFlag{
				Name:    "nats-auth-token",
				Usage:   "Deprecated: use --nats-portal-password instead",
				Sources: cli.EnvVars("NATS_AUTH_TOKEN"),
				Hidden:  true,
			},
			&cli.StringFlag{
				Name:    "base-url",
				Usage:   "Public base URL, e.g. https://portal.example.com (no trailing slash)",
				Sources: cli.EnvVars("VVS_BASE_URL"),
			},
			&cli.BoolFlag{
				Name:    "insecure-cookie",
				Usage:   "Disable Secure flag on session cookies (only for local dev without HTTPS)",
				Sources: cli.EnvVars("PORTAL_INSECURE_COOKIE"),
			},
		},
		Action: runPortal,
	}
}

func runPortal(ctx context.Context, cmd *cli.Command) error {
	addr := cmd.String("addr")
	natsURL := cmd.String("nats-url")
	baseURL := cmd.String("base-url")
	secureCookie := !cmd.Bool("insecure-cookie") // secure by default; opt out for local dev

	// Connect to NATS (vvs-core side)
	opts := []nats.Option{nats.Name("vvs-portal")}
	// Per-user auth takes precedence; fall back to legacy token for backward compat.
	if portalPwd := cmd.String("nats-portal-password"); portalPwd != "" {
		opts = append(opts, nats.UserInfo("portal", portalPwd))
	} else if legacyToken := cmd.String("nats-auth-token"); legacyToken != "" {
		opts = append(opts, nats.Token(legacyToken))
	}
	nc, err := nats.Connect(natsURL, opts...)
	if err != nil {
		return err
	}
	defer nc.Close()
	slog.Info("connected to NATS", "url", natsURL)

	client := portalnats.NewPortalNATSClient(nc, 5*time.Second)

	// portalCustomerReaderAdapter converts BridgeCustomer → PortalCustomer for the HTTP handler.
	custAdapter := &portalCustomerReaderAdapter{client: client}

	// Portal HTTP handler — backed by NATS, no DB.
	portalHandlers := portalhttp.NewHandlers(
		client, // domain.PortalTokenRepository
		client, // invoiceLister (Handle with ListInvoicesForCustomerQuery)
		&portalnats.InvoiceGetterAdapter{ // invoiceGetter — extracts customerID from portal session
			C:                 client,
			CustomerIDFromCtx: portalhttp.PortalCustomerIDFromContext,
		},
	).
		WithPDFTokens(client).
		WithCustomerReader(custAdapter).
		WithTickets(client).
		WithServices(&portalServiceClientAdapter{client: client}).
		WithBot(client).
		WithLoginClient(client).
		WithBaseURL(baseURL).
		WithSecureCookie(secureCookie)

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Public invoice PDF endpoint — validates token via NATS, renders HTML.
	r.Get("/i/{token}", publicInvoiceByToken(client))

	// Portal routes (auth + protected).
	portalHandlers.RegisterPublicRoutes(r)

	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("portal listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("portal server: %v", err)
		}
	}()

	<-stop
	slog.Info("portal shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// publicInvoiceByToken handles GET /i/{token} — validates the invoice token and fetches
// the invoice in a single NATS call, then renders the printable invoice HTML.
func publicInvoiceByToken(client *portalnats.PortalNATSClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if client == nil {
			http.Error(w, "not configured", http.StatusServiceUnavailable)
			return
		}
		plain := chi.URLParam(r, "token")
		sum := sha256.Sum256([]byte(plain))
		hashHex := hex.EncodeToString(sum[:])

		inv, err := client.GetInvoiceByTokenHash(r.Context(), hashHex)
		if err != nil || inv == nil {
			http.Error(w, "Link expired or not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		invoicehttp.InvoicePrintPage(*inv).Render(r.Context(), w)
	}
}

// portalCustomerReaderAdapter satisfies portalhttp.customerReader using PortalNATSClient.
type portalCustomerReaderAdapter struct {
	client *portalnats.PortalNATSClient
}

func (a *portalCustomerReaderAdapter) GetPortalCustomer(ctx context.Context, id string) (*portalhttp.PortalCustomer, error) {
	bc, err := a.client.GetCustomer(ctx, id)
	if err != nil {
		return nil, err
	}
	return &portalhttp.PortalCustomer{
		ID:          bc.ID,
		CompanyName: bc.CompanyName,
		Email:       bc.Email,
		IPAddress:   bc.IPAddress,
		NetworkZone: bc.NetworkZone,
	}, nil
}

// portalServiceClientAdapter satisfies portalhttp.portalServiceClient using PortalNATSClient.
type portalServiceClientAdapter struct {
	client *portalnats.PortalNATSClient
}

func (a *portalServiceClientAdapter) ListServices(ctx context.Context, customerID string) ([]*portalhttp.PortalService, error) {
	svcs, err := a.client.ListServices(ctx, customerID)
	if err != nil {
		return nil, err
	}
	out := make([]*portalhttp.PortalService, len(svcs))
	for i, s := range svcs {
		out[i] = &portalhttp.PortalService{
			ID:               s.ID,
			ProductName:      s.ProductName,
			PriceAmountCents: s.PriceAmountCents,
			Currency:         s.Currency,
			Status:           s.Status,
			BillingCycle:     s.BillingCycle,
			NextBillingDate:  s.NextBillingDate,
		}
	}
	return out, nil
}

// portalInvoiceListAdapter satisfies invoicequeries.InvoiceReadModel slice expectations.
// (compile-time check that PortalNATSClient.Handle matches invoiceLister)
var _ interface {
	Handle(context.Context, invoicequeries.ListInvoicesForCustomerQuery) ([]invoicequeries.InvoiceReadModel, error)
} = (*portalnats.PortalNATSClient)(nil)
