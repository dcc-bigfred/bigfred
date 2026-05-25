package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/keskad/loco/pkgs/server/domain"
	"github.com/keskad/loco/pkgs/server/security"
	"github.com/keskad/loco/pkgs/server/service"
)

// UserHandler bundles the admin-only user-management endpoints under
// `/api/v1/users` (§4.1 / §7a.5). The chi router is responsible for
// wrapping every route with RequireRole(domain.RoleAdmin) — this
// handler also re-runs the self-action guards (no self-deactivate /
// no self-delete) through `security.UserSecurityContext`, so a
// misrouted endpoint still fails closed.
type UserHandler struct {
	svc *service.UserService
	sec security.UserSecurityContext
}

// NewUserHandler returns a UserHandler.
func NewUserHandler(svc *service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

// userResponse is the JSON shape returned by the user-management
// endpoints. `pinHash` is deliberately omitted — the plaintext PIN
// has never been recoverable and the hash is useless to the UI.
type userResponse struct {
	ID        uint        `json:"id"`
	Login     string      `json:"login"`
	Role      domain.Role `json:"role"`
	Active    bool        `json:"active"`
	CreatedAt time.Time   `json:"createdAt"`
	UpdatedAt time.Time   `json:"updatedAt"`
}

func toUserResponse(u domain.User) userResponse {
	return userResponse{
		ID:        u.ID,
		Login:     u.Login,
		Role:      u.Role,
		Active:    u.Active,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

// List handles GET /api/v1/users.
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.List(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	out := make([]userResponse, 0, len(rows))
	for _, u := range rows {
		out = append(out, toUserResponse(u))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

type userCreateRequest struct {
	Login string      `json:"login"`
	PIN   string      `json:"pin"`
	Role  domain.Role `json:"role"`
}

// Create handles POST /api/v1/users.
func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req userCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	row, err := h.svc.Create(r.Context(), service.UserCreateInput{
		Login: req.Login,
		PIN:   req.PIN,
		Role:  req.Role,
	})
	if err != nil {
		writeUserError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toUserResponse(row))
}

// userUpdateRequest mirrors the optional fields exposed by
// UserService.Update. `pin`, if present and non-empty, rotates the
// user's PIN in one shot — separating it into a dedicated endpoint
// would not buy any safety because the admin already has full
// authority over the row.
type userUpdateRequest struct {
	Login *string      `json:"login"`
	Role  *domain.Role `json:"role"`
	PIN   *string      `json:"pin"`
}

// Update handles PUT /api/v1/users/{id}.
func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	var req userUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	// Treat an explicit empty-string PIN as "leave alone" so a
	// dialog that always submits the field doesn't reset the hash
	// to garbage. Only a non-empty value triggers re-hashing.
	if req.PIN != nil && *req.PIN == "" {
		req.PIN = nil
	}
	row, err := h.svc.Update(r.Context(), userID, service.UserUpdateInput{
		Login: req.Login,
		Role:  req.Role,
		PIN:   req.PIN,
	})
	if err != nil {
		writeUserError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toUserResponse(row))
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
	target, err := h.svc.Get(r.Context(), userID)
	if err != nil {
		writeUserError(w, err)
		return
	}
	if !active {
		if d := h.sec.CanDeactivateSelf(actor.User, target); !d.Allowed {
			writeJSONError(w, http.StatusUnprocessableEntity, d.Reason)
			return
		}
	}
	row, err := h.svc.SetActive(r.Context(), userID, active)
	if err != nil {
		writeUserError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toUserResponse(row))
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
	target, err := h.svc.Get(r.Context(), userID)
	if err != nil {
		writeUserError(w, err)
		return
	}
	if d := h.sec.CanDeleteSelf(actor.User, target); !d.Allowed {
		writeJSONError(w, http.StatusUnprocessableEntity, d.Reason)
		return
	}
	if err := h.svc.Delete(r.Context(), userID); err != nil {
		writeUserError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeUserError maps service sentinels to status codes + machine
// codes the frontend can localise.
func writeUserError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrUserNotFound):
		writeJSONError(w, http.StatusNotFound, "user_not_found")
	case errors.Is(err, service.ErrUserLoginRequired):
		writeJSONError(w, http.StatusUnprocessableEntity, "user_login_required")
	case errors.Is(err, service.ErrUserLoginInvalid):
		writeJSONError(w, http.StatusUnprocessableEntity, "user_login_invalid")
	case errors.Is(err, service.ErrUserLoginTaken):
		writeJSONError(w, http.StatusConflict, "user_login_taken")
	case errors.Is(err, service.ErrUserPINRequired):
		writeJSONError(w, http.StatusUnprocessableEntity, "user_pin_required")
	case errors.Is(err, service.ErrUserPINInvalid):
		writeJSONError(w, http.StatusUnprocessableEntity, "user_pin_invalid")
	case errors.Is(err, service.ErrUserRoleInvalid):
		writeJSONError(w, http.StatusUnprocessableEntity, "user_role_invalid")
	case errors.Is(err, service.ErrUserHasVehicles):
		writeJSONError(w, http.StatusConflict, "user_has_vehicles")
	case errors.Is(err, service.ErrUserHasTrains):
		writeJSONError(w, http.StatusConflict, "user_has_trains")
	case errors.Is(err, service.ErrCannotDeactivateSelf):
		writeJSONError(w, http.StatusUnprocessableEntity, "cannot_deactivate_self")
	case errors.Is(err, service.ErrCannotDeleteSelf):
		writeJSONError(w, http.StatusUnprocessableEntity, "cannot_delete_self")
	default:
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
	}
}
