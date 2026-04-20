package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/urfave/cli/v3"
	"github.com/atvirokodosprendimai/vvs/internal/app"
)

func main() {
	if err := godotenv.Load(); err == nil {
		log.Println("Loaded config from .env")
	}

	cmd := &cli.Command{
		Name:  "vvs",
		Usage: "VVS ISP Business Management System",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "db",
				Usage:   "SQLite database path (for direct CLI access)",
				Value:   "./data/vvs.db",
				Sources: cli.EnvVars("VVS_DB_PATH"),
			},
			&cli.StringFlag{
				Name:    "nats-url",
				Usage:   "NATS server URL (for CLI transport or server external NATS)",
				Sources: cli.EnvVars("NATS_URL"),
			},
			&cli.StringFlag{
				Name:    "api-url",
				Usage:   "VVS API base URL (for CLI HTTP transport, requires --api-token)",
				Value:   "http://localhost:8080",
				Sources: cli.EnvVars("VVS_API_URL"),
			},
			&cli.StringFlag{
				Name:    "api-token",
				Usage:   "Bearer token for REST API (/api/v1/*)",
				Sources: cli.EnvVars("VVS_API_TOKEN"),
			},
			&cli.StringFlag{
				Name:    "base-url",
				Usage:   "Public base URL for generated links, e.g. https://isp.example.com (no trailing slash)",
				Sources: cli.EnvVars("VVS_BASE_URL"),
			},
			&cli.BoolFlag{
				Name:    "demo-mode",
				Usage:   "Disable risky cron job types (shell, url) for public demo environments",
				Sources: cli.EnvVars("VVS_DEMO_MODE"),
			},
		},
		Commands: []*cli.Command{
			serveCommand(),
			cronCommands(),
			{
				Name:  "cli",
				Usage: "Manage ISP resources via API",
				Commands: []*cli.Command{
					customerCommands(),
					productCommands(),
					routerCommands(),
					serviceCommands(),
					userCommands(),
					deviceCommands(),
					cronCommands(),
					invoiceCommands(),
				},
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

func serveCommand() *cli.Command {
	return &cli.Command{
		Name:  "serve",
		Usage: "Start the VVS HTTP server",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "db",
				Usage:   "SQLite database file path",
				Value:   "./data/vvs.db",
				Sources: cli.EnvVars("VVS_DB_PATH"),
			},
			&cli.StringFlag{
				Name:    "addr",
				Usage:   "HTTP listen address",
				Value:   ":8080",
				Sources: cli.EnvVars("VVS_ADDR"),
			},
			&cli.StringFlag{
				Name:    "admin-user",
				Usage:   "Initial admin username (created/updated on startup)",
				Sources: cli.EnvVars("VVS_ADMIN_USER"),
			},
			&cli.StringFlag{
				Name:    "admin-password",
				Usage:   "Initial admin password",
				Sources: cli.EnvVars("VVS_ADMIN_PASSWORD"),
			},
			&cli.StringFlag{
				Name:    "netbox-url",
				Usage:   "NetBox base URL (optional)",
				Sources: cli.EnvVars("NETBOX_URL"),
			},
			&cli.StringFlag{
				Name:    "netbox-token",
				Usage:   "NetBox API token (optional)",
				Sources: cli.EnvVars("NETBOX_TOKEN"),
			},
			&cli.StringFlag{
				Name:    "nats-listen",
				Usage:   "Expose embedded NATS on this TCP addr (optional, e.g. :4222)",
				Sources: cli.EnvVars("NATS_LISTEN_ADDR"),
			},
			&cli.StringFlag{
				Name:    "nats-auth-token",
				Usage:   "Deprecated: use --nats-core-password instead",
				Sources: cli.EnvVars("NATS_AUTH_TOKEN"),
				Hidden:  true,
			},
			&cli.StringFlag{
				Name:    "nats-core-password",
				Usage:   "Password for the 'core' NATS user (full access); enables per-user auth when set with --nats-portal-password",
				Sources: cli.EnvVars("VVS_NATS_CORE_PASSWORD"),
			},
			&cli.StringFlag{
				Name:    "nats-portal-password",
				Usage:   "Password for the 'portal' NATS user (isp.portal.rpc.> only)",
				Sources: cli.EnvVars("VVS_NATS_PORTAL_PASSWORD"),
			},
			&cli.StringFlag{
				Name:    "email-enc-key",
				Usage:   "32-byte AES key (hex or raw) for encrypting IMAP passwords (optional)",
				Sources: cli.EnvVars("VVS_EMAIL_ENC_KEY"),
			},
			&cli.StringFlag{
				Name:    "router-enc-key",
				Usage:   "32-byte AES key (hex or raw) for encrypting router passwords (optional)",
				Sources: cli.EnvVars("VVS_ROUTER_ENC_KEY"),
			},
			&cli.StringFlag{
				Name:    "proxmox-enc-key",
				Usage:   "32-byte AES key (hex or raw) for encrypting Proxmox node token secrets (optional)",
				Sources: cli.EnvVars("VVS_PROXMOX_ENC_KEY"),
			},
			&cli.StringFlag{
				Name:    "modules",
				Usage:   "Comma-separated list of modules to enable (default: all)",
				Sources: cli.EnvVars("VVS_MODULES"),
			},
			&cli.IntFlag{
				Name:    "email-sync-interval",
				Usage:   "Email IMAP sync interval in seconds",
				Value:   300,
				Sources: cli.EnvVars("VVS_EMAIL_SYNC_INTERVAL"),
			},
			&cli.IntFlag{
				Name:    "email-page-size",
				Usage:   "Number of email threads per inbox page",
				Value:   50,
				Sources: cli.EnvVars("VVS_EMAIL_PAGE_SIZE"),
			},
			&cli.IntFlag{
				Name:    "session-lifetime",
				Usage:   "Session cookie lifetime in seconds (default 86400 = 1 day)",
				Value:   86400,
				Sources: cli.EnvVars("VVS_SESSION_LIFETIME"),
			},
			&cli.BoolFlag{
				Name:    "secure-cookie",
				Usage:   "Set Secure flag on session cookie (enable for HTTPS-only production deployments)",
				Sources: cli.EnvVars("VVS_SECURE_COOKIE"),
			},
			&cli.StringFlag{
				Name:    "metrics-addr",
				Usage:   "Address for Prometheus /metrics endpoint (e.g. :9091); empty = disabled",
				Sources: cli.EnvVars("VVS_METRICS_ADDR"),
			},
			&cli.StringFlag{
				Name:    "ollama-url",
				Usage:   "Ollama API base URL for the portal chat bot (default: http://localhost:11434)",
				Sources: cli.EnvVars("VVS_OLLAMA_URL"),
			},
			&cli.StringFlag{
				Name:    "bot-model",
				Usage:   "Ollama model for the portal chat bot (default: llama3.2)",
				Sources: cli.EnvVars("VVS_BOT_MODEL"),
			},
			&cli.BoolFlag{
				Name:    "debug",
				Usage:   "Enable verbose debug logging",
				Sources: cli.EnvVars("VVS_DEBUG"),
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			// Configure structured logging.
			logLevel := slog.LevelInfo
			if cmd.Bool("debug") {
				logLevel = slog.LevelDebug
			}
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})))

			var enabledModules []string
			if m := cmd.String("modules"); m != "" {
				for _, s := range strings.Split(m, ",") {
					if s = strings.TrimSpace(s); s != "" {
						enabledModules = append(enabledModules, s)
					}
				}
			}

			cfg := app.Config{
				DatabasePath:          cmd.String("db"),
				ListenAddr:            cmd.String("addr"),
				AdminUser:             cmd.String("admin-user"),
				AdminPassword:         cmd.String("admin-password"),
				NetBoxURL:             cmd.String("netbox-url"),
				NetBoxToken:           cmd.String("netbox-token"),
				NATSUrl:               cmd.Root().String("nats-url"),
				NATSListenAddr:        cmd.String("nats-listen"),
				NATSAuthToken:         cmd.String("nats-auth-token"),
				NATSCorePassword:      cmd.String("nats-core-password"),
				NATSPortalPassword:    cmd.String("nats-portal-password"),
				APIToken:              cmd.Root().String("api-token"),
				EmailEncKey:           cmd.String("email-enc-key"),
				RouterEncKey:          cmd.String("router-enc-key"),
				ProxmoxEncKey:         cmd.String("proxmox-enc-key"),
				EmailSyncIntervalSecs: int(cmd.Int("email-sync-interval")),
				EmailPageSize:         int(cmd.Int("email-page-size")),
				SessionLifetimeSecs:   int(cmd.Int("session-lifetime")),
				SecureCookie:          cmd.Bool("secure-cookie"),
				BaseURL:               cmd.Root().String("base-url"),
				MetricsAddr:           cmd.String("metrics-addr"),
				OllamaURL:             cmd.String("ollama-url"),
				BotModel:              cmd.String("bot-model"),
				DemoMode:              cmd.Root().Bool("demo-mode"),
				EnabledModules:        enabledModules,
				Debug:                 cmd.Bool("debug"),
			}

			application, err := app.New(cfg)
			if err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
			defer stop()

			go func() {
				if err := application.Start(); err != nil {
					slog.Error("server error", "err", err)
				}
			}()

			slog.Info("VVS ISP Manager started", "addr", cfg.ListenAddr, "debug", cfg.Debug)
			<-ctx.Done()

			slog.Info("shutting down")
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*1e9)
			defer cancel()

			return application.Shutdown(shutdownCtx)
		},
	}
}
