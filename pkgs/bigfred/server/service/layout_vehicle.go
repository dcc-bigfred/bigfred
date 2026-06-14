package service

import (
	"context"
	"errors"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/security"
	"github.com/keskad/loco/pkgs/bigfred/server/ws"
)

// Layout-roster sentinel errors.
var (
	ErrLayoutVehicleAlreadyOnRoster = errors.New("layout_vehicle_already_on_roster")
	ErrLayoutVehicleNotOnRoster     = errors.New("layout_vehicle_not_on_roster")
	ErrLayoutTrainAlreadyOnRoster   = errors.New("layout_train_already_on_roster")
	ErrLayoutTrainNotOnRoster       = errors.New("layout_train_not_on_roster")
)

// RosterVehicleEntry is one dashboard row of the layout vehicle
// roster: the catalogue Vehicle enriched with owner login and
// roster metadata so the table can render "added by X at T".
type RosterVehicleEntry struct {
	Vehicle    domain.Vehicle
	OwnerLogin string
	AddedAt    time.Time
}

// RosterTrainEntry is the train-shaped sibling of RosterVehicleEntry.
// Members carries the ordered TrainMember rows so the dashboard can
// display "ET22-1175 + 4 wagons" without a second round trip.
type RosterTrainEntry struct {
	Train      domain.Train
	OwnerLogin string
	AddedAt    time.Time
	Members    []domain.TrainMember
}

// LayoutVehicleService implements the per-layout roster CRUD (§3a.1,
// §6.3c). Two responsibilities:
//
//  1. Mutate the roster (add / remove vehicles and trains) with
//     ownership and uniqueness checks.
//  2. Enumerate the roster with denormalised data the dashboard needs.
//
// The service also fans out `layout.vehiclesChanged` and
// `layout.trainsChanged` WS events after every mutation so the
// dashboards stay live without polling (§4.2).
// layoutRosterRedisSync publishes full roster snapshots for dcc-bus.
type layoutRosterRedisSync interface {
	layoutRosterPublisher
}

type LayoutVehicleService struct {
	layoutVehicles *repo.LayoutVehicles
	layoutTrains   *repo.LayoutTrains
	vehicles       *repo.Vehicles
	trains         *repo.Trains
	members        *repo.TrainMembers
	users          *repo.Users
	hub            *ws.Hub
	redis          layoutRosterRedisSync
	sec            security.LayoutSecurityContext
}

// NewLayoutVehicleService constructs a LayoutVehicleService.
func NewLayoutVehicleService(
	layoutVehicles *repo.LayoutVehicles,
	layoutTrains *repo.LayoutTrains,
	vehicles *repo.Vehicles,
	trains *repo.Trains,
	members *repo.TrainMembers,
	users *repo.Users,
	hub *ws.Hub,
) *LayoutVehicleService {
	return &LayoutVehicleService{
		layoutVehicles: layoutVehicles,
		layoutTrains:   layoutTrains,
		vehicles:       vehicles,
		trains:         trains,
		members:        members,
		users:          users,
		hub:            hub,
		sec:            security.LayoutSecurityContext{},
	}
}

// SetRedisRosterPublisher wires Redis so roster mutations reach
// running dcc-bus daemons. Call once during server bootstrap.
func (s *LayoutVehicleService) SetRedisRosterPublisher(r layoutRosterRedisSync) {
	s.redis = r
}

// ListVehicles returns the layout vehicle roster ordered by AddedAt.
func (s *LayoutVehicleService) ListVehicles(ctx context.Context, layoutID uint) ([]RosterVehicleEntry, error) {
	rows, err := s.layoutVehicles.ListByLayout(ctx, layoutID)
	if err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.VehicleID)
	}
	vehicles, err := s.vehicles.ListByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[uint]domain.Vehicle, len(vehicles))
	for _, v := range vehicles {
		byID[v.ID] = v
	}

	out := make([]RosterVehicleEntry, 0, len(rows))
	for _, r := range rows {
		v, ok := byID[r.VehicleID]
		if !ok {
			continue
		}
		owner, err := s.users.FindByID(ctx, v.OwnerUserID)
		ownerLogin := ""
		if err == nil {
			ownerLogin = owner.Login
		}
		out = append(out, RosterVehicleEntry{
			Vehicle:    v,
			OwnerLogin: ownerLogin,
			AddedAt:    r.AddedAt,
		})
	}
	return out, nil
}

