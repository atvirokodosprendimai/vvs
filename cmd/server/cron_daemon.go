package main

import (
	"context"
	"log"
	"sync"

	"github.com/robfig/cron/v3"
	natsrpc "github.com/vvs/isp/internal/infrastructure/nats/rpc"
	"github.com/vvs/isp/internal/modules/cron/domain"
)

// Daemon runs scheduled jobs in-process using the robfig/cron goroutine scheduler.
// Call Start(ctx) — it blocks until ctx is cancelled (e.g. SIGTERM).
// Use as an alternative to calling `vvs cron run` every minute via system cron.
type Daemon struct {
	cr       *cron.Cron
	mu       sync.Mutex
	entries  map[string]cron.EntryID // jobID → scheduled EntryID
	repo     domain.JobRepository
	rpc      *natsrpc.Server
	demoMode bool
}

func NewDaemon(repo domain.JobRepository, rpc *natsrpc.Server, demoMode bool) *Daemon {
	return &Daemon{
		cr:       cron.New(),
		entries:  make(map[string]cron.EntryID),
		repo:     repo,
		rpc:      rpc,
		demoMode: demoMode,
	}
}

// Start loads jobs, schedules a reload every minute, then blocks until ctx is done.
func (d *Daemon) Start(ctx context.Context) {
	d.reload(ctx)

	// Reload every minute to pick up new/paused/deleted jobs added via CLI.
	d.cr.AddFunc("* * * * *", func() { d.reload(ctx) }) //nolint:errcheck

	d.cr.Start()
	log.Println("cron daemon: started")

	<-ctx.Done()

	log.Println("cron daemon: shutting down...")
	stopCtx := d.cr.Stop()
	<-stopCtx.Done()
	log.Println("cron daemon: stopped")
}

// reload diffs DB state against currently scheduled entries and reconciles.
func (d *Daemon) reload(ctx context.Context) {
	jobs, err := d.repo.ListAll(ctx)
	if err != nil {
		log.Printf("cron daemon: reload: %v", err)
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Index active jobs.
	active := make(map[string]*domain.Job, len(jobs))
	for _, j := range jobs {
		if j.Status == domain.StatusActive {
			active[j.ID] = j
		}
	}

	// Unschedule entries that are no longer active (paused or deleted).
	for id, entryID := range d.entries {
		if _, ok := active[id]; !ok {
			d.cr.Remove(entryID)
			delete(d.entries, id)
			log.Printf("cron daemon: unscheduled %s", id)
		}
	}

	// Schedule newly active jobs not yet in the table.
	for id, job := range active {
		if _, alreadyScheduled := d.entries[id]; alreadyScheduled {
			continue
		}
		j := job // capture for closure
		entryID, err := d.cr.AddFunc(j.Schedule, func() {
			runJob(ctx, j, d.repo, d.rpc, d.demoMode)
		})
		if err != nil {
			log.Printf("cron daemon: schedule %s (%s): %v", j.Name, j.ID, err)
			continue
		}
		d.entries[id] = entryID
		log.Printf("cron daemon: scheduled %s (%s) @ %s", j.Name, j.ID, j.Schedule)
	}

	log.Printf("cron daemon: %d job(s) scheduled", len(d.entries))
}
