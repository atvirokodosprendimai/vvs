package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"time"

	natsrpc "github.com/vvs/isp/internal/infrastructure/nats/rpc"
	"github.com/vvs/isp/internal/modules/cron/domain"
)

// RunDueJobs loads all active jobs where next_run <= now, executes each, and
// advances next_run. Intended to be called by system cron every minute.
// demoMode blocks shell and url job types (safe for public demo environments).
func RunDueJobs(ctx context.Context, repo domain.JobRepository, rpc *natsrpc.Server, demoMode bool) {
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
		runJob(ctx, job, repo, rpc, demoMode)
	}
}

func runJob(ctx context.Context, job *domain.Job, repo domain.JobRepository, rpc *natsrpc.Server, demoMode bool) {
	jobCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var runErr error
	switch job.JobType {
	case domain.TypeAction:
		runErr = runAction(jobCtx, job.Payload)
	case domain.TypeShell:
		if demoMode {
			runErr = fmt.Errorf("demo mode: shell jobs are disabled")
		} else {
			runErr = runShell(jobCtx, job.Payload)
		}
	case domain.TypeRPC:
		runErr = runRPC(jobCtx, job.Payload, rpc)
	case domain.TypeURL:
		if demoMode {
			runErr = fmt.Errorf("demo mode: url jobs are disabled")
		} else {
			runErr = runURL(jobCtx, job.Payload)
		}
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

// URLPayload is the JSON structure stored in Job.Payload for type=url.
type URLPayload struct {
	URL     string            `json:"url"`
	Method  string            `json:"method,omitempty"`  // default: GET
	Headers map[string]string `json:"headers,omitempty"` // e.g. {"Authorization": "Bearer token"}
}

func runURL(ctx context.Context, payload string) error {
	var p URLPayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return fmt.Errorf("parse url payload: %w", err)
	}
	if p.URL == "" {
		return fmt.Errorf("url payload missing url")
	}
	method := p.Method
	if method == "" {
		method = http.MethodGet
	}

	req, err := http.NewRequestWithContext(ctx, method, p.URL, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	for k, v := range p.Headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}
