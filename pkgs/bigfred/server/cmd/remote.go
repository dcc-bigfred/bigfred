package cmd

import (
	"context"
	"errors"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/remotepairing"
)

// Remote manages inbound handset pairing across protocols on one command station.
type Remote struct {
	z21        *Z21Remote
	withrottle *WithrottleRemote
	pairing    *remotepairing.Store
	users      *repo.Users
}

// NewRemote returns a Remote service backed by protocol-specific delegates.
func NewRemote(
	z21 *Z21Remote,
	withrottle *WithrottleRemote,
	pairing *remotepairing.Store,
	users *repo.Users,
) *Remote {
	return &Remote{z21: z21, withrottle: withrottle, pairing: pairing, users: users}
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
	PairingCode        string
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
	if s == nil || (s.z21 == nil && s.withrottle == nil) {
		return RemoteStatus{}, errors.New("remote service not configured")
	}
	var z21Status Z21RemoteStatus
	var wtStatus WithrottleRemoteStatus
	var z21Err, wtErr error
	if s.z21 != nil {
		z21Status, z21Err = s.z21.GetStatus(ctx, layoutID, csID, userID)
	}
	if s.withrottle != nil {
		wtStatus, wtErr = s.withrottle.GetStatus(ctx, layoutID, csID, userID)
	}
	if z21Err != nil && wtErr != nil {
		return RemoteStatus{}, z21Err
	}
	out := mergeRemoteStatus(z21Status, wtStatus)
	return out, nil
}

