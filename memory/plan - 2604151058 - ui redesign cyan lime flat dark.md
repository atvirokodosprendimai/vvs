---
tldr: Restyle all UI from orange/gray-950 to cyan+lime accents on slate base — flatter, more professional
status: active
---

# Plan: UI Redesign — Cyan + Lime on Slate

## Context

No prior UI spec. Design decisions are captured inline below.
Current stack: `html/template` + Datastar + Tailwind v4 CDN. All UI lives in `.templ` files.

## Design Decisions

### Color Palette

| Role | Old | New |
|------|-----|-----|
| Body bg | `bg-gray-950` | `bg-slate-950` |
| Sidebar / card bg | `bg-gray-900` | `bg-slate-900` |
| Input bg / secondary | `bg-gray-800` | `bg-slate-800` |
| Border default | `border-gray-800` | `border-slate-800` |
| Border input | `border-gray-700` | `border-slate-700` |
| Text primary | `text-gray-100` | `text-slate-100` |
| Text secondary | `text-gray-400` | `text-slate-400` |
| Text muted | `text-gray-500` | `text-slate-500` |
| Text body | `text-gray-300` | `text-slate-300` |
| Primary accent | `orange-500/600/700` | `cyan-500/600/700` |
| Accent text / mono | `text-orange-400` | `text-cyan-400` |
| Accent bg tint | `orange-500/10` | `cyan-500/10` |
| Focus ring | `focus:border-orange-500 focus:ring-orange-500` | `focus:border-cyan-500 focus:ring-cyan-500` |
| Success / active badge | `green-400/500` | `lime-400/500` |
| Warning badge | `yellow-400/500` | keep |
| Danger badge | `red-400/500` | keep |

### Shape / Flatness

- Cards: `rounded-xl` → `rounded-lg` (less pillowy)
- Buttons: `rounded-lg` → `rounded` (sharper, more professional)
- Badges: `rounded-full` → `rounded` (flat pill → flat chip)
- Remove `rounded-xl` from all card/table containers

### Typography

- Page headers: keep `text-2xl font-bold`
- Section headers: keep `text-lg font-semibold`
- No other type changes

## File Inventory

Shared (affects all pages):
- `internal/infrastructure/http/templates/layout.templ`
- `internal/infrastructure/http/templates/components.templ`
- `internal/infrastructure/http/dashboard.templ`
- `internal/infrastructure/http/clock.templ`

Module templates:
- `internal/modules/auth/adapters/http/templates.templ`
- `internal/modules/customer/adapters/http/templates.templ`
- `internal/modules/customer/adapters/http/fragments.templ`
- `internal/modules/product/adapters/http/templates.templ`
- `internal/modules/product/adapters/http/fragments.templ`
- `internal/modules/network/adapters/http/templates.templ`
- `internal/modules/network/adapters/http/fragments.templ`

## Phases

### Phase 1 — Shared components + layout — status: open

1. [ ] Restyle `layout.templ`
   - slate body + sidebar background
   - cyan nav hover/active accents
   - sidebar brand color: `text-cyan-400` instead of `text-orange-500`
   - clock/logout area: slate tones

2. [ ] Restyle `components.templ`
   - PageHeader: slate text
   - Card: slate bg + border, `rounded-lg` (was `rounded-xl`)
   - StatCard: cyan icon tint, slate bg
   - Button primary: cyan, `rounded`; secondary: slate, `rounded`; danger: keep red, `rounded`
   - Input / DataInput: slate bg, cyan focus ring
   - Select: slate bg, cyan focus ring
   - Badge: lime for success, keep warning/danger; `rounded` (flat chip)
   - EmptyState: slate muted
   - Pagination: slate, cyan focus

3. [ ] Restyle `dashboard.templ` — StatCard icon references (already uses shared StatCard)
   - verify no inline orange refs
   - `rounded-lg` for any inline containers

4. [ ] `templ generate` + `go build` — verify no compile errors

### Phase 2 — Auth module — status: open

5. [ ] Restyle `auth/adapters/http/templates.templ`
   - login page: slate card, cyan button, cyan focus rings
   - `rounded` buttons

### Phase 3 — Customer module — status: open

6. [ ] Restyle `customer/adapters/http/templates.templ`
   - list page: slate table, cyan pagination, lime active badge
   - form page: slate inputs, cyan submit button, `rounded` buttons
   - detail page: slate fields, cyan edit button, ARP enable/disable buttons
   - `rounded-lg` for table container

7. [ ] Restyle `customer/adapters/http/fragments.templ`
   - form error: keep red tone (matches danger)

### Phase 4 — Product module — status: open

8. [ ] Restyle `product/adapters/http/templates.templ`
   - same pattern as customer
   - mono price text: `text-cyan-400` (was `text-orange-400`)

9. [ ] Restyle `product/adapters/http/fragments.templ`

### Phase 5 — Network module — status: open

10. [ ] Restyle `network/adapters/http/templates.templ`
11. [ ] Restyle `network/adapters/http/fragments.templ`

### Phase 6 — Final build + verification — status: open

12. [ ] `templ generate ./...` — regenerate all `_templ.go` files
13. [ ] `go build ./...` — clean build
14. [ ] Browser smoke test: login → dashboard → customers → products → routers
    - verify cyan accents, lime active badges, flat cards, professional look
    - verify no orange remnants

## Verification

- `go build ./...` passes
- No `.templ` file references `orange-` or `gray-9` (use grep to confirm)
- Browser: consistent slate/cyan/lime palette across all pages
- Cards visually flatter (rounded-lg not rounded-xl)
- Buttons visually sharper (rounded not rounded-lg)

## Adjustments

<!-- Plans evolve. Document changes with timestamps. -->

## Progress Log

- 2604151058 — Plan created
