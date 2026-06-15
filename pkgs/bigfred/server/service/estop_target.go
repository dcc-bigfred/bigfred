package service

import (
	"context"
	"errors"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/helpers"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/security"
	"github.com/keskad/loco/pkgs/bigfred/server/ws"
)

// EStopTargetService orchestrates per-target emergency stop (§4.2).
type EStopTargetService struct {
	dccBus      *DccBusService
	roster      *LayoutVehicleService
	layouts     CommandStationIDsForLayout
	auth        *AuthService
	ilkSessions *repo.InterlockingSessions
	layoutIlks  *repo.LayoutInterlockings
	log         *logrus.Logger
	sec         security.EStopTargetSecurityContext
}

// EStopTargetConfig wires EStopTargetService dependencies.
type EStopTargetConfig struct {
	DccBus      *DccBusService
	Roster      *LayoutVehicleService
	Layouts     CommandStationIDsForLayout
	Auth        *AuthService
	IlkSessions *repo.InterlockingSessions
	LayoutIlks  *repo.LayoutInterlockings
	Log         *logrus.Logger
}

// NewEStopTargetService returns a ready orchestrator.
func NewEStopTargetService(cfg EStopTargetConfig) *EStopTargetService {
	log := cfg.Log
	if log == nil {
		log = logrus.New()
	}
	return &EStopTargetService{
		dccBus:      cfg.DccBus,
		roster:      cfg.Roster,
		layouts:     cfg.Layouts,
		auth:        cfg.Auth,
		ilkSessions: cfg.IlkSessions,
		layoutIlks:  cfg.LayoutIlks,
		log:         log,
	}
}

// Trigger brakes a single roster vehicle or train on every command
// station attached to the layout. Returns (true, "") on success.
func (s *EStopTargetService) Trigger(
	ctx context.Context,
	sess *ws.DriveSession,
	target domain.TakeoverTarget,
	targetID uint,
) (bool, string) {
	if s.dccBus == nil || s.roster == nil || s.layouts == nil {
		return false, "dcc_bus_not_configured"
	}

	resolved, err := s.resolveTarget(ctx, sess.LayoutID, target, targetID)
	if err != nil {
		return false, estopTargetDeniedCode(err)
	}

	eff, err := s.effectiveRoles(ctx, sess.UserID, sess.LayoutID)
	if err != nil {
		s.log.WithError(err).Warn("estop target: effective roles")
		return false, "not_authorized_to_stop"
	}
	occupant, err := s.isInterlockingOccupantOnLayout(ctx, sess.UserID, sess.LayoutID)
	if err != nil {
		s.log.WithError(err).Warn("estop target: occupant check")
		return false, "not_authorized_to_stop"
	}
	// Authorize before revealing whether the target is drivable: a
	// caller who may not stop the target gets a uniform denial rather
	// than a "dummy" hint.
	if d := s.sec.CanStop(eff, sess.UserID, occupant, resolved.ownerID, resolved.controllerUserIDs); !d.Allowed {
		return false, d.Reason
	}
	if len(resolved.addrs) == 0 {
		return false, "vehicle_is_dummy"
	}

	csIDs, err := s.layouts.CommandStationIDsForLayout(ctx, sess.LayoutID)
	if err != nil {
		s.log.WithError(err).Warn("estop target: list command stations")
		return false, "dcc_bus_unavailable"
	}
	if len(csIDs) == 0 {
		return false, "command_station_not_attached"
	}

	for _, csID := range csIDs {
		payload := contract.EStopTargetCommandWire{Addresses: resolved.addrs}
		if err := s.dccBus.PublishCommand(ctx, sess.LayoutID, csID, protocol.TypeSystemEStopTarget, payload); err != nil {
			s.log.WithError(err).WithFields(logrus.Fields{
				"layoutId":         sess.LayoutID,
				"commandStationId": csID,
				"addrs":            resolved.addrs,
			}).Warn("estop target: publish")
			return false, "dcc_bus_unavailable"
		}
	}

	s.log.WithFields(logrus.Fields{
		"layoutId":    sess.LayoutID,
		"triggeredBy": sess.Login,
		"userId":      sess.UserID,
		"target":      target,
		"targetId":    targetID,
		"addrs":       resolved.addrs,
	}).Info("estop target triggered")

	return true, ""
}

type estopTargetResolved struct {
	addrs             []uint16
	ownerID           uint
	controllerUserIDs []uint
}

func (s *EStopTargetService) resolveTarget(
	ctx context.Context,
	layoutID uint,
	target domain.TakeoverTarget,
	targetID uint,
) (estopTargetResolved, error) {
	switch target {
	case domain.TakeoverTargetVehicle:
		return s.resolveVehicle(ctx, layoutID, targetID)
	case domain.TakeoverTargetTrain:
		return s.resolveTrain(ctx, layoutID, targetID)
	default:
		return estopTargetResolved{}, ErrTakeoverInvalidState
	}
}

