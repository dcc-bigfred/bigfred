package protocol

import (
	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/z21pairing"
)

// Z21RemoteStatus is returned by GET …/z21-remote.
type Z21RemoteStatus struct {
	Paired            bool                     `json:"paired"`
	ClientKey         string                   `json:"clientKey,omitempty"`
	PairedAt          *int64                   `json:"pairedAt,omitempty"`
	LastSeenAt        *int64                   `json:"lastSeenAt,omitempty"`
	AllowAllVehicles  bool                     `json:"allowAllVehicles"`
	AllowedVehicles   []Z21RemoteVehicle       `json:"allowedVehicles"`
	PendingPairing    *Z21RemotePendingPairing `json:"pendingPairing,omitempty"`
	HandsetBrakeSecs  uint                     `json:"handsetBrakeSecs,omitempty"`
	Z21ServerEnabled  bool                     `json:"z21ServerEnabled"`
}

type Z21RemoteVehicle struct {
	VehicleID string `json:"vehicleId"`
	Addr      uint16 `json:"addr"`
}

type Z21RemotePendingPairing struct {
	PairingCV3       int    `json:"pairingCV3"`
	PairingCV4       int    `json:"pairingCV4"`
	DisplayLabel     string `json:"displayLabel"`
	ExpiresAt        int64  `json:"expiresAt"`
	HandsetBrakeSecs uint   `json:"handsetBrakeSecs,omitempty"`
}

type Z21RemotePairingResponse struct {
	PairingCV3       int    `json:"pairingCV3"`
	PairingCV4       int    `json:"pairingCV4"`
	DisplayLabel     string `json:"displayLabel"`
	ExpiresAt        int64  `json:"expiresAt"`
	HandsetBrakeSecs uint   `json:"handsetBrakeSecs"`
	Instructions     string `json:"instructions"`
}

func ToZ21RemotePendingPairing(req contract.Z21PairingReqWire) Z21RemotePendingPairing {
	return Z21RemotePendingPairing{
		PairingCV3:       req.PairingCV3,
		PairingCV4:       req.PairingCV4,
		DisplayLabel:     req.DisplayLabel,
		ExpiresAt:        z21pairing.PendingExpiresAt(req).UnixMilli(),
		HandsetBrakeSecs: contract.NormaliseHandsetBrakeSecs(req.HandsetBrakeSecs),
	}
}

func ToZ21RemotePairingResponse(req contract.Z21PairingReqWire) Z21RemotePairingResponse {
	return Z21RemotePairingResponse{
		PairingCV3:       req.PairingCV3,
		PairingCV4:       req.PairingCV4,
		DisplayLabel:     req.DisplayLabel,
		ExpiresAt:        z21pairing.PendingExpiresAt(req).UnixMilli(),
		HandsetBrakeSecs: contract.NormaliseHandsetBrakeSecs(req.HandsetBrakeSecs),
		Instructions:     "Enter CV3 and CV4 via POM, programming track, or function keys F0–F32.",
	}
}

type Z21RemoteClientsResponse struct {
	LayoutID         uint                      `json:"layoutId"`
	CommandStationID uint                      `json:"commandStationId"`
	IPStickiness     bool                      `json:"ipStickiness"`
	UpdatedAt        int64                     `json:"updatedAt"`
	Clients          []Z21RemoteClientResponse `json:"clients"`
}

type Z21RemoteClientResponse struct {
	ClientKey        string `json:"clientKey"`
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

func ToZ21RemoteClientsResponse(snap contract.Z21ClientsSnapshotWire) Z21RemoteClientsResponse {
	out := Z21RemoteClientsResponse{
		LayoutID:         snap.LayoutID,
		CommandStationID: snap.CommandStationID,
		IPStickiness:     snap.IPStickiness,
		UpdatedAt:        snap.UpdatedAt,
		Clients:          make([]Z21RemoteClientResponse, 0, len(snap.Clients)),
	}
	for _, c := range snap.Clients {
		out.Clients = append(out.Clients, Z21RemoteClientResponse{
			ClientKey:        c.ClientKey,
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

func PtrInt64(v int64) *int64 {
	return &v
}
