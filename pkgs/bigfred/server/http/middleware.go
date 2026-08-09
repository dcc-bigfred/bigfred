package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/metrics"
)

// SessionCookieName is the name of the HttpOnly cookie that carries
// the signed session JWT. Kept as a package constant so the login
// handler and the auth middleware agree on it.
const SessionCookieName = "bigfred_session"

// ImpersonateAsHeader is the admin-only subject switch for authenticated APIs.
const ImpersonateAsHeader = "X-BigFred-Impersonate-As"

// RequireAuth is the chi middleware that enforces an authenticated
// session for the wrapped handler chain. It reads the JWT from the
// session cookie (falling back to Bearer / `?token=`), verifies it via
// AuthService and attaches the resulting Identity to the request context.
func RequireAuth(auth *cmd.Auth, m *metrics.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := readSessionToken(r)
			if token == "" {
				if m != nil {
					m.RecordAuthUnauthorized(r.URL.Path)
				}
				writeJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			id, err := auth.VerifyToken(r.Context(), token)
			if err != nil {
				if m != nil {
					m.RecordAuthTokenVerifyError("verify_failed")
					m.RecordAuthUnauthorized(r.URL.Path)
				}
				if errors.Is(err, svcerrors.ErrInvalidCredentials) {
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
// authenticated user's effective role inside their active layout
// isn't in the allow-list. When impersonation is active, Effective is
// computed for the **actor** (real caller), not the subject.
func RequireRole(auth *cmd.Auth, roles ...domain.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor, ok := ActorFromContext(r.Context())
			if !ok {
				id, idOK := IdentityFromContext(r.Context())
				if !idOK {
					writeJSONError(w, http.StatusUnauthorized, "unauthorized")
					return
				}
				actor = id
			}
			eff, err := auth.Effective(r.Context(), actor.User, actor.Layout.ID)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "internal_error")
				return
			}
			for _, role := range roles {
				if eff.Has(role) {
					next.ServeHTTP(w, r)
					return
				}
			}
			writeJSONError(w, http.StatusForbidden, "forbidden")
		})
	}
}

// MaybeImpersonate switches Identity to the named user when the admin
// sends ImpersonateAsHeader. Actor remains available via ActorFromContext.
func MaybeImpersonate(auth *cmd.Auth, users *cmd.User) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			login := strings.TrimSpace(r.Header.Get(ImpersonateAsHeader))
			if login == "" {
				next.ServeHTTP(w, r)
				return
			}
			id, ok := IdentityFromContext(r.Context())
			if !ok {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			eff, err := auth.Effective(r.Context(), id.User, id.Layout.ID)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "internal_error")
				return
			}
			if !eff.Has(domain.RoleAdmin) {
				writeJSONError(w, http.StatusForbidden, svcerrors.CodeImpersonationForbidden)
				return
			}
			subject, err := users.FindByLogin(r.Context(), login)
			if err != nil {
				status, code := svcerrors.UserHTTPStatus(err)
				writeJSONError(w, status, code)
				return
			}
			if !subject.Active {
				writeJSONError(w, http.StatusForbidden, svcerrors.CodeAccountDeactivated)
				return
			}
			ctx := WithActor(r.Context(), id)
			ctx = WithIdentity(ctx, cmd.Identity{User: subject, Layout: id.Layout})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// readSessionToken extracts the session JWT from a request, preferring
// the cookie, then Authorization: Bearer, then `?token=`.
func readSessionToken(r *http.Request) string {
	if c, err := r.Cookie(SessionCookieName); err == nil && c.Value != "" {
		return c.Value
	}
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	}
	return r.URL.Query().Get("token")
}

// writeJSONError renders {"error": "..."} with the given status.
func writeJSONError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}
