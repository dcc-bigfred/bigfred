package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/protocol"
)

// RemoteHandler exposes protocol-agnostic handset pairing REST endpoints.
type RemoteHandler struct {
	svc  *cmd.Remote
	auth *cmd.Auth
}

// NewRemoteHandler returns a RemoteHandler. auth gates admin-only endpoints
// (clients list) against the caller's effective role.
func NewRemoteHandler(svc *cmd.Remote, auth *cmd.Auth) *RemoteHandler {
	return &RemoteHandler{svc: svc, auth: auth}
}

func (h *RemoteHandler) requireRemote(w http.ResponseWriter, r *http.Request) (uint, cmd.Identity, bool) {
	layoutID, actor, ok := requireOwnLayout(w, r)
	if !ok {
		return 0, actor, false
	}
	if h.svc == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "remote_not_configured")
		return 0, actor, false
	}
	return layoutID, actor, true
}

// requireAdminRemote is requireRemote plus an effective-admin check, used
// for endpoints that surface every connected handset (IPs, user logins).
func (h *RemoteHandler) requireAdminRemote(w http.ResponseWriter, r *http.Request) (uint, cmd.Identity, bool) {
	layoutID, actor, ok := h.requireRemote(w, r)
	if !ok {
		return 0, actor, false
	}
	if h.auth == nil {
		// No auth wired (tests) — fall back to permanent role.
		if !actor.HasRole(domain.RoleAdmin) {
			writeJSONError(w, http.StatusForbidden, "forbidden")
			return 0, actor, false
		}
		return layoutID, actor, true
	}
	eff, err := h.auth.Effective(r.Context(), actor.User, actor.Layout.ID)
	if err != nil || !eff.Has(domain.RoleAdmin) {
		writeJSONError(w, http.StatusForbidden, "forbidden")
		return 0, actor, false
	}
	return layoutID, actor, true
}

// GetStatus handles GET …/remotes/status.
func (h *RemoteHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	layoutID, actor, ok := h.requireRemote(w, r)
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
		writeRemoteError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toRemoteStatusResponse(status))
}

// ListClients handles GET …/remotes/clients. Admin-only: surfaces every
// connected handset (IP, user login) on the command station.
func (h *RemoteHandler) ListClients(w http.ResponseWriter, r *http.Request) {
	layoutID, _, ok := h.requireAdminRemote(w, r)
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
		writeRemoteError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(protocol.ToRemoteClientsResponse(snap))
}

type remotePairingRequest struct {
	VehicleIDs       []string `json:"vehicleIds"`
	AllowAllVehicles bool     `json:"allowAllVehicles"`
	HandsetBrakeSecs uint     `json:"handsetBrakeSecs"`
}

