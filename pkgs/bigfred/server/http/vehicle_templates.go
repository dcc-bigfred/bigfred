package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/protocol"
)

// VehicleTemplateHandler serves the template catalogue (§4.1).
type VehicleTemplateHandler struct {
	templates *cmd.VehicleTemplate
	auth      *cmd.Auth
}

// NewVehicleTemplateHandler returns a VehicleTemplateHandler.
func NewVehicleTemplateHandler(
	templates *cmd.VehicleTemplate,
	auth *cmd.Auth,
) *VehicleTemplateHandler {
	return &VehicleTemplateHandler{templates: templates, auth: auth}
}

// List handles GET /api/v1/vehicle-templates.
func (h *VehicleTemplateHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.templates.List(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	out := make([]protocol.VehicleTemplateResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, protocol.ToVehicleTemplateResponse(row))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// Get handles GET /api/v1/vehicle-templates/{id}.
func (h *VehicleTemplateHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	row, err := h.templates.Get(r.Context(), id)
	if err != nil {
		writeVehicleTemplateError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(protocol.ToVehicleTemplateResponseFromDomain(row))
}

// Create handles POST /api/v1/vehicle-templates.
func (h *VehicleTemplateHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req protocol.VehicleTemplateCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	row, err := h.templates.Create(r.Context(), req.ToCreateInput(actor.User.ID))
	if err != nil {
		writeVehicleTemplateError(w, err)
		return
	}
	resp := protocol.ToVehicleTemplateResponseFromDomain(row)
	resp.OwnerLogin = actor.User.Login
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// Update handles PUT /api/v1/vehicle-templates/{id}.
func (h *VehicleTemplateHandler) Update(w http.ResponseWriter, r *http.Request) {
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
	id, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	var req protocol.VehicleTemplateUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	row, err := h.templates.Update(r.Context(), actor.User.ID, eff, id, req.ToUpdateInput())
	if err != nil {
		writeVehicleTemplateError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(protocol.ToVehicleTemplateResponse(row))
}

func writeVehicleTemplateError(w http.ResponseWriter, err error) {
	status, code := svcerrors.VehicleTemplateHTTPStatus(err)
	writeJSONError(w, status, code)
}