// ListTrains returns the layout train roster with members hydrated.
func (s *LayoutVehicleService) ListTrains(ctx context.Context, layoutID uint) ([]RosterTrainEntry, error) {
	rows, err := s.layoutTrains.ListByLayout(ctx, layoutID)
	if err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.TrainID)
	}
	trains, err := s.trains.ListByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[uint]domain.Train, len(trains))
	for _, t := range trains {
		byID[t.ID] = t
	}

	out := make([]RosterTrainEntry, 0, len(rows))
	for _, r := range rows {
		t, ok := byID[r.TrainID]
		if !ok {
			continue
		}
		owner, err := s.users.FindByID(ctx, t.OwnerUserID)
		ownerLogin := ""
		if err == nil {
			ownerLogin = owner.Login
		}
		members, err := s.members.ListByTrain(ctx, t.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, RosterTrainEntry{
			Train:      t,
			OwnerLogin: ownerLogin,
			AddedAt:    r.AddedAt,
			Members:    members,
		})
	}
	return out, nil
}

// AddVehicle attaches a vehicle to the layout roster. Only the
// owner of the vehicle may call this; the HTTP layer is responsible
// for matching the session's layoutId against the path.
func (s *LayoutVehicleService) AddVehicle(ctx context.Context, layoutID, actorID, vehicleID uint) (RosterVehicleEntry, error) {
	v, err := s.vehicles.FindByID(ctx, vehicleID)
	if err != nil {
		if errors.Is(err, repo.ErrVehicleNotFound) {
			return RosterVehicleEntry{}, ErrVehicleNotFound
		}
		return RosterVehicleEntry{}, err
	}
	if v.OwnerUserID != actorID {
		return RosterVehicleEntry{}, ErrVehicleNotOwned
	}
	if _, err := s.layoutVehicles.FindByLayoutAndVehicle(ctx, layoutID, vehicleID); err == nil {
		return RosterVehicleEntry{}, ErrLayoutVehicleAlreadyOnRoster
	} else if !errors.Is(err, repo.ErrLayoutVehicleNotFound) {
		return RosterVehicleEntry{}, err
	}

	now := time.Now().UTC()
	row := domain.LayoutVehicle{
		LayoutID:      layoutID,
		VehicleID:     v.ID,
		AddedByUserID: actorID,
		AddedAt:       now,
	}
	if err := s.layoutVehicles.Insert(ctx, &row); err != nil {
		return RosterVehicleEntry{}, err
	}

	owner, err := s.users.FindByID(ctx, v.OwnerUserID)
	ownerLogin := ""
	if err == nil {
		ownerLogin = owner.Login
	}
	entry := RosterVehicleEntry{Vehicle: v, OwnerLogin: ownerLogin, AddedAt: row.AddedAt}
	s.broadcastVehicleChanged(layoutID, v.ID, "added")
	return entry, nil
}

// RemoveVehicle detaches a vehicle from the layout roster. Authority
// is decided by LayoutSecurityContext.CanRemoveVehicleFromRoster
// (§7a.3).
func (s *LayoutVehicleService) RemoveVehicle(ctx context.Context, layoutID, actorID, vehicleID uint, eff domain.EffectiveRoles) error {
	row, err := s.layoutVehicles.FindByLayoutAndVehicle(ctx, layoutID, vehicleID)
	if err != nil {
		if errors.Is(err, repo.ErrLayoutVehicleNotFound) {
			return ErrLayoutVehicleNotOnRoster
		}
		return err
	}
	v, err := s.vehicles.FindByID(ctx, vehicleID)
	if err != nil {
		if errors.Is(err, repo.ErrVehicleNotFound) {
			return ErrVehicleNotFound
		}
		return err
	}
	decision := s.sec.CanRemoveVehicleFromRoster(eff, actorID, v.OwnerUserID)
	if !decision.Allowed {
		switch decision.Reason {
		case "vehicle_not_owned":
			return ErrVehicleNotOwned
		default:
			return errors.New(decision.Reason)
		}
	}
	if err := s.layoutVehicles.Delete(ctx, &row); err != nil {
		return err
	}
	s.broadcastVehicleChanged(layoutID, vehicleID, "removed")
	return nil
}

