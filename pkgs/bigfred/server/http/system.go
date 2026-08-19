package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/keskad/loco/pkgs/bigfred/server/service"
)

// SystemHandler serves admin host power control (BigFredOS / microinit init).
type SystemHandler struct {
	svc *service.SystemControl
}

// NewSystemHandler returns a SystemHandler. svc may be nil (503).
func NewSystemHandler(svc *service.SystemControl) *SystemHandler {
	return &SystemHandler{svc: svc}
}

// Get handles GET /api/v1/admin/system.
func (h *SystemHandler) Get(w http.ResponseWriter, _ *http.Request) {
	if h.svc == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "system_unavailable")
		return
	}
	info, err := h.svc.Info()
	if err != nil {
		if errors.Is(err, service.ErrSystemUnavailable) {
			writeJSONError(w, http.StatusServiceUnavailable, "system_unavailable")
			return
		}
		writeJSONErrorCause(w, http.StatusInternalServerError, "internal_error", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(info)
}

type systemShutdownBody struct {
	Mode string `json:"mode"`
}

// Shutdown handles POST /api/v1/admin/system/shutdown.
func (h *SystemHandler) Shutdown(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "system_unavailable")
		return
	}
	var body systemShutdownBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	err := h.svc.RequestShutdown(body.Mode)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidShutdownMode):
			writeJSONError(w, http.StatusBadRequest, "invalid_mode")
		case errors.Is(err, service.ErrSystemNotInit):
			writeJSONError(w, http.StatusConflict, "system_not_init")
		case errors.Is(err, service.ErrSystemUnavailable):
			writeJSONError(w, http.StatusServiceUnavailable, "system_unavailable")
		default:
			writeJSONErrorCause(w, http.StatusInternalServerError, "internal_error", err)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Ports handles GET /api/v1/admin/system/ports.
func (h *SystemHandler) Ports(w http.ResponseWriter, _ *http.Request) {
	if h.svc == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "system_unavailable")
		return
	}
	ports := h.svc.Ports()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ports)
}
