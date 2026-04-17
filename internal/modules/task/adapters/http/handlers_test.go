package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	taskhttp "github.com/vvs/isp/internal/modules/task/adapters/http"
	taskcommands "github.com/vvs/isp/internal/modules/task/app/commands"
	taskqueries "github.com/vvs/isp/internal/modules/task/app/queries"
	"github.com/vvs/isp/internal/modules/task/domain"
	"github.com/vvs/isp/internal/shared/events"
)

// ── stubs ────────────────────────────────────────────────────────────────────

type taskNoopPub struct{}

func (n *taskNoopPub) Publish(_ context.Context, _ string, _ events.DomainEvent) error { return nil }

type taskNoopSub struct{}

func (n *taskNoopSub) Subscribe(_ string, _ events.EventHandler) error { return nil }
func (n *taskNoopSub) ChanSubscription(_ string) (<-chan events.DomainEvent, func()) {
	ch := make(chan events.DomainEvent)
	close(ch)
	return ch, func() {}
}
func (n *taskNoopSub) Close() error { return nil }

type taskStubRepo struct {
	tasks []*domain.Task
}

func (r *taskStubRepo) Save(_ context.Context, t *domain.Task) error {
	for i, existing := range r.tasks {
		if existing.ID == t.ID {
			r.tasks[i] = t
			return nil
		}
	}
	r.tasks = append(r.tasks, t)
	return nil
}

func (r *taskStubRepo) FindByID(_ context.Context, id string) (*domain.Task, error) {
	for _, t := range r.tasks {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *taskStubRepo) ListForCustomer(_ context.Context, customerID string) ([]*domain.Task, error) {
	var out []*domain.Task
	for _, t := range r.tasks {
		if t.CustomerID == customerID {
			out = append(out, t)
		}
	}
	return out, nil
}

func (r *taskStubRepo) ListAll(_ context.Context) ([]*domain.Task, error) {
	return r.tasks, nil
}

func (r *taskStubRepo) Delete(_ context.Context, id string) error {
	for i, t := range r.tasks {
		if t.ID == id {
			r.tasks = append(r.tasks[:i], r.tasks[i+1:]...)
			return nil
		}
	}
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func taskRouter(t *testing.T) (http.Handler, *taskStubRepo) {
	t.Helper()
	repo := &taskStubRepo{}
	pub := &taskNoopPub{}
	sub := &taskNoopSub{}
	h := taskhttp.NewHandlers(
		taskcommands.NewCreateTaskHandler(repo, pub),
		taskcommands.NewUpdateTaskHandler(repo, pub),
		taskcommands.NewDeleteTaskHandler(repo, pub),
		taskcommands.NewChangeTaskStatusHandler(repo, pub),
		taskqueries.NewListTasksForCustomerHandler(repo),
		taskqueries.NewListAllTasksHandler(repo),
		sub,
		pub,
	)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, repo
}

func createTaskViaHandler(t *testing.T, router http.Handler, title string) {
	t.Helper()
	body := `{"taskTitle":"` + title + `","taskPriority":"normal"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/tasks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rr, req)
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestTaskListPage_OK(t *testing.T) {
	router, _ := taskRouter(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
}

func TestTaskListAllSSE_ContentType(t *testing.T) {
	router, _ := taskRouter(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	router.ServeHTTP(rr, req)
	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("want text/event-stream, got %q", ct)
	}
}

func TestTaskListForCustomerSSE_ContentType(t *testing.T) {
	router, _ := taskRouter(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sse/customers/cust-1/tasks", nil)
	router.ServeHTTP(rr, req)
	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("want text/event-stream, got %q", ct)
	}
}

func TestTaskCreateSSE_CreatesTask(t *testing.T) {
	router, repo := taskRouter(t)
	createTaskViaHandler(t, router, "Install router")

	if len(repo.tasks) != 1 {
		t.Fatalf("want 1 task, got %d", len(repo.tasks))
	}
	if repo.tasks[0].Title != "Install router" {
		t.Fatalf("want title 'Install router', got %q", repo.tasks[0].Title)
	}
}

func TestTaskCreateSSE_EmptyTitle_NoCreate(t *testing.T) {
	router, repo := taskRouter(t)
	body := `{"taskTitle":"","taskPriority":"normal"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/tasks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rr, req)

	if len(repo.tasks) != 0 {
		t.Fatalf("want 0 tasks on empty title, got %d", len(repo.tasks))
	}
}

func TestTaskUpdateSSE_UpdatesTask(t *testing.T) {
	router, repo := taskRouter(t)
	createTaskViaHandler(t, router, "Old Title")

	id := repo.tasks[0].ID
	updateBody := `{"taskTitle":"New Title","taskPriority":"high"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/tasks/"+id, strings.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rr, req)

	task, _ := repo.FindByID(context.Background(), id)
	if task.Title != "New Title" {
		t.Fatalf("want 'New Title', got %q", task.Title)
	}
}

func TestTaskChangeStatusSSE_StartsTask(t *testing.T) {
	router, repo := taskRouter(t)
	createTaskViaHandler(t, router, "Work task")

	id := repo.tasks[0].ID
	statusBody := `{"taskAction":"start"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/tasks/"+id+"/status", strings.NewReader(statusBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	task, _ := repo.FindByID(context.Background(), id)
	if task.Status != domain.StatusInProgress {
		t.Fatalf("want in_progress, got %q", task.Status)
	}
}

func TestTaskDeleteSSE_DeletesTask(t *testing.T) {
	router, repo := taskRouter(t)
	createTaskViaHandler(t, router, "Temp task")

	id := repo.tasks[0].ID
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/tasks/"+id, nil)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	if len(repo.tasks) != 0 {
		t.Fatalf("want 0 tasks after delete, got %d", len(repo.tasks))
	}
}
