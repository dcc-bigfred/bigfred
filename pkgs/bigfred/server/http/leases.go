package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/protocol"
	"github.com/keskad/loco/pkgs/bigfred/server/service"
)

// LeaseHandler serves user-initiated drive lease endpoints.
type LeaseHandler struct {
	svc *service.LeaseService
}

func NewLeaseHandler(svc *service.LeaseService) *LeaseHandler {
	return &LeaseHandler{svc: svc}
}

func (h *LeaseHandler) ListReceived(w http.ResponseWriter, r *http.Request) {
	layoutID, actor, ok := requireSessionLayout(w, r)
	if !ok || h.svc == nil {
		return
	}
	rows, err := h.svc.ListReceived(r.Context(), layoutID, actor.User.ID)
	if err != nil {
		writeLeaseError(w, err)
		return
	}
	writeLeaseList(w, rows)
}

func (h *LeaseHandler) ListGranted(w http.ResponseWriter, r *http.Request) {
	layoutID, actor, ok := requireSessionLayout(w, r)
	if !ok || h.svc == nil {
		return
	}
	rows, err := h.svc.ListGranted(r.Context(), layoutID, actor.User.ID)
	if err != nil {
		writeLeaseError(w, err)
		return
	}
	writeLeaseList(w, rows)
}

func (h *LeaseHandler) Lendable(w http.ResponseWriter, r *http.Request) {
	layoutID, actor, ok := requireSessionLayout(w, r)
	if !ok || h.svc == nil {
		return
	}
	cat, err := h.svc.Lendable(r.Context(), layoutID, actor.User.ID)
	if err != nil {
		writeLeaseError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(protocol.ToLendableResponse(cat))
}

func (h *LeaseHandler) Create(w http.ResponseWriter, r *http.Request) {
	layoutID, actor, ok := requireSessionLayout(w, r)
	if !ok || h.svc == nil {
		return
	}
	var req protocol.CreateLeaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	entry, err := h.svc.Create(
		r.Context(),
		layoutID,
		actor.User,
		req.Kind,
		req.TargetID,
		req.ToUserID,
		req.SpeedLimit,
		time.Duration(req.DurationSeconds)*time.Second,
	)
	if err != nil {
		writeLeaseError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(protocol.ToLeaseEntryResponse(entry))
}

func (h *LeaseHandler) Patch(w http.ResponseWriter, r *http.Request) {
	layoutID, actor, ok := requireSessionLayout(w, r)
	if !ok || h.svc == nil {
		return
	}
	kind, targetID, ok := parseLeasePath(w, r)
	if !ok {
		return
	}
	var req protocol.PatchLeaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if req.SpeedLimit == nil && req.DurationSeconds == nil {
		writeJSONError(w, http.StatusBadRequest, "empty_patch")
		return
	}
	var entry cmd.LeaseEntry
	var err error
	if req.SpeedLimit != nil {
		entry, err = h.svc.UpdateSpeedLimit(r.Context(), layoutID, actor.User.ID, kind, targetID, *req.SpeedLimit)
	}
	if err == nil && req.DurationSeconds != nil {
		entry, err = h.svc.UpdateDuration(
			r.Context(),
			layoutID,
			actor.User.ID,
			kind,
			targetID,
			time.Duration(*req.DurationSeconds)*time.Second,
		)
	}
	if err != nil {
		writeLeaseError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(protocol.ToLeaseEntryResponse(entry))
}

func (h *LeaseHandler) Delete(w http.ResponseWriter, r *http.Request) {
	layoutID, actor, ok := requireSessionLayout(w, r)
	if !ok || h.svc == nil {
		return
	}
	kind, targetID, ok := parseLeasePath(w, r)
	if !ok {
		return
	}
	if err := h.svc.Revoke(r.Context(), layoutID, actor.User.ID, kind, targetID); err != nil {
		writeLeaseError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func requireSessionLayout(w http.ResponseWriter, r *http.Request) (uint, cmd.Identity, bool) {
	id, ok := IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return 0, cmd.Identity{}, false
	}
	return id.Layout.ID, id, true
}

func parseLeasePath(w http.ResponseWriter, r *http.Request) (domain.TakeoverTarget, string, bool) {
	kind := domain.TakeoverTarget(chi.URLParam(r, "kind"))
	if kind != domain.TakeoverTargetVehicle && kind != domain.TakeoverTargetTrain {
		writeJSONError(w, http.StatusBadRequest, "invalid_kind")
		return "", "", false
	}
	targetID := chi.URLParam(r, "id")
	if targetID == "" {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return "", "", false
	}
	return kind, targetID, true
}

func writeLeaseList(w http.ResponseWriter, rows []cmd.LeaseEntry) {
	out := make([]protocol.LeaseEntryResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, protocol.ToLeaseEntryResponse(row))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func writeLeaseError(w http.ResponseWriter, err error) {
	status, code := svcerrors.LeaseHTTPStatus(err)
	writeJSONError(w, status, code)
}
