package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/service"
)

// CommandStationHandler bundles REST endpoints for the command-station
// catalogue.
type CommandStationHandler struct {
	svc  *service.CommandStationService
	auth *service.AuthService
}

// NewCommandStationHandler returns a CommandStationHandler.
func NewCommandStationHandler(svc *service.CommandStationService, auth *service.AuthService) *CommandStationHandler {
	return &CommandStationHandler{svc: svc, auth: auth}
}

type commandStationResponse struct {
	ID            uint                      `json:"id"`
	Name          string                    `json:"name"`
	Kind          domain.CommandStationKind `json:"kind"`
	ConnectionURI string                    `json:"connectionUri"`
	SpeedSteps    uint                      `json:"speedSteps"`
}

func toCommandStationResponse(cs domain.CommandStation) commandStationResponse {
	return commandStationResponse{
		ID:            cs.ID,
		Name:          cs.Name,
		Kind:          cs.Kind,
		ConnectionURI: cs.ConnectionURI,
		SpeedSteps:    cs.SpeedSteps,
	}
}

// ListCatalogue handles GET /api/v1/command-stations/catalogue (admin).
func (h *CommandStationHandler) ListCatalogue(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.ListAll(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	out := make([]commandStationResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, toCommandStationResponse(row))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

type commandStationCreateRequest struct {
	Name          string                    `json:"name"`
	Kind          domain.CommandStationKind `json:"kind"`
	ConnectionURI string                    `json:"connectionUri"`
	SpeedSteps    uint                      `json:"speedSteps"`
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
	var req commandStationCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	row, err := h.svc.Create(r.Context(), eff, service.CommandStationCreateInput{
		Name:          req.Name,
		Kind:          req.Kind,
		ConnectionURI: req.ConnectionURI,
		SpeedSteps:    req.SpeedSteps,
	})
	if err != nil {
		writeCommandStationError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toCommandStationResponse(row))
}

type commandStationUpdateRequest struct {
	Name          *string                    `json:"name"`
	Kind          *domain.CommandStationKind `json:"kind"`
	ConnectionURI *string                    `json:"connectionUri"`
	SpeedSteps    *uint                      `json:"speedSteps"`
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
	var req commandStationUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	row, err := h.svc.Update(r.Context(), eff, id, service.CommandStationUpdateInput{
		Name:          req.Name,
		Kind:          req.Kind,
		ConnectionURI: req.ConnectionURI,
		SpeedSteps:    req.SpeedSteps,
	})
	if err != nil {
		writeCommandStationError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toCommandStationResponse(row))
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
	switch {
	case errors.Is(err, service.ErrCommandStationNotFound):
		writeJSONError(w, http.StatusNotFound, "command_station_not_found")
	case errors.Is(err, service.ErrCommandStationNameTaken):
		writeJSONError(w, http.StatusConflict, "command_station_name_taken")
	case errors.Is(err, service.ErrCommandStationNameRequired):
		writeJSONError(w, http.StatusUnprocessableEntity, "command_station_name_required")
	case errors.Is(err, service.ErrCommandStationKindInvalid):
		writeJSONError(w, http.StatusUnprocessableEntity, "command_station_kind_invalid")
	case errors.Is(err, service.ErrCommandStationSpeedInvalid):
		writeJSONError(w, http.StatusUnprocessableEntity, "command_station_speed_steps_invalid")
	case errors.Is(err, service.ErrLayoutNeedsAtLeastOneCommandStation):
		writeJSONError(w, http.StatusConflict, "layout_needs_at_least_one_command_station")
	case errors.Is(err, service.ErrCommandStationForbidden):
		writeJSONError(w, http.StatusForbidden, "forbidden")
	default:
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
	}
}
