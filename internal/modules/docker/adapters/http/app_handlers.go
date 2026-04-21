package http

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/starfederation/datastar-go/datastar"

	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/app/commands"
	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/app/queries"
	"github.com/atvirokodosprendimai/vvs/internal/modules/docker/domain"
	"github.com/atvirokodosprendimai/vvs/internal/shared/events"
)

// AppHandlers manages DockerApp (git-source deploy) endpoints.
type AppHandlers struct {
	buildApp   *commands.BuildDockerAppHandler
	stopApp    *commands.StopDockerAppHandler
	removeApp  *commands.RemoveDockerAppHandler
	listApps   *queries.ListDockerAppsHandler
	getApp     *queries.GetDockerAppHandler
	appRepo    domain.DockerAppRepository
	subscriber events.EventSubscriber
}

func NewAppHandlers(
	buildApp *commands.BuildDockerAppHandler,
	stopApp *commands.StopDockerAppHandler,
	removeApp *commands.RemoveDockerAppHandler,
	listApps *queries.ListDockerAppsHandler,
	getApp *queries.GetDockerAppHandler,
	appRepo domain.DockerAppRepository,
	subscriber events.EventSubscriber,
) *AppHandlers {
	return &AppHandlers{
		buildApp:   buildApp,
		stopApp:    stopApp,
		removeApp:  removeApp,
		listApps:   listApps,
		getApp:     getApp,
		appRepo:    appRepo,
		subscriber: subscriber,
	}
}

func (h *AppHandlers) RegisterRoutes(r chi.Router) {
	r.Get("/docker/apps", h.appsPage)
	r.Get("/docker/apps/new", h.newAppForm)
	r.Post("/docker/apps", h.createApp)
	r.Get("/docker/apps/{id}/edit", h.editAppForm)
	r.Post("/docker/apps/{id}", h.updateApp)
	r.Delete("/docker/apps/{id}", h.deleteApp)

	// SSE
	r.Get("/api/docker/apps", h.listAppsSSE)
	r.Post("/api/docker/apps/{id}/build", h.buildAppSSE)
	r.Post("/api/docker/apps/{id}/stop", h.stopAppSSE)
	r.Get("/api/docker/apps/{id}/logs", h.buildLogsSSE)
	r.Post("/api/docker/apps/{id}/webhook", h.webhookTrigger)
	r.Post("/api/docker/apps/{id}/register-webhook", h.registerWebhook)
}

func (h *AppHandlers) ModuleName() string { return "docker" }

// ── Pages ─────────────────────────────────────────────────────────────────────

func (h *AppHandlers) appsPage(w http.ResponseWriter, r *http.Request) {
	apps, _ := h.listApps.Handle(r.Context())
	AppListPage(apps).Render(r.Context(), w)
}

func (h *AppHandlers) newAppForm(w http.ResponseWriter, r *http.Request) {
	AppFormPage(queries.AppReadModel{Branch: "main", RestartPolicy: "unless-stopped"}, false).Render(r.Context(), w)
}

