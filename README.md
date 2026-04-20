# VVS ISP Manager

<img width="2586" height="1462" alt="image" src="https://github.com/user-attachments/assets/30b1e4f4-be53-4655-8aa6-97a229d5c66c" />

Business management system for an ISP. Manages customers, services, recurring invoices, payments, network devices, support tickets, IPTV subscriptions, virtual machines, and team chat. Two binaries — a core admin server and an optional public-facing customer portal.

## Tech Stack

- **Backend:** Go 1.22+ — hexagonal/DDD/CQRS architecture
- **Database:** SQLite (WAL mode, pure Go — no CGo)
- **Frontend:** [Datastar](https://data-star.dev/) SSE + [Templ](https://templ.guide/) HTML templates
- **CSS:** Tailwind v4 CDN
- **Messaging:** Embedded NATS.io (no external broker needed)
- **Payments:** Stripe Checkout (optional)
- **VM Provisioning:** Proxmox VE API (optional)

---

## Quick Start

### Prerequisites

- Go 1.22+
- [templ](https://templ.guide/quick-start/installation) CLI: `go install github.com/a-h/templ/cmd/templ@latest`

### Run in development

```bash
cp .env.example .env   # edit as needed
make run
```

Opens on http://localhost:8080. Creates `./data/vvs.db` automatically.

### Build binaries

```bash
make build
./bin/vvs serve        # core admin server
./bin/vvs-portal serve # customer portal (optional)
```

---

## Configuration

### Core server (`vvs`)

All flags have matching environment variable equivalents. Copy `.env.example` as a starting point.

| Flag | Env Var | Default | Description |
|---|---|---|---|
| `--db` | `VVS_DB_PATH` | `./data/vvs.db` | SQLite database file path |
| `--addr` | `VVS_ADDR` | `:8080` | HTTP listen address |
| `--admin-user` | `VVS_ADMIN_USER` | _(none)_ | Admin username — created/updated on startup |
| `--admin-password` | `VVS_ADMIN_PASSWORD` | _(none)_ | Admin password |
| `--api-token` | `VVS_API_TOKEN` | _(none)_ | Static bearer token for the NATS RPC API |
| `--base-url` | `VVS_BASE_URL` | _(none)_ | Public base URL e.g. `https://isp.example.com` |
| `--secure-cookie` | `VVS_SECURE_COOKIE` | `false` | Set Secure flag on session cookies (enable in prod) |
| `--session-lifetime` | `VVS_SESSION_LIFETIME` | `24h` | Admin session cookie lifetime |
| `--modules` | `VVS_MODULES` | _(all)_ | Comma-separated module allow-list (see below) |
| `--demo-mode` | `VVS_DEMO_MODE` | `false` | Disable destructive operations (demo/staging) |
| `--metrics-addr` | `VVS_METRICS_ADDR` | _(none)_ | Prometheus metrics listen address e.g. `:9090` |
| `--debug` | `VVS_DEBUG` | `false` | Verbose request logging |
| `--nats-url` | `NATS_URL` | _(none)_ | External NATS URL — skips embedded NATS |
| `--nats-listen` | `NATS_LISTEN_ADDR` | _(none)_ | Expose embedded NATS on this address (e.g. `:4222`) |
| `--nats-core-password` | `VVS_NATS_CORE_PASSWORD` | _(none)_ | Password for the internal `core` NATS user |
| `--nats-portal-password` | `VVS_NATS_PORTAL_PASSWORD` | _(none)_ | Password for the `portal` NATS user |
| `--netbox-url` | `NETBOX_URL` | _(none)_ | NetBox base URL for IPAM integration (optional) |
| `--netbox-token` | `NETBOX_TOKEN` | _(none)_ | NetBox API token (optional) |
| `--router-enc-key` | `VVS_ROUTER_ENC_KEY` | _(none)_ | 32-byte AES key for encrypting router credentials |
| `--proxmox-enc-key` | `VVS_PROXMOX_ENC_KEY` | _(none)_ | 32-byte AES key for encrypting Proxmox node credentials |
| `--email-enc-key` | `VVS_EMAIL_ENC_KEY` | _(none)_ | 32-byte AES key for encrypting IMAP credentials |
| `--email-sync-interval` | `VVS_EMAIL_SYNC_INTERVAL` | `120` | Email inbox sync interval in seconds |
| `--email-page-size` | `VVS_EMAIL_PAGE_SIZE` | `50` | Emails fetched per sync page |
| `--ollama-url` | `VVS_OLLAMA_URL` | _(none)_ | Ollama base URL for the portal chat bot |
| `--bot-model` | `VVS_BOT_MODEL` | _(none)_ | Ollama model name e.g. `llama3` |
| `--stripe-secret-key` | `VVS_STRIPE_SECRET_KEY` | _(none)_ | Stripe secret key (`sk_live_...` or `sk_test_...`) |
| `--stripe-webhook-secret` | `VVS_STRIPE_WEBHOOK_SECRET` | _(none)_ | Stripe webhook signing secret (`whsec_...`) |
| `--stripe-publishable-key` | `VVS_STRIPE_PUBLISHABLE_KEY` | _(none)_ | Stripe publishable key (`pk_live_...` or `pk_test_...`) |

### Customer portal (`vvs-portal`)

The portal binary has no database — it delegates all data access to `vvs` via NATS RPC.

| Flag | Env Var | Default | Description |
|---|---|---|---|
| `--addr` | `PORTAL_ADDR` | `:8081` | Portal HTTP listen address |
| `--nats-url` | `NATS_URL` | _(required)_ | NATS URL of the core server |
| `--nats-portal-password` | `NATS_PORTAL_PASSWORD` | _(none)_ | Password for the `portal` NATS user |
| `--base-url` | `VVS_BASE_URL` | _(none)_ | Public base URL e.g. `https://portal.example.com` |
| `--insecure-cookie` | `PORTAL_INSECURE_COOKIE` | `false` | Disable Secure flag on cookies (local dev only) |
| `--stripe-secret-key` | `VVS_STRIPE_SECRET_KEY` | _(none)_ | Stripe secret key (portal creates Checkout sessions) |
| `--stripe-webhook-secret` | `VVS_STRIPE_WEBHOOK_SECRET` | _(none)_ | Stripe webhook signing secret |
| `--stripe-publishable-key` | `VVS_STRIPE_PUBLISHABLE_KEY` | _(none)_ | Stripe publishable key |

### Seeding an admin user

```bash
VVS_ADMIN_USER=admin VVS_ADMIN_PASSWORD=changeme ./bin/vvs serve
```

The admin user is created (or updated) on every startup. Safe to re-run.

### Enabling specific modules

```bash
VVS_MODULES=customer,product,invoice ./bin/vvs serve
```

Available modules: `customer`, `product`, `service`, `network`, `device`, `invoice`, `ticket`, `deal`, `contact`, `task`, `email`, `iptv`, `proxmox`. Auth and cron are always active.

---

## Modules

| Module | Description |
|---|---|
| `auth` | Users, roles, sessions, module-level permissions |
| `customer` | Customer CRM — contacts, notes, portal balance |
| `product` | Product catalog with pricing |
| `service` | Service assignments (product → customer), lifecycle |
| `network` | Routers, ARP sync, NetBox IPAM integration |
| `device` | CPE/STB inventory, deployment, decommission |
| `invoice` | Recurring invoices, PDF generation, payment import |
| `billing` | Prepaid balance ledger (top-up, deduct, adjust) |
| `ticket` | Support ticket queue |
| `deal` | Sales pipeline (deals with stages) |
| `contact` | Address book contacts linked to customers |
| `task` | Task management with due dates and priority |
| `email` | IMAP inbox per customer with threading |
| `cron` | Scheduled jobs (action / shell / url / rpc types) |
| `iptv` | IPTV subscriptions, channels, EPG, STB management |
| `proxmox` | Proxmox VE node management + VM provisioning |
| `portal` | Customer self-service portal (separate binary) |
| `audit_log` | Immutable audit log of all CRM changes |
| `payment` | Bank payment CSV import → invoice matching |

---

## Customer Portal

The portal is a separate binary (`vvs-portal`) deployed on a public-facing server. It communicates with the core server exclusively via NATS RPC — no direct database access.

**Features:**
- Magic-link + self-service email authentication (no passwords)
- Invoice list + PDF download
- Service overview
- Support ticket creation and threading
- AI chat bot (via Ollama) with staff handoff
- VM plan catalog + one-click purchase (via Stripe or prepaid balance)
- Prepaid balance top-up via Stripe
- My VMs overview

```bash
# Core: expose NATS on port 4222 with per-user auth
VVS_NATS_LISTEN_ADDR=:4222 \
VVS_NATS_CORE_PASSWORD=corepass \
VVS_NATS_PORTAL_PASSWORD=portalpass \
./bin/vvs serve

# Portal: connect to core via NATS
NATS_URL=nats://10.0.0.1:4222 \
NATS_PORTAL_PASSWORD=portalpass \
VVS_BASE_URL=https://portal.example.com \
./bin/vvs-portal serve
```

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
make test-integration  # adapter tests (persistence layer)
make test-e2e          # Playwright end-to-end tests
```

---

## Architecture

```
cmd/
  server/              — core binary entrypoint (admin UI + NATS RPC)
  portal/              — portal binary entrypoint (customer-facing, no DB)
internal/
  app/                 — composition root (wires all modules)
  shared/              — domain primitives, events, CQRS interfaces
  modules/
    auth/              — users, roles, sessions, permissions
    customer/          — customer aggregate + CRM tabs
    product/           — product catalog
    service/           — service lifecycle (assign/suspend/cancel)
    network/           — routers, ARP sync, NetBox
    device/            — CPE/STB inventory
    invoice/           — invoices, PDF, payment import
    billing/           — prepaid balance ledger
    ticket/            — support tickets
    deal/              — sales pipeline
    contact/           — address book
    task/              — task management
    email/             — IMAP email per customer
    cron/              — scheduled jobs
    iptv/              — IPTV subscriptions + EPG
    proxmox/           — VM nodes + plans + provisioning
    portal/            — customer portal HTTP + NATS bridge
    audit_log/         — change audit trail
    payment/           — payment CSV import
  infrastructure/
    gormsqlite/        — GORM + SQLite (single writer, read pool)
    nats/              — embedded NATS publisher/subscriber
    http/              — shared HTTP server, router, layout
    stripe/            — Stripe Checkout + webhook adapter
    chat/              — internal staff chat
    notifications/     — notification store + worker
    bot/               — Ollama chat bot sessions
```

**Write path:** HTTP POST → Datastar ReadSignals → Command → Handler → SQLite → Publish NATS event

**Read path:** HTTP GET → Datastar SSE (long-lived) → Subscribe NATS → re-query SQLite → PatchElements to browser

---

## Cron Jobs

Scheduled tasks persisted in SQLite. Two modes:

| Mode | Command | Use when |
|---|---|---|
| System cron | `vvs cron run` | Call via `crontab` every minute |
| Daemon | `vvs cron daemon` | Run as a long-lived service |

### Job types

| Type | Description | Example flag |
|---|---|---|
| `action` | Built-in Go function | `--action noop` |
| `shell` | Shell command | `--command "backup.sh"` |
| `url` | HTTP webhook/ping | `--url https://... --method POST` |
| `rpc` | Internal NATS subject | `--subject isp.rpc.service.cancel` |

```bash
vvs cron add --name daily-backup --schedule "0 3 * * *" --type shell \
  --command "sqlite3 /data/vvs.db .dump > /backups/$(date +%F).sql"
```

---

## Database

SQLite file at `--db` path. Migrations run automatically on startup using [goose](https://github.com/pressly/goose). Each module has its own migration table (`goose_auth`, `goose_customer`, etc.).

To reset: `rm ./data/vvs.db` and restart.
