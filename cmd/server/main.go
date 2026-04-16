package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/urfave/cli/v3"
	"github.com/vvs/isp/internal/app"
)

func main() {
	cmd := &cli.Command{
		Name:  "vvs",
		Usage: "VVS ISP Business Management System",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "nats-url",
				Usage:   "NATS server URL (for CLI transport or server external NATS)",
				Sources: cli.EnvVars("NATS_URL"),
			},
			&cli.StringFlag{
				Name:    "api-url",
				Usage:   "VVS API base URL (for CLI HTTP transport fallback)",
				Value:   "http://localhost:8080",
				Sources: cli.EnvVars("VVS_API_URL"),
			},
			&cli.StringFlag{
				Name:    "api-token",
				Usage:   "Bearer token for REST API (/api/v1/*)",
				Sources: cli.EnvVars("VVS_API_TOKEN"),
			},
		},
		Commands: []*cli.Command{
			serveCommand(),
			{
				Name:  "cli",
				Usage: "Manage ISP resources via API",
				Commands: []*cli.Command{
					customerCommands(),
					productCommands(),
					routerCommands(),
					serviceCommands(),
					userCommands(),
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
				Name:    "modules",
				Usage:   "Comma-separated list of modules to enable (default: all)",
				Sources: cli.EnvVars("VVS_MODULES"),
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			var enabledModules []string
			if m := cmd.String("modules"); m != "" {
				for _, s := range strings.Split(m, ",") {
					if s = strings.TrimSpace(s); s != "" {
						enabledModules = append(enabledModules, s)
					}
				}
			}

			cfg := app.Config{
				DatabasePath:   cmd.String("db"),
				ListenAddr:     cmd.String("addr"),
				AdminUser:      cmd.String("admin-user"),
				AdminPassword:  cmd.String("admin-password"),
				NetBoxURL:      cmd.String("netbox-url"),
				NetBoxToken:    cmd.String("netbox-token"),
				NATSUrl:        cmd.Root().String("nats-url"),
				NATSListenAddr: cmd.String("nats-listen"),
				APIToken:       cmd.Root().String("api-token"),
				EnabledModules: enabledModules,
			}

			application, err := app.New(cfg)
			if err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
			defer stop()

			go func() {
				if err := application.Start(); err != nil {
					log.Printf("server error: %v", err)
				}
			}()

			log.Printf("VVS ISP Manager running on %s", cfg.ListenAddr)
			<-ctx.Done()

			log.Println("Shutting down...")
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*1e9)
			defer cancel()

			return application.Shutdown(shutdownCtx)
		},
	}
}
