package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/keskad/loco/pkgs/server/service"
)

// LayoutRosterHandler bundles the layout-scoped vehicle and train
// roster endpoints (§4.1 /api/v1/layouts/{id}/vehicles + /trains).
//
// The layout id in the path is matched against the caller's pinned
// session layout (§7a.1). Mismatch returns 422 layout_mismatch — a
// hand-crafted request cannot peek into another layout's roster.
type LayoutRosterHandler struct {
	svc *service.LayoutVehicleService
}

// NewLayoutRosterHandler returns a LayoutRosterHandler.
func NewLayoutRosterHandler(svc *service.LayoutVehicleService) *LayoutRosterHandler {
	return &LayoutRosterHandler{svc: svc}
}

// rosterVehicleResponse is the dashboard row shape. We piggy-back on
// `vehicleResponse` and add roster metadata.
type rosterVehicleResponse struct {
	vehicleResponse
	OwnerLogin string    `json:"ownerLogin"`
	AddedAt    time.Time `json:"addedAt"`
}

// rosterTrainResponse is the train-shaped sibling.
type rosterTrainResponse struct {
	ID         uint                  `json:"id"`
	Name       string                `json:"name"`
	OwnerID    uint                  `json:"ownerId"`
	OwnerLogin string                `json:"ownerLogin"`
	AddedAt    time.Time             `json:"addedAt"`
	Members    []trainMemberResponse `json:"members"`
}

func toRosterVehicleResponse(e service.RosterVehicleEntry) rosterVehicleResponse {
	return rosterVehicleResponse{
		vehicleResponse: toVehicleResponse(e.Vehicle),
		OwnerLogin:      e.OwnerLogin,
		AddedAt:         e.AddedAt,
	}
}

func toRosterTrainResponse(e service.RosterTrainEntry) rosterTrainResponse {
	members := make([]trainMemberResponse, 0, len(e.Members))
	for _, m := range e.Members {
		members = append(members, trainMemberResponse{
			ID:        m.ID,
			VehicleID: m.VehicleID,
			Position:  m.Position,
			Reversed:  m.Reversed,
		})
	}
	return rosterTrainResponse{
		ID:         e.Train.ID,
		Name:       e.Train.Name,
		OwnerID:    e.Train.OwnerUserID,
		OwnerLogin: e.OwnerLogin,
		AddedAt:    e.AddedAt,
		Members:    members,
	}
}

// requireOwnLayout pulls the layout id from the path and confirms it
// matches the caller's session layout. Returns (layoutId, true) when
// happy, and writes the matching 4xx response otherwise.
func requireOwnLayout(w http.ResponseWriter, r *http.Request) (uint, service.Identity, bool) {
	layoutID, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return 0, service.Identity{}, false
	}
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return 0, service.Identity{}, false
	}
	if actor.Layout.ID != layoutID {
		writeJSONError(w, http.StatusUnprocessableEntity, "layout_mismatch")
		return 0, service.Identity{}, false
	}
	return layoutID, actor, true
}

// ListVehicles handles GET /api/v1/layouts/{id}/vehicles.
func (h *LayoutRosterHandler) ListVehicles(w http.ResponseWriter, r *http.Request) {
	layoutID, _, ok := requireOwnLayout(w, r)
	if !ok {
		return
	}
	rows, err := h.svc.ListVehicles(r.Context(), layoutID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	out := make([]rosterVehicleResponse, 0, len(rows))
	for _, e := range rows {
		out = append(out, toRosterVehicleResponse(e))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// ListTrains handles GET /api/v1/layouts/{id}/trains.
func (h *LayoutRosterHandler) ListTrains(w http.ResponseWriter, r *http.Request) {
	layoutID, _, ok := requireOwnLayout(w, r)
	if !ok {
		return
	}
	rows, err := h.svc.ListTrains(r.Context(), layoutID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	out := make([]rosterTrainResponse, 0, len(rows))
	for _, e := range rows {
		out = append(out, toRosterTrainResponse(e))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

type addVehicleRosterRequest struct {
	VehicleID uint `json:"vehicleId"`
}

// AddVehicle handles POST /api/v1/layouts/{id}/vehicles.
func (h *LayoutRosterHandler) AddVehicle(w http.ResponseWriter, r *http.Request) {
	layoutID, actor, ok := requireOwnLayout(w, r)
	if !ok {
		return
	}
	var req addVehicleRosterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if req.VehicleID == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	entry, err := h.svc.AddVehicle(r.Context(), layoutID, actor.User.ID, req.VehicleID)
	if err != nil {
		writeLayoutRosterError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toRosterVehicleResponse(entry))
}

// RemoveVehicle handles DELETE /api/v1/layouts/{id}/vehicles/{vehicleId}.
func (h *LayoutRosterHandler) RemoveVehicle(w http.ResponseWriter, r *http.Request) {
	layoutID, actor, ok := requireOwnLayout(w, r)
	if !ok {
		return
	}
	vehicleID, ok := parseUintParam(r, "vehicleId")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	if err := h.svc.RemoveVehicle(r.Context(), layoutID, actor.User.ID, vehicleID); err != nil {
		writeLayoutRosterError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type addTrainRosterRequest struct {
	TrainID uint `json:"trainId"`
}

// AddTrain handles POST /api/v1/layouts/{id}/trains.
func (h *LayoutRosterHandler) AddTrain(w http.ResponseWriter, r *http.Request) {
	layoutID, actor, ok := requireOwnLayout(w, r)
	if !ok {
		return
	}
	var req addTrainRosterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if req.TrainID == 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	entry, err := h.svc.AddTrain(r.Context(), layoutID, actor.User.ID, req.TrainID)
	if err != nil {
		writeLayoutRosterError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toRosterTrainResponse(entry))
}

// RemoveTrain handles DELETE /api/v1/layouts/{id}/trains/{trainId}.
func (h *LayoutRosterHandler) RemoveTrain(w http.ResponseWriter, r *http.Request) {
	layoutID, actor, ok := requireOwnLayout(w, r)
	if !ok {
		return
	}
	trainID, ok := parseUintParam(r, "trainId")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	if err := h.svc.RemoveTrain(r.Context(), layoutID, actor.User.ID, trainID); err != nil {
		writeLayoutRosterError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeLayoutRosterError maps roster sentinels to status codes.
func writeLayoutRosterError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrVehicleNotFound):
		writeJSONError(w, http.StatusNotFound, "vehicle_not_found")
	case errors.Is(err, service.ErrTrainNotFound):
		writeJSONError(w, http.StatusNotFound, "train_not_found")
	case errors.Is(err, service.ErrVehicleNotOwned):
		writeJSONError(w, http.StatusForbidden, "vehicle_not_owned")
	case errors.Is(err, service.ErrTrainNotOwned):
		writeJSONError(w, http.StatusForbidden, "train_not_owned")
	case errors.Is(err, service.ErrLayoutVehicleAlreadyOnRoster):
		writeJSONError(w, http.StatusConflict, "layout_vehicle_already_on_roster")
	case errors.Is(err, service.ErrLayoutVehicleNotOnRoster):
		writeJSONError(w, http.StatusNotFound, "layout_vehicle_not_on_roster")
	case errors.Is(err, service.ErrLayoutTrainAlreadyOnRoster):
		writeJSONError(w, http.StatusConflict, "layout_train_already_on_roster")
	case errors.Is(err, service.ErrLayoutTrainNotOnRoster):
		writeJSONError(w, http.StatusNotFound, "layout_train_not_on_roster")
	default:
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
	}
}
