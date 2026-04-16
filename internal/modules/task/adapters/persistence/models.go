package persistence

import (
	"time"

	"github.com/vvs/isp/internal/modules/task/domain"
)

// TaskModel is the GORM model mapping to the tasks table.
type TaskModel struct {
	ID          string     `gorm:"primaryKey;type:text"`
	CustomerID  string     `gorm:"type:text;not null;default:''"`
	Title       string     `gorm:"type:text;not null"`
	Description string     `gorm:"type:text;not null;default:''"`
	Status      string     `gorm:"type:text;not null;default:'todo'"`
	Priority    string     `gorm:"type:text;not null;default:'normal'"`
	DueDate     *time.Time `gorm:"column:due_date"`
	AssigneeID  string     `gorm:"type:text;not null;default:''"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (TaskModel) TableName() string { return "tasks" }

func toModel(t *domain.Task) TaskModel {
	return TaskModel{
		ID:          t.ID,
		CustomerID:  t.CustomerID,
		Title:       t.Title,
		Description: t.Description,
		Status:      t.Status,
		Priority:    t.Priority,
		DueDate:     t.DueDate,
		AssigneeID:  t.AssigneeID,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

func (m *TaskModel) toDomain() *domain.Task {
	return &domain.Task{
		ID:          m.ID,
		CustomerID:  m.CustomerID,
		Title:       m.Title,
		Description: m.Description,
		Status:      m.Status,
		Priority:    m.Priority,
		DueDate:     m.DueDate,
		AssigneeID:  m.AssigneeID,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}