func (h *AppHandlers) editAppForm(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	app, err := h.getApp.Handle(r.Context(), id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	AppFormPage(app, true).Render(r.Context(), w)
}

// ── CRUD ──────────────────────────────────────────────────────────────────────

func (h *AppHandlers) createApp(w http.ResponseWriter, r *http.Request) {
	app, err := h.appFromForm(r, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.appRepo.Save(r.Context(), app); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/docker/apps", http.StatusSeeOther)
}

func (h *AppHandlers) updateApp(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	existing, err := h.appRepo.FindByID(r.Context(), id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	patch, err := h.appFromForm(r, existing.RegPass)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	existing.Update(
		patch.Name, patch.RepoURL, patch.Branch,
		patch.RegUser, patch.RegPass,
		patch.BuildArgs, patch.EnvVars,
		patch.Ports, patch.Volumes,
		patch.Networks, patch.RestartPolicy,
	)
	if err := h.appRepo.Save(r.Context(), existing); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/docker/apps", http.StatusSeeOther)
}

func (h *AppHandlers) deleteApp(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)
	if err := h.removeApp.Handle(r.Context(), commands.RemoveDockerAppCommand{AppID: id}); err != nil {
		slog.Error("docker: remove app", "err", err)
		sse.PatchElementTempl(AppInlineError("app-error-"+id, err.Error()))
		return
	}
	apps, _ := h.listApps.Handle(r.Context())
	sse.PatchElementTempl(AppTable(apps))
}

// ── SSE ───────────────────────────────────────────────────────────────────────

func (h *AppHandlers) listAppsSSE(w http.ResponseWriter, r *http.Request) {
	apps, err := h.listApps.Handle(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(AppTable(apps))
}

func (h *AppHandlers) buildAppSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)
	if err := h.buildApp.Handle(r.Context(), commands.BuildDockerAppCommand{AppID: id}); err != nil {
		slog.Error("docker: build app", "err", err)
		sse.PatchElementTempl(AppInlineError("app-error-"+id, err.Error()))
		return
	}
	apps, _ := h.listApps.Handle(r.Context())
	sse.PatchElementTempl(AppTable(apps))
}

func (h *AppHandlers) stopAppSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)
	if err := h.stopApp.Handle(r.Context(), commands.StopDockerAppCommand{AppID: id}); err != nil {
		slog.Error("docker: stop app", "err", err)
		sse.PatchElementTempl(AppInlineError("app-error-"+id, err.Error()))
		return
	}
	apps, _ := h.listApps.Handle(r.Context())
	sse.PatchElementTempl(AppTable(apps))
}

// buildLogsSSE streams build log lines from NATS for one app.
// Stays open until the context cancels (client disconnects).
func (h *AppHandlers) buildLogsSSE(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	subj := events.DockerAppBuildLog.Format(id)

	sse := datastar.NewSSE(w, r)
	ch, cancel := h.subscriber.ChanSubscription(subj)
	defer cancel()

	// Also subscribe to status changes so we can detect build completion
	statusCh, cancelStatus := h.subscriber.ChanSubscription(events.DockerAppStatusChanged.String())
	defer cancelStatus()

	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				return
			}
			line := string(evt.Data)
			sse.PatchElementTempl(AppBuildLogLine(line))
		case evt, ok := <-statusCh:
			if !ok {
				return
			}
			if evt.AggregateID != id {
				continue
			}
			app, err := h.getApp.Handle(r.Context(), id)
			if err != nil {
				continue
			}
			sse.PatchElementTempl(AppStatusBadge(id, app.Status))
			// Close stream when build is done (terminal states)
			switch app.Status {
			case "running", "error", "stopped":
				return
			}
		case <-r.Context().Done():
			return
		}
	}
}

// webhookTrigger is called by Gitea on push. Triggers build pipeline.
func (h *AppHandlers) webhookTrigger(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.buildApp.Handle(r.Context(), commands.BuildDockerAppCommand{AppID: id}); err != nil {
		slog.Error("docker: webhook build trigger", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// registerWebhook calls the Gitea API to create a webhook on the app's repo.
func (h *AppHandlers) registerWebhook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sse := datastar.NewSSE(w, r)

	app, err := h.appRepo.FindByID(r.Context(), id)
	if err != nil {
		sse.PatchElementTempl(AppInlineError("webhook-error-"+id, "app not found"))
		return
	}

	// Derive webhook URL from request
	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	webhookURL := fmt.Sprintf("%s://%s/api/docker/apps/%s/webhook", scheme, r.Host, id)

	// Parse owner/repo from repoURL
	owner, repo, err := parseOwnerRepo(app.RepoURL)
	if err != nil {
		sse.PatchElementTempl(AppInlineError("webhook-error-"+id, "cannot parse repo URL: "+err.Error()))
		return
	}

	giteaHost := extractGiteaHost(app.RepoURL)
	apiURL := fmt.Sprintf("https://%s/api/v1/repos/%s/%s/hooks", giteaHost, owner, repo)

	payload, _ := json.Marshal(map[string]any{
		"active": true,
		"type":   "gitea",
		"config": map[string]string{
			"url":          webhookURL,
			"content_type": "json",
		},
		"events": []string{"push"},
	})

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, apiURL, strings.NewReader(string(payload)))
	if err != nil {
		sse.PatchElementTempl(AppInlineError("webhook-error-"+id, err.Error()))
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "token "+app.RegPass) // use registry token as Gitea API token

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		sse.PatchElementTempl(AppInlineError("webhook-error-"+id, err.Error()))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		sse.PatchElementTempl(AppInlineError("webhook-error-"+id, fmt.Sprintf("Gitea API returned %d", resp.StatusCode)))
		return
	}

	sse.PatchElementTempl(AppWebhookRegistered(id, webhookURL))
}

