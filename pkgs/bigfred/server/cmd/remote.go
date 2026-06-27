package cmd

import (
	"context"
	"errors"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/remotepairing"
)

// Remote manages inbound handset pairing across protocols on one command station.
type Remote struct {
	z21 *Z21Remote
}

// NewRemote returns a Remote service backed by protocol-specific delegates.
func NewRemote(z21 *Z21Remote) *Remote {
	return &Remote{z21: z21}
}

// RemoteProtocolInfo describes one available inbound protocol on a CS.
type RemoteProtocolInfo struct {
	Protocol string
	Enabled  bool
}

// RemoteStatus is the domain view of handset pairing state for the current user.
type RemoteStatus struct {
	Protocol           string
	Paired             bool
	ClientKey          string
	PairedAt           int64
	LastSeenAt         int64
	AllowAllVehicles   bool
	AllowedVehicles    []RemoteVehicleRef
	PendingReq         bool
	PairingCV3         int
	PairingCV4         int
	DisplayLabel       string
	ExpiresAt          int64
	HandsetBrakeSecs   uint
	AvailableProtocols []RemoteProtocolInfo
}

// RemoteVehicleRef is one vehicle in the paired scope.
type RemoteVehicleRef struct {
	VehicleID string
	Addr      uint16
}

// RemoteStartPairingInput carries vehicle scope for a new pairing code.
type RemoteStartPairingInput struct {
	VehicleIDs       []string
	AllowAllVehicles bool
	HandsetBrakeSecs uint
}

// RemoteUpdateSessionInput updates scope on an active session.
type RemoteUpdateSessionInput struct {
	VehicleIDs       []string
	AllowAllVehicles *bool
	ClientKey        string
}

// GetStatus returns pairing state for the current user.
func (s *Remote) GetStatus(ctx context.Context, layoutID, csID, userID uint) (RemoteStatus, error) {
	if s == nil || s.z21 == nil {
		return RemoteStatus{}, errors.New("remote service not configured")
	}
	z21, err := s.z21.GetStatus(ctx, layoutID, csID, userID)
	if err != nil {
		return RemoteStatus{}, err
	}
	return remoteStatusFromZ21(z21), nil
}

// ListClients returns the live handset presence snapshot for one CS.
func (s *Remote) ListClients(ctx context.Context, layoutID, csID uint) (contract.RemoteClientsSnapshotWire, error) {
	if s == nil || s.z21 == nil {
		return contract.RemoteClientsSnapshotWire{}, errors.New("remote service not configured")
	}
	return s.z21.ListClients(ctx, layoutID, csID)
}

// StartPairing creates a pending pairing code for the given protocol.
func (s *Remote) StartPairing(ctx context.Context, layoutID, csID, userID uint, protocol string, in RemoteStartPairingInput) (contract.RemotePendingWire, error) {
	switch protocol {
	case contract.RemoteProtocolZ21:
		if s == nil || s.z21 == nil {
			return contract.RemotePendingWire{}, errors.New("remote service not configured")
		}
		req, err := s.z21.StartPairing(ctx, layoutID, csID, userID, Z21RemoteStartPairingInput{
			VehicleIDs:       in.VehicleIDs,
			AllowAllVehicles: in.AllowAllVehicles,
			HandsetBrakeSecs: in.HandsetBrakeSecs,
		})
		if err != nil {
			return contract.RemotePendingWire{}, err
		}
		return req, nil
	default:
		return contract.RemotePendingWire{}, svcerrors.ErrRemoteProtocolUnknown
	}
}

// CancelPairing deletes the current user's pending pairing code.
func (s *Remote) CancelPairing(ctx context.Context, layoutID, csID, userID uint) error {
	if s == nil || s.z21 == nil {
		return errors.New("remote service not configured")
	}
	return s.z21.CancelPairing(ctx, layoutID, csID, userID)
}

// UpdateSession changes vehicle scope without re-pairing.
func (s *Remote) UpdateSession(ctx context.Context, layoutID, csID, userID uint, in RemoteUpdateSessionInput) error {
	if s == nil || s.z21 == nil {
		return errors.New("remote service not configured")
	}
	return s.z21.UpdateSession(ctx, layoutID, csID, userID, Z21RemoteUpdateSessionInput{
		VehicleIDs:       in.VehicleIDs,
		AllowAllVehicles: in.AllowAllVehicles,
		ClientKey:        in.ClientKey,
	})
}

// Unpair removes the user's active handset session.
func (s *Remote) Unpair(ctx context.Context, layoutID, csID, userID uint, clientKey string) error {
	if s == nil || s.z21 == nil {
		return errors.New("remote service not configured")
	}
	return s.z21.Unpair(ctx, layoutID, csID, userID, clientKey)
}

func remoteStatusFromZ21(in Z21RemoteStatus) RemoteStatus {
	out := RemoteStatus{
		Paired:           in.Paired,
		ClientKey:        in.ClientKey,
		PairedAt:         in.PairedAt,
		LastSeenAt:       in.LastSeenAt,
		AllowAllVehicles: in.AllowAllVehicles,
		PendingReq:       in.PendingReq,
		PairingCV3:       in.PairingCV3,
		PairingCV4:       in.PairingCV4,
		DisplayLabel:     in.DisplayLabel,
		ExpiresAt:        in.ExpiresAt,
		HandsetBrakeSecs: in.HandsetBrakeSecs,
		AvailableProtocols: []RemoteProtocolInfo{{
			Protocol: contract.RemoteProtocolZ21,
			Enabled:  in.Z21ServerEnabled,
		}},
	}
	if in.Paired {
		out.Protocol = contract.RemoteProtocolZ21
	} else if in.PendingReq {
		out.Protocol = contract.RemoteProtocolZ21
	}
	for _, v := range in.AllowedVehicles {
		out.AllowedVehicles = append(out.AllowedVehicles, RemoteVehicleRef{
			VehicleID: v.VehicleID,
			Addr:      v.Addr,
		})
	}
	return out
}

// MapUserAlreadyPaired maps remotepairing store errors to service errors.
func MapUserAlreadyPaired(err error) error {
	if errors.Is(err, remotepairing.ErrUserAlreadyPaired) {
		return svcerrors.ErrRemoteUserAlreadyPaired
	}
	return err
}
