package main

import "context"

// ActionFunc is a built-in action callable by a cron job with type=action.
type ActionFunc func(ctx context.Context) error

// actions is the registry of built-in actions available to cron jobs.
var actions = map[string]ActionFunc{
	"noop": func(ctx context.Context) error { return nil },
}

// RegisterAction adds a named action to the registry. Call before RunDueJobs.
func RegisterAction(name string, fn ActionFunc) {
	actions[name] = fn
}
