package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/service"
)

// PresenceHandler serves GET /api/v1/layouts/{id}/presence.
type PresenceHandler struct {
	svc     *service.PresenceService
	dccSync *service.DccBusLayoutSync
}

// NewPresenceHandler returns a PresenceHandler. dccSync may be nil when
// supervisord / dcc-bus is disabled.
func NewPresenceHandler(svc *service.PresenceService, dccSync *service.DccBusLayoutSync) *PresenceHandler {
	return &PresenceHandler{svc: svc, dccSync: dccSync}
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

	// Dashboard poll (§7e.6): detect command-station attachment changes
	// and refresh supervisord for online layouts.
	if h.dccSync != nil {
		if err := h.dccSync.ObserveLayout(r.Context(), layoutID); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "internal_error")
			return
		}
	}

	users, err := h.svc.ListForLayoutEnsuringCaller(r.Context(), layoutID, id.User)
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