func (s *EStopTargetService) resolveVehicle(
	ctx context.Context,
	layoutID uint,
	vehicleID uint,
) (estopTargetResolved, error) {
	entries, err := s.roster.ListVehicles(ctx, layoutID)
	if err != nil {
		return estopTargetResolved{}, err
	}
	trains, err := s.roster.ListTrains(ctx, layoutID)
	if err != nil {
		return estopTargetResolved{}, err
	}
	var entry *RosterVehicleEntry
	for i := range entries {
		if entries[i].Vehicle.ID == vehicleID {
			entry = &entries[i]
			break
		}
	}
	if entry == nil {
		return estopTargetResolved{}, ErrTakeoverTargetNotOnLayout
	}
	if entry.Vehicle.DCCAddress == nil {
		return estopTargetResolved{ownerID: entry.Vehicle.OwnerUserID}, nil
	}
	lesseesByVehicle, err := s.roster.LesseesByVehicle(ctx, layoutID, entries, trains)
	if err != nil {
		return estopTargetResolved{}, err
	}
	addr := uint16(*entry.Vehicle.DCCAddress)
	controllers := helpers.MergeUserIDs(entry.Vehicle.OwnerUserID, lesseesByVehicle[vehicleID]...)
	return estopTargetResolved{
		addrs:             []uint16{addr},
		ownerID:           entry.Vehicle.OwnerUserID,
		controllerUserIDs: controllers,
	}, nil
}

func (s *EStopTargetService) resolveTrain(
	ctx context.Context,
	layoutID uint,
	trainID uint,
) (estopTargetResolved, error) {
	entries, err := s.roster.ListVehicles(ctx, layoutID)
	if err != nil {
		return estopTargetResolved{}, err
	}
	trains, err := s.roster.ListTrains(ctx, layoutID)
	if err != nil {
		return estopTargetResolved{}, err
	}
	var trainEntry *RosterTrainEntry
	for i := range trains {
		if trains[i].Train.ID == trainID {
			trainEntry = &trains[i]
			break
		}
	}
	if trainEntry == nil {
		return estopTargetResolved{}, ErrTakeoverTargetNotOnLayout
	}

	addrByVehicle := make(map[uint]uint16, len(entries))
	for _, e := range entries {
		if e.Vehicle.DCCAddress != nil {
			addrByVehicle[e.Vehicle.ID] = uint16(*e.Vehicle.DCCAddress)
		}
	}

	lesseesByVehicle, err := s.roster.LesseesByVehicle(ctx, layoutID, entries, trains)
	if err != nil {
		return estopTargetResolved{}, err
	}

	controllerSet := make(map[uint]struct{})
	controllerSet[trainEntry.Train.OwnerUserID] = struct{}{}
	addrs := make([]uint16, 0, len(trainEntry.Members))
	for _, m := range trainEntry.Members {
		if addr, ok := addrByVehicle[m.VehicleID]; ok {
			addrs = append(addrs, addr)
		}
		for _, lessee := range lesseesByVehicle[m.VehicleID] {
			controllerSet[lessee] = struct{}{}
		}
	}
	controllers := make([]uint, 0, len(controllerSet))
	for id := range controllerSet {
		controllers = append(controllers, id)
	}

	return estopTargetResolved{
		addrs:             addrs,
		ownerID:           trainEntry.Train.OwnerUserID,
		controllerUserIDs: controllers,
	}, nil
}

func (s *EStopTargetService) effectiveRoles(ctx context.Context, userID, layoutID uint) (domain.EffectiveRoles, error) {
	if s.auth == nil {
		return domain.NewEffectiveRoles(), nil
	}
	return s.auth.EffectiveForUserID(ctx, userID, layoutID)
}

func (s *EStopTargetService) isInterlockingOccupantOnLayout(
	ctx context.Context,
	userID, layoutID uint,
) (bool, error) {
	if s.ilkSessions == nil || s.layoutIlks == nil {
		return false, nil
	}
	sess, err := s.ilkSessions.FindActiveByUser(ctx, userID)
	if err != nil {
		if errors.Is(err, repo.ErrInterlockingSessionNotFound) {
			return false, nil
		}
		return false, err
	}
	ok, err := s.layoutIlks.Exists(ctx, layoutID, sess.InterlockingID)
	if err != nil {
		return false, err
	}
	return ok, nil
}

func estopTargetDeniedCode(err error) string {
	switch {
	case errors.Is(err, ErrTakeoverTargetNotOnLayout):
		return "vehicle_not_on_layout"
	case errors.Is(err, ErrTakeoverInvalidState):
		return "bad_payload"
	default:
		return "dcc_bus_unavailable"
	}
}
