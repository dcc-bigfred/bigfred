package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/keskad/loco/pkgs/server/domain"
	"github.com/keskad/loco/pkgs/server/service"
)

// SessionCookieName is the name of the HttpOnly cookie that carries
// the signed session JWT. Kept as a package constant so the login
// handler and the auth middleware agree on it.
const SessionCookieName = "bigfred_session"

// RequireAuth is the chi middleware that enforces an authenticated
// session for the wrapped handler chain. It reads the JWT from the
// session cookie (falling back to a `?token=` query parameter to
// support WS upgrades per §7a.1), verifies it via AuthService and
// attaches the resulting Identity to the request context.
func RequireAuth(auth *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := readSessionToken(r)
			if token == "" {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			id, err := auth.VerifyToken(r.Context(), token)
			if err != nil {
				if errors.Is(err, service.ErrInvalidCredentials) {
					writeJSONError(w, http.StatusUnauthorized, "unauthorized")
					return
				}
				writeJSONError(w, http.StatusInternalServerError, "internal_error")
				return
			}

			next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), id)))
		})
	}
}

// RequireRole composes on top of RequireAuth: it returns 403 when the
// authenticated user's permanent role isn't in the allow-list. Used
// by admin-only endpoints in later milestones.
func RequireRole(roles ...domain.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := IdentityFromContext(r.Context())
			if !ok {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			if !id.HasRole(roles...) {
				writeJSONError(w, http.StatusForbidden, "forbidden")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// readSessionToken extracts the session JWT from a request, preferring
// the cookie (set by /auth/login) and falling back to a `?token=`
// query parameter so a WebSocket upgrade can authenticate without
// custom headers.
func readSessionToken(r *http.Request) string {
	if c, err := r.Cookie(SessionCookieName); err == nil && c.Value != "" {
		return c.Value
	}
	return r.URL.Query().Get("token")
}

// writeJSONError renders {"error": "..."} with the given status. The
// machine-readable code lets the frontend localise without parsing
// English prose.
func writeJSONError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}