// ── Form parsing ──────────────────────────────────────────────────────────────

func (h *AppHandlers) appFromForm(r *http.Request, existingPass string) (*domain.DockerApp, error) {
	if err := r.ParseForm(); err != nil {
		return nil, err
	}
	name := r.FormValue("name")
	repoURL := r.FormValue("repo_url")
	branch := r.FormValue("branch")
	if branch == "" {
		branch = "main"
	}
	regUser := r.FormValue("reg_user")
	regPass := r.FormValue("reg_pass")
	if regPass == "" {
		regPass = existingPass // keep existing if not changed
	}

	buildArgs := parseKVForm(r, "build_arg_key", "build_arg_val")
	envVars := parseKVForm(r, "env_key", "env_val")
	ports := parsePortsForm(r)
	volumes := parseVolumesForm(r)
	networks := r.Form["networks"]
	restartPolicy := r.FormValue("restart_policy")
	if restartPolicy == "" {
		restartPolicy = "unless-stopped"
	}

	app, err := domain.NewDockerApp(name, repoURL, branch, regUser, regPass)
	if err != nil {
		return nil, err
	}
	app.BuildArgs = buildArgs
	app.EnvVars = envVars
	app.Ports = ports
	app.Volumes = volumes
	app.Networks = networks
	app.RestartPolicy = restartPolicy
	return app, nil
}

func parseKVForm(r *http.Request, keyField, valField string) []domain.KV {
	keys := r.Form[keyField]
	vals := r.Form[valField]
	var out []domain.KV
	for i, k := range keys {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		v := ""
		if i < len(vals) {
			v = vals[i]
		}
		out = append(out, domain.KV{Key: k, Value: v})
	}
	return out
}

func parsePortsForm(r *http.Request) []domain.PortMap {
	hosts := r.Form["port_host"]
	containers := r.Form["port_container"]
	protos := r.Form["port_proto"]
	var out []domain.PortMap
	for i, h := range hosts {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		c := ""
		if i < len(containers) {
			c = containers[i]
		}
		p := "tcp"
		if i < len(protos) && protos[i] != "" {
			p = protos[i]
		}
		out = append(out, domain.PortMap{Host: h, Container: c, Proto: p})
	}
	return out
}

func parseVolumesForm(r *http.Request) []domain.VolumeMount {
	hosts := r.Form["vol_host"]
	containers := r.Form["vol_container"]
	var out []domain.VolumeMount
	for i, h := range hosts {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		c := ""
		if i < len(containers) {
			c = containers[i]
		}
		out = append(out, domain.VolumeMount{Host: h, Container: c})
	}
	return out
}

// ── URL helpers ───────────────────────────────────────────────────────────────

func parseOwnerRepo(repoURL string) (owner, repo string, err error) {
	u := strings.TrimPrefix(repoURL, "https://")
	u = strings.TrimPrefix(u, "http://")
	u = strings.TrimSuffix(u, ".git")
	parts := strings.Split(u, "/")
	if len(parts) < 3 {
		return "", "", fmt.Errorf("expected https://host/owner/repo, got %s", repoURL)
	}
	return parts[1], parts[2], nil
}

func extractGiteaHost(repoURL string) string {
	u := strings.TrimPrefix(repoURL, "https://")
	u = strings.TrimPrefix(u, "http://")
	if idx := strings.Index(u, "/"); idx >= 0 {
		return u[:idx]
	}
	return u
}
