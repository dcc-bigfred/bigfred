package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/protocol"
)

// CommandStationHandler bundles REST endpoints for the command-station
// catalogue.
type CommandStationHandler struct {
	svc  *cmd.CommandStation
	auth *cmd.Auth
}

// NewCommandStationHandler returns a CommandStationHandler.
func NewCommandStationHandler(svc *cmd.CommandStation, auth *cmd.Auth) *CommandStationHandler {
	return &CommandStationHandler{svc: svc, auth: auth}
}

// ListCatalogue handles GET /api/v1/command-stations/catalogue (admin).
func (h *CommandStationHandler) ListCatalogue(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.ListAll(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	out := make([]protocol.CommandStationResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, protocol.ToCommandStationResponse(row))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// Create handles POST /api/v1/command-stations (admin).
func (h *CommandStationHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	eff, err := h.auth.Effective(r.Context(), actor.User, actor.Layout.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	var req protocol.CommandStationCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	row, err := h.svc.Create(r.Context(), eff, req.ToCreateInput())
	if err != nil {
		writeCommandStationError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(protocol.ToCommandStationResponse(row))
}

// Update handles PUT /api/v1/command-stations/{id} (admin).
func (h *CommandStationHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	eff, err := h.auth.Effective(r.Context(), actor.User, actor.Layout.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	var req protocol.CommandStationUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	row, err := h.svc.Update(r.Context(), eff, id, req.ToUpdateInput())
	if err != nil {
		writeCommandStationError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(protocol.ToCommandStationResponse(row))
}

// Delete handles DELETE /api/v1/command-stations/{id} (admin).
func (h *CommandStationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	eff, err := h.auth.Effective(r.Context(), actor.User, actor.Layout.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if err := h.svc.Delete(r.Context(), eff, id); err != nil {
		writeCommandStationError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeCommandStationError(w http.ResponseWriter, err error) {
	status, code := svcerrors.CommandStationHTTPStatus(err)
	writeJSONError(w, status, code)
}
