package persistence

import (
	"time"

	"github.com/vvs/isp/internal/modules/cron/domain"
)

// JobModel is the GORM model mapping to the cron_jobs table.
type JobModel struct {
	ID        string     `gorm:"primaryKey;type:text"`
	Name      string     `gorm:"type:text;not null;uniqueIndex"`
	Schedule  string     `gorm:"type:text;not null"`
	JobType   string     `gorm:"column:job_type;type:text;not null"`
	Payload   string     `gorm:"type:text;not null;default:''"`
	Status    string     `gorm:"type:text;not null;default:'active'"`
	LastRun   *time.Time
	LastError string    `gorm:"type:text"`
	NextRun   time.Time `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (JobModel) TableName() string { return "cron_jobs" }

func toModel(j *domain.Job) JobModel {
	return JobModel{
		ID:        j.ID,
		Name:      j.Name,
		Schedule:  j.Schedule,
		JobType:   j.JobType,
		Payload:   j.Payload,
		Status:    j.Status,
		LastRun:   j.LastRun,
		LastError: j.LastError,
		NextRun:   j.NextRun,
		CreatedAt: j.CreatedAt,
		UpdatedAt: j.UpdatedAt,
	}
}

func (m *JobModel) toDomain() *domain.Job {
	return &domain.Job{
		ID:        m.ID,
		Name:      m.Name,
		Schedule:  m.Schedule,
		JobType:   m.JobType,
		Payload:   m.Payload,
		Status:    m.Status,
		LastRun:   m.LastRun,
		LastError: m.LastError,
		NextRun:   m.NextRun,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}
