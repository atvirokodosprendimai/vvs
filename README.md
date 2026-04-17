# VVS ISP Manager

<img width="2586" height="1462" alt="image" src="https://github.com/user-attachments/assets/30b1e4f4-be53-4655-8aa6-97a229d5c66c" />


Business management system for an ISP. Manages customers, products, recurring invoices, payments, and team chat. Single binary, no external services required.

## Tech Stack

- **Backend:** Go 1.22+ — hexagonal/DDD/CQRS architecture
- **Database:** SQLite (WAL mode, pure Go — no CGo)
- **Frontend:** [Datastar](https://data-star.dev/) SSE + [Templ](https://templ.guide/) HTML templates
- **CSS:** Tailwind v4 CDN
- **Messaging:** Embedded NATS.io (no external broker needed)

---

## Quick Start

### Prerequisites

- Go 1.22+
- [templ](https://templ.guide/quick-start/installation) CLI: `go install github.com/a-h/templ/cmd/templ@latest`

### Run in development

```bash
make run
```

Opens on http://localhost:8080. Creates `./data/vvs.db` automatically.

### Build binary

```bash
make build
./bin/vvs
```

---

## Configuration

All flags have matching environment variable equivalents.

| Flag | Env Var | Default | Description |
|---|---|---|---|
| `--db` | `VVS_DB_PATH` | `./data/vvs.db` | SQLite database file path |
| `--addr` | `VVS_ADDR` | `:8080` | HTTP listen address |
| `--admin-user` | `VVS_ADMIN_USER` | _(none)_ | Admin username — created/updated on startup |
| `--admin-password` | `VVS_ADMIN_PASSWORD` | _(none)_ | Admin password |
| `--modules` | `VVS_MODULES` | _(all)_ | Comma-separated list of modules to enable |
| `--netbox-url` | `NETBOX_URL` | _(none)_ | NetBox base URL for IPAM integration (optional) |
| `--netbox-token` | `NETBOX_TOKEN` | _(none)_ | NetBox API token (optional) |
| `--nats-url` | `NATS_URL` | _(none)_ | External NATS server URL — skips embedded NATS |
| `--nats-listen` | `NATS_LISTEN_ADDR` | _(none)_ | Expose embedded NATS on this address (e.g. `:4222`) |

### Seeding an admin user

```bash
./bin/vvs --admin-user admin --admin-password changeme
# or
VVS_ADMIN_USER=admin VVS_ADMIN_PASSWORD=changeme ./bin/vvs
```

The admin user is created (or updated) on every startup. Safe to re-run.

### Enabling specific modules

```bash
./bin/vvs --modules customer,product
# or
VVS_MODULES=customer,product ./bin/vvs
```

Available modules: `customer`, `product`, `network`. Auth is always enabled.

---

## Development

### Live reload

Requires [air](https://github.com/air-verse/air):

```bash
go install github.com/air-verse/air@latest
make dev
```

### Generate templates

Templ files (`.templ`) must be regenerated after changes:

```bash
make generate
# or
templ generate ./internal/...
```

### Run tests

```bash
make test              # all tests with race detector
make test-unit         # domain + shared only (fast)
make test-integration  # adapter tests
```

---

## Architecture

```
cmd/server/          — entrypoint, CLI flags
internal/
  app/               — composition root (wires all modules)
  shared/            — domain primitives, events, CQRS interfaces
  modules/
    auth/            — users, sessions
    customer/        — customer aggregates, CRUD
    product/         — product catalog
    network/         — routers, ARP provisioning
  infrastructure/
    gormsqlite/      — GORM + SQLite (single writer, read pool)
    nats/            — embedded NATS publisher/subscriber
    http/            — shared HTTP server, router, layout, chat, notifications
    chat/            — chat store (threads, messages, members, reads)
    notifications/   — notification store + worker
```

**Write path:** HTTP POST → Datastar ReadSignals → Command → Handler → SQLite (single writer) → Publish NATS event

**Read path:** HTTP GET → Datastar SSE (long-lived) → Subscribe NATS → re-query SQLite → PatchElements to browser

**SSE connections per page:** max 2 — one global `/sse` (clock + notifications + widget chat) and one page-level SSE.

---

## Cron Jobs

Scheduled tasks persisted in SQLite. Two ways to run them:

| Mode | Command | Use when |
|---|---|---|
| System cron | `vvs cron run` | Call via `crontab` every minute |
| Daemon | `vvs cron daemon` | Run as a long-lived service |

The daemon reloads job changes (add/pause/delete) from the DB every minute — no restart needed.

### Manage jobs

```bash
# List all jobs
vvs cron list

# Add jobs
vvs cron add --name <name> --schedule "<cron>" --type <type> [type-specific flags]

# Lifecycle
vvs cron pause  <id>
vvs cron resume <id>
vvs cron delete <id>
```

Schedule is a standard 5-field cron expression: `minute hour dom month dow`.

### Job types

#### `action` — built-in Go function

```bash
vvs cron add --name noop-test --schedule "* * * * *" --type action --action noop
```

Built-in actions: `noop`. Register more in `cmd/server/cron_actions.go`.

#### `shell` — shell command

```bash
vvs cron add --name daily-backup --schedule "0 3 * * *" --type shell \
  --command "pg_dump mydb > /backups/$(date +%F).sql"
```

#### `url` — HTTP ping / webhook

```bash
# Simple GET health-check
vvs cron add --name healthcheck --schedule "*/5 * * * *" --type url \
  --url "https://example.com/health"

# POST webhook with Bearer auth and custom headers
vvs cron add --name nightly-webhook --schedule "0 2 * * *" --type url \
  --url "https://api.example.com/hook" \
  --method POST \
  --header "Authorization: Bearer mytoken" \
  --header "X-Source: vvs-cron"
```

Non-2xx responses are logged as job failures. `--method` defaults to `GET`.

#### `rpc` — internal NATS RPC subject

```bash
vvs cron add --name expire-services --schedule "0 3 * * *" --type rpc \
  --subject isp.rpc.service.cancel
```

### System cron setup (alternative to daemon)

```crontab
* * * * * /usr/local/bin/vvs --db /data/vvs.db cron run >> /var/log/vvs-cron.log 2>&1
```

---

## Database

SQLite file at `--db` path. Migrations run automatically on startup using [goose](https://github.com/pressly/goose). Each module has its own migration table (`goose_auth`, `goose_customer`, etc.).

To reset: `rm ./data/vvs.db` and restart.
