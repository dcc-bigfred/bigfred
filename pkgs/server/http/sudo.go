package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/keskad/loco/pkgs/server/security"
	"github.com/keskad/loco/pkgs/server/service"
)

// SudoHandler bundles the four endpoints that drive the layout-scoped
// elevation flow (§7a.7):
//
//	POST   /api/v1/layouts/{id}/sudo       — request a 2-min admin grant
//	DELETE /api/v1/layouts/{id}/sudo       — drop the active admin grant
//	POST   /api/v1/layouts/{id}/signalman  — permanent self-grant via PIN
//	DELETE /api/v1/layouts/{id}/signalman  — drop own signalman grant
//	POST   /api/v1/layouts/{id}/signalmen — admin grants signalman to another user
//	DELETE /api/v1/layouts/{id}/signalmen/{userId} — admin revokes signalman grant
//
// The self-grant paths trust RequireAuth; the padlock and the
// engineer's-cap icons always target the caller. GrantSignalmanToUser
// requires effective admin (permanent or sudo).
type SudoHandler struct {
	svc      *service.SudoService
	auth     *service.AuthService
	users    *service.UserService
	presence *service.PresenceService
}

// NewSudoHandler returns a SudoHandler.
func NewSudoHandler(
	svc *service.SudoService,
	auth *service.AuthService,
	users *service.UserService,
	presence *service.PresenceService,
) *SudoHandler {
	return &SudoHandler{svc: svc, auth: auth, users: users, presence: presence}
}

// pinRequest mirrors the JSON body of POST /api/v1/layouts/{id}/sudo
// and POST /api/v1/layouts/{id}/signalman:
//
//	{ "pin": "0000" }
type pinRequest struct {
	PIN string `json:"pin"`
}

type grantSignalmanRequest struct {
	UserID uint `json:"userId"`
}

