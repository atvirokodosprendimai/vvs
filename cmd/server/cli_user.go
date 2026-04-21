package main

import (
	"context"

	"github.com/urfave/cli/v3"
)

func userCommands() *cli.Command {
	return &cli.Command{
		Name:  "user",
		Usage: "Manage users",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List users",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					t, err := newTransport(cmd.Root().String("nats-url"), cmd.Root().String("api-url"), cmd.Root().String("api-token"), cmd.Root().String("db"))
					if err != nil {
						return err
					}
					var result any
					return withPrint(&result, t.do(ctx, "user.list", map[string]any{}, &result))
				},
			},
			{
				Name:  "create",
				Usage: "Create a user",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "username", Required: true, Usage: "Username"},
					&cli.StringFlag{Name: "password", Required: true, Usage: "Password"},
					&cli.StringFlag{Name: "role", Value: "operator", Usage: "Role (admin|operator)"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					t, err := newTransport(cmd.Root().String("nats-url"), cmd.Root().String("api-url"), cmd.Root().String("api-token"), cmd.Root().String("db"))
					if err != nil {
						return err
					}
					var result any
					return withPrint(&result, t.do(ctx, "user.create", map[string]any{
						"username": cmd.String("username"),
						"password": cmd.String("password"),
						"role":     cmd.String("role"),
					}, &result))
				},
			},
			{
				Name:      "delete",
				Usage:     "Delete a user",
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
					return t.do(ctx, "user.delete", map[string]string{"id": id}, nil)
				},
			},
			{
				Name:  "passwd",
				Usage: "Change a user's password",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "username", Required: true, Usage: "Username"},
					&cli.StringFlag{Name: "password", Required: true, Usage: "New password"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					t, err := newTransport(cmd.Root().String("nats-url"), cmd.Root().String("api-url"), cmd.Root().String("api-token"), cmd.Root().String("db"))
					if err != nil {
						return err
					}
					return t.do(ctx, "user.change-password", map[string]string{
						"username": cmd.String("username"),
						"password": cmd.String("password"),
					}, nil)
				},
			},
		},
	}
}
