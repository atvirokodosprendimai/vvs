---
tldr: Web UI for cron job management — list, add, pause/resume/delete — matching CLI functionality
status: active
---

# Plan: Cron Web GUI

## Context

Cron module CLI is complete ([[plan - 2604160342 - cron persistent job scheduler]]).
This adds a web UI at `/cron` matching what `vvs cli cron {list,add,pause,resume,delete}` does.
Follows the same hexagonal HTTP adapter pattern as `internal/modules/device/adapters/http/`.

---

## UI Design

```
┌─────────────────────────────────────────────────────────────────┐
│  Cron Jobs                                         [+ Add Job]  │
├───────────┬──────────────┬──────┬──────────┬──────────┬─────────┤
│ Name      │ Schedule     │ Type │ Last Run │ Next Run │ Status  │ Actions │
├───────────┼──────────────┼──────┼──────────┼──────────┼─────────┤
│ healthchk │ */5 * * * *  │ url  │ 3m ago   │ in 2m    │ active  │ Pause / Delete │
│ backup    │ 0 3 * * *    │shell │ 7h ago   │ in 17h   │ active  │ Pause / Delete │
│ old-job   │ 0 * * * *    │ rpc  │ 1h ago   │ —        │ paused  │ Resume / Delete│
└───────────┴──────────────┴──────┴──────────┴──────────┴─────────┘
```

Add modal: type selector drives visible payload fields.
- `action` → action name input
- `shell`  → command textarea
- `url`    → URL + method + repeatable headers (key:value rows)
- `rpc`    → subject input

No SSE needed — cron jobs change infrequently. Actions refresh the table via Datastar `@get('/sse/cron')`.

---

## Files to Create

```
internal/modules/cron/adapters/http/
  handlers.go      — CronHandlers struct, list page, SSE patch, add/pause/resume/delete actions
  templates.templ  — CronListPage, CronTable, addModal
```

## Files to Modify

```
internal/infrastructure/http/templates/layout.templ   — add Cron nav item (after Devices)
internal/app/app.go                                    — wire CronHandlers, register routes
```

---

## Routes

```
GET  /cron                  — list page (server-rendered)
GET  /sse/cron              — SSE: push CronTable patch on subscribe
POST /api/cron              — add job
POST /api/cron/{id}/pause   — pause
POST /api/cron/{id}/resume  — resume
DELETE /api/cron/{id}       — soft-delete
```

---

## Phases

### Phase 1 — HTTP adapter + templates - status: open

1. [ ] `internal/modules/cron/adapters/http/handlers.go`
   - `CronHandlers` struct with ListJobs, AddJob, PauseJob, ResumeJob, DeleteJob handlers
   - `listPage` — renders CronListPage with all jobs
   - `listSSE` — SSE: ListJobs → CronTable fragment → MergeFragments
   - `addJob` (POST /api/cron) — decodes form JSON, calls AddJobHandler, re-renders table via SSE
   - `pauseJob`, `resumeJob`, `deleteJob` — mutate + re-render table via SSE
   - `RegisterRoutes(r chi.Router)`

2. [ ] `internal/modules/cron/adapters/http/templates.templ`
   - `CronListPage` — uses `@templates.Layout("Cron Jobs")`
   - `CronTable` — the SSE-patchable `#cron-table` fragment
   - Status badge (active=green/orange, paused=yellow, deleted=grey)
   - Add modal — `data-show` signals per type to show/hide relevant fields

### Phase 2 — Wiring + nav - status: open

3. [ ] `layout.templ` — add Cron nav item (clock icon) between Devices and Users

4. [ ] `app.go` — create `CronHandlers`, append to `moduleRoutes`

5. [ ] `templ generate && go build ./...` — verify clean

---

## Verification

1. `templ generate && go build ./...` — clean
2. Browser: `/cron` — table loads with all jobs
3. Click "+ Add Job" → fill type=url → submit → job appears in table
4. "Pause" button → status badge changes to paused, Resume appears
5. "Delete" → row disappears (deleted jobs hidden)
6. `vvs cron add ...` via CLI → reload page → new job appears

---

## Progress Log

| Timestamp | Entry |
|-----------|-------|
| 2604160657 | Plan created |
