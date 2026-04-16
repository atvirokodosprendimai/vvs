package commands_test

import (
	"context"
	"testing"
	"time"

	"github.com/vvs/isp/internal/modules/cron/app/commands"
	"github.com/vvs/isp/internal/modules/cron/domain"
)

// ── stub repo ─────────────────────────────────────────────────────────────

type stubRepo struct {
	jobs map[string]*domain.Job
}

func newStubRepo() *stubRepo { return &stubRepo{jobs: make(map[string]*domain.Job)} }

func (r *stubRepo) Save(_ context.Context, j *domain.Job) error {
	r.jobs[j.ID] = j
	return nil
}
func (r *stubRepo) FindByID(_ context.Context, id string) (*domain.Job, error) {
	if j, ok := r.jobs[id]; ok {
		return j, nil
	}
	return nil, domain.ErrNotFound
}
func (r *stubRepo) FindByName(_ context.Context, name string) (*domain.Job, error) {
	for _, j := range r.jobs {
		if j.Name == name {
			return j, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (r *stubRepo) ListDue(_ context.Context, _ time.Time) ([]*domain.Job, error) { return nil, nil }
func (r *stubRepo) ListAll(_ context.Context) ([]*domain.Job, error) {
	out := make([]*domain.Job, 0, len(r.jobs))
	for _, j := range r.jobs {
		out = append(out, j)
	}
	return out, nil
}

// ── AddJob ────────────────────────────────────────────────────────────────

func TestAddJob_CreatesActiveJob(t *testing.T) {
	repo := newStubRepo()
	h := commands.NewAddJobHandler(repo)
	j, err := h.Handle(context.Background(), commands.AddJobCommand{
		Name: "my-job", Schedule: "0 3 * * *", JobType: domain.TypeAction, Payload: "noop",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if j.Status != domain.StatusActive {
		t.Fatalf("want active, got %s", j.Status)
	}
	if j.ID == "" {
		t.Fatal("want non-empty ID")
	}
	// persisted
	saved, _ := repo.FindByID(context.Background(), j.ID)
	if saved.Name != "my-job" {
		t.Fatalf("want saved name 'my-job', got %q", saved.Name)
	}
}

func TestAddJob_EmptyName_Error(t *testing.T) {
	h := commands.NewAddJobHandler(newStubRepo())
	_, err := h.Handle(context.Background(), commands.AddJobCommand{
		Name: "", Schedule: "* * * * *", JobType: domain.TypeAction, Payload: "noop",
	})
	if err != domain.ErrNameRequired {
		t.Fatalf("want ErrNameRequired, got %v", err)
	}
}

func TestAddJob_BadSchedule_Error(t *testing.T) {
	h := commands.NewAddJobHandler(newStubRepo())
	_, err := h.Handle(context.Background(), commands.AddJobCommand{
		Name: "x", Schedule: "not-a-cron", JobType: domain.TypeAction, Payload: "noop",
	})
	if err != domain.ErrInvalidSchedule {
		t.Fatalf("want ErrInvalidSchedule, got %v", err)
	}
}

// ── PauseJob ──────────────────────────────────────────────────────────────

func TestPauseJob_ActiveJob_Pauses(t *testing.T) {
	repo := newStubRepo()
	j := mustNewJob(t, repo, "p1")
	h := commands.NewPauseJobHandler(repo)
	if err := h.Handle(context.Background(), commands.PauseJobCommand{ID: j.ID}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	saved, _ := repo.FindByID(context.Background(), j.ID)
	if saved.Status != domain.StatusPaused {
		t.Fatalf("want paused, got %s", saved.Status)
	}
}

func TestPauseJob_NotFound_Error(t *testing.T) {
	h := commands.NewPauseJobHandler(newStubRepo())
	if err := h.Handle(context.Background(), commands.PauseJobCommand{ID: "ghost"}); err != domain.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestPauseJob_AlreadyPaused_Error(t *testing.T) {
	repo := newStubRepo()
	j := mustNewJob(t, repo, "p2")
	h := commands.NewPauseJobHandler(repo)
	_ = h.Handle(context.Background(), commands.PauseJobCommand{ID: j.ID})
	if err := h.Handle(context.Background(), commands.PauseJobCommand{ID: j.ID}); err != domain.ErrInvalidTransition {
		t.Fatalf("want ErrInvalidTransition, got %v", err)
	}
}

// ── ResumeJob ─────────────────────────────────────────────────────────────

func TestResumeJob_PausedJob_Resumes(t *testing.T) {
	repo := newStubRepo()
	j := mustNewJob(t, repo, "r1")
	_ = commands.NewPauseJobHandler(repo).Handle(context.Background(), commands.PauseJobCommand{ID: j.ID})
	h := commands.NewResumeJobHandler(repo)
	if err := h.Handle(context.Background(), commands.ResumeJobCommand{ID: j.ID}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	saved, _ := repo.FindByID(context.Background(), j.ID)
	if saved.Status != domain.StatusActive {
		t.Fatalf("want active, got %s", saved.Status)
	}
}

func TestResumeJob_ActiveJob_Error(t *testing.T) {
	repo := newStubRepo()
	j := mustNewJob(t, repo, "r2")
	h := commands.NewResumeJobHandler(repo)
	if err := h.Handle(context.Background(), commands.ResumeJobCommand{ID: j.ID}); err != domain.ErrInvalidTransition {
		t.Fatalf("want ErrInvalidTransition, got %v", err)
	}
}

// ── DeleteJob ─────────────────────────────────────────────────────────────

func TestDeleteJob_ActiveJob_SoftDeletes(t *testing.T) {
	repo := newStubRepo()
	j := mustNewJob(t, repo, "d1")
	h := commands.NewDeleteJobHandler(repo)
	if err := h.Handle(context.Background(), commands.DeleteJobCommand{ID: j.ID}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	saved, _ := repo.FindByID(context.Background(), j.ID)
	if saved.Status != domain.StatusDeleted {
		t.Fatalf("want deleted, got %s", saved.Status)
	}
}

func TestDeleteJob_Twice_Error(t *testing.T) {
	repo := newStubRepo()
	j := mustNewJob(t, repo, "d2")
	h := commands.NewDeleteJobHandler(repo)
	_ = h.Handle(context.Background(), commands.DeleteJobCommand{ID: j.ID})
	if err := h.Handle(context.Background(), commands.DeleteJobCommand{ID: j.ID}); err != domain.ErrInvalidTransition {
		t.Fatalf("want ErrInvalidTransition, got %v", err)
	}
}

// ── helpers ────────────────────────────────────────────────────────────────

func mustNewJob(t *testing.T, repo *stubRepo, name string) *domain.Job {
	t.Helper()
	j, err := domain.NewJob("id-"+name, name, "* * * * *", domain.TypeAction, "noop")
	if err != nil {
		t.Fatalf("mustNewJob: %v", err)
	}
	_ = repo.Save(context.Background(), j)
	return j
}
