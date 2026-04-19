package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	cronhttp "github.com/atvirokodosprendimai/vvs/internal/modules/cron/adapters/http"
	croncommands "github.com/atvirokodosprendimai/vvs/internal/modules/cron/app/commands"
	cronqueries "github.com/atvirokodosprendimai/vvs/internal/modules/cron/app/queries"
	"github.com/atvirokodosprendimai/vvs/internal/modules/cron/domain"
)

// ── stub repository ─────────────────────────────────────────────────────────

type stubRepo struct {
	jobs []*domain.Job
}

func (r *stubRepo) Save(_ context.Context, j *domain.Job) error {
	for i, existing := range r.jobs {
		if existing.ID == j.ID {
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

func (r *stubRepo) ListDue(_ context.Context, _ time.Time) ([]*domain.Job, error) {
	return nil, nil
}

func (r *stubRepo) ListAll(_ context.Context) ([]*domain.Job, error) {
	return r.jobs, nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func newHandlers(repo *stubRepo) *cronhttp.CronHandlers {
	return cronhttp.NewCronHandlers(
		cronqueries.NewListJobsHandler(repo),
		cronqueries.NewGetJobHandler(repo),
		croncommands.NewAddJobHandler(repo),
		croncommands.NewUpdateJobHandler(repo),
		croncommands.NewPauseJobHandler(repo),
		croncommands.NewResumeJobHandler(repo),
		croncommands.NewDeleteJobHandler(repo),
	)
}

func routerWith(h *cronhttp.CronHandlers) http.Handler {
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r
}

func seedJob(t *testing.T, repo *stubRepo, name, schedule, jobType, payload string) *domain.Job {
	t.Helper()
	j, err := domain.NewJob("job-"+name, name, schedule, jobType, payload)
	if err != nil {
		t.Fatalf("seedJob: %v", err)
	}
	_ = repo.Save(context.Background(), j)
	return j
}

// ── buildPayload tests ────────────────────────────────────────────────────────

func TestBuildPayload_Action(t *testing.T) {
	p, err := cronhttp.BuildPayload("action", "noop", "", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if p != "noop" {
		t.Fatalf("want 'noop', got %q", p)
	}
}

func TestBuildPayload_Shell(t *testing.T) {
	p, err := cronhttp.BuildPayload("shell", "", "echo hi", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if p != "echo hi" {
		t.Fatalf("want 'echo hi', got %q", p)
	}
}

func TestBuildPayload_RPC(t *testing.T) {
	p, err := cronhttp.BuildPayload("rpc", "", "", "isp.rpc.service.cancel", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p, `"isp.rpc.service.cancel"`) {
		t.Fatalf("want subject in payload, got %q", p)
	}
}

func TestBuildPayload_URL_GET(t *testing.T) {
	p, err := cronhttp.BuildPayload("url", "", "", "", "https://example.com/health", "GET", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p, `"url"`) || !strings.Contains(p, "example.com") {
		t.Fatalf("want url in payload, got %q", p)
	}
	// GET is default — should not appear in payload
	if strings.Contains(p, `"method"`) {
		t.Fatalf("GET method should be omitted, got %q", p)
	}
}

func TestBuildPayload_URL_POST_WithHeaders(t *testing.T) {
	headers := `{"Authorization":"Bearer token"}`
	p, err := cronhttp.BuildPayload("url", "", "", "", "https://api.example.com/hook", "POST", headers)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p, `"POST"`) {
		t.Fatalf("want POST in payload, got %q", p)
	}
	if !strings.Contains(p, "Authorization") {
		t.Fatalf("want headers in payload, got %q", p)
	}
}

func TestBuildPayload_URL_BadHeaders(t *testing.T) {
	// Invalid JSON for headers — should be ignored, not error
	p, err := cronhttp.BuildPayload("url", "", "", "", "https://example.com", "GET", "not-json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(p, "not-json") {
		t.Fatalf("bad headers JSON should be ignored, got %q", p)
	}
}

// ── handler tests ─────────────────────────────────────────────────────────────

func TestListPage_OK(t *testing.T) {
	repo := &stubRepo{}
	seedJob(t, repo, "test-job", "* * * * *", "action", "noop")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/cron", nil)
	routerWith(newHandlers(repo)).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "test-job") {
		t.Fatalf("want job name in HTML, got: %s", body[:min(200, len(body))])
	}
}

func TestListPage_Empty(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/cron", nil)
	routerWith(newHandlers(&stubRepo{})).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
}

func TestListSSE_ContentType(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sse/cron", nil)
	routerWith(newHandlers(&stubRepo{})).ServeHTTP(rr, req)

	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("want text/event-stream, got %q", ct)
	}
}

func TestAddSSE_CreatesJob(t *testing.T) {
	repo := &stubRepo{}
	h := newHandlers(repo)

	body := `{"name":"new-job","schedule":"0 3 * * *","jobType":"action","action":"noop"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/cron", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	routerWith(h).ServeHTTP(rr, req)

	if len(repo.jobs) != 1 {
		t.Fatalf("want 1 job in repo, got %d", len(repo.jobs))
	}
	if repo.jobs[0].Name != "new-job" {
		t.Fatalf("want name 'new-job', got %q", repo.jobs[0].Name)
	}
}

func TestPauseSSE_PausesJob(t *testing.T) {
	repo := &stubRepo{}
	j := seedJob(t, repo, "pause-me", "* * * * *", "action", "noop")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/cron/"+j.ID+"/pause", nil)
	routerWith(newHandlers(repo)).ServeHTTP(rr, req)

	saved, _ := repo.FindByID(context.Background(), j.ID)
	if saved.Status != domain.StatusPaused {
		t.Fatalf("want paused, got %s", saved.Status)
	}
}

func TestResumeSSE_ResumesJob(t *testing.T) {
	repo := &stubRepo{}
	j := seedJob(t, repo, "resume-me", "* * * * *", "action", "noop")
	_ = j.Pause()
	_ = repo.Save(context.Background(), j)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/cron/"+j.ID+"/resume", nil)
	routerWith(newHandlers(repo)).ServeHTTP(rr, req)

	saved, _ := repo.FindByID(context.Background(), j.ID)
	if saved.Status != domain.StatusActive {
		t.Fatalf("want active, got %s", saved.Status)
	}
}

func TestDeleteSSE_SoftDeletes(t *testing.T) {
	repo := &stubRepo{}
	j := seedJob(t, repo, "delete-me", "* * * * *", "action", "noop")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/cron/"+j.ID, nil)
	routerWith(newHandlers(repo)).ServeHTTP(rr, req)

	saved, _ := repo.FindByID(context.Background(), j.ID)
	if saved.Status != domain.StatusDeleted {
		t.Fatalf("want deleted, got %s", saved.Status)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
