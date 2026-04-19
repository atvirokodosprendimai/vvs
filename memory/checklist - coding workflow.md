---
name: Coding Workflow Checklist
description: Step-by-step process for every coding task in this project — TDD, build, test, commit, review. Living doc — update when process changes.
type: reference
---

# Coding Workflow Checklist

> **Living document.** When a new process step is discovered or a rule is confirmed, add it here and commit.
> Keep it scannable — one gate per section, sub-bullets for nuance.

---

## Stage 0 — Before Writing a Line

- [ ] Read MEMORY.md index — scan for applicable tier2/feedback files
- [ ] Touching Datastar/templ HTML? → **load** `tier2_datastar.md` now
- [ ] Touching email/IMAP module? → **load** `tier2_email.md` now
- [ ] Touching persistence/migrations/Go utils? → **load** `tier2_go.md` now
- [ ] Searching for a Go symbol/type/function? → `python3 scripts/mine-go.py --search "Name"` **first**, Grep only for non-Go files
- [ ] Understand existing code before modifying — Read the file, don't guess the shape

---

## Stage 1 — Write Failing Tests First (TDD)

- [ ] Domain unit tests — pure Go, no framework deps
  - Table-driven or subtests; test constructor validation + state transitions + error paths
- [ ] Command handler tests — stub repos, real handler logic
  - Pattern: `setupDB(t)` + migrations + real persistence adapter OR in-memory stub
  - Always test the not-found path: `ErrNotFound` return
- [ ] Query handler tests — stub or real DB (see tier2_tdd for which queries need real DB)
- [ ] NATS bridge tests — embedded NATS via `natspkg.StartEmbedded("127.0.0.1:0")` + stub interfaces
- [ ] HTTP handler smoke tests — `httptest.NewRecorder` + chi router; SSE: use closed-channel noopSubscriber
- [ ] **Confirm tests FAIL before implementing** — a test that passes immediately is a false green

**Coverage target:** ≥80% on domain + commands packages per module

---

## Stage 2 — Implement

- [ ] Follow hexagonal arch: domain → commands → queries → adapters (never reverse)
- [ ] HTTP handlers: define **local interface** for every injected dep — never import `adapters/persistence` from `adapters/http`
- [ ] GORM models: explicit `gorm:"column:..."` tags on acronym fields (NetBoxID→`column:netbox_id`)
- [ ] SQLite migrations: always `-- +goose Up` + `-- +goose Down`; avoid reserved keywords (key, order, references, group, index)
- [ ] GORM delete: always `FindByID` first — `tx.Delete()` silently succeeds on missing rows
- [ ] Templ: follow `tier2_datastar.md` — colon syntax, kebab-case, openWhenHidden only on @get

---

## Stage 3 — Verify

- [ ] `templ generate ./internal/...` — if any .templ file changed
- [ ] `go build ./...` — must exit 0, no errors
- [ ] `go test ./[changed package]/...` — must pass
- [ ] `go test ./...` — full suite, confirm no regressions (or targeted run if full suite is slow)
- [ ] LSP errors after templ generate: check if stale `_templ.go` file — `rm` + regenerate if needed

---

## Stage 4 — E2E Tests (new UI flows)

Run when adding a new page, route, or significant user-visible flow.

- [ ] Add spec in `e2e/` — one file per module (e.g. `e2e/iptv.spec.js`)
- [ ] Auth: use `storageState: AUTH_FILE` on chromium project (NOT global `use` block)
- [ ] Wait for SSE-loaded content: `page.waitForSelector('#table-id')` before assertions
- [ ] Mutations return SSE streams — never call `resp.json()` on mutation responses; check `resp.status()` only
- [ ] Unauthenticated tests: `page.context().clearCookies()` (not `storageState: undefined`)
- [ ] Multi-role: fresh `browser.newContext()` (no storageState) for non-admin role
- [ ] Run: `go build -o /tmp/vvs-server ./cmd/server && VVS_DB_PATH=/tmp/vvs-e2e.db /tmp/vvs-server serve &` then `npx playwright test`

---

## Stage 5 — Commit

- [ ] One atomic commit per logical change (feature, fix, migration, test)
- [ ] Stage specific files — never `git add -A` blindly; check for `.env`/secrets
- [ ] Commit message: `type(scope): what + why` (feat/fix/test/chore/refactor)
- [ ] **Update plan file** in same commit or immediately after — mark action `[x]`, add `=>` observations
- [ ] If plan file updated separately: commit it right after code commit

---

## Stage 6 — Post-Coding

- [ ] Spawn Codex review in background after any significant code:
  ```bash
  node ".../codex-companion.mjs" review "" &
  ```
  → Tell user "Codex review started" but don't block on result
- [ ] Update relevant `memory/project_*.md` files if architecture or patterns changed
- [ ] If new process pattern discovered → **update this checklist** and commit

---

## Parallel Dev Protocol (when splitting work across agents)

- [ ] Split by **file ownership** — no two agents touch the same file
- [ ] **Never assign app.go to an agent** — do wiring manually after merge
- [ ] Merge in dependency order: scaffolding → domain → persistence → HTTP → wiring
- [ ] Post-merge diff: `git diff HEAD~1 -- internal/app/app.go` — verify no wiring regressions
- [ ] Check after merge: color palette (neutral/amber, not slate/cyan), NATS strings use typed constants, field names match read models, status badges match domain constants
- [ ] Run: `templ generate && go build ./... && go test ./...` after merge

---

## Consilium (hard decisions)

- [ ] New feature design? Architecture change? Hard trade-off? → spawn Consilium
- [ ] 6 agents in parallel: CEO / CTO / Developer / UX-UI / Security / QA
- [ ] CEO ruling is final among agents; user = owner with veto
- [ ] Present summary with role labels, highlight disagreements

---

## Quick Reference — Common Mistakes to Avoid

| Mistake | Rule |
|---------|------|
| Grep for Go symbol | Use `gomine --search` first |
| Import persistence from HTTP adapter | Define local interface instead |
| `tx.Delete()` without FindByID | Silent success on missing rows |
| `data-attr:disabled` in Datastar | Use `data-class:opacity-50` + guard |
| `openWhenHidden` on `@post` | Only valid on `@get` (SSE) |
| Stalker XMLTV with stub data | Replace with real EPG from DB |
| `resp.json()` on SSE mutation response | Check `resp.status()` only |
| Omit `gorm:"column:..."` on acronym field | GORM splits e.g. NetBoxID → net_box_i_d |
| Skip `-- +goose Down` | Every migration needs Up AND Down |
| Commit without running tests | Tests must pass before commit |

---

## Update Instructions

When a new process step is confirmed or a mistake is made twice:
1. Add to the appropriate stage above (or add a new stage)
2. Add to Quick Reference if it's a recurring mistake
3. Commit: `chore: update coding workflow checklist — <what changed>`
4. Update MEMORY.md description if the checklist description becomes stale

**Last updated:** 2026-04-19
