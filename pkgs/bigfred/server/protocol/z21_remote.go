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
	Z21ServerEnabled  bool                     `json:"z21ServerEnabled"`
}

type Z21RemoteVehicle struct {
	VehicleID string `json:"vehicleId"`
	Addr      uint16 `json:"addr"`
}

type Z21RemotePendingPairing struct {
	PairingCV3   int    `json:"pairingCV3"`
	PairingCV4   int    `json:"pairingCV4"`
	DisplayLabel string `json:"displayLabel"`
	ExpiresAt    int64  `json:"expiresAt"`
}

type Z21RemotePairingResponse struct {
	PairingCV3   int    `json:"pairingCV3"`
	PairingCV4   int    `json:"pairingCV4"`
	DisplayLabel string `json:"displayLabel"`
	ExpiresAt    int64  `json:"expiresAt"`
	Instructions string `json:"instructions"`
}

func ToZ21RemotePendingPairing(req contract.Z21PairingReqWire) Z21RemotePendingPairing {
	return Z21RemotePendingPairing{
		PairingCV3:   req.PairingCV3,
		PairingCV4:   req.PairingCV4,
		DisplayLabel: req.DisplayLabel,
		ExpiresAt:    z21pairing.PendingExpiresAt(req).UnixMilli(),
	}
}

func ToZ21RemotePairingResponse(req contract.Z21PairingReqWire) Z21RemotePairingResponse {
	return Z21RemotePairingResponse{
		PairingCV3:   req.PairingCV3,
		PairingCV4:   req.PairingCV4,
		DisplayLabel: req.DisplayLabel,
		ExpiresAt:    z21pairing.PendingExpiresAt(req).UnixMilli(),
		Instructions: "Enter CV3 and CV4 in the Z21 app via POM on any locomotive.",
	}
}

func PtrInt64(v int64) *int64 {
	return &v
}
