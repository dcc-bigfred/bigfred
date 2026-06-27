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

// Z21Remote manages handset pairing for one layout command station.
type Z21Remote struct {
	handsetRemoteDeps
	pairing *remotepairing.Store
	users   *repo.Users
}

// NewZ21Remote returns a Z21Remote service.
func NewZ21Remote(
	pairing *remotepairing.Store,
	stations *repo.CommandStations,
	layoutCS *repo.LayoutCommandStations,
	roster *LayoutRoster,
	snapshot *LayoutRosterSnapshot,
	users *repo.Users,
) *Z21Remote {
	return &Z21Remote{
		pairing: pairing,
		users:   users,
		handsetRemoteDeps: handsetRemoteDeps{
			stations: stations,
			layoutCS: layoutCS,
			roster:   roster,
			snapshot: snapshot,
		},
	}
}

// Z21RemoteStatus is the domain view of handset pairing state.
type Z21RemoteStatus struct {
	Paired           bool
	ClientKey        string
	PairedAt         int64
	LastSeenAt       int64
	AllowAllVehicles bool
	AllowedVehicles  []Z21RemoteVehicleRef
	PendingReq       bool
	PairingCV3       int
	PairingCV4       int
	DisplayLabel     string
	ExpiresAt        int64
	HandsetBrakeSecs uint
	Z21ServerEnabled bool
}

// Z21RemoteVehicleRef is one vehicle in the paired scope.
type Z21RemoteVehicleRef struct {
	VehicleID string
	Addr      uint16
}

// Z21RemoteStartPairingInput carries vehicle scope for a new pairing code.
type Z21RemoteStartPairingInput struct {
	VehicleIDs       []string
	AllowAllVehicles bool
	HandsetBrakeSecs uint
}

// Z21RemoteUpdateSessionInput updates scope on an active session.
type Z21RemoteUpdateSessionInput struct {
	VehicleIDs       []string
	AllowAllVehicles *bool
	ClientKey        string
}

// GetStatus returns pairing state for the current user.
func (s *Z21Remote) GetStatus(ctx context.Context, layoutID, csID, userID uint) (Z21RemoteStatus, error) {
	cs, err := s.findCommandStation(ctx, layoutID, csID)
	if err != nil {
		return Z21RemoteStatus{}, err
	}
	out := Z21RemoteStatus{Z21ServerEnabled: cs.Z21ServerEnabled}
	if s.pairing == nil {
		return out, nil
	}
	if pending, ok, err := s.pairing.GetPendingByUser(ctx, layoutID, csID, userID); err != nil {
		return Z21RemoteStatus{}, err
	} else if ok && pending.Protocol == contract.RemoteProtocolZ21 {
		out.PendingReq = true
		out.PairingCV3 = pending.PairingCV3
		out.PairingCV4 = pending.PairingCV4
		out.DisplayLabel = pending.DisplayLabel
		out.ExpiresAt = remotepairing.PendingExpiresAt(pending).UnixMilli()
		out.HandsetBrakeSecs = contract.NormaliseHandsetBrakeSecs(pending.HandsetBrakeSecs)
	}
	sessions, err := s.pairing.ListActiveByUser(ctx, layoutID, csID, userID)
	if err != nil {
		return Z21RemoteStatus{}, err
	}
	if len(sessions) == 0 {
		return out, nil
	}
	for _, active := range sessions {
		if active.Protocol != contract.RemoteProtocolZ21 {
			continue
		}
		out.Paired = true
		out.ClientKey = active.ClientKey
		out.PairedAt = active.PairedAt
		out.LastSeenAt = active.LastSeenAt
		out.AllowAllVehicles = active.AllowAllVehicles
		out.HandsetBrakeSecs = contract.NormaliseHandsetBrakeSecs(active.HandsetBrakeSecs)
		out.AllowedVehicles = z21VehiclesFromSession(s, ctx, layoutID, active.VehicleIDs)
		break
	}
	return out, nil
}

// ListClients returns the live Z21 handset presence snapshot for one CS.
func (s *Z21Remote) ListClients(ctx context.Context, layoutID, csID uint) (contract.Z21ClientsSnapshotWire, error) {
	if _, err := s.ensureReady(ctx, layoutID, csID); err != nil {
		return contract.Z21ClientsSnapshotWire{}, err
	}
	if s.pairing == nil {
		return contract.Z21ClientsSnapshotWire{}, nil
	}
	snap, ok, err := s.pairing.GetClientsSnapshot(ctx, layoutID, csID)
	if err != nil {
		return contract.Z21ClientsSnapshotWire{}, err
	}
	if !ok {
		cs, _ := s.stations.FindByID(ctx, csID)
		return contract.Z21ClientsSnapshotWire{
			LayoutID:         layoutID,
			CommandStationID: csID,
			IPStickiness:     cs.Z21IPStickiness,
			Clients:          nil,
		}, nil
	}
	if s.users != nil {
		for i := range snap.Clients {
			if snap.Clients[i].UserID == 0 {
				continue
			}
			if u, err := s.users.FindByID(ctx, snap.Clients[i].UserID); err == nil {
				snap.Clients[i].UserLogin = u.Login
			}
		}
	}
	return snap, nil
}

