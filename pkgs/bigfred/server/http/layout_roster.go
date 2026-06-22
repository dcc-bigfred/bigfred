package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/protocol"
	"github.com/keskad/loco/pkgs/bigfred/server/security"
	"github.com/keskad/loco/pkgs/bigfred/server/service"
)

// LayoutRosterHandler bundles the layout-scoped vehicle and train
// roster endpoints (§4.1 /api/v1/layouts/{id}/vehicles + /trains).
//
// The layout id in the path is matched against the caller's pinned
// session layout (§7a.1). Mismatch returns 422 layout_mismatch — a
// hand-crafted request cannot peek into another layout's roster.
type LayoutRosterHandler struct {
	svc   *service.LayoutVehicleService
	auth  *cmd.Auth
	audit cmd.AuditPublisher
}

// NewLayoutRosterHandler returns a LayoutRosterHandler.
func NewLayoutRosterHandler(svc *service.LayoutVehicleService, auth *cmd.Auth, audit cmd.AuditPublisher) *LayoutRosterHandler {
	return &LayoutRosterHandler{svc: svc, auth: auth, audit: audit}
}

// rosterVehicleResponse is the dashboard row shape. We piggy-back on
// protocol.VehicleResponse and add roster metadata.
type rosterVehicleResponse struct {
	protocol.VehicleResponse
	OwnerLogin        string    `json:"ownerLogin"`
	OwnerOrganization string    `json:"ownerOrganization"`
	AddedAt           time.Time `json:"addedAt"`
	CanDrive   bool      `json:"canDrive"`
}

// rosterTrainResponse is the train-shaped sibling.
type rosterTrainResponse struct {
	ID                string                         `json:"id"`
	Name              string                         `json:"name"`
	OwnerID           uint                           `json:"ownerId"`
	OwnerLogin        string                         `json:"ownerLogin"`
	OwnerOrganization string                         `json:"ownerOrganization"`
	AddedAt           time.Time                      `json:"addedAt"`
	CanDrive          bool                           `json:"canDrive"`
	Members           []protocol.TrainMemberResponse `json:"members"`
}

func toRosterVehicleResponse(e service.RosterVehicleEntry, canDrive bool) rosterVehicleResponse {
	return rosterVehicleResponse{
		VehicleResponse: protocol.ToVehicleResponse(e.Vehicle),
		OwnerLogin:        e.OwnerLogin,
		OwnerOrganization: e.OwnerOrganization,
		AddedAt:           e.AddedAt,
		CanDrive:        canDrive,
	}
}

func toRosterTrainResponse(e service.RosterTrainEntry, canDrive bool) rosterTrainResponse {
	members := make([]protocol.TrainMemberResponse, 0, len(e.Members))
	for _, m := range e.Members {
		members = append(members, protocol.ToTrainMemberResponse(m))
	}
	return rosterTrainResponse{
		ID:                e.Train.ID.String(),
		Name:              e.Train.Name,
		OwnerID:           e.Train.OwnerUserID,
		OwnerLogin:        e.OwnerLogin,
		OwnerOrganization: e.OwnerOrganization,
		AddedAt:           e.AddedAt,
		CanDrive:          canDrive,
		Members:           members,
	}
}

// requireOwnLayout pulls the layout id from the path and confirms it
// matches the caller's session layout. Returns (layoutId, true) when
// happy, and writes the matching 4xx response otherwise.
func requireOwnLayout(w http.ResponseWriter, r *http.Request) (uint, cmd.Identity, bool) {
	layoutID, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return 0, cmd.Identity{}, false
	}
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return 0, cmd.Identity{}, false
	}
	if actor.Layout.ID != layoutID {
		writeJSONError(w, http.StatusUnprocessableEntity, "layout_mismatch")
		return 0, cmd.Identity{}, false
	}
	return layoutID, actor, true
}

// actorEffectiveRoles returns the caller's role membership inside
// their pinned layout (§7a.2).
func (h *LayoutRosterHandler) actorEffectiveRoles(r *http.Request, actor cmd.Identity) (domain.EffectiveRoles, error) {
	if h.auth == nil {
		if actor.User.Role == domain.RoleAdmin {
			return domain.NewEffectiveRoles(domain.RoleAdmin), nil
		}
		return domain.NewEffectiveRoles(actor.User.Role), nil
	}
	return h.auth.Effective(r.Context(), actor.User, actor.Layout.ID)
}

