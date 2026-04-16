package domain

import (
	"errors"
	"testing"
	"time"
)

func validTask(t *testing.T) *Task {
	t.Helper()
	task, err := NewTask("id-1", "cust-1", "Fix router", "Router is down", PriorityNormal, nil, "")
	if err != nil {
		t.Fatalf("unexpected error building valid task: %v", err)
	}
	return task
}

func TestNewTask(t *testing.T) {
	t.Run("valid creation", func(t *testing.T) {
		due := time.Now().Add(24 * time.Hour)
		task, err := NewTask("id-1", "cust-1", "Install cable", "Run new cable", PriorityHigh, &due, "user-1")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if task.ID != "id-1" {
			t.Errorf("ID: want %q, got %q", "id-1", task.ID)
		}
		if task.CustomerID != "cust-1" {
			t.Errorf("CustomerID: want %q, got %q", "cust-1", task.CustomerID)
		}
		if task.Title != "Install cable" {
			t.Errorf("Title: want %q, got %q", "Install cable", task.Title)
		}
		if task.Status != StatusTodo {
			t.Errorf("Status: want %q, got %q", StatusTodo, task.Status)
		}
		if task.Priority != PriorityHigh {
			t.Errorf("Priority: want %q, got %q", PriorityHigh, task.Priority)
		}
		if task.AssigneeID != "user-1" {
			t.Errorf("AssigneeID: want %q, got %q", "user-1", task.AssigneeID)
		}
	})

	t.Run("standalone task without customer", func(t *testing.T) {
		task, err := NewTask("id-1", "", "Standalone", "", PriorityLow, nil, "")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if task.CustomerID != "" {
			t.Errorf("CustomerID: want empty, got %q", task.CustomerID)
		}
	})

	t.Run("empty title returns ErrTitleRequired", func(t *testing.T) {
		_, err := NewTask("id-1", "cust-1", "", "desc", PriorityNormal, nil, "")
		if !errors.Is(err, ErrTitleRequired) {
			t.Errorf("expected ErrTitleRequired, got %v", err)
		}
	})

	t.Run("default priority normal", func(t *testing.T) {
		task, err := NewTask("id-1", "", "Task", "", "", nil, "")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if task.Priority != PriorityNormal {
			t.Errorf("Priority: want %q, got %q", PriorityNormal, task.Priority)
		}
	})
}

func TestStart(t *testing.T) {
	t.Run("todo to in_progress succeeds", func(t *testing.T) {
		task := validTask(t)
		if err := task.Start(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if task.Status != StatusInProgress {
			t.Errorf("Status: want %q, got %q", StatusInProgress, task.Status)
		}
	})

	t.Run("in_progress to in_progress returns ErrInvalidTransition", func(t *testing.T) {
		task := validTask(t)
		_ = task.Start()
		err := task.Start()
		if !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("expected ErrInvalidTransition, got %v", err)
		}
	})

	t.Run("done to in_progress returns ErrInvalidTransition", func(t *testing.T) {
		task := validTask(t)
		_ = task.Complete()
		err := task.Start()
		if !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("expected ErrInvalidTransition, got %v", err)
		}
	})

	t.Run("cancelled to in_progress returns ErrInvalidTransition", func(t *testing.T) {
		task := validTask(t)
		_ = task.Cancel()
		err := task.Start()
		if !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("expected ErrInvalidTransition, got %v", err)
		}
	})
}

func TestComplete(t *testing.T) {
	t.Run("todo to done succeeds", func(t *testing.T) {
		task := validTask(t)
		if err := task.Complete(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if task.Status != StatusDone {
			t.Errorf("Status: want %q, got %q", StatusDone, task.Status)
		}
	})

	t.Run("in_progress to done succeeds", func(t *testing.T) {
		task := validTask(t)
		_ = task.Start()
		if err := task.Complete(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if task.Status != StatusDone {
			t.Errorf("Status: want %q, got %q", StatusDone, task.Status)
		}
	})

	t.Run("done to done returns ErrAlreadyDone", func(t *testing.T) {
		task := validTask(t)
		_ = task.Complete()
		err := task.Complete()
		if !errors.Is(err, ErrAlreadyDone) {
			t.Errorf("expected ErrAlreadyDone, got %v", err)
		}
	})

	t.Run("cancelled to done returns ErrInvalidTransition", func(t *testing.T) {
		task := validTask(t)
		_ = task.Cancel()
		err := task.Complete()
		if !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("expected ErrInvalidTransition, got %v", err)
		}
	})
}

func TestCancel(t *testing.T) {
	t.Run("todo to cancelled succeeds", func(t *testing.T) {
		task := validTask(t)
		if err := task.Cancel(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if task.Status != StatusCancelled {
			t.Errorf("Status: want %q, got %q", StatusCancelled, task.Status)
		}
	})

	t.Run("in_progress to cancelled succeeds", func(t *testing.T) {
		task := validTask(t)
		_ = task.Start()
		if err := task.Cancel(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if task.Status != StatusCancelled {
			t.Errorf("Status: want %q, got %q", StatusCancelled, task.Status)
		}
	})

	t.Run("done to cancelled returns ErrInvalidTransition", func(t *testing.T) {
		task := validTask(t)
		_ = task.Complete()
		err := task.Cancel()
		if !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("expected ErrInvalidTransition, got %v", err)
		}
	})

	t.Run("cancelled to cancelled returns ErrInvalidTransition", func(t *testing.T) {
		task := validTask(t)
		_ = task.Cancel()
		err := task.Cancel()
		if !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("expected ErrInvalidTransition, got %v", err)
		}
	})
}

func TestReopen(t *testing.T) {
	t.Run("done to todo succeeds", func(t *testing.T) {
		task := validTask(t)
		_ = task.Complete()
		if err := task.Reopen(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if task.Status != StatusTodo {
			t.Errorf("Status: want %q, got %q", StatusTodo, task.Status)
		}
	})

	t.Run("cancelled to todo succeeds", func(t *testing.T) {
		task := validTask(t)
		_ = task.Cancel()
		if err := task.Reopen(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if task.Status != StatusTodo {
			t.Errorf("Status: want %q, got %q", StatusTodo, task.Status)
		}
	})

	t.Run("todo to todo returns ErrInvalidTransition", func(t *testing.T) {
		task := validTask(t)
		err := task.Reopen()
		if !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("expected ErrInvalidTransition, got %v", err)
		}
	})

	t.Run("in_progress to todo returns ErrInvalidTransition", func(t *testing.T) {
		task := validTask(t)
		_ = task.Start()
		err := task.Reopen()
		if !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("expected ErrInvalidTransition, got %v", err)
		}
	})
}
