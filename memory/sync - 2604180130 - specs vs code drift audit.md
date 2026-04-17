---
tldr: 10 discrepancies found — 9 code-ahead (specs missing/outdated), 1 spec-ahead (NATS payload pattern unimplemented)
status: open
---

# Sync: Specs vs Code Drift Audit

## Code-ahead — spec needs updating (pull)

| # | Item | Action needed |
|---|------|---------------|
| 1 | `spec - email` says "readonly — no send"; code has reply, compose, star toggle | Update spec to reflect full email UX |
| 2 | `spec - crm` says Task statuses `open\|done`; code has `todo\|in_progress\|done\|cancelled` | Update task status machine in CRM spec |
| 3 | `spec - crm` says "Kanban pipeline view (columns per stage)"; code has table + filter tabs | Update deals UI description in CRM spec |
| 4 | Invoice module — full module (VAT, line items, PDF, customer snapshots) — **no spec** | Create `spec - invoice - ...` |
| 5 | Device module — hardware inventory (register, deploy, decommission, QR) — **no spec** | Create `spec - device - ...` |
| 6 | Product module — ISP products catalog (type, price, billing period) — **no spec** | Create `spec - product - ...` |
| 7 | Service module — customer service subscriptions (active, cancelled) — **no spec** | Create `spec - service - ...` |
| 8 | Cron module — scheduled jobs — **no spec** | Create `spec - cron - ...` |
| 9 | Chat — `infrastructure/http/chat.go` + `chat.templ` — direct messaging between users — **no spec** | Create `spec - chat - ...` |

## Spec-ahead — code doesn't implement this (push)

| # | Item | Action needed |
|---|------|---------------|
| 10 | `spec - nats - per item event payload streaming` — events carry full read model payload, SSE patches individual rows without re-query; code still uses subscribe + full re-query + DeepEqual everywhere | Plan + implement per-item payload streaming |

## Resolution

All 10 deferred. Use `/eidos:sync` or `/eidos:plan` to action individual items.

Suggested priority:
- Items 4–9 (missing specs): pull with `/eidos:pull` per module — cheap, documents what exists
- Item 1–3 (outdated specs): update existing specs — medium effort
- Item 10 (NATS pattern): architectural change — plan first with `/eidos:plan`
