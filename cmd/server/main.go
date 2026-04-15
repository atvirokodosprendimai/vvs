package main

import (
	"context"
	"log"
	"os"
	"os/signal"
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
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cfg := app.Config{
				DatabasePath:  cmd.String("db"),
				ListenAddr:    cmd.String("addr"),
				AdminUser:     cmd.String("admin-user"),
				AdminPassword: cmd.String("admin-password"),
				NetBoxURL:     cmd.String("netbox-url"),
				NetBoxToken:   cmd.String("netbox-token"),
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

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
