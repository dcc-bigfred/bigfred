package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/keskad/loco/pkgs/server/domain"
	"github.com/keskad/loco/pkgs/server/service"
)

// PresenceHandler serves GET /api/v1/layouts/{id}/presence.
type PresenceHandler struct {
	svc *service.PresenceService
}

// NewPresenceHandler returns a PresenceHandler.
func NewPresenceHandler(svc *service.PresenceService) *PresenceHandler {
	return &PresenceHandler{svc: svc}
}

// List handles GET /api/v1/layouts/{id}/presence.
func (h *PresenceHandler) List(w http.ResponseWriter, r *http.Request) {
	layoutID, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	id, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if id.Layout.ID != layoutID {
		writeJSONError(w, http.StatusForbidden, "layout_mismatch")
		return
	}

	users, err := h.svc.ListForLayout(r.Context(), layoutID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if users == nil {
		users = []domain.PresenceUser{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(users)
}
