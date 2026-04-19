package http

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	authdomain "github.com/vvs/isp/internal/modules/auth/domain"
	authqueries "github.com/vvs/isp/internal/modules/auth/app/queries"
	"github.com/vvs/isp/internal/infrastructure/http/apimw"
	"github.com/vvs/isp/internal/infrastructure/metrics"
	"gorm.io/gorm"
)

// metricsMiddleware records HTTP request duration and count into Prometheus.
// Uses the chi route pattern as the path label to avoid label cardinality explosion.
func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		next.ServeHTTP(ww, r)
		status := ww.Status()
		if status == 0 {
			status = http.StatusOK
		}
		path := "/unknown"
		if rctx := chi.RouteContext(r.Context()); rctx != nil && rctx.RoutePattern() != "" {
			path = rctx.RoutePattern()
		}
		statusStr := strconv.Itoa(status)
		elapsed := time.Since(start).Seconds()
		metrics.HTTPRequestDuration.WithLabelValues(r.Method, path, statusStr).Observe(elapsed)
		metrics.HTTPRequestsTotal.WithLabelValues(r.Method, path, statusStr).Inc()
	})
}

// requestLogger is a chi-compatible request logger using slog.
// Unlike middleware.Logger, it treats status 0 (SSE connections where datastar flushes
// headers via http.NewResponseController, bypassing the WrapResponseWriter hook) as 200.
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		defer func() {
			status := ww.Status()
			if status == 0 {
				status = http.StatusOK // SSE: headers flushed via ResponseController, bypasses WrapResponseWriter
			}
			slog.Info("http",
				"method", r.Method,
				"path", r.URL.Path,
				"status", status,
				"duration", fmt.Sprintf("%dms", time.Since(start).Milliseconds()),
				"bytes", ww.BytesWritten(),
			)
		}()
		next.ServeHTTP(ww, r)
	})
}

type ModuleRoutes interface {
	RegisterRoutes(r chi.Router)
}

// ModuleNamed is optionally implemented by module handlers that map to a named permission module.
// When implemented, all routes for that module are wrapped in RequireModuleAccess(ModuleName()).
type ModuleNamed interface {
	ModuleName() authdomain.Module
}

// PublicModuleRoutes is optionally implemented by module handlers that have public (unauthenticated) routes.
type PublicModuleRoutes interface {
	RegisterPublicRoutes(r chi.Router)
}

// APIRoutes is implemented by module handlers that expose REST JSON endpoints.
type APIRoutes interface {
	RegisterAPIRoutes(r chi.Router)
}

func NewRouter(reader *gorm.DB, currentUser *authqueries.GetCurrentUserHandler, permRepo authdomain.RolePermissionsRepository, notif *NotifHandler, chatHandler *ChatHandler, global *GlobalHandler, apiToken string, rpc RPCDispatcher, modules ...ModuleRoutes) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(requestLogger)
	r.Use(metricsMiddleware)
	r.Use(middleware.RealIP)

	// Static files (unauthenticated)
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Module public routes (unauthenticated)
	for _, m := range modules {
		if p, ok := m.(PublicModuleRoutes); ok {
			p.RegisterPublicRoutes(r)
		}
	}

	// Protected routes behind auth middleware
	r.Group(func(r chi.Router) {
		r.Use(RequireAuth(currentUser))
		r.Use(InjectModulePermissions(permRepo))
		r.Use(RequireWrite) // blocks viewer role from all mutations

		// Dashboard
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			DashboardPage().Render(r.Context(), w)
		})

		// Global SSE (clock + notifications + chat widget)
		r.Get("/sse", global.globalSSE)
		r.Get("/api/dashboard/stats", newDashboardStatsHandler(reader))

		// Reports
		r.Group(func(r chi.Router) {
			r.Use(RequireModuleAccess(authdomain.ModuleReports))
			r.Get("/reports", func(w http.ResponseWriter, r *http.Request) {
				ReportsPage().Render(r.Context(), w)
			})
			r.Get("/api/reports/data", newReportsHandler(reader))
		})

			// CRM overview
			r.Get("/crm", func(w http.ResponseWriter, r *http.Request) {
				CRMDashboardPage().Render(r.Context(), w)
			})
			r.Get("/api/crm/stats", newCRMStatsHandler(reader))
			r.Get("/api/crm/pipeline", newCRMPipelineHandler(reader))
			r.Get("/api/crm/tickets", newCRMTicketsHandler(reader))
			r.Get("/api/crm/tasks", newCRMTasksHandler(reader))

		// Notifications
		r.Post("/api/notifications/read", notif.markRead)

		// Chat
		r.Post("/api/chat/send", chatHandler.chatSend)

		// Chat full page
		r.Get("/chat", chatHandler.chatPage)
		r.Get("/sse/chat-page", chatHandler.chatPageSSE)
		r.Post("/api/chat/threads/direct", chatHandler.createDirect)
		r.Post("/api/chat/threads/channel", chatHandler.createChannel)
		r.Post("/api/chat/threads/{threadID}/members", chatHandler.addMember)
		r.Post("/api/chat/threads/{threadID}/read", chatHandler.markRead)

		// Register all module routes; wrap named modules in RequireModuleAccess
		for _, m := range modules {
			if named, ok := m.(ModuleNamed); ok {
				r.Group(func(r chi.Router) {
					r.Use(RequireModuleAccess(named.ModuleName()))
					m.RegisterRoutes(r)
				})
			} else {
				m.RegisterRoutes(r)
			}
		}
	})

	// REST JSON API — bearer token protected
	r.Group(func(r chi.Router) {
		r.Use(apimw.BearerToken(apiToken))
		for _, m := range modules {
			if a, ok := m.(APIRoutes); ok {
				a.RegisterAPIRoutes(r)
			}
		}
		// Generic RPC dispatch endpoint for CLI HTTP transport
		if rpc != nil {
			r.Post("/api/v1/rpc/*", rpcHandler(rpc))
		}
	})

	return r
}
