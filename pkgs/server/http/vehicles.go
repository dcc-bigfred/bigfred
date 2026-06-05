package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/keskad/loco/pkgs/server/domain"
	"github.com/keskad/loco/pkgs/server/service"
)

// VehicleHandler bundles REST endpoints for the per-user vehicle
// catalogue (§4.1).
type VehicleHandler struct {
	svc            *service.VehicleService
	layoutVehicles *service.LayoutVehicleService
	pool           *service.DCCPoolService
	auth           *service.AuthService
}

// NewVehicleHandler returns a VehicleHandler.
func NewVehicleHandler(
	svc *service.VehicleService,
	layoutVehicles *service.LayoutVehicleService,
	pool *service.DCCPoolService,
	auth *service.AuthService,
) *VehicleHandler {
	return &VehicleHandler{svc: svc, layoutVehicles: layoutVehicles, pool: pool, auth: auth}
}

// vehicleResponse is the JSON shape the frontend consumes. DCCAddress
// is a pointer so the dummy case ("DCC: —") is representable.
type vehicleResponse struct {
	ID         uint               `json:"id"`
	Name       string             `json:"name"`
	Kind       domain.VehicleKind `json:"kind"`
	Number     string             `json:"number"`
	DCCAddress *uint16            `json:"dccAddress"`
	IsDummy    bool               `json:"isDummy"`
	OwnerID    uint               `json:"ownerId"`

	Rp1Function             uint8                      `json:"rp1Function"`
	EmergencyLightsFunction uint8                      `json:"emergencyLightsFunction"`
	DeadManSwitchOption     domain.DeadManSwitchOption `json:"deadManSwitchOption"`
}

func toVehicleResponse(v domain.Vehicle) vehicleResponse {
	return vehicleResponse{
		ID:                      v.ID,
		Name:                    v.Name,
		Kind:                    v.Kind,
		Number:                  v.Number,
		DCCAddress:              v.DCCAddress,
		IsDummy:                 v.IsDummy(),
		OwnerID:                 v.OwnerUserID,
		Rp1Function:             v.Rp1Function,
		EmergencyLightsFunction: v.EmergencyLightsFunction,
		DeadManSwitchOption:     v.DeadManSwitchOption,
	}
}

