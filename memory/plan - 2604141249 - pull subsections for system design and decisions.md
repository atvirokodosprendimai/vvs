# Plan — pull subsections for system design and decisions

## Goal
Produce focused spec files for each bounded concern in the VVS ISP codebase.
Overview pull and architecture spec already created.

## Phases

### Phase 1 — Shared foundations
**Status:** todo

#### Actions
- [ ] `/eidos:pull` shared kernel — `internal/shared/` — spec: `spec - shared kernel - domain primitives and interfaces.md`
  - Money, CompanyCode, Pagination, CQRS generics, DomainEvent/EventPublisher/EventSubscriber

### Phase 2 — Core business modules
**Status:** todo

#### Actions
- [ ] `/eidos:pull` customer module — `internal/modules/customer/` — spec: `spec - customer - lifecycle and identity.md`
- [ ] `/eidos:pull` product module — `internal/modules/product/` — spec: `spec - product - catalogue and pricing.md`

### Phase 3 — Financial modules
**Status:** todo

#### Actions
- [ ] `/eidos:pull` invoice module — `internal/modules/invoice/` — spec: `spec - invoice - lifecycle and calculation.md`
  - Focus: draft→finalized→paid/void lifecycle, line recalculation, sequential numbering
- [ ] `/eidos:pull` recurring module — `internal/modules/recurring/` — spec: `spec - recurring - schedule and generation.md`
  - Focus: Schedule value object, IsDue predicate, next-run calculation

### Phase 4 — Operations modules
**Status:** todo

#### Actions
- [ ] `/eidos:pull` payment module — `internal/modules/payment/` — spec: `spec - payment - import and matching.md`
  - Focus: SEPA import, status lifecycle, manual matching
- [ ] `/eidos:pull` debt module + itax.lt — `internal/modules/debt/` + `internal/infrastructure/itaxtlt/` — spec: `spec - debt - credit risk and itax integration.md`
  - Focus: DebtorProvider port, sync command, live SSE re-render pattern

### Phase 5 — Frontend design system
**Status:** todo

#### Actions
- [ ] `/eidos:pull` UI layer — all `*.templ` files — spec: `spec - frontend - datastar reactivity and design system.md`
  - Focus: SSE handler patterns, signal conventions, dark mode design tokens, component library

## Notes
Architecture overview spec already written: `eidos/spec - architecture - system design and key decisions.md`
Overview pull material: `memory/pull - 2604141249 - system design and decisions overview.md`
