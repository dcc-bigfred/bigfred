package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/service"
)

// FunctionHandler serves function list/edit for vehicles and templates
// through the same handler type (§6.3e, §4.1).
type FunctionHandler struct {
	functions *service.FunctionService
	auth      *service.AuthService
}

// NewFunctionHandler returns a FunctionHandler.
func NewFunctionHandler(functions *service.FunctionService, auth *service.AuthService) *FunctionHandler {
	return &FunctionHandler{functions: functions, auth: auth}
}

type functionResponse struct {
	Num      uint8              `json:"num"`
	Name     string             `json:"name"`
	Icon     domain.FunctionIcon `json:"icon"`
	Position int                `json:"position"`
	Source   string             `json:"source,omitempty"`
}

type functionUpsertRequest struct {
	Name     string              `json:"name"`
	Icon     domain.FunctionIcon `json:"icon"`
	Position int                 `json:"position"`
}

type functionReorderRequest struct {
	Positions []functionReorderEntry `json:"positions"`
}

type functionReorderEntry struct {
	Num      uint8 `json:"num"`
	Position int   `json:"position"`
}

type functionIconResponse struct {
	Icon string `json:"icon"`
}

func toFunctionResponses(rows []service.ResolvedFunction) []functionResponse {
	out := make([]functionResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, functionResponse{
			Num:      r.Num,
			Name:     r.Name,
			Icon:     r.Icon,
			Position: r.Position,
			Source:   r.Source,
		})
	}
	return out
}

type functionCatalogueEntryResponse struct {
	VehicleID   uint               `json:"vehicleId"`
	VehicleName string             `json:"vehicleName"`
	OwnerID     uint               `json:"ownerId"`
	OwnerLogin  string             `json:"ownerLogin"`
	DCCAddress  *uint16            `json:"dccAddress"`
	Kind        domain.VehicleKind `json:"kind"`
	Functions   []functionResponse `json:"functions"`
}

