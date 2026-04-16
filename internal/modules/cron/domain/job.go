package domain

import (
	"errors"
	"time"

	"github.com/robfig/cron/v3"
)

const (
	StatusActive  = "active"
	StatusPaused  = "paused"
	StatusDeleted = "deleted"

	TypeRPC    = "rpc"
	TypeAction = "action"
	TypeShell  = "shell"
	TypeURL    = "url"
)

var (
	ErrNotFound          = errors.New("cron job not found")
	ErrInvalidTransition = errors.New("invalid status transition")
	ErrNameRequired      = errors.New("job name required")
	ErrInvalidSchedule   = errors.New("invalid cron schedule")
)

// parser is a standard 5-field (minute hour dom month dow) parser.
var parser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// NextTime computes the next run time for a cron expression from a given base time.
func NextTime(schedule string, from time.Time) (time.Time, error) {
	s, err := parser.Parse(schedule)
	if err != nil {
		return time.Time{}, ErrInvalidSchedule
	}
	return s.Next(from), nil
}

// Job is a scheduled task persisted in the database.
type Job struct {
	ID        string
	Name      string
	Schedule  string // 5-field cron expression
	JobType   string // rpc | action | shell
	Payload   string // isp.rpc.* subject, action name, or shell command
	Status    string
	LastRun   *time.Time
	LastError string
	NextRun   time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewJob(id, name, schedule, jobType, payload string) (*Job, error) {
	if name == "" {
		return nil, ErrNameRequired
	}
	nextRun, err := NextTime(schedule, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	return &Job{
		ID:        id,
		Name:      name,
		Schedule:  schedule,
		JobType:   jobType,
		Payload:   payload,
		Status:    StatusActive,
		NextRun:   nextRun,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// AdvanceNextRun recalculates NextRun after a successful or failed execution.
// Advances from max(lastRun, current NextRun) so NextRun always moves forward.
func (j *Job) AdvanceNextRun(lastRun time.Time, lastErr string) {
	t := lastRun
	j.LastRun = &t
	j.LastError = lastErr
	base := j.NextRun
	if lastRun.After(base) {
		base = lastRun
	}
	next, err := NextTime(j.Schedule, base)
	if err == nil {
		j.NextRun = next
	}
	j.UpdatedAt = time.Now().UTC()
}

func (j *Job) Pause() error {
	if j.Status != StatusActive {
		return ErrInvalidTransition
	}
	j.Status = StatusPaused
	j.UpdatedAt = time.Now().UTC()
	return nil
}

func (j *Job) Resume() error {
	if j.Status != StatusPaused {
		return ErrInvalidTransition
	}
	j.Status = StatusActive
	// Recalculate NextRun from now (may have drifted while paused)
	if next, err := NextTime(j.Schedule, time.Now().UTC()); err == nil {
		j.NextRun = next
	}
	j.UpdatedAt = time.Now().UTC()
	return nil
}

func (j *Job) Delete() error {
	if j.Status == StatusDeleted {
		return ErrInvalidTransition
	}
	j.Status = StatusDeleted
	j.UpdatedAt = time.Now().UTC()
	return nil
}
