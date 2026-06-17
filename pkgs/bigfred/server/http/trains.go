package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/protocol"
	"github.com/keskad/loco/pkgs/bigfred/server/service"
)

// TrainHandler bundles REST endpoints for the per-user train
// catalogue (§4.1).
type TrainHandler struct {
	svc          *cmd.Train
	layoutTrains *service.LayoutVehicleService
	auth         *cmd.Auth
}

// NewTrainHandler returns a TrainHandler.
func NewTrainHandler(
	svc *cmd.Train,
	layoutTrains *service.LayoutVehicleService,
	auth *cmd.Auth,
) *TrainHandler {
	return &TrainHandler{svc: svc, layoutTrains: layoutTrains, auth: auth}
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
	out := make([]protocol.TrainResponse, 0, len(rows))
	for _, d := range rows {
		out = append(out, protocol.ToTrainResponse(d))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// Create handles POST /api/v1/trains.
func (h *TrainHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req protocol.TrainCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	d, err := h.svc.Create(r.Context(), req.ToCreateInput(actor.User.ID))
	if err != nil {
		writeTrainError(w, err)
		return
	}
	if err := h.layoutTrains.SyncLayoutRosterForTrain(r.Context(), d.Train.ID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(protocol.ToTrainResponse(d))
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
	var req protocol.TrainUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	eff, err := h.auth.Effective(r.Context(), actor.User, actor.Layout.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	d, err := h.svc.Update(r.Context(), actor.User.ID, trainID, eff, req.ToUpdateInput())
	if err != nil {
		writeTrainError(w, err)
		return
	}
	if err := h.layoutTrains.BroadcastTrainUpdated(r.Context(), d.Train.ID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(protocol.ToTrainResponse(d))
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

// PatchMember handles PATCH /api/v1/trains/{id}/members/{memberId}.
func (h *TrainHandler) PatchMember(w http.ResponseWriter, r *http.Request) {
	trainID, ok := parseUintParam(r, "id")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	memberID, ok := parseUintParam(r, "memberId")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	actor, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req protocol.TrainMemberPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	eff, err := h.auth.Effective(r.Context(), actor.User, actor.Layout.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	member, err := h.svc.UpdateMemberMultiplier(r.Context(), actor.User.ID, trainID, memberID, eff, req.SpeedMultiplier)
	if err != nil {
		writeTrainError(w, err)
		return
	}
	if err := h.layoutTrains.BroadcastTrainUpdated(r.Context(), trainID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(protocol.ToTrainMemberResponse(member))
}

func writeTrainError(w http.ResponseWriter, err error) {
	status, code := svcerrors.TrainHTTPStatus(err)
	writeJSONError(w, status, code)
}
