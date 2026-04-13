# VVS ISP Manager - Architecture Spec

## Status: In Progress

## Overview
Business management system for ISP operations. Single binary, embedded SQLite + NATS, reactive UI via Datastar SSE.

## Architecture Pattern
- **Hexagonal Architecture** (Ports & Adapters)
- **Domain-Driven Design** (Aggregates, Value Objects, Repository ports)
- **CQRS** (Command/Query Responsibility Segregation)
  - Single writer goroutine (WriteSerializer) for all mutations
  - Multiple reader connections via SQLite WAL mode
  - NATS pub/sub bridges write events to read-side SSE streams

## Tech Stack
| Component | Technology |
|-----------|-----------|
| Language | Go 1.22+ |
| Database | SQLite (no CGo) via glebarez/sqlite + GORM |
| Migrations | Goose v3 (per-module version tables) |
| Messaging | Embedded NATS server |
| Frontend | Datastar SSE + Templ + Tailwind CSS v4 CDN |
| CLI | urfave/cli v3 |

## Module Structure
Each module follows identical layout:
```
module/
  domain/      # Aggregate root, value objects, repository port (interface)
  app/
    commands/  # Write-side handlers (single writer path)
    queries/   # Read-side handlers (concurrent readers)
  adapters/
    persistence/  # GORM repository implementation
    http/         # Datastar SSE handlers + templ templates
    importers/    # (payment module only) File import adapters
  migrations/     # Goose SQL migrations (embedded)
```

## Modules
1. **Customer** - CLI-00001 coded customer management
2. **Product** - ISP service catalog (Internet, VoIP, Hosting, Custom)
3. **Invoice** - INV-2026-00001 numbered invoices with line items
4. **Recurring** - Scheduled invoice generation templates
5. **Payment** - Bank statement import (SEPA CSV) with invoice matching

## CQRS Event Flow
```
Command -> WriteSerializer -> SQLite -> NATS event -> SSE to browser
                                                   -> Read model projection update
```

## NATS Subjects
`isp.{module}.{event}` e.g. `isp.customer.created`, `isp.invoice.paid`

## Completed
- [x] Shared kernel (Money, CompanyCode, Pagination, errors)
- [x] CQRS interfaces (Command/Query handlers)
- [x] Event system (DomainEvent, EventPublisher, EventSubscriber)
- [x] Infrastructure (SQLite, WriteSerializer, embedded NATS, HTTP server)
- [x] Base templates (Layout, Sidebar, Components)
- [x] Customer module (full stack)
- [ ] Product module (in progress - agent)
- [ ] Invoice module (in progress - agent)
- [ ] Recurring module (in progress - agent)
- [ ] Payment module (in progress - agent)
- [ ] Dashboard
- [ ] App wiring (app.go composition root)
