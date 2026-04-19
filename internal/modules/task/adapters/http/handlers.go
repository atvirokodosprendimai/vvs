package http

import (
	"errors"
	"log"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"
	"github.com/atvirokodosprendimai/vvs/internal/modules/task/app/commands"
	"github.com/atvirokodosprendimai/vvs/internal/modules/task/app/queries"
	"github.com/atvirokodosprendimai/vvs/internal/modules/task/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
	authdomain "github.com/atvirokodosprendimai/vvs/internal/modules/auth/domain"
)

// Handlers holds all dependencies for task HTTP handlers.
type Handlers struct {
	createCmd      *commands.CreateTaskHandler
	updateCmd      *commands.UpdateTaskHandler
	deleteCmd      *commands.DeleteTaskHandler
	changeStatusCmd *commands.ChangeTaskStatusHandler
	listForCustomer *queries.ListTasksForCustomerHandler
	listAll         *queries.ListAllTasksHandler
	subscriber      events.EventSubscriber
	publisher       events.EventPublisher
}

func NewHandlers(
	createCmd *commands.CreateTaskHandler,
	updateCmd *commands.UpdateTaskHandler,
	deleteCmd *commands.DeleteTaskHandler,
	changeStatusCmd *commands.ChangeTaskStatusHandler,
	listForCustomer *queries.ListTasksForCustomerHandler,
	listAll *queries.ListAllTasksHandler,
	subscriber events.EventSubscriber,
	publisher events.EventPublisher,
) *Handlers {
	return &Handlers{
		createCmd:       createCmd,
		updateCmd:       updateCmd,
		deleteCmd:       deleteCmd,
		changeStatusCmd: changeStatusCmd,
		listForCustomer: listForCustomer,
		listAll:         listAll,
		subscriber:      subscriber,
		publisher:       publisher,
	}
}

func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.Get("/tasks", h.listPage)
	r.Get("/api/tasks", h.listAllSSE)
	r.Get("/sse/customers/{id}/tasks", h.listForCustomerSSE)
	r.Post("/api/tasks", h.createSSE)
	r.Post("/api/customers/{id}/tasks", h.createForCustomerSSE)
	r.Put("/api/tasks/{taskID}", h.updateSSE)
	r.Put("/api/tasks/{taskID}/status", h.changeStatusSSE)
	r.Delete("/api/tasks/{taskID}", h.deleteSSE)
}

// listPage renders the standalone tasks page.
func (h *Handlers) listPage(w http.ResponseWriter, r *http.Request) {
	TasksPage().Render(r.Context(), w)
}

