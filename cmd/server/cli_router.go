package main

import (
	"context"

	"github.com/urfave/cli/v3"
)

func routerCommands() *cli.Command {
	return &cli.Command{
		Name:  "router",
		Usage: "Manage network routers",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List routers",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					t, err := newTransport(cmd.Root().String("nats-url"), cmd.Root().String("api-url"), cmd.Root().String("api-token"))
					if err != nil {
						return err
					}
					var result any
					return withPrint(&result, t.do(ctx, "router.list", map[string]any{}, &result))
				},
			},
			{
				Name:      "get",
				Usage:     "Get router by ID",
				ArgsUsage: "<id>",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					id := cmd.Args().First()
					if id == "" {
						return cli.Exit("id required", 1)
					}
					t, err := newTransport(cmd.Root().String("nats-url"), cmd.Root().String("api-url"), cmd.Root().String("api-token"))
					if err != nil {
						return err
					}
					var result any
					return withPrint(&result, t.do(ctx, "router.get", map[string]string{"id": id}, &result))
				},
			},
			{
				Name:  "create",
				Usage: "Create a router",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "name", Required: true, Usage: "Router name"},
					&cli.StringFlag{Name: "type", Value: "mikrotik", Usage: "Router type (mikrotik|arista)"},
					&cli.StringFlag{Name: "host", Required: true, Usage: "Hostname or IP"},
					&cli.IntFlag{Name: "port", Value: 8728, Usage: "API port"},
					&cli.StringFlag{Name: "username", Usage: "API username"},
					&cli.StringFlag{Name: "password", Usage: "API password"},
					&cli.StringFlag{Name: "notes", Usage: "Notes"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					t, err := newTransport(cmd.Root().String("nats-url"), cmd.Root().String("api-url"), cmd.Root().String("api-token"))
					if err != nil {
						return err
					}
					var result any
					return withPrint(&result, t.do(ctx, "router.create", map[string]any{
						"Name":       cmd.String("name"),
						"RouterType": cmd.String("type"),
						"Host":       cmd.String("host"),
						"Port":       cmd.Int("port"),
						"Username":   cmd.String("username"),
						"Password":   cmd.String("password"),
						"Notes":      cmd.String("notes"),
					}, &result))
				},
			},
			{
				Name:      "update",
				Usage:     "Update a router",
				ArgsUsage: "<id>",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "name", Usage: "Router name"},
					&cli.StringFlag{Name: "type", Usage: "Router type"},
					&cli.StringFlag{Name: "host", Usage: "Hostname or IP"},
					&cli.IntFlag{Name: "port", Usage: "API port"},
					&cli.StringFlag{Name: "username", Usage: "API username"},
					&cli.StringFlag{Name: "password", Usage: "API password"},
					&cli.StringFlag{Name: "notes", Usage: "Notes"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					id := cmd.Args().First()
					if id == "" {
						return cli.Exit("id required", 1)
					}
					t, err := newTransport(cmd.Root().String("nats-url"), cmd.Root().String("api-url"), cmd.Root().String("api-token"))
					if err != nil {
						return err
					}
					return t.do(ctx, "router.update", map[string]any{
						"ID":         id,
						"Name":       cmd.String("name"),
						"RouterType": cmd.String("type"),
						"Host":       cmd.String("host"),
						"Port":       cmd.Int("port"),
						"Username":   cmd.String("username"),
						"Password":   cmd.String("password"),
						"Notes":      cmd.String("notes"),
					}, nil)
				},
			},
			{
				Name:      "delete",
				Usage:     "Delete a router",
				ArgsUsage: "<id>",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					id := cmd.Args().First()
					if id == "" {
						return cli.Exit("id required", 1)
					}
					t, err := newTransport(cmd.Root().String("nats-url"), cmd.Root().String("api-url"), cmd.Root().String("api-token"))
					if err != nil {
						return err
					}
					return t.do(ctx, "router.delete", map[string]string{"id": id}, nil)
				},
			},
			{
				Name:      "sync-arp",
				Usage:     "Enable or disable ARP-based access for a customer",
				ArgsUsage: "<customerID>",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "action", Value: "enable", Usage: "enable|disable"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					customerID := cmd.Args().First()
					if customerID == "" {
						return cli.Exit("customerID required", 1)
					}
					t, err := newTransport(cmd.Root().String("nats-url"), cmd.Root().String("api-url"), cmd.Root().String("api-token"))
					if err != nil {
						return err
					}
					return t.do(ctx, "router.sync-arp", map[string]string{
						"CustomerID": customerID,
						"Action":     cmd.String("action"),
					}, nil)
				},
			},
		},
	}
}