// StartPairing creates a pending CV3/CV4 pair for the user.
func (s *Z21Remote) StartPairing(ctx context.Context, layoutID, csID, userID uint, in Z21RemoteStartPairingInput) (contract.Z21PairingReqWire, error) {
	if _, err := s.ensureReady(ctx, layoutID, csID); err != nil {
		return contract.Z21PairingReqWire{}, err
	}
	if s.pairing == nil {
		return contract.Z21PairingReqWire{}, errors.New("z21 pairing store not configured")
	}
	vehicleIDs, addrs, err := s.resolvePairingScope(ctx, layoutID, userID, handsetPairingScopeInput{
		VehicleIDs:       in.VehicleIDs,
		AllowAllVehicles: in.AllowAllVehicles,
	})
	if err != nil {
		return contract.Z21PairingReqWire{}, err
	}
	brakeSecs := in.HandsetBrakeSecs
	if brakeSecs == 0 {
		brakeSecs = contract.Z21HandsetBrakeSecsDefault
	} else if !contract.ValidHandsetBrakeSecs(brakeSecs) {
		return contract.Z21PairingReqWire{}, svcerrors.ErrZ21HandsetBrakeSecsInvalid
	}
	req, err := s.pairing.CreateZ21PairingRequest(ctx, remotepairing.CreateZ21PairingInput{
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

// CancelPairing deletes the current user's pending pairing code.
func (s *Z21Remote) CancelPairing(ctx context.Context, layoutID, csID, userID uint) error {
	if _, err := s.findCommandStation(ctx, layoutID, csID); err != nil {
		return err
	}
	if s.pairing == nil {
		return errors.New("z21 pairing store not configured")
	}
	return s.pairing.CancelPendingPairing(ctx, layoutID, csID, userID)
}

// UpdateSession changes vehicle scope without re-pairing.
func (s *Z21Remote) UpdateSession(ctx context.Context, layoutID, csID, userID uint, in Z21RemoteUpdateSessionInput) error {
	if _, err := s.findCommandStation(ctx, layoutID, csID); err != nil {
		return err
	}
	if s.pairing == nil {
		return errors.New("z21 pairing store not configured")
	}
	clientKey := in.ClientKey
	if clientKey == "" {
		sessions, err := s.pairing.ListActiveByUser(ctx, layoutID, csID, userID)
		if err != nil {
			return err
		}
		if len(sessions) == 0 {
			return svcerrors.ErrZ21SessionNotFound
		}
		clientKey = sessions[0].ClientKey
	}
	active, ok, err := s.pairing.GetActiveByClientKey(ctx, layoutID, csID, clientKey)
	if err != nil {
		return err
	}
	if !ok || active.UserID != userID {
		return svcerrors.ErrZ21SessionNotFound
	}
	allowAll := active.AllowAllVehicles
	if in.AllowAllVehicles != nil {
		allowAll = *in.AllowAllVehicles
	}
	vehicleIDs, addrs, err := s.resolvePairingScope(ctx, layoutID, userID, handsetPairingScopeInput{
		VehicleIDs:       in.VehicleIDs,
		AllowAllVehicles: allowAll,
	})
	if err != nil {
		return err
	}
	_, ok, err = s.pairing.UpdateSessionScope(ctx, layoutID, csID, clientKey, vehicleIDs, addrs, allowAll)
	if err != nil {
		return err
	}
	if !ok {
		return svcerrors.ErrZ21SessionNotFound
	}
	// Notify dcc-bus so the daemon's in-process session mirror picks up
	// the new scope without a per-packet Redis read.
	_ = s.pairing.PublishSessionSync(ctx, layoutID, csID, clientKey, contract.RemoteSessionSyncScope)
	return nil
}

// Unpair removes the user's active handset session.
func (s *Z21Remote) Unpair(ctx context.Context, layoutID, csID, userID uint, clientKey string) error {
	if _, err := s.findCommandStation(ctx, layoutID, csID); err != nil {
		return err
	}
	if s.pairing == nil {
		return errors.New("z21 pairing store not configured")
	}
	if clientKey != "" {
		active, ok, err := s.pairing.GetActiveByClientKey(ctx, layoutID, csID, clientKey)
		if err != nil {
			return err
		}
		if !ok || active.UserID != userID {
			return svcerrors.ErrZ21SessionNotFound
		}
		if err := s.pairing.Unpair(ctx, layoutID, csID, clientKey); err != nil {
			return err
		}
		_ = s.pairing.PublishSessionSync(ctx, layoutID, csID, clientKey, contract.RemoteSessionSyncUnpair)
		return nil
	}
	// Empty clientKey: resolve the user's active session so the sync event
	// targets the right handset, then unpair explicitly.
	sessions, err := s.pairing.ListActiveByUser(ctx, layoutID, csID, userID)
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		return nil
	}
	resolved := sessions[0].ClientKey
	if err := s.pairing.Unpair(ctx, layoutID, csID, resolved); err != nil {
		return err
	}
	_ = s.pairing.PublishSessionSync(ctx, layoutID, csID, resolved, contract.RemoteSessionSyncUnpair)
	return nil
}

func (s *Z21Remote) ensureReady(ctx context.Context, layoutID, csID uint) (domain.CommandStation, error) {
	cs, err := s.findCommandStation(ctx, layoutID, csID)
	if err != nil {
		return domain.CommandStation{}, err
	}
	if !cs.Z21ServerEnabled {
		return domain.CommandStation{}, svcerrors.ErrZ21ServerDisabled
	}
	return cs, nil
}

func z21VehiclesFromSession(s *Z21Remote, ctx context.Context, layoutID uint, vehicleIDs []string) []Z21RemoteVehicleRef {
	refs := s.vehiclesFromSession(ctx, layoutID, vehicleIDs)
	out := make([]Z21RemoteVehicleRef, 0, len(refs))
	for _, r := range refs {
		out = append(out, Z21RemoteVehicleRef{VehicleID: r.VehicleID, Addr: r.Addr})
	}
	return out
}
