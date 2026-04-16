---
tldr: Add `vvs cron daemon` — long-running process that schedules jobs using robfig/cron/v3, reloads DB every minute for changes, replaces need for system cron
status: completed
---

# Plan: Cron Daemon

## Context

Currently `vvs cron run` is a one-shot command — system cron calls it every minute,
it finds due jobs, runs them, exits. This requires the OS cron to be configured.

`vvs cron daemon` runs indefinitely as a process: loads jobs from DB at startup,
schedules them via the `robfig/cron/v3` goroutine scheduler, and reloads job
changes from DB every minute (adds new, removes deleted/paused).

- Related: [[plan - 2604160342 - cron persistent job scheduler]] (existing cron module)

---

## Design Decisions

**Reload strategy:** Poll DB every minute alongside job execution.
The scheduler's tick is used as the reload trigger — simple, no NATS dependency.

**Cron library:** `robfig/cron/v3` is already a dep. Use `cron.New(cron.WithSeconds())` 
or just the minute-resolution `cron.New()` for goroutine scheduling (as opposed to `NextTime` only).

**Overlap with `vvs cron run`:** The two are alternatives — use one or the other.
Document: don't run both against the same DB simultaneously.

**File lock:** Use a `.lock` file in the same dir as the DB to prevent duplicate daemon instances.

**Graceful shutdown:** Listen for SIGTERM/SIGINT, stop cron scheduler, wait for in-flight jobs to finish.

---

## Architecture

```
vvs cron daemon
  ├── Open DB (direct, same as cron run)
  ├── Load all active jobs → schedule each with cron.AddFunc(schedule, fn)
  ├── Add a reload job that runs every minute:
  │     - ListAll from DB
  │     - For each job: add if new/active, remove if deleted/paused
  └── Wait for SIGTERM → cron.Stop() → exit
```

Internal state: `map[jobID]cron.EntryID` to track scheduled entries.

---

## Files to Create

```
cmd/server/cron_daemon.go   — Daemon struct, Start/Stop, reload loop
```

## Files to Modify

```
cmd/server/cli_cron.go      — add `daemon` subcommand
```

---

## Phases

### Phase 1 — Daemon implementation - status: completed

1. [x] `cmd/server/cron_daemon.go`
   - => `Daemon` struct: `*cron.Cron`, `map[string]cron.EntryID`, `JobRepository`, `*natsrpc.Server`
   - => `reload` diffs active jobs vs scheduled entries: unschedule removed, schedule new
   - => `Start(ctx)` blocks until ctx done, then calls `cron.Stop()` and waits for in-flight jobs
   - => Reload also scheduled as `* * * * *` cron entry alongside job entries

2. [x] `cmd/server/cli_cron.go` — `daemon` subcommand added
   - => Opens DB twice (repo + NewDirect for rpcServer) same pattern as `cron run`
   - => `signal.NotifyContext` handles SIGTERM/SIGINT

---

## Verification

1. `go build ./...` — clean
2. `vvs cron add --name test --schedule "* * * * *" --type action --action noop`
3. `vvs cron daemon` — starts, logs "scheduled N jobs"
4. Wait ~1 min → see "job test ok" log output
5. In another terminal: `vvs cron add --name test2 ...` → daemon picks it up on next reload
6. `vvs cron pause <id>` → daemon removes it from schedule on next reload
7. SIGTERM → daemon logs "shutdown", exits cleanly

---

## Progress Log

| Timestamp | Entry |
|-----------|-------|
| 2604160356 | Plan created |
| 2604160356 | Implemented and committed 6796643 |
