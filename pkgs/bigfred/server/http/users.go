package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/protocol"
)

// UserHandler bundles the admin-only user-management endpoints under
// `/api/v1/users` (§4.1 / §7a.5). The chi router wraps every route
// with RequireRole(domain.RoleAdmin); UserService re-checks via
// CanManageUsers for defense in depth.
type UserHandler struct {
	svc   *cmd.User
	auth  *cmd.Auth
	audit cmd.AuditPublisher
}

// NewUserHandler returns a UserHandler.
func NewUserHandler(svc *cmd.User, auth *cmd.Auth, audit cmd.AuditPublisher) *UserHandler {
	return &UserHandler{svc: svc, auth: auth, audit: audit}
}

// List handles GET /api/v1/users.
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	eff, err := h.auth.Effective(r.Context(), actor.User, actor.Layout.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	rows, err := h.svc.ListWithDCCPools(r.Context(), eff)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	out := make([]protocol.UserResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, protocol.ToUserResponse(row.User, row.DCCPool))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// Create handles POST /api/v1/users.
func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	eff, err := h.auth.Effective(r.Context(), actor.User, actor.Layout.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	var req protocol.UserCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	row, err := h.svc.Create(r.Context(), eff, req.ToCreateInput())
	if err != nil {
		writeUserError(w, err)
		return
	}
	pool, err := h.svc.GetDCCPool(r.Context(), row.ID)
	if err != nil {
		writeUserError(w, err)
		return
	}
	if h.audit != nil {
		_ = h.audit.Publish(r.Context(), 0,
			cmd.AuditActor{UserID: actor.User.ID, Login: actor.User.Login},
			"audit_user_created", map[string]string{"target": row.Login})
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(protocol.ToUserResponse(row, pool))
}

// Update handles PUT /api/v1/users/{id}.
func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	eff, err := h.auth.Effective(r.Context(), actor.User, actor.Layout.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	var req protocol.UserUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	row, err := h.svc.Update(r.Context(), eff, userID, req.ToUpdateInput())
	if err != nil {
		writeUserError(w, err)
		return
	}
	pool, err := h.svc.GetDCCPool(r.Context(), row.ID)
	if err != nil {
		writeUserError(w, err)
		return
	}
	if h.audit != nil {
		_ = h.audit.Publish(r.Context(), 0,
			cmd.AuditActor{UserID: actor.User.ID, Login: actor.User.Login},
			"audit_user_updated", map[string]string{"target": row.Login})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(protocol.ToUserResponse(row, pool))
}

// Activate handles POST /api/v1/users/{id}/activate. Idempotent.
func (h *UserHandler) Activate(w http.ResponseWriter, r *http.Request) {
	h.setActive(w, r, true)
}

// Deactivate handles POST /api/v1/users/{id}/deactivate. The actor
// cannot deactivate themselves (cannot_deactivate_self).
func (h *UserHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	h.setActive(w, r, false)
}

func (h *UserHandler) setActive(w http.ResponseWriter, r *http.Request, active bool) {
	userID, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	eff, err := h.auth.Effective(r.Context(), actor.User, actor.Layout.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	row, err := h.svc.SetActive(r.Context(), eff, actor.User.ID, userID, active)
	if err != nil {
		writeUserError(w, err)
		return
	}
	pool, err := h.svc.GetDCCPool(r.Context(), row.ID)
	if err != nil {
		writeUserError(w, err)
		return
	}
	if h.audit != nil {
		msg := "audit_user_deactivated"
		if active {
			msg = "audit_user_activated"
		}
		_ = h.audit.Publish(r.Context(), 0,
			cmd.AuditActor{UserID: actor.User.ID, Login: actor.User.Login},
			msg, map[string]string{"target": row.Login})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(protocol.ToUserResponse(row, pool))
}

// Delete handles DELETE /api/v1/users/{id}.
func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	eff, err := h.auth.Effective(r.Context(), actor.User, actor.Layout.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	target, _ := h.svc.Get(r.Context(), userID)
	if err := h.svc.Delete(r.Context(), eff, actor.User.ID, userID); err != nil {
		writeUserError(w, err)
		return
	}
	if h.audit != nil {
		_ = h.audit.Publish(r.Context(), 0,
			cmd.AuditActor{UserID: actor.User.ID, Login: actor.User.Login},
			"audit_user_deleted", map[string]string{"target": target.Login})
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeUserError maps service sentinels to status codes + machine
// codes the frontend can localise.
func writeUserError(w http.ResponseWriter, err error) {
	status, code := svcerrors.UserHTTPStatus(err)
	writeJSONError(w, status, code)
}
