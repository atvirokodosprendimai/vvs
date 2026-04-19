---
tldr: Sidebar nav category open/close state persists across page loads via localStorage — data-init restores, data-effect saves
status: completed
---

# Plan: Persist Sidebar Nav State Across Page Loads

## Context

- Sidebar: `internal/infrastructure/http/templates/layout.templ`
- Signals: `_navCrm`, `_navFinance`, `_navNetwork`, `_navSystem` (prefixed `_` = not sent to backend)
- Current problem: `data-signals` hardcodes defaults on every page load, all groups reset on navigation

### Why not cookies?

Server-side cookie reading would require the backend to parse the cookie and pass nav state to `Sidebar()` on every page render — adds handler/template coupling for a pure UI preference.

### Why not `data-persist`?

`data-persist` is a Datastar Pro (commercial) attribute. Not available in the OSS bundle.

### Chosen approach: localStorage via `data-init` + `data-effect`

- `data-init` on `<nav>`: read `localStorage.getItem('navState')` once on mount, patch signals
- `data-effect` on `<nav>`: write all `_nav*` signals to localStorage whenever any changes

No backend changes. No new routes. One template edit.

---

## Phases

### Phase 1 — Implement localStorage persistence — status: completed

1. [x] Add `data-init` + `data-effect` to `Sidebar` in `layout.templ`
   - => `data-init` placed before `data-effect` (Datastar eval order)
   - => try/catch in init guards against malformed localStorage data
   - => `??` operator falls back to current signal value if key absent
   - => key `navState`, shape `{crm, finance, network, system}` booleans

---

## Verification

- [ ] Open CRM group, navigate to another page → CRM group still open
- [ ] Close Finance group, reload page → Finance group still closed
- [ ] First load (no localStorage entry) → defaults apply (CRM/Finance/Network open, System closed)
- [ ] Open DevTools → Application → LocalStorage → `navState` key present and updates on toggle

## Progress Log

- **2604172330** — Phase 1 done. Commit 0904d29. Two attributes on `<nav>` in `layout.templ`, no backend changes.
