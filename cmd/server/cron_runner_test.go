package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/vvs/isp/internal/modules/cron/domain"
)

// ── stub repo ────────────────────────────────────────────────────────────

type testRepo struct {
	jobs map[string]*domain.Job
}

func newTestRepo() *testRepo { return &testRepo{jobs: make(map[string]*domain.Job)} }

func (r *testRepo) Save(_ context.Context, j *domain.Job) error {
	r.jobs[j.ID] = j
	return nil
}
func (r *testRepo) FindByID(_ context.Context, id string) (*domain.Job, error) {
	if j, ok := r.jobs[id]; ok {
		return j, nil
	}
	return nil, domain.ErrNotFound
}
func (r *testRepo) FindByName(_ context.Context, name string) (*domain.Job, error) {
	for _, j := range r.jobs {
		if j.Name == name {
			return j, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (r *testRepo) ListDue(_ context.Context, before time.Time) ([]*domain.Job, error) {
	var due []*domain.Job
	for _, j := range r.jobs {
		if j.Status == domain.StatusActive && !j.NextRun.After(before) {
			due = append(due, j)
		}
	}
	return due, nil
}
func (r *testRepo) ListAll(_ context.Context) ([]*domain.Job, error) {
	out := make([]*domain.Job, 0, len(r.jobs))
	for _, j := range r.jobs {
		out = append(out, j)
	}
	return out, nil
}

// ── runAction ────────────────────────────────────────────────────────────

func TestRunAction_KnownAction_OK(t *testing.T) {
	if err := runAction(context.Background(), "noop"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunAction_UnknownAction_Error(t *testing.T) {
	if err := runAction(context.Background(), "no-such-action"); err == nil {
		t.Fatal("want error for unknown action")
	}
}

func TestRegisterAction_Available(t *testing.T) {
	called := false
	RegisterAction("test-action", func(_ context.Context) error {
		called = true
		return nil
	})
	if err := runAction(context.Background(), "test-action"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("registered action was not called")
	}
	// cleanup
	delete(actions, "test-action")
}

// ── runURL ───────────────────────────────────────────────────────────────

func TestRunURL_GET_200_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	payload := fmt.Sprintf(`{"url":%q}`, srv.URL)
	if err := runURL(context.Background(), payload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunURL_Non2xx_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	payload := fmt.Sprintf(`{"url":%q}`, srv.URL)
	if err := runURL(context.Background(), payload); err == nil {
		t.Fatal("want error for 500 response")
	}
}

func TestRunURL_SetsHeaders(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	payload := fmt.Sprintf(`{"url":%q,"headers":{"Authorization":"Bearer secret"}}`, srv.URL)
	if err := runURL(context.Background(), payload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "Bearer secret" {
		t.Fatalf("want 'Bearer secret', got %q", gotAuth)
	}
}

func TestRunURL_POSTMethod(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	payload := fmt.Sprintf(`{"url":%q,"method":"POST"}`, srv.URL)
	if err := runURL(context.Background(), payload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("want POST, got %q", gotMethod)
	}
}

func TestRunURL_MissingURL_Error(t *testing.T) {
	if err := runURL(context.Background(), `{}`); err == nil {
		t.Fatal("want error for missing url")
	}
}

func TestRunURL_InvalidPayload_Error(t *testing.T) {
	if err := runURL(context.Background(), `not-json`); err == nil {
		t.Fatal("want error for invalid JSON payload")
	}
}

// ── RunDueJobs ───────────────────────────────────────────────────────────

func TestRunDueJobs_ExecutesDueJob(t *testing.T) {
	repo := newTestRepo()

	j, _ := domain.NewJob("id-due", "due-job", "* * * * *", domain.TypeAction, "noop")
	// force next_run into the past so it's due
	j.NextRun = time.Now().UTC().Add(-2 * time.Minute)
	_ = repo.Save(context.Background(), j)

	RunDueJobs(context.Background(), repo, nil, false)

	saved, _ := repo.FindByID(context.Background(), "id-due")
	if saved.LastRun == nil {
		t.Fatal("want LastRun set after RunDueJobs")
	}
	if saved.LastError != "" {
		t.Fatalf("want no error, got %q", saved.LastError)
	}
}

func TestRunDueJobs_SkipsPausedJob(t *testing.T) {
	repo := newTestRepo()

	j, _ := domain.NewJob("id-paused", "paused-job", "* * * * *", domain.TypeAction, "noop")
	j.NextRun = time.Now().UTC().Add(-2 * time.Minute)
	_ = j.Pause()
	_ = repo.Save(context.Background(), j)

	RunDueJobs(context.Background(), repo, nil, false)

	saved, _ := repo.FindByID(context.Background(), "id-paused")
	if saved.LastRun != nil {
		t.Fatal("paused job should not run")
	}
}

func TestRunDueJobs_AdvancesNextRun(t *testing.T) {
	repo := newTestRepo()

	j, _ := domain.NewJob("id-adv", "advance-job", "* * * * *", domain.TypeAction, "noop")
	j.NextRun = time.Now().UTC().Add(-2 * time.Minute)
	before := j.NextRun
	_ = repo.Save(context.Background(), j)

	RunDueJobs(context.Background(), repo, nil, false)

	saved, _ := repo.FindByID(context.Background(), "id-adv")
	if !saved.NextRun.After(before) {
		t.Fatalf("want NextRun to advance, before=%v after=%v", before, saved.NextRun)
	}
}

func TestRunDueJobs_RecordsJobError(t *testing.T) {
	repo := newTestRepo()

	RegisterAction("always-fail", func(_ context.Context) error {
		return fmt.Errorf("boom")
	})
	defer delete(actions, "always-fail")

	j, _ := domain.NewJob("id-fail", "fail-job", "* * * * *", domain.TypeAction, "always-fail")
	j.NextRun = time.Now().UTC().Add(-2 * time.Minute)
	_ = repo.Save(context.Background(), j)

	RunDueJobs(context.Background(), repo, nil, false)

	saved, _ := repo.FindByID(context.Background(), "id-fail")
	if saved.LastError != "boom" {
		t.Fatalf("want error 'boom', got %q", saved.LastError)
	}
}
