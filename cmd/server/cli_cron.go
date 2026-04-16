package main

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
	"github.com/vvs/isp/internal/app"
	"github.com/vvs/isp/internal/infrastructure/gormsqlite"
	cronpersistence "github.com/vvs/isp/internal/modules/cron/adapters/persistence"
)

func cronCommands() *cli.Command {
	return &cli.Command{
		Name:  "cron",
		Usage: "Manage scheduled jobs",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List all cron jobs",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					t, err := newTransport(cmd.Root().String("nats-url"), cmd.Root().String("api-url"), cmd.Root().String("api-token"), cmd.Root().String("db"))
					if err != nil {
						return err
					}
					var result any
					return withPrint(&result, t.do(ctx, "cron.list", nil, &result))
				},
			},
			{
				Name:      "get",
				Usage:     "Get a cron job by ID",
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
					return withPrint(&result, t.do(ctx, "cron.get", map[string]string{"id": id}, &result))
				},
			},
			{
				Name:  "add",
				Usage: "Add a new cron job",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "name", Required: true, Usage: "Job name (unique)"},
					&cli.StringFlag{Name: "schedule", Required: true, Usage: "5-field cron expression (e.g. '0 3 * * *')"},
					&cli.StringFlag{Name: "type", Required: true, Usage: "Job type: action | shell | rpc"},
					&cli.StringFlag{Name: "action", Usage: "Built-in action name (for type=action)"},
					&cli.StringFlag{Name: "command", Usage: "Shell command (for type=shell)"},
					&cli.StringFlag{Name: "subject", Usage: "RPC subject (for type=rpc, e.g. isp.rpc.service.cancel)"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					jobType := cmd.String("type")
					payload, err := buildCronPayload(jobType, cmd.String("action"), cmd.String("command"), cmd.String("subject"))
					if err != nil {
						return cli.Exit(err.Error(), 1)
					}
					t, err := newTransport(cmd.Root().String("nats-url"), cmd.Root().String("api-url"), cmd.Root().String("api-token"), cmd.Root().String("db"))
					if err != nil {
						return err
					}
					var result any
					return withPrint(&result, t.do(ctx, "cron.add", map[string]any{
						"Name":     cmd.String("name"),
						"Schedule": cmd.String("schedule"),
						"JobType":  jobType,
						"Payload":  payload,
					}, &result))
				},
			},
			{
				Name:      "pause",
				Usage:     "Pause a cron job",
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
					return t.do(ctx, "cron.pause", map[string]string{"id": id}, nil)
				},
			},
			{
				Name:      "resume",
				Usage:     "Resume a paused cron job",
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
					return t.do(ctx, "cron.resume", map[string]string{"id": id}, nil)
				},
			},
			{
				Name:      "delete",
				Usage:     "Soft-delete a cron job",
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
					return t.do(ctx, "cron.delete", map[string]string{"id": id}, nil)
				},
			},
			cronRunCommand(),
		},
	}
}

// cronRunCommand is the `vvs cli cron run` and `vvs cron run` implementation.
func cronRunCommand() *cli.Command {
	return &cli.Command{
		Name:  "run",
		Usage: "Execute all due cron jobs (call from system cron every minute)",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			dbPath := cmd.Root().String("db")

			// Open DB for the cron repo (list due jobs, save results).
			gdb, err := gormsqlite.Open(dbPath)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer gdb.Close()

			// NewDirect opens the DB again and wires all handlers for rpc-type jobs.
			rpcServer, cleanup, err := app.NewDirect(dbPath)
			if err != nil {
				return fmt.Errorf("init rpc: %w", err)
			}
			defer cleanup()

			repo := cronpersistence.NewGormJobRepository(gdb)
			RunDueJobs(ctx, repo, rpcServer)
			return nil
		},
	}
}

func buildCronPayload(jobType, action, command, subject string) (string, error) {
	switch jobType {
	case "action":
		if action == "" {
			return "", fmt.Errorf("--action required for type=action")
		}
		return action, nil
	case "shell":
		if command == "" {
			return "", fmt.Errorf("--command required for type=shell")
		}
		return command, nil
	case "rpc":
		if subject == "" {
			return "", fmt.Errorf("--subject required for type=rpc")
		}
		return fmt.Sprintf(`{"subject":%q,"body":{}}`, subject), nil
	default:
		return "", fmt.Errorf("unknown type %q: must be action|shell|rpc", jobType)
	}
}
