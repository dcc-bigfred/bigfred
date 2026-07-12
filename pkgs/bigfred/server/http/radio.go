package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/service"
)

// RadioHandler exposes read-only radio replay REST shims (§4.1).
type RadioHandler struct {
	radio *service.RadioService
}

// NewRadioHandler returns a RadioHandler.
func NewRadioHandler(radio *service.RadioService) *RadioHandler {
	return &RadioHandler{radio: radio}
}

type radioMessageResponse struct {
	Messages []contract.RadioMessageWire `json:"messages"`
}

// ReplayInterlocking handles GET /api/v1/interlockings/{id}/radio.
func (h *RadioHandler) ReplayInterlocking(w http.ResponseWriter, r *http.Request) {
	id, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.radio == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "radio_not_configured")
		return
	}
	ilkID, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}

	rows, err := h.radio.ReplayInterlocking(r.Context(), id.Layout.ID, ilkID, id.User.ID, parseRadioLimit(r, h.radio))
	if err != nil {
		writeRadioError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toRadioResponse(rows))
}

// ReplayMine handles GET /api/v1/radio/mine.
func (h *RadioHandler) ReplayMine(w http.ResponseWriter, r *http.Request) {
	id, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.radio == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "radio_not_configured")
		return
	}

	rows, err := h.radio.ReplayUser(r.Context(), id.Layout.ID, id.User.ID, parseRadioLimit(r, h.radio))
	if err != nil {
		writeRadioError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toRadioResponse(rows))
}

func parseRadioLimit(r *http.Request, radio *service.RadioService) int {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func toRadioResponse(rows []domain.RadioMessage) radioMessageResponse {
	out := radioMessageResponse{Messages: make([]contract.RadioMessageWire, 0, len(rows))}
	for _, row := range rows {
		out.Messages = append(out.Messages, contract.MessageWireFromDomain(row))
	}
	return out
}

func writeRadioError(w http.ResponseWriter, err error) {
	code := service.RadioDeniedCode(err)
	switch code {
	case "not_signalman", "not_interlocking_occupant":
		writeJSONError(w, http.StatusForbidden, code)
	case "radio_chat_disabled":
		writeJSONError(w, http.StatusForbidden, code)
	case "radio_invalid_target", "radio_invalid_context", "radio_invalid_phrase", "radio_note_too_long":
		writeJSONError(w, http.StatusUnprocessableEntity, code)
	default:
		writeJSONError(w, http.StatusBadRequest, code)
	}
}
