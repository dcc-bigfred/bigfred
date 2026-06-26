package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/protocol"
)

// Z21RemoteHandler exposes handset pairing REST endpoints.
type Z21RemoteHandler struct {
	svc *cmd.Z21Remote
}

// NewZ21RemoteHandler returns a Z21RemoteHandler.
func NewZ21RemoteHandler(svc *cmd.Z21Remote) *Z21RemoteHandler {
	return &Z21RemoteHandler{svc: svc}
}

func (h *Z21RemoteHandler) requireZ21Remote(w http.ResponseWriter, r *http.Request) (uint, cmd.Identity, bool) {
	layoutID, actor, ok := requireOwnLayout(w, r)
	if !ok {
		return 0, actor, false
	}
	if h.svc == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "z21_remote_not_configured")
		return 0, actor, false
	}
	return layoutID, actor, true
}

// GetStatus handles GET …/z21-remote.
func (h *Z21RemoteHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	layoutID, actor, ok := h.requireZ21Remote(w, r)
	if !ok {
		return
	}
	csID, ok := parseUintParam(r, "csid")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	status, err := h.svc.GetStatus(r.Context(), layoutID, csID, actor.User.ID)
	if err != nil {
		writeZ21RemoteError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toZ21RemoteStatusResponse(status))
}

// ListClients handles GET …/z21-remote/clients.
func (h *Z21RemoteHandler) ListClients(w http.ResponseWriter, r *http.Request) {
	layoutID, _, ok := h.requireZ21Remote(w, r)
	if !ok {
		return
	}
	csID, ok := parseUintParam(r, "csid")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	snap, err := h.svc.ListClients(r.Context(), layoutID, csID)
	if err != nil {
		writeZ21RemoteError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(protocol.ToZ21RemoteClientsResponse(snap))
}

type z21RemotePairingRequest struct {
	VehicleIDs       []string `json:"vehicleIds"`
	AllowAllVehicles bool     `json:"allowAllVehicles"`
	HandsetBrakeSecs uint     `json:"handsetBrakeSecs"`
}

// StartPairing handles POST …/z21-remote/pairing.
func (h *Z21RemoteHandler) StartPairing(w http.ResponseWriter, r *http.Request) {
	layoutID, actor, ok := h.requireZ21Remote(w, r)
	if !ok {
		return
	}
	csID, ok := parseUintParam(r, "csid")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	var req z21RemotePairingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	pending, err := h.svc.StartPairing(r.Context(), layoutID, csID, actor.User.ID, cmd.Z21RemoteStartPairingInput{
		VehicleIDs:       req.VehicleIDs,
		AllowAllVehicles: req.AllowAllVehicles,
		HandsetBrakeSecs: req.HandsetBrakeSecs,
	})
	if err != nil {
		writeZ21RemoteError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(protocol.ToZ21RemotePairingResponse(pending))
}

// CancelPairing handles DELETE …/z21-remote/pairing.
func (h *Z21RemoteHandler) CancelPairing(w http.ResponseWriter, r *http.Request) {
	layoutID, actor, ok := h.requireZ21Remote(w, r)
	if !ok {
		return
	}
	csID, ok := parseUintParam(r, "csid")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	if err := h.svc.CancelPairing(r.Context(), layoutID, csID, actor.User.ID); err != nil {
		writeZ21RemoteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type z21RemoteSessionRequest struct {
	VehicleIDs       []string `json:"vehicleIds"`
	AllowAllVehicles *bool    `json:"allowAllVehicles"`
}

// UpdateSession handles PATCH …/z21-remote/session.
func (h *Z21RemoteHandler) UpdateSession(w http.ResponseWriter, r *http.Request) {
	layoutID, actor, ok := h.requireZ21Remote(w, r)
	if !ok {
		return
	}
	csID, ok := parseUintParam(r, "csid")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	var req z21RemoteSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if err := h.svc.UpdateSession(r.Context(), layoutID, csID, actor.User.ID, cmd.Z21RemoteUpdateSessionInput{
		VehicleIDs:       req.VehicleIDs,
		AllowAllVehicles: req.AllowAllVehicles,
		ClientKey:        r.URL.Query().Get("clientKey"),
	}); err != nil {
		writeZ21RemoteError(w, err)
		return
	}
	status, err := h.svc.GetStatus(r.Context(), layoutID, csID, actor.User.ID)
	if err != nil {
		writeZ21RemoteError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toZ21RemoteStatusResponse(status))
}

// Unpair handles DELETE …/z21-remote/session.
func (h *Z21RemoteHandler) Unpair(w http.ResponseWriter, r *http.Request) {
	layoutID, actor, ok := h.requireZ21Remote(w, r)
	if !ok {
		return
	}
	csID, ok := parseUintParam(r, "csid")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	if err := h.svc.Unpair(r.Context(), layoutID, csID, actor.User.ID, r.URL.Query().Get("clientKey")); err != nil {
		writeZ21RemoteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func toZ21RemoteStatusResponse(in cmd.Z21RemoteStatus) protocol.Z21RemoteStatus {
	out := protocol.Z21RemoteStatus{
		Paired:           in.Paired,
		ClientKey:        in.ClientKey,
		AllowAllVehicles: in.AllowAllVehicles,
		Z21ServerEnabled: in.Z21ServerEnabled,
		HandsetBrakeSecs: in.HandsetBrakeSecs,
		AllowedVehicles:  []protocol.Z21RemoteVehicle{},
	}
	if in.Paired {
		out.PairedAt = protocol.PtrInt64(in.PairedAt)
		out.LastSeenAt = protocol.PtrInt64(in.LastSeenAt)
	}
	for _, v := range in.AllowedVehicles {
		out.AllowedVehicles = append(out.AllowedVehicles, protocol.Z21RemoteVehicle{
			VehicleID: v.VehicleID,
			Addr:      v.Addr,
		})
	}
	if in.PendingReq {
		out.PendingPairing = &protocol.Z21RemotePendingPairing{
			PairingCV3:       in.PairingCV3,
			PairingCV4:       in.PairingCV4,
			DisplayLabel:     in.DisplayLabel,
			ExpiresAt:        in.ExpiresAt,
			HandsetBrakeSecs: in.HandsetBrakeSecs,
		}
	}
	return out
}

func writeZ21RemoteError(w http.ResponseWriter, err error) {
	status, code := svcerrors.Z21RemoteHTTPStatus(err)
	writeJSONError(w, status, code)
}
