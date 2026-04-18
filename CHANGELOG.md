# Changelog

- 2026-04-18 `4a6dd3d` feat: invoice Phase 7 — NATS RPC subjects + CLI subcommands
- 2026-04-18 `910d6c9` chore: add CHANGELOG.md with hook-driven auto-append
- 2026-04-18 `4ec8d0d` feat: audit_log domain layer — entity, repo interface, Logger interface, migration
- 2026-04-18 `94a136c` feat: audit_log persistence, commands, queries, HTTP layer
- 2026-04-18 `011b715` feat: audit log — app wiring, nav, integration into customer/ticket/invoice handlers
- 2026-04-18 `4a24fb1` feat: subscription lifecycle — billing cycle, next billing date, cron invoice generation
- 2026-04-18 `2c611fc` test: integration tests for generate-due-invoices billing cron action
- 2026-04-18 `94daeb0` feat: Today Dashboard — 4-column action panel at top of dashboard
- 2026-04-18 `64c1d0a` feat: audit log fast-follow — service mutations + customer audit tab
- 2026-04-18 `0812e08` test: add Playwright E2E test suite (25 tests, all green)
- 2026-04-18 `554f52a` feat(payment): domain — CSV parser, matcher, TDD (12 tests pass)
- 2026-04-18 `311e74a` feat(payment): FindByCode on invoice repo + import command handlers
- 2026-04-18 `9b1bef4` feat(payment): HTTP handlers, Templ UI, app.go wiring
- 2026-04-18 `e780523` chore(payment): remove Dev C conflicting stub files
- 2026-04-18 `363cff2` fix(payment): Codex review hardening — float precision, body limit, partial errors
