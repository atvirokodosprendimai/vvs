package http

import (
	"encoding/json"
	"html"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"

	croncommands "github.com/vvs/isp/internal/modules/cron/app/commands"
	cronqueries "github.com/vvs/isp/internal/modules/cron/app/queries"
	"github.com/vvs/isp/internal/modules/cron/domain"
)

type CronHandlers struct {
	listQuery *cronqueries.ListJobsHandler
	addCmd    *croncommands.AddJobHandler
	pauseCmd  *croncommands.PauseJobHandler
	resumeCmd *croncommands.ResumeJobHandler
	deleteCmd *croncommands.DeleteJobHandler
}

func NewCronHandlers(
	listQuery *cronqueries.ListJobsHandler,
	addCmd *croncommands.AddJobHandler,
	pauseCmd *croncommands.PauseJobHandler,
	resumeCmd *croncommands.ResumeJobHandler,
	deleteCmd *croncommands.DeleteJobHandler,
) *CronHandlers {
	return &CronHandlers{
		listQuery: listQuery,
		addCmd:    addCmd,
		pauseCmd:  pauseCmd,
		resumeCmd: resumeCmd,
		deleteCmd: deleteCmd,
	}
}

func (h *CronHandlers) RegisterRoutes(r chi.Router) {
	r.Get("/cron", h.listPage)
	r.Get("/sse/cron", h.listSSE)
	r.Post("/api/cron", h.addSSE)
	r.Post("/api/cron/{id}/pause", h.pauseSSE)
	r.Post("/api/cron/{id}/resume", h.resumeSSE)
	r.Delete("/api/cron/{id}", h.deleteSSE)
}

// ── Pages ──────────────────────────────────────────────────────────────────

func (h *CronHandlers) listPage(w http.ResponseWriter, r *http.Request) {
	jobs, err := h.listQuery.Handle(r.Context())
	if err != nil {
		log.Printf("cron: listPage: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	CronListPage(jobs).Render(r.Context(), w)
}

// ── SSE ────────────────────────────────────────────────────────────────────

func (h *CronHandlers) listSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	jobs, err := h.listQuery.Handle(r.Context())
	if err != nil {
		log.Printf("cron: listSSE: %v", err)
		return
	}
	sse.PatchElementTempl(CronTable(jobs))
}

func (h *CronHandlers) pushTable(sse *datastar.ServerSentEventGenerator, r *http.Request) {
	jobs, err := h.listQuery.Handle(r.Context())
	if err != nil {
		log.Printf("cron: pushTable: %v", err)
		return
	}
	sse.PatchElementTempl(CronTable(jobs))
}

// ── SSE mutations ──────────────────────────────────────────────────────────

func (h *CronHandlers) addSSE(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		Name     string `json:"name"`
		Schedule string `json:"schedule"`
		JobType  string `json:"jobType"`
		// type-specific payload fields
		Action  string `json:"action"`
		Command string `json:"command"`
		Subject string `json:"subject"`
		URL     string `json:"url"`
		Method  string `json:"method"`
		Headers string `json:"headers"` // JSON: {"Key":"Value",...}
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		log.Printf("cron: addSSE: ReadSignals: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sse := datastar.NewSSE(w, r)

	payload, err := buildPayload(signals.JobType, signals.Action, signals.Command, signals.Subject, signals.URL, signals.Method, signals.Headers)
	if err != nil {
		sse.PatchElements(`<div id="cron-form-errors" class="text-red-400 text-xs mt-1">invalid payload</div>`)
		return
	}

	if _, err := h.addCmd.Handle(r.Context(), croncommands.AddJobCommand{
		Name:     signals.Name,
		Schedule: signals.Schedule,
		JobType:  signals.JobType,
		Payload:  payload,
	}); err != nil {
		sse.PatchElements(`<div id="cron-form-errors" class="text-red-400 text-xs mt-1">` + html.EscapeString(err.Error()) + `</div>`)
		return
	}

	// Clear error div + close modal via signal patch
	sse.PatchElements(`<div id="cron-form-errors"></div>`)
	sse.PatchSignals([]byte(`{"_addOpen":false}`))
	h.pushTable(sse, r)
}

func (h *CronHandlers) pauseSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	id := chi.URLParam(r, "id")
	if err := h.pauseCmd.Handle(r.Context(), croncommands.PauseJobCommand{ID: id}); err != nil {
		log.Printf("cron: pause %s: %v", id, err)
		return
	}
	h.pushTable(sse, r)
}

func (h *CronHandlers) resumeSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	id := chi.URLParam(r, "id")
	if err := h.resumeCmd.Handle(r.Context(), croncommands.ResumeJobCommand{ID: id}); err != nil {
		log.Printf("cron: resume %s: %v", id, err)
		return
	}
	h.pushTable(sse, r)
}

func (h *CronHandlers) deleteSSE(w http.ResponseWriter, r *http.Request) {
	sse := datastar.NewSSE(w, r)
	id := chi.URLParam(r, "id")
	if err := h.deleteCmd.Handle(r.Context(), croncommands.DeleteJobCommand{ID: id}); err != nil {
		log.Printf("cron: delete %s: %v", id, err)
		return
	}
	h.pushTable(sse, r)
}

// ── helpers ────────────────────────────────────────────────────────────────

// BuildPayload is exported for testing.
func BuildPayload(jobType, action, command, subject, rawURL, method, headersJSON string) (string, error) {
	return buildPayload(jobType, action, command, subject, rawURL, method, headersJSON)
}

func buildPayload(jobType, action, command, subject, rawURL, method, headersJSON string) (string, error) {
	switch jobType {
	case domain.TypeAction:
		return action, nil
	case domain.TypeShell:
		return command, nil
	case domain.TypeRPC:
		return `{"subject":` + jsonString(subject) + `,"body":{}}`, nil
	case domain.TypeURL:
		p := map[string]any{"url": rawURL}
		if method != "" && method != "GET" {
			p["method"] = method
		}
		if headersJSON != "" && headersJSON != "{}" {
			var hmap map[string]string
			if err := json.Unmarshal([]byte(headersJSON), &hmap); err == nil && len(hmap) > 0 {
				p["headers"] = hmap
			}
		}
		b, _ := json.Marshal(p)
		return string(b), nil
	default:
		return "", nil
	}
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
