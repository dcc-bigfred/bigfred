package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/protocol"
	"github.com/keskad/loco/pkgs/bigfred/server/security"
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
	svc      *cmd.Sudo
	auth     *cmd.Auth
	users    *cmd.User
	presence *cmd.Presence
}

// NewSudoHandler returns a SudoHandler.
func NewSudoHandler(
	svc *cmd.Sudo,
	auth *cmd.Auth,
	users *cmd.User,
	presence *cmd.Presence,
) *SudoHandler {
	return &SudoHandler{svc: svc, auth: auth, users: users, presence: presence}
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
	var req protocol.PinRequest
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
	_ = json.NewEncoder(w).Encode(protocol.SudoResponse{
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

	var req protocol.GrantSignalmanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if req.UserID == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if _, err := h.users.Get(r.Context(), req.UserID); err != nil {
		if errors.Is(err, svcerrors.ErrUserNotFound) {
			writeJSONError(w, http.StatusNotFound, svcerrors.CodeUserNotFound)
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
		if errors.Is(err, svcerrors.ErrUserNotFound) {
			writeJSONError(w, http.StatusNotFound, svcerrors.CodeUserNotFound)
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
func (h *SudoHandler) resolveLayoutActor(w http.ResponseWriter, r *http.Request) (cmd.Identity, uint, bool) {
	id, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return cmd.Identity{}, 0, false
	}
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return cmd.Identity{}, 0, false
	}
	if actor.Layout.ID != id {
		writeJSONError(w, http.StatusUnprocessableEntity, "sudo_layout_mismatch")
		return cmd.Identity{}, 0, false
	}
	return actor, id, true
}

// writeSudoError maps SudoService sentinels to status + machine
// codes the frontend can localise. The `Retry-After` hint is set
// for the lockout case so a polite client can back off.
func writeSudoError(w http.ResponseWriter, svc *cmd.Sudo, userID, layoutID uint, err error) {
	status, code := svcerrors.SudoHTTPStatus(err)
	if errors.Is(err, svcerrors.ErrSudoLocked) {
		secs := svc.LockedRetryAfter(userID, layoutID)
		if secs > 0 {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", secs))
		}
	}
	writeJSONError(w, status, code)
}