// ListCatalogue handles GET /api/v1/vehicles/function-catalogue.
func (h *FunctionHandler) ListCatalogue(w http.ResponseWriter, r *http.Request) {
	rows, err := h.functions.ListFunctionCatalogue(r.Context())
	if err != nil {
		writeFunctionError(w, err)
		return
	}
	out := make([]functionCatalogueEntryResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, functionCatalogueEntryResponse{
			VehicleID:   row.VehicleID,
			VehicleName: row.VehicleName,
			OwnerID:     row.OwnerID,
			OwnerLogin:  row.OwnerLogin,
			DCCAddress:  row.DCCAddress,
			Kind:        row.Kind,
			Functions:   toFunctionResponses(row.Functions),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// ListIcons handles GET /api/v1/function-icons.
func (h *FunctionHandler) ListIcons(w http.ResponseWriter, r *http.Request) {
	icons := h.functions.ListIcons()
	out := make([]functionIconResponse, 0, len(icons))
	for _, icon := range icons {
		out = append(out, functionIconResponse{Icon: string(icon)})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// --- Vehicle routes (/vehicles/{id}/functions) ---

// ListVehicle handles GET /api/v1/vehicles/{id}/functions.
func (h *FunctionHandler) ListVehicle(w http.ResponseWriter, r *http.Request) {
	vehicleID, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	rows, err := h.functions.ListForVehicle(r.Context(), vehicleID)
	if err != nil {
		writeFunctionError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toFunctionResponses(rows))
}

// UpsertVehicle handles PUT /api/v1/vehicles/{id}/functions/{num}.
func (h *FunctionHandler) UpsertVehicle(w http.ResponseWriter, r *http.Request) {
	vehicleID, num, actorID, ok := h.parseVehicleMutation(w, r)
	if !ok {
		return
	}
	var req functionUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	row, err := h.functions.UpsertVehicleSlot(r.Context(), actorID, vehicleID, num, toUpsertInput(req))
	if err != nil {
		writeFunctionError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toDccFunctionResponse(row, "vehicle"))
}

// DeleteVehicle handles DELETE /api/v1/vehicles/{id}/functions/{num}.
func (h *FunctionHandler) DeleteVehicle(w http.ResponseWriter, r *http.Request) {
	vehicleID, num, actorID, ok := h.parseVehicleMutation(w, r)
	if !ok {
		return
	}
	if err := h.functions.DeleteVehicleSlot(r.Context(), actorID, vehicleID, num); err != nil {
		writeFunctionError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type functionReplaceFromRequest struct {
	TemplateID      uint `json:"templateId"`
	SourceVehicleID uint `json:"sourceVehicleId"`
}

// AttachVehicle handles POST /api/v1/vehicles/{id}/functions/attach.
// Body must set exactly one of templateId (re-link to template) or
// sourceVehicleId (copy snapshot from another vehicle).
func (h *FunctionHandler) AttachVehicle(w http.ResponseWriter, r *http.Request) {
	vehicleID, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req functionReplaceFromRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	fromTemplate := req.TemplateID != 0
	fromVehicle := req.SourceVehicleID != 0
	if fromTemplate == fromVehicle {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	var (
		rows []service.ResolvedFunction
		err  error
	)
	if fromTemplate {
		rows, err = h.functions.AttachVehicleToTemplate(
			r.Context(), actor.User.ID, vehicleID, req.TemplateID,
		)
	} else {
		rows, err = h.functions.CopyVehicleFunctionsFromVehicle(
			r.Context(), actor.User.ID, vehicleID, req.SourceVehicleID,
		)
	}
	if err != nil {
		writeFunctionError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toFunctionResponses(rows))
}

// ReorderVehicle handles POST /api/v1/vehicles/{id}/functions/reorder.
func (h *FunctionHandler) ReorderVehicle(w http.ResponseWriter, r *http.Request) {
	vehicleID, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	entries, ok := h.decodeReorder(w, r)
	if !ok {
		return
	}
	if err := h.functions.ReorderVehicleSlots(r.Context(), actor.User.ID, vehicleID, entries); err != nil {
		writeFunctionError(w, err)
		return
	}
	h.ListVehicle(w, r)
}

// --- Template routes (/vehicle-templates/{id}/functions) ---

// ListTemplate handles GET /api/v1/vehicle-templates/{id}/functions.
func (h *FunctionHandler) ListTemplate(w http.ResponseWriter, r *http.Request) {
	templateID, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	rows, err := h.functions.ListForTemplate(r.Context(), templateID)
	if err != nil {
		writeFunctionError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toFunctionResponses(rows))
}

// UpsertTemplate handles PUT /api/v1/vehicle-templates/{id}/functions/{num}.
func (h *FunctionHandler) UpsertTemplate(w http.ResponseWriter, r *http.Request) {
	templateID, num, actorID, eff, ok := h.parseTemplateMutation(w, r)
	if !ok {
		return
	}
	var req functionUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	row, err := h.functions.UpsertTemplateSlot(r.Context(), actorID, eff, templateID, num, toUpsertInput(req))
	if err != nil {
		writeFunctionError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toDccFunctionResponse(row, "template"))
}

// DeleteTemplate handles DELETE /api/v1/vehicle-templates/{id}/functions/{num}.
func (h *FunctionHandler) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	templateID, num, actorID, eff, ok := h.parseTemplateMutation(w, r)
	if !ok {
		return
	}
	if err := h.functions.DeleteTemplateSlot(r.Context(), actorID, eff, templateID, num); err != nil {
		writeFunctionError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ReorderTemplate handles POST /api/v1/vehicle-templates/{id}/functions/reorder.
func (h *FunctionHandler) ReorderTemplate(w http.ResponseWriter, r *http.Request) {
	templateID, ok := parseUintParam(r, "id")
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
	entries, ok := h.decodeReorder(w, r)
	if !ok {
		return
	}
	if err := h.functions.ReorderTemplateSlots(r.Context(), actor.User.ID, eff, templateID, entries); err != nil {
		writeFunctionError(w, err)
		return
	}
	h.ListTemplate(w, r)
}

func (h *FunctionHandler) parseVehicleMutation(w http.ResponseWriter, r *http.Request) (vehicleID uint, num uint8, actorID uint, ok bool) {
	vehicleID, ok = parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return 0, 0, 0, false
	}
	num, ok = parseFunctionNumParam(w, r)
	if !ok {
		return 0, 0, 0, false
	}
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return 0, 0, 0, false
	}
	return vehicleID, num, actor.User.ID, true
}

func (h *FunctionHandler) parseTemplateMutation(w http.ResponseWriter, r *http.Request) (
	templateID uint, num uint8, actorID uint, eff domain.EffectiveRoles, ok bool,
) {
	templateID, ok = parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return 0, 0, 0, domain.EffectiveRoles{}, false
	}
	num, ok = parseFunctionNumParam(w, r)
	if !ok {
		return 0, 0, 0, domain.EffectiveRoles{}, false
	}
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return 0, 0, 0, domain.EffectiveRoles{}, false
	}
	var err error
	eff, err = h.auth.Effective(r.Context(), actor.User, actor.Layout.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return 0, 0, 0, domain.EffectiveRoles{}, false
	}
	return templateID, num, actor.User.ID, eff, true
}

func (h *FunctionHandler) decodeReorder(w http.ResponseWriter, r *http.Request) ([]service.FunctionReorderEntry, bool) {
	var req functionReorderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return nil, false
	}
	out := make([]service.FunctionReorderEntry, 0, len(req.Positions))
	for _, p := range req.Positions {
		out = append(out, service.FunctionReorderEntry{Num: p.Num, Position: p.Position})
	}
	return out, true
}

func parseFunctionNumParam(w http.ResponseWriter, r *http.Request) (uint8, bool) {
	raw := chi.URLParam(r, "num")
	n, err := strconv.ParseUint(raw, 10, 8)
	if err != nil || n > 31 {
		writeJSONError(w, http.StatusBadRequest, "invalid_num")
		return 0, false
	}
	return uint8(n), true
}

func toUpsertInput(req functionUpsertRequest) service.FunctionUpsertInput {
	return service.FunctionUpsertInput{
		Name:     req.Name,
		Icon:     req.Icon,
		Position: req.Position,
	}
}

func toDccFunctionResponse(row domain.DccFunction, source string) functionResponse {
	return functionResponse{
		Num:      row.Num,
		Name:     row.Name,
		Icon:     row.Icon,
		Position: row.Position,
		Source:   source,
	}
}

func writeFunctionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrVehicleNotFound),
		errors.Is(err, service.ErrVehicleTemplateNotFound):
		writeJSONError(w, http.StatusNotFound, "not_found")
	case errors.Is(err, service.ErrFunctionNotFound):
		writeJSONError(w, http.StatusNotFound, "function_not_found")
	case errors.Is(err, service.ErrOnlyOwnerCanEdit):
		writeJSONError(w, http.StatusForbidden, "only_owner_can_edit")
	case errors.Is(err, service.ErrTemplateNotOwned):
		writeJSONError(w, http.StatusForbidden, "template_not_owned")
	case errors.Is(err, service.ErrFunctionReplaceSourceInvalid):
		writeJSONError(w, http.StatusUnprocessableEntity, "function_replace_source_invalid")
	case errors.Is(err, service.ErrFunctionNumInvalid),
		errors.Is(err, service.ErrFunctionIconInvalid),
		errors.Is(err, service.ErrFunctionNameRequired):
		writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, service.ErrFunctionNumTaken):
		writeJSONError(w, http.StatusConflict, "function_num_taken")
	default:
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
	}
}
