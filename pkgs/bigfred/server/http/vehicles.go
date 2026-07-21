package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/protocol"
	"github.com/keskad/loco/pkgs/bigfred/server/service"
)

// VehicleHandler bundles REST endpoints for the per-user vehicle
// catalogue (§4.1).
type VehicleHandler struct {
	svc            *cmd.Vehicle
	layoutVehicles *service.LayoutVehicleService
	pool           *cmd.DCCPool
	auth           *cmd.Auth
}

// NewVehicleHandler returns a VehicleHandler.
func NewVehicleHandler(
	svc *cmd.Vehicle,
	layoutVehicles *service.LayoutVehicleService,
	pool *cmd.DCCPool,
	auth *cmd.Auth,
) *VehicleHandler {
	return &VehicleHandler{svc: svc, layoutVehicles: layoutVehicles, pool: pool, auth: auth}
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
	out := make([]protocol.VehicleResponse, 0, len(rows))
	for _, v := range rows {
		out = append(out, protocol.ToVehicleResponse(v))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// ListCatalogue handles GET /api/v1/vehicles/catalogue — every
// registered vehicle with owner metadata and on-layout flag for the
// caller's pinned session layout.
func (h *VehicleHandler) ListCatalogue(w http.ResponseWriter, r *http.Request) {
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	rows, err := h.svc.ListCatalogue(r.Context(), actor.Layout.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	out := make([]protocol.VehicleCatalogueResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, protocol.ToVehicleCatalogueResponse(row))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// Create handles POST /api/v1/vehicles.
func (h *VehicleHandler) Create(w http.ResponseWriter, r *http.Request) {
	id, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req protocol.VehicleCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	row, err := h.svc.Create(r.Context(), req.ToCreateInput(id.User.ID))
	if err != nil {
		writeVehicleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(protocol.ToVehicleResponse(row))
}

// Update handles PUT /api/v1/vehicles/{id}.
func (h *VehicleHandler) Update(w http.ResponseWriter, r *http.Request) {
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
	var req protocol.VehicleUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	eff, err := h.auth.Effective(r.Context(), actor.User, actor.Layout.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	row, err := h.svc.Update(r.Context(), actor.User.ID, vehicleID, eff, req.ToUpdateInput())
	if err != nil {
		writeVehicleError(w, err)
		return
	}
	if err := h.layoutVehicles.BroadcastVehicleUpdated(r.Context(), row.ID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(protocol.ToVehicleResponse(row))
}

// Delete handles DELETE /api/v1/vehicles/{id}. Cascades the layout
// roster cleanup so dangling rows never surface on the dashboard.
func (h *VehicleHandler) Delete(w http.ResponseWriter, r *http.Request) {
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

// UpsertByExternalID handles PUT /api/v1/vehicles/by-external-id/{externalId}.
// Creates the vehicle when the external id is unknown, otherwise overwrites
// the caller's row. Mirrors Update wiring (auth + broadcast).
func (h *VehicleHandler) UpsertByExternalID(w http.ResponseWriter, r *http.Request) {
	externalID := chi.URLParam(r, "externalId")
	if externalID == "" {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req protocol.VehicleCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	eff, err := h.auth.Effective(r.Context(), actor.User, actor.Layout.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	row, created, err := h.svc.UpsertByExternalID(r.Context(), actor.User.ID, eff, externalID, req.ToCreateInput(actor.User.ID))
	if err != nil {
		writeVehicleError(w, err)
		return
	}
	if err := h.layoutVehicles.BroadcastVehicleUpdated(r.Context(), row.ID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if created {
		w.WriteHeader(http.StatusCreated)
	}
	_ = json.NewEncoder(w).Encode(protocol.ToVehicleResponse(row))
}

// DeleteByExternalID handles DELETE /api/v1/vehicles/by-external-id/{externalId}.
func (h *VehicleHandler) DeleteByExternalID(w http.ResponseWriter, r *http.Request) {
	externalID := chi.URLParam(r, "externalId")
	if externalID == "" {
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
	row, err := h.svc.DeleteByExternalID(r.Context(), actor.User.ID, eff, externalID)
	if err != nil {
		writeVehicleError(w, err)
		return
	}
	if err := h.layoutVehicles.PurgeVehicle(r.Context(), row.ID); err != nil {
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

func writeVehicleError(w http.ResponseWriter, err error) {
	status, code := svcerrors.VehicleHTTPStatus(err)
	writeJSONError(w, status, code)
}
