package http

import (
	"context"
	"net/http"
	"strings"

	authhttp "github.com/vvs/isp/internal/modules/auth/adapters/http"
	"github.com/vvs/isp/internal/modules/auth/app/queries"
)

// RequireAuth is a chi middleware that validates the vvs_session cookie.
// On success it stores the *domain.User in the request context.
// On failure it redirects to /login (except for /login and /static/* paths).
func RequireAuth(currentUser *queries.GetCurrentUserHandler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			if path == "/login" || strings.HasPrefix(path, "/static/") {
				next.ServeHTTP(w, r)
				return
			}

			cookie, err := r.Cookie("vvs_session")
			if err != nil {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}

			u, err := currentUser.Handle(r.Context(), cookie.Value)
			if err != nil || u == nil {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}

			ctx := authhttp.WithUser(r.Context(), u)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserFromContext is a convenience re-export for use outside the auth adapter.
func UserFromContext(ctx context.Context) interface{} {
	return authhttp.UserFromContext(ctx)
}
