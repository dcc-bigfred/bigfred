package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/protocol"
	"github.com/keskad/loco/pkgs/bigfred/server/service"
)

// InterlockingHandler bundles REST endpoints for the interlocking
// catalogue and layout-scoped listing.
type InterlockingHandler struct {
	svc       *cmd.Interlocking
	occupancy *service.InterlockingOccupancyService
	auth      *cmd.Auth
}

// NewInterlockingHandler returns an InterlockingHandler.
func NewInterlockingHandler(
	svc *cmd.Interlocking,
	occupancy *service.InterlockingOccupancyService,
	auth *cmd.Auth,
) *InterlockingHandler {
	return &InterlockingHandler{svc: svc, occupancy: occupancy, auth: auth}
}

func toInterlockingWithOccupant(row service.InterlockingWithOccupant) protocol.InterlockingResponse {
	out := protocol.ToInterlockingResponse(row.Interlocking)
	if row.Occupant != nil {
		out.Occupant = &protocol.OccupantResponse{
			UserID: row.Occupant.UserID,
			Login:  row.Occupant.Login,
		}
	}
	return out
}

// List handles GET /api/v1/interlockings — filtered to the caller's
// active layout with occupant enrichment (§4.1 / §6.3c).
func (h *InterlockingHandler) List(w http.ResponseWriter, r *http.Request) {
	id, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	rows, err := h.occupancy.ListForLayout(r.Context(), id.Layout.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	out := make([]protocol.InterlockingResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, toInterlockingWithOccupant(row))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// ListCatalogue handles GET /api/v1/interlockings/catalogue (admin
// only) — the full catalogue for the admin CRUD screen.
func (h *InterlockingHandler) ListCatalogue(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.ListAll(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	out := make([]protocol.InterlockingResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, protocol.ToInterlockingResponse(row))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// Get handles GET /api/v1/interlockings/{id}.
// Get handles GET /api/v1/interlockings/{id}. The response is scoped
// to the caller's active layout (§6.3d): the row is returned with
// `occupant` filled when present, and 404 when the box is not on the
// layout's whitelist — admins included, since the interlocking view
// shows occupation actions that only make sense inside a layout.
func (h *InterlockingHandler) Get(w http.ResponseWriter, r *http.Request) {
	interlockingID, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	id, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	row, err := h.occupancy.GetForLayout(r.Context(), id.Layout.ID, interlockingID)
	if err != nil {
		switch {
		case errors.Is(err, svcerrors.ErrInterlockingNotFound),
			errors.Is(err, service.ErrInterlockingNotInLayout):
			writeJSONError(w, http.StatusNotFound, "interlocking_not_found")
		default:
			writeJSONError(w, http.StatusInternalServerError, "internal_error")
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toInterlockingWithOccupant(row))
}

// Create handles POST /api/v1/interlockings (admin only).
func (h *InterlockingHandler) Create(w http.ResponseWriter, r *http.Request) {
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
	var req protocol.InterlockingCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	row, err := h.svc.Create(r.Context(), eff, req.ToCreateInput())
	if err != nil {
		writeInterlockingError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(protocol.ToInterlockingResponse(row))
}

// Update handles PUT /api/v1/interlockings/{id} (admin only).
func (h *InterlockingHandler) Update(w http.ResponseWriter, r *http.Request) {
	interlockingID, ok := parseUintParam(r, "id")
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
	var req protocol.InterlockingUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	row, err := h.svc.Update(r.Context(), eff, interlockingID, req.ToUpdateInput())
	if err != nil {
		writeInterlockingError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(protocol.ToInterlockingResponse(row))
}

// Delete handles DELETE /api/v1/interlockings/{id} (admin only).
func (h *InterlockingHandler) Delete(w http.ResponseWriter, r *http.Request) {
	interlockingID, ok := parseUintParam(r, "id")
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
	if err := h.svc.Delete(r.Context(), eff, interlockingID); err != nil {
		writeInterlockingError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type interlockingJoinRequest struct {
	Force bool `json:"force"`
}

// Join handles POST /api/v1/interlockings/{id}/join (signalman).
func (h *InterlockingHandler) Join(w http.ResponseWriter, r *http.Request) {
	interlockingID, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	id, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	isSignalman, err := h.auth.IsEffectiveSignalman(r.Context(), id.User, id.Layout.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if !isSignalman {
		writeJSONError(w, http.StatusForbidden, "not_signalman")
		return
	}

	var req interlockingJoinRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_body")
			return
		}
	}

	result, err := h.occupancy.Join(r.Context(), service.JoinInput{
		InterlockingID: interlockingID,
		LayoutID:       id.Layout.ID,
		Actor:          id.User,
		Force:          req.Force,
	})
	if err != nil {
		writeInterlockingOccupancyError(w, err)
		return
	}

	resp := toInterlockingWithOccupant(service.InterlockingWithOccupant{
		Interlocking: result.Interlocking,
		Occupant:     &result.Occupant,
	})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// Leave handles POST /api/v1/interlockings/{id}/leave. No ongoing
// signalman/admin role is required — the service only ends a session
// when the caller is its current occupant, so a user who staffed the
// box under a since-expired sudo elevation can still release it.
func (h *InterlockingHandler) Leave(w http.ResponseWriter, r *http.Request) {
	interlockingID, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	id, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.occupancy.Leave(r.Context(), interlockingID, id.Layout.ID, id.User); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeInterlockingOccupancyError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, svcerrors.ErrInterlockingNotFound):
		writeJSONError(w, http.StatusNotFound, "interlocking_not_found")
	case errors.Is(err, service.ErrInterlockingOccupied):
		writeJSONError(w, http.StatusConflict, "interlocking_occupied")
	case errors.Is(err, service.ErrInterlockingNotInLayout):
		writeJSONError(w, http.StatusUnprocessableEntity, "interlocking_not_in_layout")
	case errors.Is(err, service.ErrNotSignalman):
		writeJSONError(w, http.StatusForbidden, "not_signalman")
	default:
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
	}
}

func writeInterlockingError(w http.ResponseWriter, err error) {
	status, code := svcerrors.InterlockingHTTPStatus(err)
	writeJSONError(w, status, code)
}
