package main

import (
	"context"

	"github.com/urfave/cli/v3"
)

func customerCommands() *cli.Command {
	return &cli.Command{
		Name:  "customer",
		Usage: "Manage customers",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List customers",
				Flags: []cli.Flag{
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
					return withPrint(&result, t.do(ctx, "customer.list", map[string]any{
						"search":   cmd.String("search"),
						"page":     cmd.Int("page"),
						"pageSize": cmd.Int("page-size"),
					}, &result))
				},
			},
			{
				Name:      "get",
				Usage:     "Get customer by ID",
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
					return withPrint(&result, t.do(ctx, "customer.get", map[string]string{"id": id}, &result))
				},
			},
			{
				Name:  "create",
				Usage: "Create a customer",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "company", Required: true, Usage: "Company name"},
					&cli.StringFlag{Name: "contact", Usage: "Contact name"},
					&cli.StringFlag{Name: "email", Usage: "Email"},
					&cli.StringFlag{Name: "phone", Usage: "Phone"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					t, err := newTransport(cmd.Root().String("nats-url"), cmd.Root().String("api-url"), cmd.Root().String("api-token"), cmd.Root().String("db"))
					if err != nil {
						return err
					}
					var result any
					return withPrint(&result, t.do(ctx, "customer.create", map[string]any{
						"CompanyName": cmd.String("company"),
						"ContactName": cmd.String("contact"),
						"Email":       cmd.String("email"),
						"Phone":       cmd.String("phone"),
					}, &result))
				},
			},
			{
				Name:      "update",
				Usage:     "Update a customer",
				ArgsUsage: "<id>",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "company", Usage: "Company name"},
					&cli.StringFlag{Name: "contact", Usage: "Contact name"},
					&cli.StringFlag{Name: "email", Usage: "Email"},
					&cli.StringFlag{Name: "phone", Usage: "Phone"},
					&cli.StringFlag{Name: "street", Usage: "Street"},
					&cli.StringFlag{Name: "city", Usage: "City"},
					&cli.StringFlag{Name: "postal-code", Usage: "Postal code"},
					&cli.StringFlag{Name: "country", Usage: "Country"},
					&cli.StringFlag{Name: "tax-id", Usage: "Tax ID"},
					&cli.StringFlag{Name: "notes", Usage: "Notes"},
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
					return t.do(ctx, "customer.update", map[string]any{
						"ID":          id,
						"CompanyName": cmd.String("company"),
						"ContactName": cmd.String("contact"),
						"Email":       cmd.String("email"),
						"Phone":       cmd.String("phone"),
						"Street":      cmd.String("street"),
						"City":        cmd.String("city"),
						"PostalCode":  cmd.String("postal-code"),
						"Country":     cmd.String("country"),
						"TaxID":       cmd.String("tax-id"),
						"Notes":       cmd.String("notes"),
					}, nil)
				},
			},
			{
				Name:      "delete",
				Usage:     "Delete a customer",
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
					return t.do(ctx, "customer.delete", map[string]string{"id": id}, nil)
				},
			},
		},
	}
}

func withPrint(v any, err error) error {
	if err != nil {
		return err
	}
	return printJSON(v)
}