// AddTrain attaches a train to the layout roster (owner-only).
func (s *LayoutVehicleService) AddTrain(ctx context.Context, layoutID, actorID, trainID uint) (RosterTrainEntry, error) {
	t, err := s.trains.FindByID(ctx, trainID)
	if err != nil {
		if errors.Is(err, repo.ErrTrainNotFound) {
			return RosterTrainEntry{}, ErrTrainNotFound
		}
		return RosterTrainEntry{}, err
	}
	if t.OwnerUserID != actorID {
		return RosterTrainEntry{}, ErrTrainNotOwned
	}
	if _, err := s.layoutTrains.FindByLayoutAndTrain(ctx, layoutID, trainID); err == nil {
		return RosterTrainEntry{}, ErrLayoutTrainAlreadyOnRoster
	} else if !errors.Is(err, repo.ErrLayoutTrainNotFound) {
		return RosterTrainEntry{}, err
	}

	now := time.Now().UTC()
	row := domain.LayoutTrain{
		LayoutID:      layoutID,
		TrainID:       t.ID,
		AddedByUserID: actorID,
		AddedAt:       now,
	}
	if err := s.layoutTrains.Insert(ctx, &row); err != nil {
		return RosterTrainEntry{}, err
	}

	owner, err := s.users.FindByID(ctx, t.OwnerUserID)
	ownerLogin := ""
	if err == nil {
		ownerLogin = owner.Login
	}
	members, err := s.members.ListByTrain(ctx, t.ID)
	if err != nil {
		return RosterTrainEntry{}, err
	}
	entry := RosterTrainEntry{Train: t, OwnerLogin: ownerLogin, AddedAt: row.AddedAt, Members: members}
	s.broadcastTrainChanged(layoutID, t.ID, "added")
	return entry, nil
}

// RemoveTrain detaches a train from the layout roster. Authority is
// decided by LayoutSecurityContext.CanRemoveTrainFromRoster (§7a.3).
func (s *LayoutVehicleService) RemoveTrain(ctx context.Context, layoutID, actorID, trainID uint, eff domain.EffectiveRoles) error {
	row, err := s.layoutTrains.FindByLayoutAndTrain(ctx, layoutID, trainID)
	if err != nil {
		if errors.Is(err, repo.ErrLayoutTrainNotFound) {
			return ErrLayoutTrainNotOnRoster
		}
		return err
	}
	t, err := s.trains.FindByID(ctx, trainID)
	if err != nil {
		if errors.Is(err, repo.ErrTrainNotFound) {
			return ErrTrainNotFound
		}
		return err
	}
	decision := s.sec.CanRemoveTrainFromRoster(eff, actorID, t.OwnerUserID)
	if !decision.Allowed {
		switch decision.Reason {
		case "train_not_owned":
			return ErrTrainNotOwned
		default:
			return errors.New(decision.Reason)
		}
	}
	if err := s.layoutTrains.Delete(ctx, &row); err != nil {
		return err
	}
	s.broadcastTrainChanged(layoutID, trainID, "removed")
	return nil
}

