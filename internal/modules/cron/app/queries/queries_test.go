package queries_test

import (
	"context"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/vvs/internal/modules/cron/app/queries"
	"github.com/atvirokodosprendimai/vvs/internal/modules/cron/domain"
)

// ── stub repo ─────────────────────────────────────────────────────────────

type stubRepo struct {
	jobs []*domain.Job
}

func (r *stubRepo) Save(_ context.Context, j *domain.Job) error {
	for i, e := range r.jobs {
		if e.ID == j.ID {
			r.jobs[i] = j
			return nil
		}
	}
	r.jobs = append(r.jobs, j)
	return nil
}
func (r *stubRepo) FindByID(_ context.Context, id string) (*domain.Job, error) {
	for _, j := range r.jobs {
		if j.ID == id {
			return j, nil
		}
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
func (r *stubRepo) ListAll(_ context.Context) ([]*domain.Job, error)               { return r.jobs, nil }

// ── helpers ────────────────────────────────────────────────────────────────

func seed(t *testing.T, repo *stubRepo, id, name string) *domain.Job {
	t.Helper()
	j, err := domain.NewJob(id, name, "0 3 * * *", domain.TypeAction, "noop")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	_ = repo.Save(context.Background(), j)
	return j
}

// ── ListJobs ─────────────────────────────────────────────────────────────

func TestListJobs_ReturnsAll(t *testing.T) {
	repo := &stubRepo{}
	seed(t, repo, "id-1", "job-a")
	seed(t, repo, "id-2", "job-b")

	h := queries.NewListJobsHandler(repo)
	result, err := h.Handle(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("want 2 jobs, got %d", len(result))
	}
}

func TestListJobs_Empty(t *testing.T) {
	h := queries.NewListJobsHandler(&stubRepo{})
	result, err := h.Handle(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("want 0 jobs, got %d", len(result))
	}
}

func TestListJobs_FieldsPopulated(t *testing.T) {
	repo := &stubRepo{}
	seed(t, repo, "id-x", "check-fields")

	h := queries.NewListJobsHandler(repo)
	result, _ := h.Handle(context.Background())
	j := result[0]

	if j.ID != "id-x" {
		t.Errorf("want ID 'id-x', got %q", j.ID)
	}
	if j.Name != "check-fields" {
		t.Errorf("want name 'check-fields', got %q", j.Name)
	}
	if j.Schedule != "0 3 * * *" {
		t.Errorf("want schedule '0 3 * * *', got %q", j.Schedule)
	}
	if j.Status != domain.StatusActive {
		t.Errorf("want active, got %s", j.Status)
	}
	if j.NextRun.IsZero() {
		t.Error("want non-zero NextRun")
	}
}

// ── GetJob ────────────────────────────────────────────────────────────────

func TestGetJob_Found(t *testing.T) {
	repo := &stubRepo{}
	seed(t, repo, "id-get", "get-job")

	h := queries.NewGetJobHandler(repo)
	result, err := h.Handle(context.Background(), "id-get")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "id-get" {
		t.Fatalf("want 'id-get', got %q", result.ID)
	}
}

func TestGetJob_NotFound(t *testing.T) {
	h := queries.NewGetJobHandler(&stubRepo{})
	_, err := h.Handle(context.Background(), "no-such-id")
	if err != domain.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
