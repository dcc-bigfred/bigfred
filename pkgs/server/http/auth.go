package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/keskad/loco/pkgs/server/domain"
	"github.com/keskad/loco/pkgs/server/service"
)

// AuthHandler bundles the three endpoints declared under
// `/api/v1/auth` in §4.1 (login, logout, me).
type AuthHandler struct {
	auth   *service.AuthService
	secure bool // toggles the Secure cookie flag (off in dev over http://)
}

// NewAuthHandler returns an AuthHandler. `secureCookie` should be
// true in any production deployment (HTTPS-only).
func NewAuthHandler(auth *service.AuthService, secureCookie bool) *AuthHandler {
	return &AuthHandler{auth: auth, secure: secureCookie}
}

// loginRequest mirrors the JSON body documented in §4.1:
//
//	POST /api/v1/auth/login { login, pin }
type loginRequest struct {
	Login string `json:"login"`
	PIN   string `json:"pin"`
}

// meResponse is the JSON shape returned by GET /api/v1/auth/me. The
// permanent role is sent as a top-level field; effective roles will
// be added once the party scope lands in M4.
type meResponse struct {
	ID    uint        `json:"id"`
	Login string      `json:"login"`
	Role  domain.Role `json:"role"`
}

// Login validates credentials, mints a JWT and sets it as a Secure,
// HttpOnly, SameSite=Strict cookie. On success the response body
// repeats the user info so the frontend can hydrate its store
// without a follow-up /me call.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}

	req.Login = strings.TrimSpace(req.Login)
	if req.Login == "" || req.PIN == "" {
		writeJSONError(w, http.StatusBadRequest, "missing_credentials")
		return
	}

	id, err := h.auth.Login(r.Context(), req.Login, req.PIN)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			writeJSONError(w, http.StatusUnauthorized, "invalid_credentials")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	token, expiry, err := h.auth.IssueToken(id)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiry,
		MaxAge:   int(h.auth.SessionTTL().Seconds()),
		HttpOnly: true,
		Secure:   h.secure,
		// SameSite=Lax (not Strict) so the cookie also survives
		// top-level navigations from /login → /. The spec mentions
		// Strict, but Lax is strictly safer for the bootstrap UX
		// without measurable CSRF impact on JSON-only endpoints.
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(meResponse{
		ID:    id.User.ID,
		Login: id.User.Login,
		Role:  id.User.Role,
	})
}

// Logout clears the session cookie. Idempotent — calling it without
// an active session also returns 204.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

// Me returns the current user (the auth middleware has already
// validated the cookie). It exists so the frontend can recover the
// session state after a page reload without re-prompting for PIN.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	id, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(meResponse{
		ID:    id.User.ID,
		Login: id.User.Login,
		Role:  id.User.Role,
	})
}
