package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/protocol"
)

// FunctionHandler serves function list/edit for vehicles and templates (§6.3e).
type FunctionHandler struct {
	functions    *cmd.Function
	auth         *cmd.Auth
	functionSync vehicleFunctionRedisSync
}

type vehicleFunctionRedisSync interface {
	SyncVehicleFunctionsForVehicle(ctx context.Context, vehicleID domain.VehicleID) error
	SyncVehicleFunctionsForTemplate(ctx context.Context, templateID uint) error
}

// NewFunctionHandler returns a FunctionHandler.
func NewFunctionHandler(functions *cmd.Function, auth *cmd.Auth) *FunctionHandler {
	return &FunctionHandler{functions: functions, auth: auth}
}

// SetVehicleFunctionSync wires Redis republication after catalogue edits.
func (h *FunctionHandler) SetVehicleFunctionSync(sync vehicleFunctionRedisSync) {
	h.functionSync = sync
}

func (h *FunctionHandler) syncVehicleFunctions(ctx context.Context, vehicleID domain.VehicleID) {
	if h.functionSync == nil || vehicleID.IsZero() {
		return
	}
	_ = h.functionSync.SyncVehicleFunctionsForVehicle(ctx, vehicleID)
}

func (h *FunctionHandler) syncTemplateFunctions(ctx context.Context, templateID uint) {
	if h.functionSync == nil || templateID == 0 {
		return
	}
	_ = h.functionSync.SyncVehicleFunctionsForTemplate(ctx, templateID)
}

// ListCatalogue handles GET /api/v1/vehicles/function-catalogue.
func (h *FunctionHandler) ListCatalogue(w http.ResponseWriter, r *http.Request) {
	rows, err := h.functions.ListFunctionCatalogue(r.Context())
	if err != nil {
		writeFunctionError(w, err)
		return
	}
	out := make([]protocol.FunctionCatalogueEntryResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, protocol.ToFunctionCatalogueEntry(row))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// ListIcons handles GET /api/v1/function-icons.
func (h *FunctionHandler) ListIcons(w http.ResponseWriter, r *http.Request) {
	icons := h.functions.ListIcons()
	out := make([]protocol.FunctionIconResponse, 0, len(icons))
	for _, icon := range icons {
		out = append(out, protocol.FunctionIconResponse{Icon: string(icon)})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// ListVehicle handles GET /api/v1/vehicles/{id}/functions.
func (h *FunctionHandler) ListVehicle(w http.ResponseWriter, r *http.Request) {
	vehicleID, ok := parseVehicleIDParam(r, "id")
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
	_ = json.NewEncoder(w).Encode(protocol.ToFunctionResponses(rows))
}

// UpsertVehicle handles PUT /api/v1/vehicles/{id}/functions/{num}.
func (h *FunctionHandler) UpsertVehicle(w http.ResponseWriter, r *http.Request) {
	vehicleID, num, actorID, ok := h.parseVehicleMutation(w, r)
	if !ok {
		return
	}
	var req protocol.FunctionUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	row, err := h.functions.UpsertVehicleSlot(r.Context(), actorID, vehicleID, num, req.ToUpsertInput())
	if err != nil {
		writeFunctionError(w, err)
		return
	}
	h.syncVehicleFunctions(r.Context(), vehicleID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(protocol.ToFunctionResponse(row, "vehicle"))
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
	h.syncVehicleFunctions(r.Context(), vehicleID)
	w.WriteHeader(http.StatusNoContent)
}

// AttachVehicle handles POST /api/v1/vehicles/{id}/functions/attach.
func (h *FunctionHandler) AttachVehicle(w http.ResponseWriter, r *http.Request) {
	vehicleID, ok := parseVehicleIDParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req protocol.FunctionReplaceFromRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	fromTemplate := req.TemplateID != 0
	fromVehicle := req.SourceVehicleID != ""
	if fromTemplate == fromVehicle {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	var (
		rows []cmd.ResolvedFunction
		err  error
	)
	if fromTemplate {
		rows, err = h.functions.AttachVehicleToTemplate(
			r.Context(), actor.User.ID, vehicleID, req.TemplateID,
		)
	} else {
		rows, err = h.functions.CopyVehicleFunctionsFromVehicle(
			r.Context(), actor.User.ID, vehicleID, domain.VehicleID(req.SourceVehicleID),
		)
	}
	if err != nil {
		writeFunctionError(w, err)
		return
	}
	h.syncVehicleFunctions(r.Context(), vehicleID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(protocol.ToFunctionResponses(rows))
}

// ReorderVehicle handles POST /api/v1/vehicles/{id}/functions/reorder.
func (h *FunctionHandler) ReorderVehicle(w http.ResponseWriter, r *http.Request) {
	vehicleID, ok := parseVehicleIDParam(r, "id")
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
	h.syncVehicleFunctions(r.Context(), vehicleID)
	h.ListVehicle(w, r)
}

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
	_ = json.NewEncoder(w).Encode(protocol.ToFunctionResponses(rows))
}

// UpsertTemplate handles PUT /api/v1/vehicle-templates/{id}/functions/{num}.
func (h *FunctionHandler) UpsertTemplate(w http.ResponseWriter, r *http.Request) {
	templateID, num, actorID, eff, ok := h.parseTemplateMutation(w, r)
	if !ok {
		return
	}
	var req protocol.FunctionUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	row, err := h.functions.UpsertTemplateSlot(r.Context(), actorID, eff, templateID, num, req.ToUpsertInput())
	if err != nil {
		writeFunctionError(w, err)
		return
	}
	h.syncTemplateFunctions(r.Context(), templateID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(protocol.ToFunctionResponse(row, "template"))
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
	h.syncTemplateFunctions(r.Context(), templateID)
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
	h.syncTemplateFunctions(r.Context(), templateID)
	h.ListTemplate(w, r)
}

func (h *FunctionHandler) parseVehicleMutation(w http.ResponseWriter, r *http.Request) (vehicleID domain.VehicleID, num uint8, actorID uint, ok bool) {
	vehicleID, ok = parseVehicleIDParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return "", 0, 0, false
	}
	num, ok = parseFunctionNumParam(w, r)
	if !ok {
		return "", 0, 0, false
	}
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return "", 0, 0, false
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

func (h *FunctionHandler) decodeReorder(w http.ResponseWriter, r *http.Request) ([]cmd.FunctionReorderEntry, bool) {
	var req protocol.FunctionReorderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return nil, false
	}
	return req.ToReorderEntries(), true
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

func writeFunctionError(w http.ResponseWriter, err error) {
	status, code := svcerrors.FunctionHTTPStatus(err)
	writeJSONError(w, status, code)
}
