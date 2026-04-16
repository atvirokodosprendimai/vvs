package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	authqueries "github.com/vvs/isp/internal/modules/auth/app/queries"
	"github.com/vvs/isp/internal/infrastructure/http/apimw"
	"gorm.io/gorm"
)

type ModuleRoutes interface {
	RegisterRoutes(r chi.Router)
}

// APIRoutes is implemented by module handlers that expose REST JSON endpoints.
type APIRoutes interface {
	RegisterAPIRoutes(r chi.Router)
}

func NewRouter(reader *gorm.DB, currentUser *authqueries.GetCurrentUserHandler, notif *NotifHandler, chatHandler *ChatHandler, global *GlobalHandler, apiToken string, rpc RPCDispatcher, modules ...ModuleRoutes) http.Handler {
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

		// Global SSE (clock + notifications + chat widget)
		r.Get("/sse", global.globalSSE)
		r.Get("/api/dashboard/stats", newDashboardStatsHandler(reader))

		// Notifications
		r.Post("/api/notifications/read", notif.markRead)

		// Chat
		r.Post("/api/chat/send", chatHandler.chatSend)

		// Chat full page
		r.Get("/chat", chatHandler.chatPage)
		r.Get("/sse/chat/threads", chatHandler.threadsSSE)
		r.Get("/sse/chat/messages/{threadID}", chatHandler.threadMessagesSSE)
		r.Post("/api/chat/threads/direct", chatHandler.createDirect)
		r.Post("/api/chat/threads/channel", chatHandler.createChannel)
		r.Post("/api/chat/threads/{threadID}/members", chatHandler.addMember)
		r.Post("/api/chat/threads/{threadID}/read", chatHandler.markRead)

		// Register all module routes
		for _, m := range modules {
			m.RegisterRoutes(r)
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
