package domain_test

import (
	"testing"
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/modules/cron/domain"
)

func newTestJob(t *testing.T) *domain.Job {
	t.Helper()
	j, err := domain.NewJob("id-1", "test-job", "* * * * *", domain.TypeAction, "noop")
	if err != nil {
		t.Fatalf("NewJob: %v", err)
	}
	return j
}

func TestNewJob_RequiresName(t *testing.T) {
	_, err := domain.NewJob("id-1", "", "* * * * *", domain.TypeAction, "noop")
	if err != domain.ErrNameRequired {
		t.Fatalf("want ErrNameRequired, got %v", err)
	}
}

func TestNewJob_InvalidSchedule(t *testing.T) {
	_, err := domain.NewJob("id-1", "bad", "not-a-cron", domain.TypeAction, "noop")
	if err != domain.ErrInvalidSchedule {
		t.Fatalf("want ErrInvalidSchedule, got %v", err)
	}
}

func TestNewJob_DefaultsToActive(t *testing.T) {
	j := newTestJob(t)
	if j.Status != domain.StatusActive {
		t.Fatalf("want active, got %s", j.Status)
	}
}

func TestNewJob_NextRunInFuture(t *testing.T) {
	j := newTestJob(t)
	if !j.NextRun.After(time.Now()) {
		t.Fatalf("NextRun should be in the future, got %v", j.NextRun)
	}
}

func TestPause_FromActive(t *testing.T) {
	j := newTestJob(t)
	if err := j.Pause(); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	if j.Status != domain.StatusPaused {
		t.Fatalf("want paused, got %s", j.Status)
	}
}

func TestPause_AlreadyPaused_Fails(t *testing.T) {
	j := newTestJob(t)
	_ = j.Pause()
	if err := j.Pause(); err != domain.ErrInvalidTransition {
		t.Fatalf("want ErrInvalidTransition, got %v", err)
	}
}

func TestResume_FromPaused(t *testing.T) {
	j := newTestJob(t)
	_ = j.Pause()
	if err := j.Resume(); err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if j.Status != domain.StatusActive {
		t.Fatalf("want active, got %s", j.Status)
	}
}

func TestResume_FromActive_Fails(t *testing.T) {
	j := newTestJob(t)
	if err := j.Resume(); err != domain.ErrInvalidTransition {
		t.Fatalf("want ErrInvalidTransition, got %v", err)
	}
}

func TestDelete_FromActive(t *testing.T) {
	j := newTestJob(t)
	if err := j.Delete(); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if j.Status != domain.StatusDeleted {
		t.Fatalf("want deleted, got %s", j.Status)
	}
}

func TestDelete_Twice_Fails(t *testing.T) {
	j := newTestJob(t)
	_ = j.Delete()
	if err := j.Delete(); err != domain.ErrInvalidTransition {
		t.Fatalf("want ErrInvalidTransition, got %v", err)
	}
}

func TestAdvanceNextRun(t *testing.T) {
	j := newTestJob(t)
	before := j.NextRun
	ran := time.Now().UTC()
	j.AdvanceNextRun(ran, "")
	if j.LastRun == nil || !j.LastRun.Equal(ran) {
		t.Fatalf("LastRun not set correctly")
	}
	if j.LastError != "" {
		t.Fatalf("LastError should be empty")
	}
	if !j.NextRun.After(before) {
		t.Fatalf("NextRun should advance after AdvanceNextRun")
	}
}

func TestAdvanceNextRun_WithError(t *testing.T) {
	j := newTestJob(t)
	j.AdvanceNextRun(time.Now().UTC(), "something failed")
	if j.LastError != "something failed" {
		t.Fatalf("want error string, got %q", j.LastError)
	}
}

func TestNextTime_ValidSchedule(t *testing.T) {
	next, err := domain.NextTime("0 3 * * *", time.Now().UTC())
	if err != nil {
		t.Fatalf("NextTime: %v", err)
	}
	if next.IsZero() {
		t.Fatal("NextTime returned zero time")
	}
}

func TestNextTime_InvalidSchedule(t *testing.T) {
	_, err := domain.NextTime("bad schedule", time.Now().UTC())
	if err != domain.ErrInvalidSchedule {
		t.Fatalf("want ErrInvalidSchedule, got %v", err)
	}
}
