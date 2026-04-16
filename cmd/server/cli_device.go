package main

import (
	"context"

	"github.com/urfave/cli/v3"
)

func deviceCommands() *cli.Command {
	return &cli.Command{
		Name:  "device",
		Usage: "Manage IoT devices",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List devices",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "status", Usage: "Filter by status (in_stock|deployed|decommissioned)"},
					&cli.StringFlag{Name: "customer", Usage: "Filter by customer ID"},
					&cli.StringFlag{Name: "type", Usage: "Filter by device type"},
					&cli.StringFlag{Name: "search", Usage: "Search term"},
					&cli.IntFlag{Name: "page", Value: 1},
					&cli.IntFlag{Name: "page-size", Value: 25},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					t, err := newTransport(cmd.Root().String("nats-url"), cmd.Root().String("api-url"), cmd.Root().String("api-token"), cmd.Root().String("db"))
					if err != nil {
						return err
					}
					var result any
					return withPrint(&result, t.do(ctx, "device.list", map[string]any{
						"status":     cmd.String("status"),
						"customerID": cmd.String("customer"),
						"deviceType": cmd.String("type"),
						"search":     cmd.String("search"),
						"page":       cmd.Int("page"),
						"pageSize":   cmd.Int("page-size"),
					}, &result))
				},
			},
			{
				Name:      "get",
				Usage:     "Get device by ID",
				ArgsUsage: "<id>",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					id := cmd.Args().First()
					if id == "" {
						return cli.Exit("id required", 1)
					}
					t, err := newTransport(cmd.Root().String("nats-url"), cmd.Root().String("api-url"), cmd.Root().String("api-token"), cmd.Root().String("db"))
					if err != nil {
						return err
					}
					var result any
					return withPrint(&result, t.do(ctx, "device.get", map[string]string{"id": id}, &result))
				},
			},
			{
				Name:  "register",
				Usage: "Register a new device into stock",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "name", Required: true, Usage: "Device name"},
					&cli.StringFlag{Name: "type", Value: "other", Usage: "Device type (modem|router|ont|switch|sensor|other)"},
					&cli.StringFlag{Name: "serial", Usage: "Serial number"},
					&cli.StringFlag{Name: "notes", Usage: "Notes"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					t, err := newTransport(cmd.Root().String("nats-url"), cmd.Root().String("api-url"), cmd.Root().String("api-token"), cmd.Root().String("db"))
					if err != nil {
						return err
					}
					var result any
					return withPrint(&result, t.do(ctx, "device.register", map[string]any{
						"Name":         cmd.String("name"),
						"DeviceType":   cmd.String("type"),
						"SerialNumber": cmd.String("serial"),
						"Notes":        cmd.String("notes"),
					}, &result))
				},
			},
			{
				Name:      "deploy",
				Usage:     "Deploy a device to a customer",
				ArgsUsage: "<id>",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "customer", Required: true, Usage: "Customer ID"},
					&cli.StringFlag{Name: "location", Usage: "Location (address or bin)"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					id := cmd.Args().First()
					if id == "" {
						return cli.Exit("id required", 1)
					}
					t, err := newTransport(cmd.Root().String("nats-url"), cmd.Root().String("api-url"), cmd.Root().String("api-token"), cmd.Root().String("db"))
					if err != nil {
						return err
					}
					var result any
					return withPrint(&result, t.do(ctx, "device.deploy", map[string]any{
						"ID":         id,
						"CustomerID": cmd.String("customer"),
						"Location":   cmd.String("location"),
					}, &result))
				},
			},
			{
				Name:      "return",
				Usage:     "Return a deployed device to stock",
				ArgsUsage: "<id>",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					id := cmd.Args().First()
					if id == "" {
						return cli.Exit("id required", 1)
					}
					t, err := newTransport(cmd.Root().String("nats-url"), cmd.Root().String("api-url"), cmd.Root().String("api-token"), cmd.Root().String("db"))
					if err != nil {
						return err
					}
					return t.do(ctx, "device.return", map[string]string{"id": id}, nil)
				},
			},
			{
				Name:      "decommission",
				Usage:     "Permanently decommission a device",
				ArgsUsage: "<id>",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					id := cmd.Args().First()
					if id == "" {
						return cli.Exit("id required", 1)
					}
					t, err := newTransport(cmd.Root().String("nats-url"), cmd.Root().String("api-url"), cmd.Root().String("api-token"), cmd.Root().String("db"))
					if err != nil {
						return err
					}
					return t.do(ctx, "device.decommission", map[string]string{"id": id}, nil)
				},
			},
			{
				Name:      "update",
				Usage:     "Update device metadata",
				ArgsUsage: "<id>",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "name", Usage: "Device name"},
					&cli.StringFlag{Name: "notes", Usage: "Notes"},
					&cli.StringFlag{Name: "location", Usage: "Location"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					id := cmd.Args().First()
					if id == "" {
						return cli.Exit("id required", 1)
					}
					t, err := newTransport(cmd.Root().String("nats-url"), cmd.Root().String("api-url"), cmd.Root().String("api-token"), cmd.Root().String("db"))
					if err != nil {
						return err
					}
					var result any
					return withPrint(&result, t.do(ctx, "device.update", map[string]any{
						"ID":       id,
						"Name":     cmd.String("name"),
						"Notes":    cmd.String("notes"),
						"Location": cmd.String("location"),
					}, &result))
				},
			},
		},
	}
}
