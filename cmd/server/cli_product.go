package main

import (
	"context"

	"github.com/urfave/cli/v3"
)

func productCommands() *cli.Command {
	return &cli.Command{
		Name:  "product",
		Usage: "Manage products",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List products",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "search", Usage: "Search term"},
					&cli.StringFlag{Name: "type", Usage: "Filter by type"},
					&cli.IntFlag{Name: "page", Value: 1},
					&cli.IntFlag{Name: "page-size", Value: 25},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					t, err := newTransport(cmd.Root().String("nats-url"), cmd.Root().String("api-url"), cmd.Root().String("api-token"), cmd.Root().String("db"))
					if err != nil {
						return err
					}
					var result any
					return withPrint(&result, t.do(ctx, "product.list", map[string]any{
						"search":   cmd.String("search"),
						"type":     cmd.String("type"),
						"page":     cmd.Int("page"),
						"pageSize": cmd.Int("page-size"),
					}, &result))
				},
			},
			{
				Name:      "get",
				Usage:     "Get product by ID",
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
					return withPrint(&result, t.do(ctx, "product.get", map[string]string{"id": id}, &result))
				},
			},
			{
				Name:  "create",
				Usage: "Create a product",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "name", Required: true, Usage: "Product name"},
					&cli.StringFlag{Name: "description", Usage: "Description"},
					&cli.StringFlag{Name: "type", Value: "internet", Usage: "Product type"},
					&cli.Int64Flag{Name: "price", Usage: "Price in cents"},
					&cli.StringFlag{Name: "currency", Value: "EUR", Usage: "Currency"},
					&cli.StringFlag{Name: "billing", Value: "monthly", Usage: "Billing period"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					t, err := newTransport(cmd.Root().String("nats-url"), cmd.Root().String("api-url"), cmd.Root().String("api-token"), cmd.Root().String("db"))
					if err != nil {
						return err
					}
					var result any
					return withPrint(&result, t.do(ctx, "product.create", map[string]any{
						"Name":          cmd.String("name"),
						"Description":   cmd.String("description"),
						"Type":          cmd.String("type"),
						"PriceAmount":   cmd.Int64("price"),
						"PriceCurrency": cmd.String("currency"),
						"BillingPeriod": cmd.String("billing"),
					}, &result))
				},
			},
			{
				Name:      "update",
				Usage:     "Update a product",
				ArgsUsage: "<id>",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "name", Usage: "Product name"},
					&cli.StringFlag{Name: "description", Usage: "Description"},
					&cli.StringFlag{Name: "type", Usage: "Product type"},
					&cli.Int64Flag{Name: "price", Usage: "Price in cents"},
					&cli.StringFlag{Name: "currency", Usage: "Currency"},
					&cli.StringFlag{Name: "billing", Usage: "Billing period"},
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
					return t.do(ctx, "product.update", map[string]any{
						"ID":            id,
						"Name":          cmd.String("name"),
						"Description":   cmd.String("description"),
						"Type":          cmd.String("type"),
						"PriceAmount":   cmd.Int64("price"),
						"PriceCurrency": cmd.String("currency"),
						"BillingPeriod": cmd.String("billing"),
					}, nil)
				},
			},
			{
				Name:      "delete",
				Usage:     "Delete a product",
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
					return t.do(ctx, "product.delete", map[string]string{"id": id}, nil)
				},
			},
		},
	}
}
