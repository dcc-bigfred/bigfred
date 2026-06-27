package cmd

import (
	"context"
	"errors"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/security"
)

type handsetRemoteDeps struct {
	stations *repo.CommandStations
	layoutCS *repo.LayoutCommandStations
	roster   *LayoutRoster
	snapshot *LayoutRosterSnapshot
}

func (d handsetRemoteDeps) findCommandStation(ctx context.Context, layoutID, csID uint) (domain.CommandStation, error) {
	if d.stations == nil || d.layoutCS == nil {
		return domain.CommandStation{}, errors.New("handset remote not configured")
	}
	if _, err := d.layoutCS.Find(ctx, layoutID, csID); err != nil {
		if errors.Is(err, repo.ErrLayoutCommandStationNotFound) {
			return domain.CommandStation{}, svcerrors.ErrZ21CommandStationNotOnLayout
		}
		return domain.CommandStation{}, err
	}
	cs, err := d.stations.FindByID(ctx, csID)
	if err != nil {
		if errors.Is(err, repo.ErrCommandStationNotFound) {
			return domain.CommandStation{}, svcerrors.ErrCommandStationNotFound
		}
		return domain.CommandStation{}, err
	}
	return cs, nil
}

type handsetPairingScopeInput struct {
	VehicleIDs       []string
	AllowAllVehicles bool
}

func (d handsetRemoteDeps) resolvePairingScope(ctx context.Context, layoutID, userID uint, in handsetPairingScopeInput) ([]string, []uint16, error) {
	if in.AllowAllVehicles {
		if len(in.VehicleIDs) > 0 {
			return nil, nil, svcerrors.ErrZ21PairingScopeInvalid
		}
		return nil, nil, nil
	}
	if len(in.VehicleIDs) == 0 {
		return nil, nil, svcerrors.ErrZ21PairingScopeInvalid
	}
	if d.roster == nil || d.snapshot == nil {
		return nil, nil, errors.New("layout roster not configured")
	}
	rows, err := d.roster.ListVehicles(ctx, layoutID)
	if err != nil {
		return nil, nil, err
	}
	trains, err := d.roster.ListTrains(ctx, layoutID)
	if err != nil {
		return nil, nil, err
	}
	lessees, err := d.snapshot.LesseesByVehicle(ctx, rows, trains)
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
		driveSec := security.DriveSecurityContext{}
		if !driveSec.CanDrive(domain.User{ID: userID}, entry.Vehicle.OwnerUserID, domain.VehicleLesseeUserIDs(lessees[entry.Vehicle.ID])).Allowed {
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

func (d handsetRemoteDeps) vehiclesFromSession(ctx context.Context, layoutID uint, vehicleIDs []string) []RemoteVehicleRef {
	if len(vehicleIDs) == 0 || d.roster == nil {
		return nil
	}
	rows, err := d.roster.ListVehicles(ctx, layoutID)
	if err != nil {
		return nil
	}
	byID := make(map[string]RosterVehicleEntry, len(rows))
	for _, e := range rows {
		byID[string(e.Vehicle.ID)] = e
	}
	out := make([]RemoteVehicleRef, 0, len(vehicleIDs))
	for _, id := range vehicleIDs {
		e, ok := byID[id]
		if !ok || e.Vehicle.DCCAddress == nil {
			continue
		}
		out = append(out, RemoteVehicleRef{VehicleID: id, Addr: *e.Vehicle.DCCAddress})
	}
	return out
}
