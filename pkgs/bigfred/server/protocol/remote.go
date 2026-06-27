package protocol

import (
	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/remotepairing"
)

// RemoteProtocolInfo describes one inbound protocol on a command station.
type RemoteProtocolInfo struct {
	Protocol string `json:"protocol"`
	Enabled  bool   `json:"enabled"`
}

// RemoteStatus is returned by GET …/remotes/status.
type RemoteStatus struct {
	Protocol           string                `json:"protocol,omitempty"`
	Paired             bool                  `json:"paired"`
	ClientKey          string                `json:"clientKey,omitempty"`
	PairedAt           *int64                `json:"pairedAt,omitempty"`
	LastSeenAt         *int64                `json:"lastSeenAt,omitempty"`
	AllowAllVehicles   bool                  `json:"allowAllVehicles"`
	AllowedVehicles    []RemoteVehicle       `json:"allowedVehicles"`
	PendingPairing     *RemotePendingPairing `json:"pendingPairing,omitempty"`
	HandsetBrakeSecs   uint                  `json:"handsetBrakeSecs,omitempty"`
	AvailableProtocols []RemoteProtocolInfo  `json:"availableProtocols"`
	// Z21ServerEnabled is kept for the Z21 UI until a dedicated protocol picker ships.
	Z21ServerEnabled bool `json:"z21ServerEnabled"`
}

// RemoteVehicle is one vehicle in the paired scope.
type RemoteVehicle struct {
	VehicleID string `json:"vehicleId"`
	Addr      uint16 `json:"addr"`
}

// RemotePendingPairing is an in-flight pairing code.
type RemotePendingPairing struct {
	Protocol         string `json:"protocol"`
	PairingCV3       int    `json:"pairingCV3,omitempty"`
	PairingCV4       int    `json:"pairingCV4,omitempty"`
	PairingCode      string `json:"pairingCode,omitempty"`
	DisplayLabel     string `json:"displayLabel"`
	ExpiresAt        int64  `json:"expiresAt"`
	HandsetBrakeSecs uint   `json:"handsetBrakeSecs,omitempty"`
}

// RemotePairingResponse is returned by POST …/remotes/{protocol}/pairing.
type RemotePairingResponse struct {
	Protocol         string `json:"protocol"`
	PairingCV3       int    `json:"pairingCV3,omitempty"`
	PairingCV4       int    `json:"pairingCV4,omitempty"`
	PairingCode      string `json:"pairingCode,omitempty"`
	DisplayLabel     string `json:"displayLabel"`
	ExpiresAt        int64  `json:"expiresAt"`
	HandsetBrakeSecs uint   `json:"handsetBrakeSecs"`
	Instructions     string `json:"instructions"`
}

// RemoteClientsResponse is returned by GET …/remotes/clients.
type RemoteClientsResponse struct {
	LayoutID         uint                   `json:"layoutId"`
	CommandStationID uint                   `json:"commandStationId"`
	IPStickiness     bool                   `json:"ipStickiness"`
	UpdatedAt        int64                  `json:"updatedAt"`
	Clients          []RemoteClientResponse `json:"clients"`
}

// RemoteClientResponse describes one inbound handset participant.
type RemoteClientResponse struct {
	ClientKey        string `json:"clientKey"`
	Protocol         string `json:"protocol,omitempty"`
	IP               string `json:"ip"`
	Port             int    `json:"port"`
	Paired           bool   `json:"paired"`
	UserID           uint   `json:"userId,omitempty"`
	UserLogin        string `json:"userLogin,omitempty"`
	LastSeenAt       int64  `json:"lastSeenAt"`
	ConnectedAt      int64  `json:"connectedAt"`
	SessionExpiresAt int64  `json:"sessionExpiresAt,omitempty"`
	IdleBraked       bool   `json:"idleBraked"`
}

// ToRemotePairingResponse maps a pending wire to the pairing response.
func ToRemotePairingResponse(req contract.RemotePendingWire) RemotePairingResponse {
	out := RemotePairingResponse{
		Protocol:         req.Protocol,
		DisplayLabel:     req.DisplayLabel,
		ExpiresAt:        remotepairing.PendingExpiresAt(req).UnixMilli(),
		HandsetBrakeSecs: contract.NormaliseHandsetBrakeSecs(req.HandsetBrakeSecs),
	}
	if req.Protocol == contract.RemoteProtocolZ21 {
		out.PairingCV3 = req.PairingCV3
		out.PairingCV4 = req.PairingCV4
		out.Instructions = "Enter CV3 and CV4 via POM, programming track, or function keys F0–F32."
	}
	if req.Protocol == contract.RemoteProtocolWithrottle {
		out.PairingCode = req.PairingCode
		out.Instructions = "In Engine Driver: acquire the Pair with BigFred sentinel loco and press F-keys for each digit, or set Device Name (N) to the 6-digit code."
	}
	return out
}

// ToRemoteClientsResponse maps a clients snapshot to the wire response.
func ToRemoteClientsResponse(snap contract.RemoteClientsSnapshotWire) RemoteClientsResponse {
	out := RemoteClientsResponse{
		LayoutID:         snap.LayoutID,
		CommandStationID: snap.CommandStationID,
		IPStickiness:     snap.IPStickiness,
		UpdatedAt:        snap.UpdatedAt,
		Clients:          make([]RemoteClientResponse, 0, len(snap.Clients)),
	}
	for _, c := range snap.Clients {
		out.Clients = append(out.Clients, RemoteClientResponse{
			ClientKey:        c.ClientKey,
			Protocol:         c.Protocol,
			IP:               c.IP,
			Port:             c.Port,
			Paired:           c.Paired,
			UserID:           c.UserID,
			UserLogin:        c.UserLogin,
			LastSeenAt:       c.LastSeenAt,
			ConnectedAt:      c.ConnectedAt,
			SessionExpiresAt: c.SessionExpiresAt,
			IdleBraked:       c.IdleBraked,
		})
	}
	return out
}

// PtrInt64 returns a pointer to v.
func PtrInt64(v int64) *int64 {
	return &v
}
