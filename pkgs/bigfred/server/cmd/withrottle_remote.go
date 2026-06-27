package cmd

import (
	"context"
	"errors"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/remotepairing"
)

// WithrottleRemote manages handset pairing for one layout command station.
type WithrottleRemote struct {
	handsetRemoteDeps
	pairing *remotepairing.Store
}

// NewWithrottleRemote returns a WithrottleRemote service.
func NewWithrottleRemote(
	pairing *remotepairing.Store,
	stations *repo.CommandStations,
	layoutCS *repo.LayoutCommandStations,
	roster *LayoutRoster,
	snapshot *LayoutRosterSnapshot,
) *WithrottleRemote {
	return &WithrottleRemote{
		pairing: pairing,
		handsetRemoteDeps: handsetRemoteDeps{
			stations: stations,
			layoutCS: layoutCS,
			roster:   roster,
			snapshot: snapshot,
		},
	}
}

// WithrottleRemoteStatus is the domain view of WiThrottle pairing state.
type WithrottleRemoteStatus struct {
	Paired                 bool
	ClientKey              string
	PairedAt               int64
	LastSeenAt             int64
	AllowAllVehicles       bool
	AllowedVehicles      []RemoteVehicleRef
	PendingReq             bool
	PairingCode            string
	DisplayLabel           string
	ExpiresAt              int64
	HandsetBrakeSecs       uint
	WithrottleServerEnabled bool
}

// WithrottleRemoteStartPairingInput carries vehicle scope for a new pairing code.
type WithrottleRemoteStartPairingInput struct {
	VehicleIDs       []string
	AllowAllVehicles bool
	HandsetBrakeSecs uint
}

// GetStatus returns pairing state for the current user.
func (s *WithrottleRemote) GetStatus(ctx context.Context, layoutID, csID, userID uint) (WithrottleRemoteStatus, error) {
	cs, err := s.findCommandStation(ctx, layoutID, csID)
	if err != nil {
		return WithrottleRemoteStatus{}, err
	}
	out := WithrottleRemoteStatus{WithrottleServerEnabled: cs.WithrottleServerEnabled}
	if s.pairing == nil {
		return out, nil
	}
	if pending, ok, err := s.pairing.GetPendingByUser(ctx, layoutID, csID, userID); err != nil {
		return WithrottleRemoteStatus{}, err
	} else if ok && pending.Protocol == contract.RemoteProtocolWithrottle {
		out.PendingReq = true
		out.PairingCode = pending.PairingCode
		out.DisplayLabel = pending.DisplayLabel
		out.ExpiresAt = remotepairing.PendingExpiresAt(pending).UnixMilli()
		out.HandsetBrakeSecs = contract.NormaliseHandsetBrakeSecs(pending.HandsetBrakeSecs)
	}
	sessions, err := s.pairing.ListActiveByUser(ctx, layoutID, csID, userID)
	if err != nil {
		return WithrottleRemoteStatus{}, err
	}
	for _, active := range sessions {
		if active.Protocol != contract.RemoteProtocolWithrottle {
			continue
		}
		out.Paired = true
		out.ClientKey = active.ClientKey
		out.PairedAt = active.PairedAt
		out.LastSeenAt = active.LastSeenAt
		out.AllowAllVehicles = active.AllowAllVehicles
		out.HandsetBrakeSecs = contract.NormaliseHandsetBrakeSecs(active.HandsetBrakeSecs)
		out.AllowedVehicles = s.vehiclesFromSession(ctx, layoutID, active.VehicleIDs)
		break
	}
	return out, nil
}

// StartPairing creates a pending 6-digit code for the user.
func (s *WithrottleRemote) StartPairing(ctx context.Context, layoutID, csID, userID uint, in WithrottleRemoteStartPairingInput) (contract.RemotePendingWire, error) {
	if _, err := s.ensureReady(ctx, layoutID, csID); err != nil {
		return contract.RemotePendingWire{}, err
	}
	if s.pairing == nil {
		return contract.RemotePendingWire{}, errors.New("withrottle pairing store not configured")
	}
	vehicleIDs, addrs, err := s.resolvePairingScope(ctx, layoutID, userID, handsetPairingScopeInput{
		VehicleIDs:       in.VehicleIDs,
		AllowAllVehicles: in.AllowAllVehicles,
	})
	if err != nil {
		return contract.RemotePendingWire{}, err
	}
	brakeSecs := in.HandsetBrakeSecs
	if brakeSecs == 0 {
		brakeSecs = contract.Z21HandsetBrakeSecsDefault
	} else if !contract.ValidHandsetBrakeSecs(brakeSecs) {
		return contract.RemotePendingWire{}, svcerrors.ErrZ21HandsetBrakeSecsInvalid
	}
	req, err := s.pairing.CreateWithrottlePairingRequest(ctx, remotepairing.CreateWithrottlePairingInput{
		LayoutID:         layoutID,
		CommandStationID: csID,
		UserID:           userID,
		VehicleIDs:       vehicleIDs,
		AllowedAddrs:     addrs,
		AllowAllVehicles: in.AllowAllVehicles,
		HandsetBrakeSecs: brakeSecs,
	})
	return req, MapUserAlreadyPaired(err)
}

func (s *WithrottleRemote) ensureReady(ctx context.Context, layoutID, csID uint) (domain.CommandStation, error) {
	cs, err := s.findCommandStation(ctx, layoutID, csID)
	if err != nil {
		return domain.CommandStation{}, err
	}
	if !cs.WithrottleServerEnabled {
		return domain.CommandStation{}, svcerrors.ErrWithrottleServerDisabled
	}
	return cs, nil
}