// List handles GET /api/v1/vehicles — own vehicles only for now.
// Leasing and signalman-overrides will join the union in the
// milestone that introduces VehicleLease.
func (h *VehicleHandler) List(w http.ResponseWriter, r *http.Request) {
	id, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	rows, err := h.svc.ListOwned(r.Context(), id.User.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	out := make([]vehicleResponse, 0, len(rows))
	for _, v := range rows {
		out = append(out, toVehicleResponse(v))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

type vehicleCreateRequest struct {
	Name       string             `json:"name"`
	Kind       domain.VehicleKind `json:"kind"`
	Number     string             `json:"number"`
	DCCAddress *uint16            `json:"dccAddress"`

	Rp1Function             *uint8                      `json:"rp1Function"`
	EmergencyLightsFunction *uint8                      `json:"emergencyLightsFunction"`
	DeadManSwitchOption     *domain.DeadManSwitchOption `json:"deadManSwitchOption"`
}

// Create handles POST /api/v1/vehicles.
func (h *VehicleHandler) Create(w http.ResponseWriter, r *http.Request) {
	id, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req vehicleCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	row, err := h.svc.Create(r.Context(), service.VehicleCreateInput{
		OwnerUserID:             id.User.ID,
		Name:                    req.Name,
		Kind:                    req.Kind,
		Number:                  req.Number,
		DCCAddress:              req.DCCAddress,
		Rp1Function:             req.Rp1Function,
		EmergencyLightsFunction: req.EmergencyLightsFunction,
		DeadManSwitchOption:     req.DeadManSwitchOption,
	})
	if err != nil {
		writeVehicleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toVehicleResponse(row))
}

// vehicleUpdateRequest mirrors the tri-state in VehicleUpdateInput.
// `dccAddressSet` is true when the client wants to mutate the
// column; when it is false, the existing value stays put.
type vehicleUpdateRequest struct {
	Name          *string             `json:"name"`
	Kind          *domain.VehicleKind `json:"kind"`
	Number        *string             `json:"number"`
	DCCAddress    *uint16             `json:"dccAddress"`
	DCCAddressSet bool                `json:"dccAddressSet"`

	Rp1Function             *uint8                      `json:"rp1Function"`
	EmergencyLightsFunction *uint8                      `json:"emergencyLightsFunction"`
	DeadManSwitchOption     *domain.DeadManSwitchOption `json:"deadManSwitchOption"`
}

// Update handles PUT /api/v1/vehicles/{id}.
func (h *VehicleHandler) Update(w http.ResponseWriter, r *http.Request) {
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
	var req vehicleUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	in := service.VehicleUpdateInput{
		Name:                    req.Name,
		Kind:                    req.Kind,
		Number:                  req.Number,
		Rp1Function:             req.Rp1Function,
		EmergencyLightsFunction: req.EmergencyLightsFunction,
		DeadManSwitchOption:     req.DeadManSwitchOption,
	}
	if req.DCCAddressSet {
		in.DCCAddress = service.VehicleAddressPatch{IsSet: true, Value: req.DCCAddress}
	}
	eff, err := h.auth.Effective(r.Context(), actor.User, actor.Layout.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	row, err := h.svc.Update(r.Context(), actor.User.ID, vehicleID, eff, in)
	if err != nil {
		writeVehicleError(w, err)
		return
	}
	if err := h.layoutVehicles.BroadcastVehicleUpdated(r.Context(), row.ID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toVehicleResponse(row))
}

// Delete handles DELETE /api/v1/vehicles/{id}. Cascades the layout
// roster cleanup so dangling rows never surface on the dashboard.
func (h *VehicleHandler) Delete(w http.ResponseWriter, r *http.Request) {
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
	eff, err := h.auth.Effective(r.Context(), actor.User, actor.Layout.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if _, err := h.svc.Delete(r.Context(), actor.User.ID, vehicleID, eff); err != nil {
		writeVehicleError(w, err)
		return
	}
	if err := h.layoutVehicles.PurgeVehicle(r.Context(), vehicleID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListPool handles GET /api/v1/auth/me/dcc-pool — used by the
// vehicle add/edit dialog to render "your pool: 1..9999" hints.
func (h *VehicleHandler) ListPool(w http.ResponseWriter, r *http.Request) {
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	rows, err := h.pool.List(r.Context(), actor.User.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	type rangeResp struct {
		From uint16 `json:"from"`
		To   uint16 `json:"to"`
	}
	out := make([]rangeResp, 0, len(rows))
	for _, r := range rows {
		out = append(out, rangeResp{From: r.FromAddr, To: r.ToAddr})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// writeVehicleError maps service sentinels to status codes + machine
// codes the frontend can localise.
func writeVehicleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrVehicleNotFound):
		writeJSONError(w, http.StatusNotFound, "vehicle_not_found")
	case errors.Is(err, service.ErrVehicleNameRequired):
		writeJSONError(w, http.StatusUnprocessableEntity, "vehicle_name_required")
	case errors.Is(err, service.ErrVehicleKindInvalid):
		writeJSONError(w, http.StatusUnprocessableEntity, "vehicle_kind_invalid")
	case errors.Is(err, service.ErrDCCAddressTaken):
		writeJSONError(w, http.StatusConflict, "dcc_address_taken")
	case errors.Is(err, service.ErrDCCAddressOutsidePool):
		writeJSONError(w, http.StatusUnprocessableEntity, "dcc_address_outside_pool")
	case errors.Is(err, service.ErrVehicleNotOwned):
		writeJSONError(w, http.StatusForbidden, "vehicle_not_owned")
	case errors.Is(err, service.ErrVehicleInUse):
		writeJSONError(w, http.StatusConflict, "vehicle_in_use")
	case errors.Is(err, service.ErrVehicleDccFunctionInvalid):
		writeJSONError(w, http.StatusUnprocessableEntity, "vehicle_dcc_function_invalid")
	case errors.Is(err, service.ErrVehicleDeadManSwitchInvalid):
		writeJSONError(w, http.StatusUnprocessableEntity, "vehicle_deadman_switch_invalid")
	default:
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
	}
}
