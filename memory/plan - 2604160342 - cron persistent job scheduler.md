---
tldr: DB-persisted cron job scheduler — cron expression, NATS RPC subjects / built-in actions / shell commands, one-shot run mode
status: completed
---

# Plan: Cron Persistent Job Scheduler

## Context

Need a scheduled task system backed by SQLite.
`vvs cron run` is called by system cron every minute — it finds due jobs, runs them, and calculates the next `next_run` timestamp.
Jobs are registered via `vvs cli cron add` and stored in the DB.

- Architecture spec: [[spec - architecture - system design and key decisions]]
- Events spec: [[spec - events - event driven module boundaries and nats subject taxonomy]]

---

## Domain Design

**Module:** `internal/modules/cron/` (hexagonal, own migration)

**Aggregate: `Job`**
```
ID          string    UUIDv7
Name        string    human label (unique)
Schedule    string    5-field cron expression (e.g. "0 3 * * *")
JobType     string    rpc | action | shell
Payload     string    JSON for rpc/action, shell command string for shell
Status      string    active | paused | deleted
LastRun     *time.Time
LastError   string
NextRun     time.Time  (computed from Schedule + now on create/after run)
CreatedAt   time.Time
UpdatedAt   time.Time
```

**Job types:**
- `rpc` — dispatches an `isp.rpc.*` subject via the RPC dispatcher (same as CLI transport)
- `action` — calls a named built-in action registered in code (extensible registry)
- `shell` — `exec.CommandContext(ctx, "sh", "-c", payload)`

**Status machine:**
```
(none) ─add──▶ active
                 │ ▲
           pause │ │ resume
                 ▼ │
              paused
                 │
          delete │
                 ▼
              deleted  (soft-delete, kept for history)
```

**Cron library:** `github.com/robfig/cron/v3` — standard 5-field parser only (no seconds field, no `@every`).
Used only for `NextTime(schedule, from)` — not for goroutine scheduling. The scheduler itself is the OS cron.

---

## Run semantics

```
vvs cron run (called by system cron every minute)
  1. Open DB (direct mode, like CLI)
  2. SELECT * FROM cron_jobs WHERE status='active' AND next_run <= NOW()
  3. For each due job:
     a. Execute (rpc / action / shell) with 30s timeout
     b. UPDATE last_run, last_error, next_run=NextTime(schedule, NOW())
  4. Exit
```

No locking needed — SQLite serializes writes. If two calls overlap, the second
picks up any jobs the first missed (next_run already advanced past the overlap).

---

## Built-in Actions Registry

```go
type ActionFunc func(ctx context.Context) error

var actions = map[string]ActionFunc{
    "noop": func(ctx context.Context) error { return nil },
    // More registered at startup: "service.expire-overdue", etc.
}
```

Actions are registered in `cmd/server/cron_actions.go` — same binary, no import cycles.

---

## CLI Commands

```
vvs cli cron list                     — list all jobs (table: name, schedule, type, last run, next run, status)
vvs cli cron get <id>                 — get one job
vvs cli cron add
    --name "expire-services"
    --schedule "0 3 * * *"
    --type rpc                        — rpc | action | shell
    --subject isp.rpc.service.cancel  — for type=rpc (stored in payload as JSON)
    --action noop                     — for type=action
    --command "pg_dump ..."           — for type=shell
vvs cli cron pause <id>
vvs cli cron resume <id>
vvs cli cron delete <id>
vvs cli cron run                      — execute due jobs (called by system cron)
```

`vvs cron run` is also available as a top-level subcommand alias: `vvs cron run`.

---

## Migration

```sql
-- 001_create_cron_jobs.sql
CREATE TABLE cron_jobs (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    schedule    TEXT NOT NULL,
    job_type    TEXT NOT NULL CHECK(job_type IN ('rpc','action','shell')),
    payload     TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'active'
                CHECK(status IN ('active','paused','deleted')),
    last_run    DATETIME,
    last_error  TEXT,
    next_run    DATETIME NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_cron_jobs_next_run ON cron_jobs(next_run) WHERE status='active';
```

---

## Files to Create

```
internal/modules/cron/
  domain/
    job.go           — Job aggregate, status consts, ErrNotFound
    repository.go    — JobRepository interface
    job_test.go      — status transitions
  app/
    commands/
      add.go         — AddJobHandler
      pause.go       — PauseJobHandler
      resume.go      — ResumeJobHandler
      delete.go      — DeleteJobHandler
    queries/
      list.go        — ListJobsHandler + JobReadModel
      get.go         — GetJobHandler
  adapters/
    persistence/
      gorm_repository.go
      models.go
  migrations/
    001_create_cron_jobs.sql
    embed.go

cmd/server/
  cron_runner.go     — RunDueJobs(ctx, db, rpcDispatcher) — loads due jobs, executes each
  cron_actions.go    — built-in action registry + default actions
  cli_cron.go        — vvs cli cron {list,get,add,pause,resume,delete} + vvs cron run
```

## Files to Modify

```
internal/app/app.go      — add cron migration
internal/app/direct.go   — add cron repo + handlers for CLI direct mode
internal/infrastructure/nats/rpc/server.go — add cron.* RPC subjects
cmd/server/main.go       — add `vvs cron run` top-level command + `vvs cli cron` subcommands
```

---

## Phases

### Phase 1 — Domain + persistence + migration - status: completed

1. [x] domain/job.go — Job aggregate, Add/Pause/Resume/Delete methods, status machine
   - => AdvanceNextRun advances from max(lastRun, currentNextRun) so NextRun always moves forward
2. [x] domain/job_test.go — 14 tests, all pass
3. [x] migration 001_create_cron_jobs.sql + embed.go
4. [x] adapters/persistence — GormJobRepository (Save, FindByID, FindByName, ListDue, ListAll)

### Phase 2 — Command/query handlers - status: completed

5. [x] app/commands: AddJob, PauseJob, ResumeJob, DeleteJob
6. [x] app/queries: ListJobs, GetJob + JobReadModel

### Phase 3 — Runner + CLI - status: completed

7. [x] cmd/server/cron_runner.go — RunDueJobs
   - => rpc payload format: `{"subject":"isp.rpc.*","body":{}}`
   - => cronRunCommand opens DB twice (once for repo, once for rpcServer via NewDirect) — fine for SQLite

8. [x] cmd/server/cron_actions.go — action registry, default "noop" action

9. [x] cmd/server/cli_cron.go
   - `vvs cli cron {list, get, add, pause, resume, delete, run}`
   - `vvs cron run` — top-level alias via `vvs cron run`

10. [x] Wire into main.go + app.go + direct.go + natsrpc (6 subjects: cron.list/get/add/pause/resume/delete)

---

## Verification

1. `go test ./internal/modules/cron/... -v -race` — transitions pass
2. `go build ./...` — clean
3. `vvs cli cron add --name test --schedule "* * * * *" --type action --action noop`
4. `vvs cli cron list` → shows job, next_run ≈ next minute
5. `vvs cron run` → executes noop, next_run advances by 1 minute
6. `vvs cli cron list` → last_run updated, next_run +1m
7. `vvs cron run` again before next_run → job skipped (not due)
8. Pause → `vvs cron run` skips paused job

---

## Progress Log

| Timestamp | Entry |
|-----------|-------|
| 2604160342 | Plan created |
| 2604161600 | All phases complete — domain tests pass, persistence + CLI wired, committed 08af2e0 |
