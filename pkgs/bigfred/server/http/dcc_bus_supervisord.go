package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/keskad/loco/pkgs/bigfred/server/service"
)

// DccBusSupervisordHandler exposes admin REST endpoints for the
// dcc-bus supervisord program tied to the caller's session layout
// and a command-station id.
type DccBusSupervisordHandler struct {
	dccBus *service.DccBusService
}

// NewDccBusSupervisordHandler returns a DccBusSupervisordHandler.
func NewDccBusSupervisordHandler(dccBus *service.DccBusService) *DccBusSupervisordHandler {
	return &DccBusSupervisordHandler{dccBus: dccBus}
}

type dccBusSupervisordStatusResponse struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	PID     int    `json:"pid,omitempty"`
	Running bool   `json:"running"`
}

// GetStatus handles GET /api/v1/admin/dcc-bus/{commandStationId}/supervisord.
func (h *DccBusSupervisordHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	layoutID, csID, ok := h.requireSessionCS(w, r)
	if !ok {
		return
	}
	st, err := h.dccBus.ProgramStatus(r.Context(), layoutID, csID)
	if err != nil {
		if errors.Is(err, service.ErrSupervisordNotWired) {
			writeJSONError(w, http.StatusServiceUnavailable, "service_unavailable")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	running := strings.EqualFold(st.Status, "RUNNING")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(dccBusSupervisordStatusResponse{
		Name:    st.Name,
		Status:  st.Status,
		PID:     st.PID,
		Running: running,
	})
}

// Action handles POST /api/v1/admin/dcc-bus/{commandStationId}/supervisord/{action}.
// action is one of start|stop|restart.
func (h *DccBusSupervisordHandler) Action(w http.ResponseWriter, r *http.Request) {
	layoutID, csID, ok := h.requireSessionCS(w, r)
	if !ok {
		return
	}
	action := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "action")))
	var err error
	switch action {
	case "start":
		err = h.dccBus.StartDccBus(r.Context(), layoutID, csID)
	case "stop":
		err = h.dccBus.StopDccBus(r.Context(), layoutID, csID)
	case "restart":
		err = h.dccBus.RestartDccBus(r.Context(), layoutID, csID)
	default:
		writeJSONError(w, http.StatusBadRequest, "invalid_action")
		return
	}
	if err != nil {
		if errors.Is(err, service.ErrSupervisordNotWired) {
			writeJSONError(w, http.StatusServiceUnavailable, "service_unavailable")
			return
		}
		writeJSONError(w, http.StatusUnprocessableEntity, "action_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DccBusSupervisordHandler) requireSessionCS(w http.ResponseWriter, r *http.Request) (layoutID, csID uint, ok bool) {
	if h.dccBus == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "service_unavailable")
		return 0, 0, false
	}
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return 0, 0, false
	}
	if actor.Layout.ID == 0 {
		writeJSONError(w, http.StatusUnprocessableEntity, "layout_mismatch")
		return 0, 0, false
	}
	csID, ok = parseUintParam(r, "commandStationId")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return 0, 0, false
	}
	return actor.Layout.ID, csID, true
}
