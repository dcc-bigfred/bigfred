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

// DccBusServicesHandler exposes admin REST endpoints for dcc-bus
// services programs tied to a command-station id (across layouts).
type DccBusServicesHandler struct {
	dccBus *service.DccBusService
}

// NewDccBusServicesHandler returns a DccBusServicesHandler.
func NewDccBusServicesHandler(dccBus *service.DccBusService) *DccBusServicesHandler {
	return &DccBusServicesHandler{dccBus: dccBus}
}

type dccBusServicesListResponse struct {
	Programs []service.DccBusProgramStatus `json:"programs"`
}

// GetStatus handles GET /api/v1/admin/dcc-bus/{commandStationId}/services.
// Returns every dcc-bus program for the command station across all layouts.
func (h *DccBusServicesHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	csID, ok := h.requireAdminCS(w, r)
	if !ok {
		return
	}
	programs, err := h.dccBus.ProgramsForCommandStation(r.Context(), csID)
	if err != nil {
		if errors.Is(err, service.ErrServicesNotWired) {
			writeJSONError(w, http.StatusServiceUnavailable, "service_unavailable")
			return
		}
		writeJSONErrorCause(w, http.StatusInternalServerError, "internal_error", err)
		return
	}
	if programs == nil {
		programs = []service.DccBusProgramStatus{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(dccBusServicesListResponse{Programs: programs})
}

// Action handles POST /api/v1/admin/dcc-bus/{commandStationId}/services/{action}?layoutId=N.
// action is one of start|stop|restart. layoutId selects which layout's program to control.
func (h *DccBusServicesHandler) Action(w http.ResponseWriter, r *http.Request) {
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
		if errors.Is(err, service.ErrServicesNotWired) {
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

func (h *DccBusServicesHandler) requireAdminCS(w http.ResponseWriter, r *http.Request) (csID uint, ok bool) {
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