// ListClients returns the live handset presence snapshot for one CS.
func (s *Remote) ListClients(ctx context.Context, layoutID, csID uint) (contract.RemoteClientsSnapshotWire, error) {
	if s == nil || s.pairing == nil {
		return contract.RemoteClientsSnapshotWire{}, errors.New("remote service not configured")
	}
	cs, err := s.findCommandStation(ctx, layoutID, csID)
	if err != nil {
		return contract.RemoteClientsSnapshotWire{}, err
	}
	if !cs.Z21ServerEnabled && !cs.WithrottleServerEnabled {
		return contract.RemoteClientsSnapshotWire{}, svcerrors.ErrRemoteServerDisabled
	}
	snap, ok, err := s.pairing.GetClientsSnapshot(ctx, layoutID, csID)
	if err != nil {
		return contract.RemoteClientsSnapshotWire{}, err
	}
	if !ok {
		snap = contract.RemoteClientsSnapshotWire{
			LayoutID:         layoutID,
			CommandStationID: csID,
			IPStickiness:     cs.Z21IPStickiness,
			Clients:          nil,
		}
	}
	if s.users != nil {
		ids := make([]uint, 0, len(snap.Clients))
		seen := make(map[uint]struct{}, len(snap.Clients))
		for _, cl := range snap.Clients {
			if cl.UserID == 0 {
				continue
			}
			if _, ok := seen[cl.UserID]; ok {
				continue
			}
			seen[cl.UserID] = struct{}{}
			ids = append(ids, cl.UserID)
		}
		byID, err := s.users.FindByIDs(ctx, ids)
		if err == nil {
			for i := range snap.Clients {
				if u, ok := byID[snap.Clients[i].UserID]; ok {
					snap.Clients[i].UserLogin = u.Login
				}
			}
		}
	}
	return snap, nil
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
	case contract.RemoteProtocolWithrottle:
		if s == nil || s.withrottle == nil {
			return contract.RemotePendingWire{}, errors.New("remote service not configured")
		}
		req, err := s.withrottle.StartPairing(ctx, layoutID, csID, userID, WithrottleRemoteStartPairingInput{
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
	if s == nil || s.pairing == nil {
		return errors.New("remote service not configured")
	}
	if _, err := s.findCommandStation(ctx, layoutID, csID); err != nil {
		return err
	}
	return s.pairing.CancelPendingPairing(ctx, layoutID, csID, userID)
}

// UpdateSession changes vehicle scope without re-pairing.
func (s *Remote) UpdateSession(ctx context.Context, layoutID, csID, userID uint, in RemoteUpdateSessionInput) error {
	protocol, err := s.resolveSessionProtocol(ctx, layoutID, csID, userID, in.ClientKey)
	if err != nil {
		return err
	}
	switch protocol {
	case contract.RemoteProtocolZ21:
		if s.z21 == nil {
			return errors.New("remote service not configured")
		}
		return s.z21.UpdateSession(ctx, layoutID, csID, userID, Z21RemoteUpdateSessionInput{
			VehicleIDs:       in.VehicleIDs,
			AllowAllVehicles: in.AllowAllVehicles,
			ClientKey:        in.ClientKey,
		})
	case contract.RemoteProtocolWithrottle:
		if s.withrottle == nil {
			return errors.New("remote service not configured")
		}
		return s.withrottle.UpdateSession(ctx, layoutID, csID, userID, WithrottleRemoteUpdateSessionInput{
			VehicleIDs:       in.VehicleIDs,
			AllowAllVehicles: in.AllowAllVehicles,
			ClientKey:        in.ClientKey,
		})
	default:
		return svcerrors.ErrRemoteProtocolUnknown
	}
}

// Unpair removes the user's active handset session.
func (s *Remote) Unpair(ctx context.Context, layoutID, csID, userID uint, clientKey string) error {
	protocol, err := s.resolveSessionProtocol(ctx, layoutID, csID, userID, clientKey)
	if err != nil {
		if clientKey == "" && errors.Is(err, svcerrors.ErrRemoteSessionNotFound) {
			return nil
		}
		return err
	}
	switch protocol {
	case contract.RemoteProtocolZ21:
		if s.z21 == nil {
			return errors.New("remote service not configured")
		}
		return s.z21.Unpair(ctx, layoutID, csID, userID, clientKey)
	case contract.RemoteProtocolWithrottle:
		if s.withrottle == nil {
			return errors.New("remote service not configured")
		}
		return s.withrottle.Unpair(ctx, layoutID, csID, userID, clientKey)
	default:
		return svcerrors.ErrRemoteProtocolUnknown
	}
}

func (s *Remote) findCommandStation(ctx context.Context, layoutID, csID uint) (domain.CommandStation, error) {
	if s.z21 != nil {
		return s.z21.findCommandStation(ctx, layoutID, csID)
	}
	if s.withrottle != nil {
		return s.withrottle.findCommandStation(ctx, layoutID, csID)
	}
	return domain.CommandStation{}, errors.New("remote service not configured")
}

func (s *Remote) resolveSessionProtocol(ctx context.Context, layoutID, csID, userID uint, clientKey string) (string, error) {
	if s == nil || s.pairing == nil {
		return "", errors.New("remote service not configured")
	}
	if clientKey != "" {
		active, ok, err := s.pairing.GetActiveByClientKey(ctx, layoutID, csID, clientKey)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", svcerrors.ErrRemoteSessionNotFound
		}
		return active.Protocol, nil
	}
	sessions, err := s.pairing.ListActiveByUser(ctx, layoutID, csID, userID)
	if err != nil {
		return "", err
	}
	if len(sessions) == 0 {
		return "", svcerrors.ErrRemoteSessionNotFound
	}
	return sessions[0].Protocol, nil
}

func mergeRemoteStatus(z21 Z21RemoteStatus, wt WithrottleRemoteStatus) RemoteStatus {
	out := RemoteStatus{
		AvailableProtocols: []RemoteProtocolInfo{
			{Protocol: contract.RemoteProtocolZ21, Enabled: z21.Z21ServerEnabled},
			{Protocol: contract.RemoteProtocolWithrottle, Enabled: wt.WithrottleServerEnabled},
		},
	}
	if wt.Paired {
		out.Protocol = contract.RemoteProtocolWithrottle
		out.Paired = true
		out.ClientKey = wt.ClientKey
		out.PairedAt = wt.PairedAt
		out.LastSeenAt = wt.LastSeenAt
		out.AllowAllVehicles = wt.AllowAllVehicles
		out.HandsetBrakeSecs = wt.HandsetBrakeSecs
		out.AllowedVehicles = append([]RemoteVehicleRef(nil), wt.AllowedVehicles...)
	} else if z21.Paired {
		out.Protocol = contract.RemoteProtocolZ21
		out.Paired = true
		out.ClientKey = z21.ClientKey
		out.PairedAt = z21.PairedAt
		out.LastSeenAt = z21.LastSeenAt
		out.AllowAllVehicles = z21.AllowAllVehicles
		out.HandsetBrakeSecs = z21.HandsetBrakeSecs
		for _, v := range z21.AllowedVehicles {
			out.AllowedVehicles = append(out.AllowedVehicles, RemoteVehicleRef{
				VehicleID: v.VehicleID,
				Addr:      v.Addr,
			})
		}
	}
	if wt.PendingReq {
		out.PendingReq = true
		out.Protocol = contract.RemoteProtocolWithrottle
		out.PairingCode = wt.PairingCode
		out.DisplayLabel = wt.DisplayLabel
		out.ExpiresAt = wt.ExpiresAt
		out.HandsetBrakeSecs = wt.HandsetBrakeSecs
	} else if z21.PendingReq {
		out.PendingReq = true
		out.Protocol = contract.RemoteProtocolZ21
		out.PairingCV3 = z21.PairingCV3
		out.PairingCV4 = z21.PairingCV4
		out.DisplayLabel = z21.DisplayLabel
		out.ExpiresAt = z21.ExpiresAt
		out.HandsetBrakeSecs = z21.HandsetBrakeSecs
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
