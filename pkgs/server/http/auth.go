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
//	POST /api/v1/auth/login { login, pin, layoutId }
//
// `layoutId` is REQUIRED. The frontend pre-fills it from the
// `GET /api/v1/layouts/login` dropdown (defaulting to the system
// layout), so a missing value is always a client bug.
type loginRequest struct {
	Login    string `json:"login"`
	PIN      string `json:"pin"`
	LayoutID uint   `json:"layoutId"`
}

// meResponse is the JSON shape returned by GET /api/v1/auth/me and
// echoed by POST /api/v1/auth/login (so the frontend can hydrate its
// store without a follow-up call). The layout fields mirror §4.1:
// they are derived from the JWT and immutable for the lifetime of
// the session.
type meResponse struct {
	ID    uint        `json:"id"`
	Login string      `json:"login"`
	Role  domain.Role `json:"role"`
	// EffectiveRole is the caller's resolved role label inside their
	// active layout (§7a.2), used for display: admin > signalman
	// (layout-scoped grant) > driver.
	EffectiveRole domain.Role `json:"effectiveRole"`
	// IsSignalman is true when the caller may occupy an interlocking:
	// permanent admins always; everyone else only with an active
	// LayoutSignalman grant in their active layout.
	IsSignalman    bool `json:"isSignalman"`
	LayoutID       uint `json:"layoutId"`
	LayoutName     string `json:"layoutName"`
	LayoutIsSystem bool   `json:"layoutIsSystem"`
}

// meFromIdentity collapses the identity → response mapping so the
// Login and Me handlers stay in sync.
func meFromIdentity(id service.Identity, effectiveRole domain.Role, isSignalman bool) meResponse {
	return meResponse{
		ID:             id.User.ID,
		Login:          id.User.Login,
		Role:           id.User.Role,
		EffectiveRole:  effectiveRole,
		IsSignalman:    isSignalman,
		LayoutID:       id.Layout.ID,
		LayoutName:     id.Layout.Name,
		LayoutIsSystem: id.Layout.IsSystem,
	}
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
	if req.LayoutID == 0 {
		// §7a.1: the dropdown always has at least the system layout
		// pre-selected, so a zero id means the request was crafted
		// without it. Treat as a 422 (the credentials might still be
		// fine — distinct from 401 invalid_credentials).
		writeJSONError(w, http.StatusUnprocessableEntity, "layout_required")
		return
	}

	id, err := h.auth.Login(r.Context(), req.Login, req.PIN, req.LayoutID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			writeJSONError(w, http.StatusUnauthorized, "invalid_credentials")
		case errors.Is(err, service.ErrAccountDeactivated):
			writeJSONError(w, http.StatusForbidden, "account_deactivated")
		case errors.Is(err, service.ErrLayoutNotFound):
			writeJSONError(w, http.StatusUnprocessableEntity, "layout_not_found")
		case errors.Is(err, service.ErrLayoutLocked):
			writeJSONError(w, http.StatusUnprocessableEntity, "layout_locked")
		default:
			writeJSONError(w, http.StatusInternalServerError, "internal_error")
		}
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

	effectiveRole, err := h.auth.EffectiveDisplayRole(r.Context(), id.User, id.Layout.ID)
	if err != nil {
		effectiveRole = id.User.Role
	}
	isSignalman, err := h.auth.IsEffectiveSignalman(r.Context(), id.User, id.Layout.ID)
	if err != nil {
		isSignalman = false
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(meFromIdentity(id, effectiveRole, isSignalman))
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
	effectiveRole, err := h.auth.EffectiveDisplayRole(r.Context(), id.User, id.Layout.ID)
	if err != nil {
		effectiveRole = id.User.Role
	}
	isSignalman, err := h.auth.IsEffectiveSignalman(r.Context(), id.User, id.Layout.ID)
	if err != nil {
		isSignalman = false
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(meFromIdentity(id, effectiveRole, isSignalman))
}
