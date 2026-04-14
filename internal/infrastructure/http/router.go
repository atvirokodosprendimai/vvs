package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	authqueries "github.com/vvs/isp/internal/modules/auth/app/queries"
	"gorm.io/gorm"
)

type ModuleRoutes interface {
	RegisterRoutes(r chi.Router)
}

func NewRouter(reader *gorm.DB, currentUser *authqueries.GetCurrentUserHandler, modules ...ModuleRoutes) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)
	r.Use(middleware.RealIP)

	// Static files (unauthenticated)
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Protected routes behind auth middleware
	r.Group(func(r chi.Router) {
		r.Use(RequireAuth(currentUser))

		// Dashboard
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			DashboardPage().Render(r.Context(), w)
		})

		// SSE endpoints
		r.Get("/sse/clock", clockSSE)
		r.Get("/api/dashboard/stats", newDashboardStatsHandler(reader))

		// Register all module routes
		for _, m := range modules {
			m.RegisterRoutes(r)
		}
	})

	return r
}
