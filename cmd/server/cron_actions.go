package main

import (
	"context"
	"time"

	"github.com/vvs/isp/internal/infrastructure/metrics"
)

// ActionFunc is a built-in action callable by a cron job with type=action.
type ActionFunc func(ctx context.Context) error

// actions is the registry of built-in actions available to cron jobs.
var actions = map[string]ActionFunc{
	"noop": func(ctx context.Context) error { return nil },
}

// RegisterAction adds a named action to the registry. Call before RunDueJobs.
// The action is automatically wrapped to record Prometheus cron metrics.
func RegisterAction(name string, fn ActionFunc) {
	actions[name] = func(ctx context.Context) error {
		start := time.Now()
		err := fn(ctx)
		elapsed := time.Since(start).Seconds()
		metrics.CronDuration.WithLabelValues(name).Observe(elapsed)
		metrics.CronLastRunTime.WithLabelValues(name).SetToCurrentTime()
		return err
	}
}