// listAllSSE streams all tasks via SSE, filtered by taskStatusFilter and taskSearch signals.
func (h *Handlers) listAllSSE(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		StatusFilter string `json:"taskStatusFilter"`
		Search       string `json:"taskSearch"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("task listAllSSE: ReadSignals: %v", err)
	}
	if signals.StatusFilter == "" {
		signals.StatusFilter = "active"
	}

	sse := datastar.NewSSE(w, r)

	ch, cancel := h.subscriber.ChanSubscription(events.TaskAll.String())
	defer cancel()

	all, err := h.listAll.Handle(r.Context())
	if err != nil {
		log.Printf("task handler: listAllSSE: %v", err)
		return
	}
	current := filterTasks(all, signals.StatusFilter, signals.Search)
	sse.PatchElementTempl(TasksTable(current))

	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
			all, err := h.listAll.Handle(r.Context())
			if err != nil {
				log.Printf("task handler: listAllSSE refresh: %v", err)
				continue
			}
			next := filterTasks(all, signals.StatusFilter, signals.Search)
			if !reflect.DeepEqual(current, next) {
				sse.PatchElementTempl(TasksTable(next))
				current = next
			}
		case <-r.Context().Done():
			return
		}
	}
}

func filterTasks(tasks []queries.TaskReadModel, statusFilter, search string) []queries.TaskReadModel {
	search = strings.ToLower(strings.TrimSpace(search))
	var out []queries.TaskReadModel
	for _, t := range tasks {
		terminal := t.Status == domain.StatusDone || t.Status == domain.StatusCancelled
		switch statusFilter {
		case "done":
			if !terminal {
				continue
			}
		default: // "active"
			if terminal {
				continue
			}
		}
		if search != "" {
			if !strings.Contains(strings.ToLower(t.Title), search) &&
				!strings.Contains(strings.ToLower(t.CustomerID), search) {
				continue
			}
		}
		out = append(out, t)
	}
	return out
}

// listForCustomerSSE streams tasks for a customer via SSE.
func (h *Handlers) listForCustomerSSE(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "id")
	if customerID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)

	ch, cancel := h.subscriber.ChanSubscription(events.TaskAll.String())
	defer cancel()

	q := queries.ListTasksForCustomerQuery{CustomerID: customerID}

	current, err := h.listForCustomer.Handle(r.Context(), q)
	if err != nil {
		log.Printf("task handler: listForCustomerSSE: %v", err)
		return
	}
	sse.PatchElementTempl(TaskList(customerID, current))

	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
			next, err := h.listForCustomer.Handle(r.Context(), q)
			if err != nil {
				log.Printf("task handler: listForCustomerSSE refresh: %v", err)
				continue
			}
			if !reflect.DeepEqual(current, next) {
				sse.PatchElementTempl(TaskList(customerID, next))
				current = next
			}
		case <-r.Context().Done():
			return
		}
	}
}

// createSSE creates a standalone task (no customer).
func (h *Handlers) createSSE(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		TaskTitle       string `json:"taskTitle"`
		TaskDescription string `json:"taskDescription"`
		TaskPriority    string `json:"taskPriority"`
		TaskDueDate     string `json:"taskDueDate"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("task handler: createSSE ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)

	dueDate := parseDueDate(signals.TaskDueDate)

	_, err := h.createCmd.Handle(r.Context(), commands.CreateTaskCommand{
		Title:       signals.TaskTitle,
		Description: signals.TaskDescription,
		Priority:    signals.TaskPriority,
		DueDate:     dueDate,
	})
	if err != nil {
		log.Printf("task handler: createSSE Handle: %v", err)
		if errors.Is(err, domain.ErrTitleRequired) {
			sse.PatchElementTempl(taskFormError("tasks-error", "Title is required"))
			return
		}
		sse.PatchElementTempl(taskFormError("tasks-error", "internal server error"))
		return
	}

	sse.Redirect("/tasks")
}

// createForCustomerSSE creates a task for a specific customer.
func (h *Handlers) createForCustomerSSE(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "id")
	if customerID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var signals struct {
		TaskTitle       string `json:"taskTitle"`
		TaskDescription string `json:"taskDescription"`
		TaskPriority    string `json:"taskPriority"`
		TaskDueDate     string `json:"taskDueDate"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("task handler: createForCustomerSSE ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)

	dueDate := parseDueDate(signals.TaskDueDate)

	_, err := h.createCmd.Handle(r.Context(), commands.CreateTaskCommand{
		CustomerID:  customerID,
		Title:       signals.TaskTitle,
		Description: signals.TaskDescription,
		Priority:    signals.TaskPriority,
		DueDate:     dueDate,
	})
	if err != nil {
		log.Printf("task handler: createForCustomerSSE Handle: %v", err)
		if errors.Is(err, domain.ErrTitleRequired) {
			sse.PatchElementTempl(taskFormError("customer-task-error", "Title is required"))
			return
		}
		sse.PatchElementTempl(taskFormError("customer-task-error", "internal server error"))
		return
	}

	// Close the modal and rely on SSE subscription to refresh the list.
	sse.PatchSignals([]byte(`{"_taskModalOpen":false}`))
}

// updateSSE updates an existing task.
func (h *Handlers) updateSSE(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	if taskID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var signals struct {
		TaskTitle       string `json:"taskTitle"`
		TaskDescription string `json:"taskDescription"`
		TaskPriority    string `json:"taskPriority"`
		TaskDueDate     string `json:"taskDueDate"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("task handler: updateSSE ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sse := datastar.NewSSE(w, r)

	dueDate := parseDueDate(signals.TaskDueDate)

	_, err := h.updateCmd.Handle(r.Context(), commands.UpdateTaskCommand{
		ID:          taskID,
		Title:       signals.TaskTitle,
		Description: signals.TaskDescription,
		Priority:    signals.TaskPriority,
		DueDate:     dueDate,
	})
	if err != nil {
		log.Printf("task handler: updateSSE Handle: %v", err)
		if errors.Is(err, domain.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, domain.ErrTitleRequired) {
			sse.PatchElementTempl(taskFormError("customer-task-error", "Title is required"))
			return
		}
		sse.PatchElementTempl(taskFormError("customer-task-error", "internal server error"))
		return
	}

	sse.PatchSignals([]byte(`{"_taskModalOpen":false}`))
}

// changeStatusSSE transitions the status of a task.
func (h *Handlers) changeStatusSSE(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	if taskID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var signals struct {
		TaskAction string `json:"taskAction"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("task handler: changeStatusSSE ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if _, err := h.changeStatusCmd.Handle(r.Context(), commands.ChangeTaskStatusCommand{
		ID:     taskID,
		Action: signals.TaskAction,
	}); err != nil {
		log.Printf("task handler: changeStatusSSE Handle: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// deleteSSE deletes a task.
func (h *Handlers) deleteSSE(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	if taskID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if err := h.deleteCmd.Handle(r.Context(), commands.DeleteTaskCommand{ID: taskID}); err != nil {
		log.Printf("task handler: deleteSSE Handle: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// parseDueDate parses a YYYY-MM-DD string into a *time.Time. Returns nil on empty or error.
func parseDueDate(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil
	}
	return &t
}

func (h *Handlers) ModuleName() authdomain.Module { return authdomain.ModuleTasks }