// sudoResponse echoes the persisted admin grant so the UI can start
// a countdown immediately.
type sudoResponse struct {
	GrantedAt time.Time `json:"grantedAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// resolveLayout asserts that the caller is authenticated, the path
// parameter is a valid uint, and the layout matches the JWT-pinned
// session layout. On failure it writes the appropriate error response
// and returns ok=false.
func (h *SudoHandler) resolveLayout(w http.ResponseWriter, r *http.Request) (userID, layoutID uint, ok bool) {
	id, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return 0, 0, false
	}
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return 0, 0, false
	}
	if actor.Layout.ID != id {
		writeJSONError(w, http.StatusUnprocessableEntity, "sudo_layout_mismatch")
		return 0, 0, false
	}
	return actor.User.ID, id, true
}

// decodePIN extracts the PIN field from the request body.
func decodePIN(w http.ResponseWriter, r *http.Request) (string, bool) {
	var req pinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return "", false
	}
	return req.PIN, true
}

// RequestSudo handles POST /api/v1/layouts/{id}/sudo — admin
// elevation for `cfg.SudoTTL` (default 2 min).
func (h *SudoHandler) RequestSudo(w http.ResponseWriter, r *http.Request) {
	userID, layoutID, ok := h.resolveLayout(w, r)
	if !ok {
		return
	}
	pin, ok := decodePIN(w, r)
	if !ok {
		return
	}

	row, err := h.svc.Sudo(r.Context(), userID, layoutID, pin)
	if err != nil {
		writeSudoError(w, h.svc, userID, layoutID, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sudoResponse{
		GrantedAt: row.GrantedAt,
		ExpiresAt: row.ExpiresAt,
	})
}

// RevokeSudo handles DELETE /api/v1/layouts/{id}/sudo. Idempotent.
func (h *SudoHandler) RevokeSudo(w http.ResponseWriter, r *http.Request) {
	userID, layoutID, ok := h.resolveLayout(w, r)
	if !ok {
		return
	}
	if err := h.svc.Revoke(r.Context(), userID, layoutID); err != nil {
		writeSudoError(w, h.svc, userID, layoutID, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RequestSignalman handles POST /api/v1/layouts/{id}/signalman —
// permanent self-grant of the layout-scoped signalman role gated
// by the layout admin PIN.
func (h *SudoHandler) RequestSignalman(w http.ResponseWriter, r *http.Request) {
	userID, layoutID, ok := h.resolveLayout(w, r)
	if !ok {
		return
	}
	pin, ok := decodePIN(w, r)
	if !ok {
		return
	}
	if err := h.svc.GrantSignalman(r.Context(), userID, layoutID, pin); err != nil {
		writeSudoError(w, h.svc, userID, layoutID, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RevokeSignalman handles DELETE /api/v1/layouts/{id}/signalman.
// Idempotent.
func (h *SudoHandler) RevokeSignalman(w http.ResponseWriter, r *http.Request) {
	userID, layoutID, ok := h.resolveLayout(w, r)
	if !ok {
		return
	}
	if err := h.svc.RevokeSignalman(r.Context(), userID, layoutID); err != nil {
		writeSudoError(w, h.svc, userID, layoutID, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GrantSignalmanToUser handles POST /api/v1/layouts/{id}/signalmen.
// Requires effective admin inside the session layout.
func (h *SudoHandler) GrantSignalmanToUser(w http.ResponseWriter, r *http.Request) {
	actor, layoutID, ok := h.resolveLayoutActor(w, r)
	if !ok {
		return
	}
	eff, err := h.auth.Effective(r.Context(), actor.User, layoutID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if d := (security.LayoutSecurityContext{}).CanGrantSignalmanToUser(eff); !d.Allowed {
		writeJSONError(w, http.StatusForbidden, d.Reason)
		return
	}

	var req grantSignalmanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if req.UserID == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if _, err := h.users.Get(r.Context(), req.UserID); err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			writeJSONError(w, http.StatusNotFound, "user_not_found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	if err := h.svc.GrantSignalmanToUser(r.Context(), actor.User.ID, req.UserID, layoutID); err != nil {
		writeSudoError(w, h.svc, actor.User.ID, layoutID, err)
		return
	}
	h.presence.RefreshAndBroadcast(r.Context(), layoutID)
	w.WriteHeader(http.StatusNoContent)
}

// RevokeSignalmanFromUser handles DELETE
// /api/v1/layouts/{id}/signalmen/{userId}. Requires effective admin
// inside the session layout.
func (h *SudoHandler) RevokeSignalmanFromUser(w http.ResponseWriter, r *http.Request) {
	actor, layoutID, ok := h.resolveLayoutActor(w, r)
	if !ok {
		return
	}
	eff, err := h.auth.Effective(r.Context(), actor.User, layoutID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if d := (security.LayoutSecurityContext{}).CanGrantSignalmanToUser(eff); !d.Allowed {
		writeJSONError(w, http.StatusForbidden, d.Reason)
		return
	}

	targetUserID, ok := parseUintParam(r, "userId")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	if _, err := h.users.Get(r.Context(), targetUserID); err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			writeJSONError(w, http.StatusNotFound, "user_not_found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	if err := h.svc.RevokeSignalman(r.Context(), targetUserID, layoutID); err != nil {
		writeSudoError(w, h.svc, actor.User.ID, layoutID, err)
		return
	}
	h.presence.RefreshAndBroadcast(r.Context(), layoutID)
	w.WriteHeader(http.StatusNoContent)
}

// resolveLayoutActor returns the authenticated identity and layout
// id when the path matches the JWT-pinned session layout.
func (h *SudoHandler) resolveLayoutActor(w http.ResponseWriter, r *http.Request) (service.Identity, uint, bool) {
	id, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return service.Identity{}, 0, false
	}
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return service.Identity{}, 0, false
	}
	if actor.Layout.ID != id {
		writeJSONError(w, http.StatusUnprocessableEntity, "sudo_layout_mismatch")
		return service.Identity{}, 0, false
	}
	return actor, id, true
}

// writeSudoError maps SudoService sentinels to status + machine
// codes the frontend can localise. The `Retry-After` hint is set
// for the lockout case so a polite client can back off.
func writeSudoError(w http.ResponseWriter, svc *service.SudoService, userID, layoutID uint, err error) {
	switch {
	case errors.Is(err, service.ErrSudoInvalidInput):
		writeJSONError(w, http.StatusUnprocessableEntity, "sudo_layout_mismatch")
	case errors.Is(err, service.ErrSudoInvalidPIN):
		writeJSONError(w, http.StatusUnauthorized, "sudo_invalid_pin")
	case errors.Is(err, service.ErrSudoLocked):
		secs := svc.LockedRetryAfter(userID, layoutID)
		if secs > 0 {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", secs))
		}
		writeJSONError(w, http.StatusTooManyRequests, "sudo_locked")
	case errors.Is(err, service.ErrLayoutAdminPINUnset):
		writeJSONError(w, http.StatusUnprocessableEntity, "layout_admin_pin_unset")
	case errors.Is(err, service.ErrLayoutNotFound):
		writeJSONError(w, http.StatusNotFound, "layout_not_found")
	default:
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
	}
}
