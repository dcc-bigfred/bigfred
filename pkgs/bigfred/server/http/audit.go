package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/server/service"
)

// AuditHandler exposes the audit-log read endpoint.
type AuditHandler struct {
	audit *service.AuditService
}

// NewAuditHandler returns an AuditHandler. audit may be nil (Redis
// unavailable); the handler returns an empty list in that case.
func NewAuditHandler(audit *service.AuditService) *AuditHandler {
	return &AuditHandler{audit: audit}
}

type auditLogResponse struct {
	Entries []contract.AuditEntryWire `json:"entries"`
}

// List handles GET /api/v1/audit-log.
// Query params:
//   - limit: integer, max entries to return (default 200, max 500)
func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	limit := 200
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 500 {
		limit = 500
	}

	if h.audit == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(auditLogResponse{Entries: []contract.AuditEntryWire{}})
		return
	}

	entries, err := h.audit.List(r.Context(), limit)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if entries == nil {
		entries = []contract.AuditEntryWire{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(auditLogResponse{Entries: entries})
}
