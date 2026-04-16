package main

import (
	"context"

	"github.com/urfave/cli/v3"
)

func serviceCommands() *cli.Command {
	return &cli.Command{
		Name:  "service",
		Usage: "Manage customer services",
		Commands: []*cli.Command{
			{
				Name:      "list",
				Usage:     "List services for a customer",
				ArgsUsage: "<customerID>",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					customerID := cmd.Args().First()
					if customerID == "" {
						return cli.Exit("customerID required", 1)
					}
					t, err := newTransport(cmd.Root().String("nats-url"), cmd.Root().String("api-url"), cmd.Root().String("api-token"), cmd.Root().String("db"))
					if err != nil {
						return err
					}
					var result any
					return withPrint(&result, t.do(ctx, "service.list", map[string]string{"CustomerID": customerID}, &result))
				},
			},
			{
				Name:      "assign",
				Usage:     "Assign a service to a customer",
				ArgsUsage: "<customerID>",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "product-id", Required: true, Usage: "Product ID"},
					&cli.StringFlag{Name: "product-name", Required: true, Usage: "Product name (snapshot)"},
					&cli.Int64Flag{Name: "price", Usage: "Price override in cents (0 = use product price)"},
					&cli.StringFlag{Name: "currency", Value: "EUR", Usage: "Currency"},
					&cli.StringFlag{Name: "start-date", Usage: "Start date YYYY-MM-DD (default: today)"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					customerID := cmd.Args().First()
					if customerID == "" {
						return cli.Exit("customerID required", 1)
					}
					startDate := cmd.String("start-date")
					if startDate == "" {
						startDate = "2006-01-02" // will be overridden by server default
					}
					t, err := newTransport(cmd.Root().String("nats-url"), cmd.Root().String("api-url"), cmd.Root().String("api-token"), cmd.Root().String("db"))
					if err != nil {
						return err
					}
					var result any
					return withPrint(&result, t.do(ctx, "service.assign", map[string]any{
						"CustomerID":  customerID,
						"ProductID":   cmd.String("product-id"),
						"ProductName": cmd.String("product-name"),
						"PriceAmount": cmd.Int64("price"),
						"Currency":    cmd.String("currency"),
						"StartDate":   startDate,
					}, &result))
				},
			},
			{
				Name:      "suspend",
				Usage:     "Suspend a service",
				ArgsUsage: "<serviceID>",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					id := cmd.Args().First()
					if id == "" {
						return cli.Exit("serviceID required", 1)
					}
					t, err := newTransport(cmd.Root().String("nats-url"), cmd.Root().String("api-url"), cmd.Root().String("api-token"), cmd.Root().String("db"))
					if err != nil {
						return err
					}
					return t.do(ctx, "service.suspend", map[string]string{"id": id}, nil)
				},
			},
			{
				Name:      "reactivate",
				Usage:     "Reactivate a suspended service",
				ArgsUsage: "<serviceID>",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					id := cmd.Args().First()
					if id == "" {
						return cli.Exit("serviceID required", 1)
					}
					t, err := newTransport(cmd.Root().String("nats-url"), cmd.Root().String("api-url"), cmd.Root().String("api-token"), cmd.Root().String("db"))
					if err != nil {
						return err
					}
					return t.do(ctx, "service.reactivate", map[string]string{"id": id}, nil)
				},
			},
			{
				Name:      "cancel",
				Usage:     "Cancel a service",
				ArgsUsage: "<serviceID>",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					id := cmd.Args().First()
					if id == "" {
						return cli.Exit("serviceID required", 1)
					}
					t, err := newTransport(cmd.Root().String("nats-url"), cmd.Root().String("api-url"), cmd.Root().String("api-token"), cmd.Root().String("db"))
					if err != nil {
						return err
					}
					return t.do(ctx, "service.cancel", map[string]string{"id": id}, nil)
				},
			},
		},
	}
}
