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
	invoicehttp "github.com/vvs/isp/internal/modules/invoice/adapters/http"
	invoicequeries "github.com/vvs/isp/internal/modules/invoice/app/queries"
	portalhttp "github.com/vvs/isp/internal/modules/portal/adapters/http"
	portalnats "github.com/vvs/isp/internal/modules/portal/adapters/nats"
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
				Name:    "nats-auth-token",
				Usage:   "NATS auth token (optional, for plain-auth NATS connections)",
				Sources: cli.EnvVars("NATS_AUTH_TOKEN"),
			},
			&cli.StringFlag{
				Name:    "base-url",
				Usage:   "Public base URL, e.g. https://portal.example.com (no trailing slash)",
				Sources: cli.EnvVars("VVS_BASE_URL"),
			},
			&cli.BoolFlag{
				Name:    "secure-cookie",
				Usage:   "Set Secure flag on session cookies (requires HTTPS)",
				Sources: cli.EnvVars("PORTAL_SECURE_COOKIE"),
			},
		},
		Action: runPortal,
	}
}

func runPortal(ctx context.Context, cmd *cli.Command) error {
	addr := cmd.String("addr")
	natsURL := cmd.String("nats-url")
	natsToken := cmd.String("nats-auth-token")
	baseURL := cmd.String("base-url")
	secureCookie := cmd.Bool("secure-cookie")

	// Connect to NATS (vvs-core side)
	opts := []nats.Option{nats.Name("vvs-portal")}
	if natsToken != "" {
		opts = append(opts, nats.Token(natsToken))
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
		client,                                     // domain.PortalTokenRepository
		client,                                     // invoiceLister (Handle with ListInvoicesForCustomerQuery)
		&portalnats.InvoiceGetterAdapter{C: client}, // invoiceGetter (Handle with id string)
	).
		WithPDFTokens(client).
		WithCustomerReader(custAdapter).
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

// publicInvoiceByToken handles GET /i/{token} — validates the invoice token via NATS,
// fetches the invoice via NATS, and renders the printable invoice HTML.
func publicInvoiceByToken(client *portalnats.PortalNATSClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		plain := chi.URLParam(r, "token")
		sum := sha256.Sum256([]byte(plain))
		hashHex := hex.EncodeToString(sum[:])

		invoiceID, err := client.ValidateInvoiceToken(r.Context(), hashHex)
		if err != nil {
			http.Error(w, "Link expired or not found", http.StatusNotFound)
			return
		}

		inv, err := client.GetInvoice(r.Context(), invoiceID)
		if err != nil || inv == nil {
			http.Error(w, "Invoice not found", http.StatusNotFound)
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
	}, nil
}

// portalInvoiceListAdapter satisfies invoicequeries.InvoiceReadModel slice expectations.
// (compile-time check that PortalNATSClient.Handle matches invoiceLister)
var _ interface {
	Handle(context.Context, invoicequeries.ListInvoicesForCustomerQuery) ([]invoicequeries.InvoiceReadModel, error)
} = (*portalnats.PortalNATSClient)(nil)