// ListVehicles handles GET /api/v1/layouts/{id}/vehicles.
func (h *LayoutRosterHandler) ListVehicles(w http.ResponseWriter, r *http.Request) {
	layoutID, actor, ok := requireOwnLayout(w, r)
	if !ok {
		return
	}
	rows, err := h.svc.ListVehicles(r.Context(), layoutID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	trains, err := h.svc.ListTrains(r.Context(), layoutID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	lessees, err := h.svc.LesseesByVehicle(r.Context(), layoutID, rows, trains)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	out := make([]rosterVehicleResponse, 0, len(rows))
	for _, e := range rows {
		canDrive := (security.DriveSecurityContext{}).CanDrive(actor.User, e.Vehicle.OwnerUserID, domain.VehicleLesseeUserIDs(lessees[e.Vehicle.ID])).Allowed
		out = append(out, toRosterVehicleResponse(e, canDrive))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// ListTrains handles GET /api/v1/layouts/{id}/trains.
func (h *LayoutRosterHandler) ListTrains(w http.ResponseWriter, r *http.Request) {
	layoutID, actor, ok := requireOwnLayout(w, r)
	if !ok {
		return
	}
	rows, err := h.svc.ListTrains(r.Context(), layoutID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	lessees, err := h.svc.LesseesByTrain(r.Context(), layoutID, rows)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	out := make([]rosterTrainResponse, 0, len(rows))
	for _, e := range rows {
		canDrive := (security.DriveSecurityContext{}).CanDrive(actor.User, e.Train.OwnerUserID, domain.TrainLesseeUserIDs(lessees[e.Train.ID])).Allowed
		out = append(out, toRosterTrainResponse(e, canDrive))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

type addVehicleRosterRequest struct {
	VehicleID string `json:"vehicleId"`
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
	if req.VehicleID == "" {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	vehicleID, ok := domain.ParseVehicleID(req.VehicleID)
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	eff, err := h.actorEffectiveRoles(r, actor)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	entry, err := h.svc.AddVehicle(r.Context(), layoutID, actor.User.ID, vehicleID, eff)
	if err != nil {
		writeLayoutRosterError(w, err)
		return
	}
	if h.audit != nil {
		_ = h.audit.Publish(r.Context(), layoutID,
			cmd.AuditActor{UserID: actor.User.ID, Login: actor.User.Login},
			"audit_roster_vehicle_added", map[string]string{"vehicle": entry.Vehicle.Name})
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toRosterVehicleResponse(entry, true))
}

// RemoveVehicle handles DELETE /api/v1/layouts/{id}/vehicles/{vehicleId}.
func (h *LayoutRosterHandler) RemoveVehicle(w http.ResponseWriter, r *http.Request) {
	layoutID, actor, ok := requireOwnLayout(w, r)
	if !ok {
		return
	}
	vehicleID, ok := parseVehicleIDParam(r, "vehicleId")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	eff, err := h.actorEffectiveRoles(r, actor)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	vehicleEntry, _ := h.svc.ListVehicles(r.Context(), layoutID)
	vehicleName := ""
	for _, e := range vehicleEntry {
		if e.Vehicle.ID == vehicleID {
			vehicleName = e.Vehicle.Name
			break
		}
	}
	if err := h.svc.RemoveVehicle(r.Context(), layoutID, actor.User.ID, vehicleID, eff); err != nil {
		writeLayoutRosterError(w, err)
		return
	}
	if h.audit != nil {
		_ = h.audit.Publish(r.Context(), layoutID,
			cmd.AuditActor{UserID: actor.User.ID, Login: actor.User.Login},
			"audit_roster_vehicle_removed", map[string]string{"vehicle": vehicleName})
	}
	w.WriteHeader(http.StatusNoContent)
}

type addTrainRosterRequest struct {
	TrainID string `json:"trainId"`
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
	if req.TrainID == "" {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	trainID, ok := domain.ParseTrainID(req.TrainID)
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	eff, err := h.actorEffectiveRoles(r, actor)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	entry, err := h.svc.AddTrain(r.Context(), layoutID, actor.User.ID, trainID, eff)
	if err != nil {
		writeLayoutRosterError(w, err)
		return
	}
	if h.audit != nil {
		_ = h.audit.Publish(r.Context(), layoutID,
			cmd.AuditActor{UserID: actor.User.ID, Login: actor.User.Login},
			"audit_roster_train_added", map[string]string{"train": entry.Train.Name})
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toRosterTrainResponse(entry, true))
}

// RemoveTrain handles DELETE /api/v1/layouts/{id}/trains/{trainId}.
func (h *LayoutRosterHandler) RemoveTrain(w http.ResponseWriter, r *http.Request) {
	layoutID, actor, ok := requireOwnLayout(w, r)
	if !ok {
		return
	}
	trainID, ok := parseTrainIDParam(r, "trainId")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	eff, err := h.actorEffectiveRoles(r, actor)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	trainEntries, _ := h.svc.ListTrains(r.Context(), layoutID)
	trainName := ""
	for _, e := range trainEntries {
		if e.Train.ID == trainID {
			trainName = e.Train.Name
			break
		}
	}
	if err := h.svc.RemoveTrain(r.Context(), layoutID, actor.User.ID, trainID, eff); err != nil {
		writeLayoutRosterError(w, err)
		return
	}
	if h.audit != nil {
		_ = h.audit.Publish(r.Context(), layoutID,
			cmd.AuditActor{UserID: actor.User.ID, Login: actor.User.Login},
			"audit_roster_train_removed", map[string]string{"train": trainName})
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeLayoutRosterError maps roster sentinels to status codes.
func writeLayoutRosterError(w http.ResponseWriter, err error) {
	status, code := svcerrors.LayoutRosterHTTPStatus(err)
	writeJSONError(w, status, code)
}
