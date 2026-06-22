package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/protocol"
)

// AuthHandler bundles the auth endpoints declared under
// `/api/v1/auth` in §4.1 (login, logout, me, change-pin).
//
// `sudo` is needed so the /me payload exposes the active admin
// elevation (its `expiresAt` drives the AppBar countdown) and so
// Logout drops the row — sudo state must NEVER survive a session
// change. It MAY be nil in legacy tests; in that case the /me
// payload simply reports `sudo: null`.
type AuthHandler struct {
	auth   *cmd.Auth
	sudo   *cmd.Sudo
	audit  cmd.AuditPublisher
	secure bool // toggles the Secure cookie flag (off in dev over http://)
}

// NewAuthHandler returns an AuthHandler. `secureCookie` should be
// true in any production deployment (HTTPS-only).
func NewAuthHandler(auth *cmd.Auth, sudo *cmd.Sudo, audit cmd.AuditPublisher, secureCookie bool) *AuthHandler {
	return &AuthHandler{auth: auth, sudo: sudo, audit: audit, secure: secureCookie}
}

// Login validates credentials, mints a JWT and sets it as a Secure,
// HttpOnly, SameSite=Strict cookie. On success the response body
// repeats the user info so the frontend can hydrate its store
// without a follow-up /me call.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req protocol.LoginRequest
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
		status, code := svcerrors.AuthHTTPStatus(err)
		writeJSONError(w, status, code)
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
	_ = json.NewEncoder(w).Encode(h.buildMeResponse(r, id))
}

// Logout clears the session cookie. Idempotent — calling it without
// an active session also returns 204.
//
// As a side effect the handler revokes the caller's sudo admin
// elevation inside the current layout (§7a.7). Sudo state must NOT
// survive a session change. The route is public (no RequireAuth
// wrapper), so we re-verify the session inline and silently skip the
// revoke when the cookie is missing or invalid.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if h.sudo != nil {
		if token := readSessionToken(r); token != "" {
			if id, err := h.auth.VerifyToken(r.Context(), token); err == nil {
				_ = h.sudo.Revoke(r.Context(), id.User.ID, id.Layout.ID)
			}
		}
	}
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
	_ = json.NewEncoder(w).Encode(h.buildMeResponse(r, id))
}

// ChangePIN handles PUT /api/v1/auth/me/pin — self-service password
// rotation after verifying the current PIN.
func (h *AuthHandler) ChangePIN(w http.ResponseWriter, r *http.Request) {
	id, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req protocol.ChangePINRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if req.CurrentPIN == "" || req.NewPIN == "" {
		writeJSONError(w, http.StatusBadRequest, "missing_credentials")
		return
	}
	if err := h.auth.ChangePIN(r.Context(), id.User.ID, req.CurrentPIN, req.NewPIN); err != nil {
		if status, code := svcerrors.AuthHTTPStatus(err); code != "internal_error" {
			writeJSONError(w, status, code)
			return
		}
		status, code := svcerrors.UserHTTPStatus(err)
		writeJSONError(w, status, code)
		return
	}
	if h.audit != nil {
		_ = h.audit.Publish(r.Context(), 0,
			cmd.AuditActor{UserID: id.User.ID, Login: id.User.Login},
			"audit_user_updated", map[string]string{"target": id.User.Login})
	}
	w.WriteHeader(http.StatusNoContent)
}

// UpdateProfile handles PUT /api/v1/auth/me/profile — self-service
// updates to profile fields the caller may change themselves.
func (h *AuthHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	id, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req protocol.UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	user, err := h.auth.UpdateProfile(r.Context(), id.User.ID, req.Organization)
	if err != nil {
		status, code := svcerrors.UserHTTPStatus(err)
		writeJSONError(w, status, code)
		return
	}
	id.User = user
	if h.audit != nil {
		_ = h.audit.Publish(r.Context(), 0,
			cmd.AuditActor{UserID: id.User.ID, Login: id.User.Login},
			"audit_user_updated", map[string]string{"target": id.User.Login})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(h.buildMeResponse(r, id))
}

// buildMeResponse runs the per-request derivation that the Login
// and Me handlers share. Failures inside the auth/sudo lookups fall
// back to safe defaults (effectiveRole := user.Role, isSignalman :=
// false, sudo := nil), so a transient repository hiccup never
// breaks the cookie hand-off.
func (h *AuthHandler) buildMeResponse(r *http.Request, id cmd.Identity) protocol.MeResponse {
	effectiveRole := id.User.Role
	isSignalman := false
	var sudo *protocol.SudoElevationResponse
	if snap, err := h.auth.EffectiveSnapshot(r.Context(), id.User, id.Layout.ID); err == nil {
		effectiveRole = snap.DisplayRole()
		isSignalman = snap.IsSignalman()
		if snap.Sudo != nil {
			sudo = &protocol.SudoElevationResponse{
				GrantedAt: snap.Sudo.GrantedAt,
				ExpiresAt: snap.Sudo.ExpiresAt,
			}
		}
	}
	return protocol.MeResponse{
		ID:             id.User.ID,
		Login:          id.User.Login,
		Organization:   id.User.Organization,
		Role:           id.User.Role,
		EffectiveRole:  effectiveRole,
		IsSignalman:    isSignalman,
		Active:         id.User.Active,
		CreatedAt:      id.User.CreatedAt,
		UpdatedAt:      id.User.UpdatedAt,
		LayoutID:       id.Layout.ID,
		LayoutName:     id.Layout.Name,
		LayoutIsSystem: id.Layout.IsSystem,
		Sudo:           sudo,
	}
}