// PurgeVehicle deletes every roster row pointing at a vehicle and
// fans out `layout.vehiclesChanged action="removed"` to each affected
// layout so the dashboards stay live. Called by the HTTP wiring
// after a successful VehicleService.Delete.
func (s *LayoutVehicleService) PurgeVehicle(ctx context.Context, vehicleID uint) error {
	rows, err := s.layoutVehicles.ListByVehicle(ctx, vehicleID)
	if err != nil {
		return err
	}
	if err := s.layoutVehicles.DeleteAllForVehicle(ctx, vehicleID); err != nil {
		return err
	}
	for _, r := range rows {
		s.broadcastVehicleChanged(r.LayoutID, vehicleID, "removed")
	}
	return nil
}

// PurgeTrain is the train-shaped sibling of PurgeVehicle. Fans out
// `layout.trainsChanged action="removed"` to every layout that had
// the train on its roster.
func (s *LayoutVehicleService) PurgeTrain(ctx context.Context, trainID uint) error {
	rows, err := s.layoutTrains.ListByTrain(ctx, trainID)
	if err != nil {
		return err
	}
	if err := s.layoutTrains.DeleteAllForTrain(ctx, trainID); err != nil {
		return err
	}
	for _, r := range rows {
		s.broadcastTrainChanged(r.LayoutID, trainID, "removed")
	}
	return nil
}

// BroadcastVehicleUpdated emits `layout.vehiclesChanged action="updated"`
// to every layout that has the vehicle on its roster. Used after a
// VehicleService.Update so the dashboard picks up renames / kind
// changes / DCC swaps without polling.
func (s *LayoutVehicleService) BroadcastVehicleUpdated(ctx context.Context, vehicleID uint) error {
	rows, err := s.layoutVehicles.ListByVehicle(ctx, vehicleID)
	if err != nil {
		return err
	}
	for _, r := range rows {
		s.broadcastVehicleChanged(r.LayoutID, vehicleID, "updated")
	}
	if err := s.syncRosterForVehicleInTrains(ctx, vehicleID); err != nil {
		return err
	}
	return nil
}

// BroadcastTrainUpdated notifies dashboards and republishes Redis
// train snapshots on every layout roster that references trainID.
func (s *LayoutVehicleService) BroadcastTrainUpdated(ctx context.Context, trainID uint) error {
	rows, err := s.layoutTrains.ListByTrain(ctx, trainID)
	if err != nil {
		return err
	}
	for _, r := range rows {
		s.hub.BroadcastToLayout(r.LayoutID, "layout.trainsChanged", TrainChangedPayload{
			LayoutID: r.LayoutID,
			Action:   "updated",
			TrainID:  trainID,
		})
	}
	return s.SyncLayoutRosterForTrain(ctx, trainID)
}

// VehicleChangedPayload is the JSON body of `layout.vehiclesChanged`.
type VehicleChangedPayload struct {
	LayoutID  uint   `json:"layoutId"`
	Action    string `json:"action"` // "added" | "removed"
	VehicleID uint   `json:"vehicleId"`
}

// TrainChangedPayload is the JSON body of `layout.trainsChanged`.
type TrainChangedPayload struct {
	LayoutID uint   `json:"layoutId"`
	Action   string `json:"action"` // "added" | "removed"
	TrainID  uint   `json:"trainId"`
}

func (s *LayoutVehicleService) broadcastVehicleChanged(layoutID, vehicleID uint, action string) {
	s.hub.BroadcastToLayout(layoutID, "layout.vehiclesChanged", VehicleChangedPayload{
		LayoutID:  layoutID,
		Action:    action,
		VehicleID: vehicleID,
	})
	s.syncRosterToRedis(layoutID)
}

func (s *LayoutVehicleService) syncRosterToRedis(layoutID uint) {
	if s.redis == nil || layoutID == 0 {
		return
	}
	_ = s.SyncLayoutRosterToRedis(context.Background(), layoutID)
}

func (s *LayoutVehicleService) broadcastTrainChanged(layoutID, trainID uint, action string) {
	s.hub.BroadcastToLayout(layoutID, "layout.trainsChanged", TrainChangedPayload{
		LayoutID: layoutID,
		Action:   action,
		TrainID:  trainID,
	})
	s.syncRosterToRedis(layoutID)
}
