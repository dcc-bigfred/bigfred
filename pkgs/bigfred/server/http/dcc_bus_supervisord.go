package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/keskad/loco/pkgs/bigfred/server/service"
)

// DccBusSupervisordHandler exposes admin REST endpoints for dcc-bus
// supervisord programs tied to a command-station id (across layouts).
type DccBusSupervisordHandler struct {
	dccBus *service.DccBusService
}

// NewDccBusSupervisordHandler returns a DccBusSupervisordHandler.
func NewDccBusSupervisordHandler(dccBus *service.DccBusService) *DccBusSupervisordHandler {
	return &DccBusSupervisordHandler{dccBus: dccBus}
}

type dccBusSupervisordListResponse struct {
	Programs []service.DccBusProgramStatus `json:"programs"`
}

// GetStatus handles GET /api/v1/admin/dcc-bus/{commandStationId}/supervisord.
// Returns every dcc-bus program for the command station across all layouts.
func (h *DccBusSupervisordHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	csID, ok := h.requireAdminCS(w, r)
	if !ok {
		return
	}
	programs, err := h.dccBus.ProgramsForCommandStation(r.Context(), csID)
	if err != nil {
		if errors.Is(err, service.ErrSupervisordNotWired) {
			writeJSONError(w, http.StatusServiceUnavailable, "service_unavailable")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if programs == nil {
		programs = []service.DccBusProgramStatus{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(dccBusSupervisordListResponse{Programs: programs})
}

// Action handles POST /api/v1/admin/dcc-bus/{commandStationId}/supervisord/{action}?layoutId=N.
// action is one of start|stop|restart. layoutId selects which layout's program to control.
func (h *DccBusSupervisordHandler) Action(w http.ResponseWriter, r *http.Request) {
	csID, ok := h.requireAdminCS(w, r)
	if !ok {
		return
	}
	layoutID, ok := parseLayoutIDQuery(r)
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_layout")
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":   "action_failed",
			"message": err.Error(),
		})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DccBusSupervisordHandler) requireAdminCS(w http.ResponseWriter, r *http.Request) (csID uint, ok bool) {
	if h.dccBus == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "service_unavailable")
		return 0, false
	}
	_, ok = IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return 0, false
	}
	csID, ok = parseUintParam(r, "commandStationId")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return 0, false
	}
	return csID, true
}

func parseLayoutIDQuery(r *http.Request) (uint, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("layoutId"))
	if raw == "" {
		return 0, false
	}
	n, err := strconv.ParseUint(raw, 10, 32)
	if err != nil || n == 0 {
		return 0, false
	}
	return uint(n), true
}
