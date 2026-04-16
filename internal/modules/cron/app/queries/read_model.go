package queries

import "time"

type JobReadModel struct {
	ID        string
	Name      string
	Schedule  string
	JobType   string
	Payload   string
	Status    string
	LastRun   *time.Time
	LastError string
	NextRun   time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}
