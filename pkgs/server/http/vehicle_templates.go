package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/keskad/loco/pkgs/server/domain"
	"github.com/keskad/loco/pkgs/server/service"
)

// VehicleTemplateHandler serves the template catalogue (§4.1).
type VehicleTemplateHandler struct {
	templates *service.VehicleTemplateService
}

// NewVehicleTemplateHandler returns a VehicleTemplateHandler.
func NewVehicleTemplateHandler(templates *service.VehicleTemplateService) *VehicleTemplateHandler {
	return &VehicleTemplateHandler{templates: templates}
}

type vehicleTemplateFunctionResponse struct {
	Num      uint8              `json:"num"`
	Name     string             `json:"name"`
	Icon     domain.FunctionIcon `json:"icon"`
	Position int                `json:"position"`
}

type vehicleTemplateResponse struct {
	ID          uint                            `json:"id"`
	Name        string                          `json:"name"`
	Description string                          `json:"description"`
	OwnerID     uint                            `json:"ownerId"`
	OwnerLogin  string                          `json:"ownerLogin"`
	Version     int                             `json:"version"`
	Functions   []vehicleTemplateFunctionResponse `json:"functions"`
}

func toVehicleTemplateResp(t service.VehicleTemplateListEntry) vehicleTemplateResponse {
	fns := make([]vehicleTemplateFunctionResponse, 0, len(t.Functions))
	for _, fn := range t.Functions {
		fns = append(fns, vehicleTemplateFunctionResponse{
			Num:      fn.Num,
			Name:     fn.Name,
			Icon:     fn.Icon,
			Position: fn.Position,
		})
	}
	return vehicleTemplateResponse{
		ID:          t.ID,
		Name:        t.Name,
		Description: t.Description,
		OwnerID:     t.OwnerUserID,
		OwnerLogin:  t.OwnerLogin,
		Version:     t.Version,
		Functions:   fns,
	}
}

func toVehicleTemplateRespFromDomain(t domain.VehicleTemplate) vehicleTemplateResponse {
	return vehicleTemplateResponse{
		ID:          t.ID,
		Name:        t.Name,
		Description: t.Description,
		OwnerID:     t.OwnerUserID,
		OwnerLogin:  "",
		Version:     t.Version,
		Functions:   nil,
	}
}

// List handles GET /api/v1/vehicle-templates.
func (h *VehicleTemplateHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.templates.List(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	out := make([]vehicleTemplateResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, toVehicleTemplateResp(row))
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
	_ = json.NewEncoder(w).Encode(toVehicleTemplateRespFromDomain(row))
}

type vehicleTemplateCreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Create handles POST /api/v1/vehicle-templates.
func (h *VehicleTemplateHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req vehicleTemplateCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	row, err := h.templates.Create(r.Context(), service.VehicleTemplateCreateInput{
		OwnerUserID: actor.User.ID,
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		writeVehicleTemplateError(w, err)
		return
	}
	resp := toVehicleTemplateRespFromDomain(row)
	resp.OwnerLogin = actor.User.Login
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

func writeVehicleTemplateError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrVehicleTemplateNotFound):
		writeJSONError(w, http.StatusNotFound, "vehicle_template_not_found")
	case errors.Is(err, service.ErrVehicleTemplateNameRequired):
		writeJSONError(w, http.StatusUnprocessableEntity, "vehicle_template_name_required")
	case errors.Is(err, service.ErrVehicleTemplateNameTaken):
		writeJSONError(w, http.StatusConflict, "vehicle_template_name_taken")
	default:
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
	}
}
