package main

import (
	"context"
	"time"

	"github.com/urfave/cli/v3"
)

func invoiceCommands() *cli.Command {
	return &cli.Command{
		Name:  "invoice",
		Usage: "Manage invoices",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List invoices",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "status", Usage: "Filter by status (draft|finalized|paid|void)"},
					&cli.StringFlag{Name: "customer-id", Usage: "Filter by customer ID"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					t, err := newTransport(cmd.Root().String("nats-url"), cmd.Root().String("api-url"), cmd.Root().String("api-token"), cmd.Root().String("db"))
					if err != nil {
						return err
					}
					var result any
					if custID := cmd.String("customer-id"); custID != "" {
						return withPrint(&result, t.do(ctx, "invoice.list-for-customer", map[string]string{
							"CustomerID": custID,
						}, &result))
					}
					return withPrint(&result, t.do(ctx, "invoice.list", map[string]string{
						"Status": cmd.String("status"),
					}, &result))
				},
			},
			{
				Name:      "get",
				Usage:     "Get invoice by ID",
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
					return withPrint(&result, t.do(ctx, "invoice.get", map[string]string{"id": id}, &result))
				},
			},
			{
				Name:  "create",
				Usage: "Create a draft invoice",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "customer-id", Required: true, Usage: "Customer ID"},
					&cli.StringFlag{Name: "customer-name", Required: true, Usage: "Customer name"},
					&cli.StringFlag{Name: "customer-code", Usage: "Customer code"},
					&cli.StringFlag{Name: "issue-date", Usage: "Issue date (YYYY-MM-DD, default: today)"},
					&cli.StringFlag{Name: "due-date", Usage: "Due date (YYYY-MM-DD, default: 30 days)"},
					&cli.StringFlag{Name: "notes", Usage: "Invoice notes"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					now := time.Now().UTC()
					issueDate := now
					dueDate := now.AddDate(0, 0, 30)
					if s := cmd.String("issue-date"); s != "" {
						if t, err := time.Parse("2006-01-02", s); err == nil {
							issueDate = t
						}
					}
					if s := cmd.String("due-date"); s != "" {
						if t, err := time.Parse("2006-01-02", s); err == nil {
							dueDate = t
						}
					}
					t, err := newTransport(cmd.Root().String("nats-url"), cmd.Root().String("api-url"), cmd.Root().String("api-token"), cmd.Root().String("db"))
					if err != nil {
						return err
					}
					var result any
					return withPrint(&result, t.do(ctx, "invoice.create", map[string]any{
						"CustomerID":   cmd.String("customer-id"),
						"CustomerName": cmd.String("customer-name"),
						"CustomerCode": cmd.String("customer-code"),
						"IssueDate":    issueDate,
						"DueDate":      dueDate,
						"Notes":        cmd.String("notes"),
					}, &result))
				},
			},
			{
				Name:      "finalize",
				Usage:     "Finalize a draft invoice",
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
					return withPrint(&result, t.do(ctx, "invoice.finalize", map[string]string{"id": id}, &result))
				},
			},
			{
				Name:      "mark-paid",
				Usage:     "Mark a finalized invoice as paid",
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
					return withPrint(&result, t.do(ctx, "invoice.mark-paid", map[string]string{"id": id}, &result))
				},
			},
			{
				Name:      "void",
				Usage:     "Void an invoice",
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
					return withPrint(&result, t.do(ctx, "invoice.void", map[string]string{"id": id}, &result))
				},
			},
			{
				Name:  "generate",
				Usage: "Generate invoice from active subscriptions",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "customer-id", Required: true, Usage: "Customer ID"},
					&cli.StringFlag{Name: "customer-name", Required: true, Usage: "Customer name"},
					&cli.StringFlag{Name: "customer-code", Usage: "Customer code"},
					&cli.IntFlag{Name: "vat-rate", Value: 21, Usage: "Default VAT rate (%)"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					t, err := newTransport(cmd.Root().String("nats-url"), cmd.Root().String("api-url"), cmd.Root().String("api-token"), cmd.Root().String("db"))
					if err != nil {
						return err
					}
					var result any
					return withPrint(&result, t.do(ctx, "invoice.generate", map[string]any{
						"CustomerID":     cmd.String("customer-id"),
						"CustomerName":   cmd.String("customer-name"),
						"CustomerCode":   cmd.String("customer-code"),
						"DefaultVATRate": cmd.Int("vat-rate"),
					}, &result))
				},
			},
		},
	}
}
