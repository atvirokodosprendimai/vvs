package apimw

import (
	"net/http"
	"strings"

	"github.com/atvirokodosprendimai/vvs/internal/infrastructure/http/jsonapi"
)

// BearerToken returns middleware that enforces Authorization: Bearer <token>.
// If token is empty the middleware rejects all requests with 503.
func BearerToken(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token == "" {
				jsonapi.WriteError(w, http.StatusServiceUnavailable, "REST API not configured")
				return
			}
			got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if got != token {
				jsonapi.WriteError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
