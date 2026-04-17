---
tldr: Sidebar nav category open/close state persists across page loads via localStorage — data-init restores, data-effect saves
status: active
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

### Phase 1 — Implement localStorage persistence — status: active

1. [ ] Add `data-init` + `data-effect` to `Sidebar` in `layout.templ`
   - `data-init` restores signals from stored JSON (falls back to defaults if absent)
   - `data-effect` saves current state on every signal change
   - Key: `'navState'`, value: `{crm, finance, network, system}` booleans

---

## Verification

- [ ] Open CRM group, navigate to another page → CRM group still open
- [ ] Close Finance group, reload page → Finance group still closed
- [ ] First load (no localStorage entry) → defaults apply (CRM/Finance/Network open, System closed)
- [ ] Open DevTools → Application → LocalStorage → `navState` key present and updates on toggle

## Progress Log

<!-- entries added after each action -->