// StartPairing handles POST …/remotes/{protocol}/pairing.
func (h *RemoteHandler) StartPairing(w http.ResponseWriter, r *http.Request) {
	layoutID, actor, ok := h.requireRemote(w, r)
	if !ok {
		return
	}
	csID, ok := parseUintParam(r, "csid")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	protocolName := chi.URLParam(r, "protocol")
	if protocolName == "" {
		writeJSONError(w, http.StatusBadRequest, "invalid_protocol")
		return
	}
	var req remotePairingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	pending, err := h.svc.StartPairing(r.Context(), layoutID, csID, actor.User.ID, protocolName, cmd.RemoteStartPairingInput{
		UserLogin:        actor.User.Login,
		VehicleIDs:       req.VehicleIDs,
		AllowAllVehicles: req.AllowAllVehicles,
		HandsetBrakeSecs: req.HandsetBrakeSecs,
	})
	if err != nil {
		writeRemoteError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(protocol.ToRemotePairingResponse(pending))
}

// CancelPairing handles DELETE …/remotes/pairing.
func (h *RemoteHandler) CancelPairing(w http.ResponseWriter, r *http.Request) {
	layoutID, actor, ok := h.requireRemote(w, r)
	if !ok {
		return
	}
	csID, ok := parseUintParam(r, "csid")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	if err := h.svc.CancelPairing(r.Context(), layoutID, csID, actor.User.ID); err != nil {
		writeRemoteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type remoteSessionRequest struct {
	VehicleIDs       []string `json:"vehicleIds"`
	AllowAllVehicles *bool    `json:"allowAllVehicles"`
}

// UpdateSession handles PATCH …/remotes/session.
func (h *RemoteHandler) UpdateSession(w http.ResponseWriter, r *http.Request) {
	layoutID, actor, ok := h.requireRemote(w, r)
	if !ok {
		return
	}
	csID, ok := parseUintParam(r, "csid")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	var req remoteSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if err := h.svc.UpdateSession(r.Context(), layoutID, csID, actor.User.ID, cmd.RemoteUpdateSessionInput{
		VehicleIDs:       req.VehicleIDs,
		AllowAllVehicles: req.AllowAllVehicles,
		ClientKey:        r.URL.Query().Get("clientKey"),
	}); err != nil {
		writeRemoteError(w, err)
		return
	}
	status, err := h.svc.GetStatus(r.Context(), layoutID, csID, actor.User.ID)
	if err != nil {
		writeRemoteError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toRemoteStatusResponse(status))
}

// Unpair handles DELETE …/remotes/session.
func (h *RemoteHandler) Unpair(w http.ResponseWriter, r *http.Request) {
	layoutID, actor, ok := h.requireRemote(w, r)
	if !ok {
		return
	}
	csID, ok := parseUintParam(r, "csid")
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	if err := h.svc.Unpair(r.Context(), layoutID, csID, actor.User.ID, r.URL.Query().Get("clientKey")); err != nil {
		writeRemoteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func toRemoteStatusResponse(in cmd.RemoteStatus) protocol.RemoteStatus {
	z21Enabled := false
	available := make([]protocol.RemoteProtocolInfo, 0, len(in.AvailableProtocols))
	for _, p := range in.AvailableProtocols {
		available = append(available, protocol.RemoteProtocolInfo{
			Protocol: p.Protocol,
			Enabled:  p.Enabled,
		})
		if p.Protocol == contract.RemoteProtocolZ21 {
			z21Enabled = p.Enabled
		}
	}
	out := protocol.RemoteStatus{
		Protocol:           in.Protocol,
		Paired:             in.Paired,
		ClientKey:          in.ClientKey,
		AllowAllVehicles:   in.AllowAllVehicles,
		HandsetBrakeSecs:   in.HandsetBrakeSecs,
		AvailableProtocols: available,
		Z21ServerEnabled:   z21Enabled,
		AllowedVehicles:    make([]protocol.RemoteVehicle, 0, len(in.AllowedVehicles)),
	}
	if in.Paired {
		out.PairedAt = protocol.PtrInt64(in.PairedAt)
		out.LastSeenAt = protocol.PtrInt64(in.LastSeenAt)
	}
	for _, v := range in.AllowedVehicles {
		out.AllowedVehicles = append(out.AllowedVehicles, protocol.RemoteVehicle{
			VehicleID: v.VehicleID,
			Addr:      v.Addr,
		})
	}
	if in.PendingReq {
		out.PendingPairing = &protocol.RemotePendingPairing{
			Protocol:         in.Protocol,
			PairingCV3:       in.PairingCV3,
			PairingCV4:       in.PairingCV4,
			PairingCode:      in.PairingCode,
			DisplayLabel:     in.DisplayLabel,
			ExpiresAt:        in.ExpiresAt,
			HandsetBrakeSecs: in.HandsetBrakeSecs,
		}
	}
	return out
}

func writeRemoteError(w http.ResponseWriter, err error) {
	status, code := svcerrors.RemoteHTTPStatus(err)
	writeJSONError(w, status, code)
}
