package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/keskad/loco/pkgs/bigfred/server/service"
)

// DiagnosticsHandler serves admin-only read access to whitelisted
// supervisord / redis / dcc-bus log files.
type DiagnosticsHandler struct {
	svc *service.DiagnosticsService
}

// NewDiagnosticsHandler returns a DiagnosticsHandler.
func NewDiagnosticsHandler(svc *service.DiagnosticsService) *DiagnosticsHandler {
	return &DiagnosticsHandler{svc: svc}
}

// ListSources returns the file whitelist catalogue.
func (h *DiagnosticsHandler) ListSources(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "diagnostics_unavailable")
		return
	}
	src, err := h.svc.Sources()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(src)
}

// ReadContent returns a tail slice of one whitelisted file.
func (h *DiagnosticsHandler) ReadContent(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "diagnostics_unavailable")
		return
	}
	fileID := r.URL.Query().Get("fileId")
	if fileID == "" {
		writeJSONError(w, http.StatusBadRequest, "invalid_query")
		return
	}
	tailLines := 500
	if raw := r.URL.Query().Get("tailLines"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			writeJSONError(w, http.StatusBadRequest, "invalid_query")
			return
		}
		tailLines = n
	}

	content, err := h.svc.Read(fileID, tailLines)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrDiagnosticsForbidden):
			writeJSONError(w, http.StatusForbidden, "diagnostics_forbidden")
		case errors.Is(err, service.ErrDiagnosticsNotFound):
			writeJSONError(w, http.StatusNotFound, "diagnostics_not_found")
		default:
			writeJSONError(w, http.StatusInternalServerError, "internal_error")
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(content)
}
