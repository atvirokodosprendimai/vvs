package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"time"

	"github.com/vvs/isp/internal/modules/cron/domain"
	natsrpc "github.com/vvs/isp/internal/infrastructure/nats/rpc"
)

// RunDueJobs loads all active jobs where next_run <= now, executes each, and
// advances next_run. Intended to be called by system cron every minute.
func RunDueJobs(ctx context.Context, repo domain.JobRepository, rpc *natsrpc.Server) {
	now := time.Now().UTC()
	jobs, err := repo.ListDue(ctx, now)
	if err != nil {
		log.Printf("cron: list due jobs: %v", err)
		return
	}
	if len(jobs) == 0 {
		log.Println("cron: no jobs due")
		return
	}
	log.Printf("cron: %d job(s) due", len(jobs))
	for _, job := range jobs {
		runJob(ctx, job, repo, rpc)
	}
}

func runJob(ctx context.Context, job *domain.Job, repo domain.JobRepository, rpc *natsrpc.Server) {
	jobCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var runErr error
	switch job.JobType {
	case domain.TypeAction:
		runErr = runAction(jobCtx, job.Payload)
	case domain.TypeShell:
		runErr = runShell(jobCtx, job.Payload)
	case domain.TypeRPC:
		runErr = runRPC(jobCtx, job.Payload, rpc)
	default:
		runErr = fmt.Errorf("unknown job type: %s", job.JobType)
	}

	errStr := ""
	if runErr != nil {
		errStr = runErr.Error()
		log.Printf("cron: job %s (%s) failed: %v", job.Name, job.ID, runErr)
	} else {
		log.Printf("cron: job %s (%s) ok", job.Name, job.ID)
	}

	job.AdvanceNextRun(time.Now().UTC(), errStr)
	if err := repo.Save(ctx, job); err != nil {
		log.Printf("cron: save job %s: %v", job.ID, err)
	}
}

func runAction(ctx context.Context, name string) error {
	fn, ok := actions[name]
	if !ok {
		return fmt.Errorf("unknown action: %s", name)
	}
	return fn(ctx)
}

func runShell(ctx context.Context, command string) error {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, out)
	}
	if len(out) > 0 {
		log.Printf("cron shell output: %s", out)
	}
	return nil
}

func runRPC(ctx context.Context, payload string, rpc *natsrpc.Server) error {
	var req struct {
		Subject string          `json:"subject"`
		Body    json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		return fmt.Errorf("parse rpc payload: %w", err)
	}
	if req.Subject == "" {
		return fmt.Errorf("rpc payload missing subject")
	}
	_, err := rpc.Dispatch(ctx, req.Subject, req.Body)
	return err
}
