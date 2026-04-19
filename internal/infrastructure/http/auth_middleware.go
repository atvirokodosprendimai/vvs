package http

import (
	"context"
	"net/http"
	"strings"

	authhttp "github.com/vvs/isp/internal/modules/auth/adapters/http"
	"github.com/vvs/isp/internal/modules/auth/app/queries"
	authdomain "github.com/vvs/isp/internal/modules/auth/domain"
)

// WithPermissions stores a PermissionSet in the context.
// Delegates to authdomain so templates can read it without a circular import.
func WithPermissions(ctx context.Context, ps authdomain.PermissionSet) context.Context {
	return authdomain.WithPermissions(ctx, ps)
}

// PermissionsFromCtx retrieves the PermissionSet from context.
// Returns an empty (deny-all) set if not found.
func PermissionsFromCtx(ctx context.Context) authdomain.PermissionSet {
	return authdomain.PermissionsFromCtx(ctx)
}

// InjectModulePermissions loads the role's PermissionSet from DB into the request context.
// Admin role gets a hardcoded full-access set (no DB hit).
// Must run after RequireAuth.
func InjectModulePermissions(permRepo authdomain.RolePermissionsRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := authhttp.UserFromContext(r.Context())
			if u == nil {
				next.ServeHTTP(w, r)
				return
			}
			var ps authdomain.PermissionSet
			if u.Role == authdomain.RoleAdmin {
				ps = authdomain.AdminPermissionSet()
			} else {
				var err error
				ps, err = permRepo.FindByRole(r.Context(), u.Role)
				if err != nil || len(ps) == 0 {
					// Fallback to defaults if DB is empty (e.g. fresh install before migration)
					ps = authdomain.DefaultPermissions(u.Role)
				}
			}
			next.ServeHTTP(w, r.WithContext(WithPermissions(r.Context(), ps)))
		})
	}
}

// RequireModuleAccess returns a middleware that enforces view/edit access for a specific module.
// View access is required for all methods; edit access is required for mutating methods.
func RequireModuleAccess(module authdomain.Module) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ps := PermissionsFromCtx(r.Context())
			if !ps.CanView(module) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			switch r.Method {
			case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
				if !ps.CanEdit(module) {
					http.Error(w, "Forbidden", http.StatusForbidden)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// selfServicePaths lists mutating routes that all authenticated users (including viewers) may access.
var selfServicePaths = map[string]bool{
	"/api/users/me/password": true,
}

// RequireWrite is a middleware that blocks viewer-role users from all mutating
// HTTP methods (POST, PUT, PATCH, DELETE). It must be placed after RequireAuth.
// Self-service paths (e.g. changing one's own password) are exempt.
func RequireWrite(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			if !selfServicePaths[r.URL.Path] {
				u := authhttp.UserFromContext(r.Context())
				if u != nil && !u.CanWrite() {
					http.Error(w, "Forbidden: read-only account", http.StatusForbidden)
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAuth is a chi middleware that validates the vvs_session cookie.
// On success it stores the *domain.User in the request context.
// On failure it redirects to /login (except for /login and /static/* paths).
func RequireAuth(currentUser *queries.GetCurrentUserHandler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			if path == "/login" || path == "/api/login" ||
				path == "/login/totp" || path == "/api/login/totp" ||
				strings.HasPrefix(path, "/static/") {
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
