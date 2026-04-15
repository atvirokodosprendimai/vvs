---
tldr: Restyle all UI from orange/gray-950 to cyan+lime accents on slate base — flatter, more professional
status: completed
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

### Phase 1 — Shared components + layout — status: completed

1. [x] Restyle `layout.templ`
   - slate body + sidebar background
   - cyan nav hover/active accents
   - sidebar brand color: `text-cyan-400` instead of `text-orange-500`
   - clock/logout area: slate tones

2. [x] Restyle `components.templ`
   - PageHeader: slate text
   - Card: slate bg + border, `rounded-lg` (was `rounded-xl`)
   - StatCard: cyan icon tint, slate bg
   - Button primary: cyan, `rounded`; secondary: slate, `rounded`; danger: keep red, `rounded`
   - Input / DataInput: slate bg, cyan focus ring
   - Select: slate bg, cyan focus ring
   - Badge: lime for success, keep warning/danger; `rounded` (flat chip)
   - EmptyState: slate muted
   - Pagination: slate, cyan focus

3. [x] Restyle `dashboard.templ`
   - => gap-3, smaller w-4 icons, section header uppercase tracking-wider
4. [x] `templ generate` + `go build` — clean

### Phase 2 — Auth module — status: completed

5. [x] Restyle `auth/adapters/http/templates.templ`
   - => login: max-w-xs, slate card, cyan-400 brand, rounded inputs/button
   - => users table: rounded-lg, roleBadge cyan for admin
   - => modal: slate, rounded inputs

### Phase 3 — Customer module — status: completed

6. [x] Restyle `customer/adapters/http/templates.templ`
   - => table rounded-lg, lime active badge, cyan code mono, hover:slate-800/40
   - => form max-w-xl gap-3 rounded, xs uppercase labels
   - => detail xs labels, slate-200 values, em-dash empty, lime ARP enable button
7. [x] `customer/adapters/http/fragments.templ` — rounded text-xs

### Phase 4 — Product module — status: completed

8. [x] Restyle `product/adapters/http/templates.templ`
   - => cyan-400 price mono, rounded-lg table, lowercase badges, rounded forms
9. [x] `product/adapters/http/fragments.templ` — rounded text-xs

### Phase 5 — Network module — status: completed

10. [x] Restyle `network/adapters/http/templates.templ`
    - => cyan-400 host mono, rounded-lg table, rounded forms
11. [x] `network/adapters/http/fragments.templ` — rounded text-xs

### Phase 6 — Final build + verification — status: completed

12. [x] `templ generate ./...` — all pass
13. [x] `go build ./...` — clean build
14. [x] Grep verified: zero `orange-`, `gray-9/8/7`, `rounded-xl` in any .templ file

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
