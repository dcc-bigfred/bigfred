package cmd

import (
	"context"
	"errors"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/security"
	"github.com/keskad/loco/pkgs/bigfred/z21pairing"
)

// Z21Remote manages handset pairing for one layout command station.
type Z21Remote struct {
	pairing  *z21pairing.Store
	stations *repo.CommandStations
	layoutCS *repo.LayoutCommandStations
	roster   *LayoutRoster
	snapshot *LayoutRosterSnapshot
	driveSec security.DriveSecurityContext
}

// NewZ21Remote returns a Z21Remote service.
func NewZ21Remote(
	pairing *z21pairing.Store,
	stations *repo.CommandStations,
	layoutCS *repo.LayoutCommandStations,
	roster *LayoutRoster,
	snapshot *LayoutRosterSnapshot,
) *Z21Remote {
	return &Z21Remote{
		pairing:  pairing,
		stations: stations,
		layoutCS: layoutCS,
		roster:   roster,
		snapshot: snapshot,
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
}

// Z21RemoteUpdateSessionInput updates scope on an active session.
type Z21RemoteUpdateSessionInput struct {
	VehicleIDs       []string
	AllowAllVehicles *bool
	ClientKey        string
}

// GetStatus returns pairing state for the current user.
func (s *Z21Remote) GetStatus(ctx context.Context, layoutID, csID, userID uint) (Z21RemoteStatus, error) {
	cs, err := s.ensureReady(ctx, layoutID, csID)
	if err != nil {
		return Z21RemoteStatus{}, err
	}
	out := Z21RemoteStatus{Z21ServerEnabled: cs.Z21ServerEnabled}
	if s.pairing == nil {
		return out, nil
	}
	if pending, ok, err := s.pairing.GetPendingByUser(ctx, layoutID, csID, userID); err != nil {
		return Z21RemoteStatus{}, err
	} else if ok {
		out.PendingReq = true
		out.PairingCV3 = pending.PairingCV3
		out.PairingCV4 = pending.PairingCV4
		out.DisplayLabel = pending.DisplayLabel
		out.ExpiresAt = z21pairing.PendingExpiresAt(pending).UnixMilli()
	}
	sessions, err := s.pairing.ListActiveByUser(ctx, layoutID, csID, userID)
	if err != nil {
		return Z21RemoteStatus{}, err
	}
	if len(sessions) == 0 {
		return out, nil
	}
	active := sessions[0]
	out.Paired = true
	out.ClientKey = active.ClientKey
	out.PairedAt = active.PairedAt
	out.LastSeenAt = active.LastSeenAt
	out.AllowAllVehicles = active.AllowAllVehicles
	out.AllowedVehicles = s.vehiclesFromSession(ctx, layoutID, active.VehicleIDs)
	return out, nil
}

// StartPairing creates a pending CV3/CV4 pair for the user.
func (s *Z21Remote) StartPairing(ctx context.Context, layoutID, csID, userID uint, in Z21RemoteStartPairingInput) (contract.Z21PairingReqWire, error) {
	if _, err := s.ensureReady(ctx, layoutID, csID); err != nil {
		return contract.Z21PairingReqWire{}, err
	}
	if s.pairing == nil {
		return contract.Z21PairingReqWire{}, errors.New("z21 pairing store not configured")
	}
	vehicleIDs, addrs, err := s.resolvePairingScope(ctx, layoutID, userID, in)
	if err != nil {
		return contract.Z21PairingReqWire{}, err
	}
	return s.pairing.CreatePairingRequest(ctx, z21pairing.CreatePairingRequestInput{
		LayoutID:         layoutID,
		CommandStationID: csID,
		UserID:           userID,
		VehicleIDs:       vehicleIDs,
		AllowedAddrs:     addrs,
		AllowAllVehicles: in.AllowAllVehicles,
	})
}

// UpdateSession changes vehicle scope without re-pairing.
func (s *Z21Remote) UpdateSession(ctx context.Context, layoutID, csID, userID uint, in Z21RemoteUpdateSessionInput) error {
	if _, err := s.ensureReady(ctx, layoutID, csID); err != nil {
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
	vehicleIDs, addrs, err := s.resolvePairingScope(ctx, layoutID, userID, Z21RemoteStartPairingInput{
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
	return nil
}

// Unpair removes the user's active handset session.
func (s *Z21Remote) Unpair(ctx context.Context, layoutID, csID, userID uint, clientKey string) error {
	if _, err := s.ensureReady(ctx, layoutID, csID); err != nil {
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
		return s.pairing.Unpair(ctx, layoutID, csID, clientKey)
	}
	return s.pairing.UnpairAllForUser(ctx, layoutID, csID, userID)
}

func (s *Z21Remote) ensureReady(ctx context.Context, layoutID, csID uint) (domain.CommandStation, error) {
	if s.stations == nil || s.layoutCS == nil {
		return domain.CommandStation{}, errors.New("z21 remote not configured")
	}
	if _, err := s.layoutCS.Find(ctx, layoutID, csID); err != nil {
		if errors.Is(err, repo.ErrLayoutCommandStationNotFound) {
			return domain.CommandStation{}, svcerrors.ErrZ21CommandStationNotOnLayout
		}
		return domain.CommandStation{}, err
	}
	cs, err := s.stations.FindByID(ctx, csID)
	if err != nil {
		if errors.Is(err, repo.ErrCommandStationNotFound) {
			return domain.CommandStation{}, svcerrors.ErrCommandStationNotFound
		}
		return domain.CommandStation{}, err
	}
	if !cs.Z21ServerEnabled {
		return domain.CommandStation{}, svcerrors.ErrZ21ServerDisabled
	}
	return cs, nil
}

func (s *Z21Remote) resolvePairingScope(ctx context.Context, layoutID, userID uint, in Z21RemoteStartPairingInput) ([]string, []uint16, error) {
	if in.AllowAllVehicles {
		if len(in.VehicleIDs) > 0 {
			return nil, nil, svcerrors.ErrZ21PairingScopeInvalid
		}
		return nil, nil, nil
	}
	if len(in.VehicleIDs) == 0 {
		return nil, nil, svcerrors.ErrZ21PairingScopeInvalid
	}
	if s.roster == nil || s.snapshot == nil {
		return nil, nil, errors.New("layout roster not configured")
	}
	rows, err := s.roster.ListVehicles(ctx, layoutID)
	if err != nil {
		return nil, nil, err
	}
	trains, err := s.roster.ListTrains(ctx, layoutID)
	if err != nil {
		return nil, nil, err
	}
	lessees, err := s.snapshot.LesseesByVehicle(ctx, rows, trains)
	if err != nil {
		return nil, nil, err
	}
	onRoster := make(map[string]struct{}, len(rows))
	byID := make(map[string]RosterVehicleEntry, len(rows))
	for _, e := range rows {
		id := string(e.Vehicle.ID)
		onRoster[id] = struct{}{}
		byID[id] = e
	}
	vehicleIDs := make([]string, 0, len(in.VehicleIDs))
	addrs := make([]uint16, 0, len(in.VehicleIDs))
	seenAddr := make(map[uint16]struct{}, len(in.VehicleIDs))
	for _, id := range in.VehicleIDs {
		if _, ok := onRoster[id]; !ok {
			return nil, nil, svcerrors.ErrZ21VehicleNotOnRoster
		}
		entry := byID[id]
		if !s.driveSec.CanDrive(domain.User{ID: userID}, entry.Vehicle.OwnerUserID, domain.VehicleLesseeUserIDs(lessees[entry.Vehicle.ID])).Allowed {
			return nil, nil, svcerrors.ErrZ21VehicleNotDrivable
		}
		if entry.Vehicle.DCCAddress == nil {
			return nil, nil, svcerrors.ErrZ21VehicleNoDCCAddress
		}
		addr := *entry.Vehicle.DCCAddress
		vehicleIDs = append(vehicleIDs, id)
		if _, dup := seenAddr[addr]; !dup {
			seenAddr[addr] = struct{}{}
			addrs = append(addrs, addr)
		}
	}
	return vehicleIDs, addrs, nil
}

func (s *Z21Remote) vehiclesFromSession(ctx context.Context, layoutID uint, vehicleIDs []string) []Z21RemoteVehicleRef {
	if len(vehicleIDs) == 0 || s.roster == nil {
		return nil
	}
	rows, err := s.roster.ListVehicles(ctx, layoutID)
	if err != nil {
		return nil
	}
	byID := make(map[string]RosterVehicleEntry, len(rows))
	for _, e := range rows {
		byID[string(e.Vehicle.ID)] = e
	}
	out := make([]Z21RemoteVehicleRef, 0, len(vehicleIDs))
	for _, id := range vehicleIDs {
		e, ok := byID[id]
		if !ok || e.Vehicle.DCCAddress == nil {
			continue
		}
		out = append(out, Z21RemoteVehicleRef{VehicleID: id, Addr: *e.Vehicle.DCCAddress})
	}
	return out
}
