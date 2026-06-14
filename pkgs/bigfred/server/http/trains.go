package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/keskad/loco/pkgs/bigfred/server/service"
)

// TrainHandler bundles REST endpoints for the per-user train
// catalogue (§4.1).
type TrainHandler struct {
	svc          *service.TrainService
	layoutTrains *service.LayoutVehicleService
	auth         *service.AuthService
}

// NewTrainHandler returns a TrainHandler.
func NewTrainHandler(
	svc *service.TrainService,
	layoutTrains *service.LayoutVehicleService,
	auth *service.AuthService,
) *TrainHandler {
	return &TrainHandler{svc: svc, layoutTrains: layoutTrains, auth: auth}
}

type trainMemberRequest struct {
	VehicleID uint `json:"vehicleId"`
	Reversed  bool `json:"reversed"`
}

type trainMemberResponse struct {
	ID        uint `json:"id"`
	VehicleID uint `json:"vehicleId"`
	Position  int  `json:"position"`
	Reversed  bool `json:"reversed"`
}

type trainResponse struct {
	ID      uint                  `json:"id"`
	Name    string                `json:"name"`
	OwnerID uint                  `json:"ownerId"`
	Members []trainMemberResponse `json:"members"`
}

func toTrainResponse(d service.TrainDetail) trainResponse {
	members := make([]trainMemberResponse, 0, len(d.Members))
	for _, m := range d.Members {
		members = append(members, trainMemberResponse{
			ID:        m.ID,
			VehicleID: m.VehicleID,
			Position:  m.Position,
			Reversed:  m.Reversed,
		})
	}
	return trainResponse{
		ID:      d.Train.ID,
		Name:    d.Train.Name,
		OwnerID: d.Train.OwnerUserID,
		Members: members,
	}
}

// List handles GET /api/v1/trains — own trains only for now.
func (h *TrainHandler) List(w http.ResponseWriter, r *http.Request) {
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
	out := make([]trainResponse, 0, len(rows))
	for _, d := range rows {
		out = append(out, toTrainResponse(d))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

type trainCreateRequest struct {
	Name    string               `json:"name"`
	Members []trainMemberRequest `json:"members"`
}

// Create handles POST /api/v1/trains.
func (h *TrainHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req trainCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	members := make([]service.TrainMemberInput, 0, len(req.Members))
	for _, m := range req.Members {
		members = append(members, service.TrainMemberInput{
			VehicleID: m.VehicleID,
			Reversed:  m.Reversed,
		})
	}
	d, err := h.svc.Create(r.Context(), service.TrainCreateInput{
		OwnerUserID: actor.User.ID,
		Name:        req.Name,
		Members:     members,
	})
	if err != nil {
		writeTrainError(w, err)
		return
	}
	// Train may already be on a layout roster; refresh dcc-bus snapshots.
	if err := h.layoutTrains.SyncLayoutRosterForTrain(r.Context(), d.Train.ID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toTrainResponse(d))
}

type trainUpdateRequest struct {
	Name       *string              `json:"name"`
	Members    []trainMemberRequest `json:"members"`
	MembersSet bool                 `json:"membersSet"`
}

// Update handles PUT /api/v1/trains/{id}.
func (h *TrainHandler) Update(w http.ResponseWriter, r *http.Request) {
	trainID, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req trainUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	in := service.TrainUpdateInput{Name: req.Name}
	if req.MembersSet {
		members := make([]service.TrainMemberInput, 0, len(req.Members))
		for _, m := range req.Members {
			members = append(members, service.TrainMemberInput{
				VehicleID: m.VehicleID,
				Reversed:  m.Reversed,
			})
		}
		in.Members = &members
	}
	eff, err := h.auth.Effective(r.Context(), actor.User, actor.Layout.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	d, err := h.svc.Update(r.Context(), actor.User.ID, trainID, eff, in)
	if err != nil {
		writeTrainError(w, err)
		return
	}
	if err := h.layoutTrains.BroadcastTrainUpdated(r.Context(), d.Train.ID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toTrainResponse(d))
}

// Delete handles DELETE /api/v1/trains/{id}. Cascades the layout
// roster cleanup.
func (h *TrainHandler) Delete(w http.ResponseWriter, r *http.Request) {
	trainID, ok := parseUintParam(r, "id")
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
	if _, err := h.svc.Delete(r.Context(), actor.User.ID, trainID, eff); err != nil {
		writeTrainError(w, err)
		return
	}
	if err := h.layoutTrains.PurgeTrain(r.Context(), trainID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeTrainError maps service sentinels to status codes.
func writeTrainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrTrainNotFound):
		writeJSONError(w, http.StatusNotFound, "train_not_found")
	case errors.Is(err, service.ErrTrainNameRequired):
		writeJSONError(w, http.StatusUnprocessableEntity, "train_name_required")
	case errors.Is(err, service.ErrTrainNameTaken):
		writeJSONError(w, http.StatusConflict, "train_name_taken")
	case errors.Is(err, service.ErrTrainNoMembers):
		writeJSONError(w, http.StatusUnprocessableEntity, "train_no_members")
	case errors.Is(err, service.ErrTrainMemberNotOwned):
		writeJSONError(w, http.StatusForbidden, "train_member_not_owned")
	case errors.Is(err, service.ErrTrainMemberMissing):
		writeJSONError(w, http.StatusUnprocessableEntity, "train_member_missing")
	case errors.Is(err, service.ErrTrainNotOwned):
		writeJSONError(w, http.StatusForbidden, "train_not_owned")
	default:
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
	}
}
