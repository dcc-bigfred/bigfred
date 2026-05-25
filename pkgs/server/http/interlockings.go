package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/keskad/loco/pkgs/server/domain"
	"github.com/keskad/loco/pkgs/server/service"
)

// InterlockingHandler bundles REST endpoints for the interlocking
// catalogue and layout-scoped listing.
type InterlockingHandler struct {
	svc *service.InterlockingService
}

// NewInterlockingHandler returns an InterlockingHandler.
func NewInterlockingHandler(svc *service.InterlockingService) *InterlockingHandler {
	return &InterlockingHandler{svc: svc}
}

type interlockingResponse struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	Location string `json:"location"`
}

func toInterlockingResponse(i domain.Interlocking) interlockingResponse {
	return interlockingResponse{
		ID:       i.ID,
		Name:     i.Name,
		Location: i.Location,
	}
}

// List handles GET /api/v1/interlockings. Admins receive the full
// catalogue; everyone else receives only rows whitelisted for their
// active layout (§4.1).
func (h *InterlockingHandler) List(w http.ResponseWriter, r *http.Request) {
	id, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var rows []domain.Interlocking
	var err error
	if id.HasRole(domain.RoleAdmin) {
		rows, err = h.svc.ListAll(r.Context())
	} else {
		rows, err = h.svc.ListForLayout(r.Context(), id.Layout.ID)
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	out := make([]interlockingResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, toInterlockingResponse(row))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// Get handles GET /api/v1/interlockings/{id}.
func (h *InterlockingHandler) Get(w http.ResponseWriter, r *http.Request) {
	interlockingID, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	row, err := h.svc.Get(r.Context(), interlockingID)
	if err != nil {
		writeInterlockingError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toInterlockingResponse(row))
}

type interlockingCreateRequest struct {
	Name     string `json:"name"`
	Location string `json:"location"`
}

// Create handles POST /api/v1/interlockings (admin only).
func (h *InterlockingHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req interlockingCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	row, err := h.svc.Create(r.Context(), service.InterlockingCreateInput{
		Name:     req.Name,
		Location: req.Location,
	})
	if err != nil {
		writeInterlockingError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toInterlockingResponse(row))
}

type interlockingUpdateRequest struct {
	Name     *string `json:"name"`
	Location *string `json:"location"`
}

// Update handles PUT /api/v1/interlockings/{id} (admin only).
func (h *InterlockingHandler) Update(w http.ResponseWriter, r *http.Request) {
	interlockingID, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	var req interlockingUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	row, err := h.svc.Update(r.Context(), interlockingID, service.InterlockingUpdateInput{
		Name:     req.Name,
		Location: req.Location,
	})
	if err != nil {
		writeInterlockingError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toInterlockingResponse(row))
}

// Delete handles DELETE /api/v1/interlockings/{id} (admin only).
func (h *InterlockingHandler) Delete(w http.ResponseWriter, r *http.Request) {
	interlockingID, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	if err := h.svc.Delete(r.Context(), interlockingID); err != nil {
		writeInterlockingError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeInterlockingError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInterlockingNotFound):
		writeJSONError(w, http.StatusNotFound, "interlocking_not_found")
	case errors.Is(err, service.ErrInterlockingNameTaken):
		writeJSONError(w, http.StatusConflict, "interlocking_name_taken")
	case errors.Is(err, service.ErrInterlockingNameRequired):
		writeJSONError(w, http.StatusUnprocessableEntity, "interlocking_name_required")
	default:
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
	}
}
