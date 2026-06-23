package cmd

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
)

const (
	LeaseMinDuration = time.Minute
	LeaseMaxDuration = 24 * time.Hour
)

var (
	ErrLeaseNotFound          = svcerrors.ErrLeaseNotFound
	ErrLeaseConflict          = svcerrors.ErrLeaseConflict
	ErrLeaseNotOwner          = svcerrors.ErrLeaseNotOwner
	ErrLeaseNotParty          = svcerrors.ErrLeaseNotParty
	ErrLeaseSelf              = svcerrors.ErrLeaseSelf
	ErrLeaseTargetNotOnLayout = svcerrors.ErrLeaseTargetNotOnLayout
	ErrLeaseInvalidSpeedLimit = svcerrors.ErrLeaseInvalidSpeedLimit
	ErrLeaseInvalidDuration   = svcerrors.ErrLeaseInvalidDuration
	ErrLeaseTargetNotDrivable = svcerrors.ErrLeaseTargetNotDrivable
	ErrLeaseStoreUnavailable  = svcerrors.ErrLeaseStoreUnavailable
)

// LeaseEntry is one active lease row enriched for REST responses.
type LeaseEntry struct {
	Kind       domain.TakeoverTarget
	TargetID   string
	TargetName string
	FromUserID uint
	FromLogin  string
	ToUserID   uint
	ToLogin    string
	ExpiresAt  time.Time
	SpeedLimit uint8
}

// LendableTarget is a vehicle or train the owner may lease out.
type LendableTarget struct {
	Kind       domain.TakeoverTarget
	TargetID   string
	TargetName string
}

// LendableUser is a system account eligible as a lessee.
type LendableUser struct {
	UserID       uint
	Login        string
	Organization string
}

// LendableCatalogue lists lendable targets and candidate users.
type LendableCatalogue struct {
	Targets []LendableTarget
	Users   []LendableUser
}

type LeaseHubPort interface {
	BroadcastLease(layoutID, userID uint, typ string, payload contract.LeaseEventWire)
}

type LeaseRosterPort interface {
	ListVehicles(ctx context.Context, layoutID uint) ([]RosterVehicleEntry, error)
	ListTrains(ctx context.Context, layoutID uint) ([]RosterTrainEntry, error)
	SyncLayoutRoster(ctx context.Context, layoutID uint) error
}

// Lease orchestrates user-initiated vehicle/train drive leases.
type Lease struct {
	vehicleLeases  repo.VehicleLeaseStore
	trainLeases    repo.TrainLeaseStore
	layoutVehicles *repo.LayoutVehicles
	layoutTrains   *repo.LayoutTrains
	vehicles       *repo.Vehicles
	trains         *repo.Trains
	users          *repo.Users
	roster         LeaseRosterPort
	hub            LeaseHubPort
	audit          AuditPublisher
	brake          LeaseBrakePort

	mu     sync.Mutex
	timers map[string]*leaseTimerState
}

type leaseTimerState struct {
	kind     domain.TakeoverTarget
	targetID string
	entry    LeaseEntry
	timer    *time.Timer
}

type layoutRosterIndex struct {
	vehicleIDs map[string]struct{}
	trainIDs   map[string]struct{}
}

type userLoginCache map[uint]string

type loadedLease struct {
	entry   LeaseEntry
	vehicle *domain.VehicleLease
	train   *domain.TrainLease
}

type LeaseConfig struct {
	VehicleLeases  repo.VehicleLeaseStore
	TrainLeases    repo.TrainLeaseStore
	LayoutVehicles *repo.LayoutVehicles
	LayoutTrains   *repo.LayoutTrains
	Vehicles       *repo.Vehicles
	Trains         *repo.Trains
	Users          *repo.Users
	Roster         LeaseRosterPort
	Hub            LeaseHubPort
	Audit          AuditPublisher
	Brake          LeaseBrakePort
}

func NewLease(cfg LeaseConfig) *Lease {
	return &Lease{
		vehicleLeases:  cfg.VehicleLeases,
		trainLeases:    cfg.TrainLeases,
		layoutVehicles: cfg.LayoutVehicles,
		layoutTrains:   cfg.LayoutTrains,
		vehicles:       cfg.Vehicles,
		trains:         cfg.Trains,
		users:          cfg.Users,
		roster:         cfg.Roster,
		hub:            cfg.Hub,
		audit:          cfg.Audit,
		brake:          cfg.Brake,
		timers:        make(map[string]*leaseTimerState),
	}
}

// RecoverPending reschedules expiry timers after restart.
func (s *Lease) RecoverPending(ctx context.Context) error {
	now := time.Now().UTC()
	if s.vehicleLeases != nil {
		rows, err := s.vehicleLeases.ListAll(ctx)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if !row.IsActive(now) {
				continue
			}
			entry := s.buildVehicleLeaseEntry(ctx, row, nil)
			s.scheduleExpiry(domain.TakeoverTargetVehicle, row.VehicleID.String(), row.ExpiresAt, entry)
		}
	}
	if s.trainLeases != nil {
		rows, err := s.trainLeases.ListAll(ctx)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if !row.IsActive(now) {
				continue
			}
			entry := s.buildTrainLeaseEntry(ctx, row, nil)
			s.scheduleExpiry(domain.TakeoverTargetTrain, row.TrainID.String(), row.ExpiresAt, entry)
		}
	}
	return nil
}

func leaseTimerID(kind domain.TakeoverTarget, targetID string) string {
	return string(kind) + ":" + targetID
}

func (s *Lease) requireStores() error {
	if s.vehicleLeases == nil || s.trainLeases == nil {
		return ErrLeaseStoreUnavailable
	}
	return nil
}

func mapLeaseStoreErr(err error) error {
	if errors.Is(err, repo.ErrLeaseNotFound) {
		return ErrLeaseNotFound
	}
	return err
}

func (s *Lease) refreshTimerEntry(kind domain.TakeoverTarget, targetID string, entry LeaseEntry) {
	id := leaseTimerID(kind, targetID)
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.timers[id]; ok {
		t.entry = entry
	}
}

func (s *Lease) layoutIDsForVehicle(ctx context.Context, vehicleID domain.VehicleID) []uint {
	if s.layoutVehicles == nil {
		return nil
	}
	rows, err := s.layoutVehicles.ListByVehicle(ctx, vehicleID)
	if err != nil {
		return nil
	}
	out := make([]uint, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.LayoutID)
	}
	return out
}

func (s *Lease) layoutIDsForTrain(ctx context.Context, trainID domain.TrainID) []uint {
	if s.layoutTrains == nil {
		return nil
	}
	rows, err := s.layoutTrains.ListByTrain(ctx, trainID)
	if err != nil {
		return nil
	}
	out := make([]uint, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.LayoutID)
	}
	return out
}

func (s *Lease) scheduleExpiry(kind domain.TakeoverTarget, targetID string, expiresAt time.Time, entry LeaseEntry) {
	id := leaseTimerID(kind, targetID)
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.timers[id]; ok && t.timer != nil {
		t.timer.Stop()
	}
	delay := time.Until(expiresAt)
	state := &leaseTimerState{kind: kind, targetID: targetID, entry: entry}
	if delay <= 0 {
		go s.expireLease(kind, targetID, entry)
		return
	}
	state.timer = time.AfterFunc(delay, func() {
		s.expireLease(kind, targetID, entry)
	})
	s.timers[id] = state
}

func (s *Lease) cancelTimer(kind domain.TakeoverTarget, targetID string) {
	id := leaseTimerID(kind, targetID)
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.timers[id]; ok && t.timer != nil {
		t.timer.Stop()
		delete(s.timers, id)
	}
}

func (s *Lease) expireLease(kind domain.TakeoverTarget, targetID string, entry LeaseEntry) {
	ctx := context.Background()
	now := time.Now().UTC()
	var layoutIDs []uint

	switch kind {
	case domain.TakeoverTargetVehicle:
		row, ok, err := s.vehicleLeases.Get(ctx, domain.VehicleID(targetID))
		if err != nil {
			return
		}
		if ok && row.IsActive(now) {
			fresh := s.buildVehicleLeaseEntry(ctx, row, nil)
			s.scheduleExpiry(kind, targetID, row.ExpiresAt, fresh)
			return
		}
		layoutIDs = s.layoutIDsForVehicle(ctx, domain.VehicleID(targetID))
	case domain.TakeoverTargetTrain:
		row, ok, err := s.trainLeases.Get(ctx, domain.TrainID(targetID))
		if err != nil {
			return
		}
		if ok && row.IsActive(now) {
			fresh := s.buildTrainLeaseEntry(ctx, row, nil)
			s.scheduleExpiry(kind, targetID, row.ExpiresAt, fresh)
			return
		}
		layoutIDs = s.layoutIDsForTrain(ctx, domain.TrainID(targetID))
	default:
		s.cancelTimer(kind, targetID)
		return
	}

	s.cancelTimer(kind, targetID)
	owner := domain.User{ID: entry.FromUserID, Login: entry.FromLogin}
	for _, layoutID := range layoutIDs {
		s.brakeLeasedTarget(ctx, layoutID, kind, targetID)
		if s.roster != nil {
			_ = s.roster.SyncLayoutRoster(ctx, layoutID)
		}
		s.publishEvent(ctx, layoutID, owner, contract.TypeLeaseExpired, entry)
	}
}

func (s *Lease) brakeLeasedTarget(ctx context.Context, layoutID uint, kind domain.TakeoverTarget, targetID string) {
	if s.brake == nil {
		return
	}
	_ = s.brake.StopLeasedTarget(ctx, layoutID, kind, targetID)
}

func formatUint8(v uint8) string {
	return strconv.FormatUint(uint64(v), 10)
}

// ListReceived returns leases where the user is the lessee.
func (s *Lease) ListReceived(ctx context.Context, layoutID, userID uint) ([]LeaseEntry, error) {
	idx, err := s.buildLayoutRosterIndex(ctx, layoutID)
	if err != nil {
		return nil, err
	}
	logins := make(userLoginCache)
	var out []LeaseEntry
	if s.vehicleLeases != nil {
		rows, err := s.vehicleLeases.ListByLessee(ctx, userID)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			entry, ok := s.enrichVehicleLeaseIndexed(ctx, row, idx, logins)
			if ok {
				out = append(out, entry)
			}
		}
	}
	if s.trainLeases != nil {
		rows, err := s.trainLeases.ListByLessee(ctx, userID)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			entry, ok := s.enrichTrainLeaseIndexed(ctx, row, idx, logins)
			if ok {
				out = append(out, entry)
			}
		}
	}
	return out, nil
}

// ListGranted returns leases the user granted to others. Effective
// admins see every active lease on the layout roster.
func (s *Lease) ListGranted(
	ctx context.Context,
	layoutID uint,
	actor domain.User,
	eff domain.EffectiveRoles,
) ([]LeaseEntry, error) {
	idx, err := s.buildLayoutRosterIndex(ctx, layoutID)
	if err != nil {
		return nil, err
	}
	logins := make(userLoginCache)
	now := time.Now().UTC()
	var out []LeaseEntry
	if s.vehicleLeases != nil {
		var rows []domain.VehicleLease
		var err error
		if eff.Has(domain.RoleAdmin) {
			rows, err = s.vehicleLeases.ListAll(ctx)
		} else {
			rows, err = s.vehicleLeases.ListByOwner(ctx, actor.ID)
		}
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if !row.IsActive(now) {
				continue
			}
			entry, ok := s.enrichVehicleLeaseIndexed(ctx, row, idx, logins)
			if ok {
				out = append(out, entry)
			}
		}
	}
	if s.trainLeases != nil {
		var rows []domain.TrainLease
		var err error
		if eff.Has(domain.RoleAdmin) {
			rows, err = s.trainLeases.ListAll(ctx)
		} else {
			rows, err = s.trainLeases.ListByOwner(ctx, actor.ID)
		}
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if !row.IsActive(now) {
				continue
			}
			entry, ok := s.enrichTrainLeaseIndexed(ctx, row, idx, logins)
			if ok {
				out = append(out, entry)
			}
		}
	}
	return out, nil
}

// Lendable returns targets and users for the create-lease dialog.
func (s *Lease) Lendable(ctx context.Context, layoutID, ownerID uint) (LendableCatalogue, error) {
	if s.roster == nil {
		return LendableCatalogue{}, nil
	}
	vehicles, err := s.roster.ListVehicles(ctx, layoutID)
	if err != nil {
		return LendableCatalogue{}, err
	}
	trains, err := s.roster.ListTrains(ctx, layoutID)
	if err != nil {
		return LendableCatalogue{}, err
	}
	now := time.Now().UTC()
	leasedVehicles := map[domain.VehicleID]struct{}{}
	leasedTrains := map[domain.TrainID]struct{}{}
	if s.vehicleLeases != nil {
		vIDs := make([]domain.VehicleID, 0)
		for _, e := range vehicles {
			if e.Vehicle.OwnerUserID == ownerID {
				vIDs = append(vIDs, e.Vehicle.ID)
			}
		}
		if len(vIDs) > 0 {
			rows, err := s.vehicleLeases.ListActive(ctx, vIDs, now)
			if err != nil {
				return LendableCatalogue{}, err
			}
			for _, row := range rows {
				leasedVehicles[row.VehicleID] = struct{}{}
			}
		}
	}
	if s.trainLeases != nil {
		tIDs := make([]domain.TrainID, 0)
		for _, e := range trains {
			if e.Train.OwnerUserID == ownerID {
				tIDs = append(tIDs, e.Train.ID)
			}
		}
		if len(tIDs) > 0 {
			rows, err := s.trainLeases.ListActive(ctx, tIDs, now)
			if err != nil {
				return LendableCatalogue{}, err
			}
			for _, row := range rows {
				leasedTrains[row.TrainID] = struct{}{}
			}
		}
	}
	targets := make([]LendableTarget, 0)
	for _, e := range vehicles {
		if e.Vehicle.OwnerUserID != ownerID {
			continue
		}
		if e.Vehicle.DCCAddress == nil {
			continue
		}
		if _, leased := leasedVehicles[e.Vehicle.ID]; leased {
			continue
		}
		targets = append(targets, LendableTarget{
			Kind:       domain.TakeoverTargetVehicle,
			TargetID:   e.Vehicle.ID.String(),
			TargetName: e.Vehicle.Name,
		})
	}
	for _, e := range trains {
		if e.Train.OwnerUserID != ownerID {
			continue
		}
		if _, leased := leasedTrains[e.Train.ID]; leased {
			continue
		}
		targets = append(targets, LendableTarget{
			Kind:       domain.TakeoverTargetTrain,
			TargetID:   e.Train.ID.String(),
			TargetName: e.Train.Name,
		})
	}
	var users []LendableUser
	if s.users != nil {
		rows, err := s.users.ListAll(ctx)
		if err != nil {
			return LendableCatalogue{}, err
		}
		users = make([]LendableUser, 0, len(rows))
		for _, u := range rows {
			if u.ID == ownerID || !u.Active {
				continue
			}
			users = append(users, LendableUser{
				UserID:       u.ID,
				Login:        u.Login,
				Organization: u.Organization,
			})
		}
	}
	return LendableCatalogue{Targets: targets, Users: users}, nil
}

// Create grants a drive lease from the target owner to lessee. The
// actor must be the owner or an effective admin acting on their behalf.
func (s *Lease) Create(
	ctx context.Context,
	layoutID uint,
	actor domain.User,
	eff domain.EffectiveRoles,
	kind domain.TakeoverTarget,
	targetID string,
	toUserID uint,
	speedLimit uint8,
	duration time.Duration,
) (LeaseEntry, error) {
	if err := s.requireStores(); err != nil {
		return LeaseEntry{}, err
	}
	owner, err := s.resolveTargetOwner(ctx, kind, targetID)
	if err != nil {
		return LeaseEntry{}, err
	}
	if actor.ID != owner.ID && !eff.Has(domain.RoleAdmin) {
		return LeaseEntry{}, ErrLeaseNotOwner
	}
	if err := validateLeaseInput(toUserID, owner.ID, speedLimit, duration); err != nil {
		return LeaseEntry{}, err
	}
	if err := s.ensureLessee(ctx, toUserID); err != nil {
		return LeaseEntry{}, err
	}
	if err := s.ensureOnLayout(ctx, layoutID, owner.ID, kind, targetID); err != nil {
		return LeaseEntry{}, err
	}
	if err := s.ensureNotLeased(ctx, kind, targetID); err != nil {
		return LeaseEntry{}, err
	}
	now := time.Now().UTC()
	expires := now.Add(duration)
	idx, err := s.buildLayoutRosterIndex(ctx, layoutID)
	if err != nil {
		return LeaseEntry{}, err
	}
	logins := make(userLoginCache)
	var entry LeaseEntry
	switch kind {
	case domain.TakeoverTargetVehicle:
		vID := domain.VehicleID(targetID)
		row := domain.VehicleLease{
			VehicleID:  vID,
			FromUserID: owner.ID,
			ToUserID:   toUserID,
			SpeedLimit: speedLimit,
			StartedAt:  now,
			ExpiresAt:  expires,
			Source:     "manual",
		}
		created, err := s.vehicleLeases.Create(ctx, &row, false)
		if err != nil {
			return LeaseEntry{}, err
		}
		if !created {
			return LeaseEntry{}, ErrLeaseConflict
		}
		var enriched bool
		entry, enriched = s.enrichVehicleLeaseIndexed(ctx, row, idx, logins)
		if !enriched {
			return LeaseEntry{}, ErrLeaseTargetNotOnLayout
		}
		s.scheduleExpiry(kind, targetID, expires, entry)
	case domain.TakeoverTargetTrain:
		tID := domain.TrainID(targetID)
		row := domain.TrainLease{
			TrainID:    tID,
			FromUserID: owner.ID,
			ToUserID:   toUserID,
			SpeedLimit: speedLimit,
			StartedAt:  now,
			ExpiresAt:  expires,
			Source:     "manual",
		}
		created, err := s.trainLeases.Create(ctx, &row, false)
		if err != nil {
			return LeaseEntry{}, err
		}
		if !created {
			return LeaseEntry{}, ErrLeaseConflict
		}
		var enriched bool
		entry, enriched = s.enrichTrainLeaseIndexed(ctx, row, idx, logins)
		if !enriched {
			return LeaseEntry{}, ErrLeaseTargetNotOnLayout
		}
		s.scheduleExpiry(kind, targetID, expires, entry)
	default:
		return LeaseEntry{}, ErrLeaseTargetNotOnLayout
	}
	if s.roster != nil {
		_ = s.roster.SyncLayoutRoster(ctx, layoutID)
	}
	s.publishEvent(ctx, layoutID, actor, contract.TypeLeaseCreated, entry)
	return entry, nil
}

// Revoke ends a lease. Owner, lessee, or an effective admin may call.
func (s *Lease) Revoke(
	ctx context.Context,
	layoutID uint,
	actor domain.User,
	eff domain.EffectiveRoles,
	kind domain.TakeoverTarget,
	targetID string,
) error {
	if err := s.requireStores(); err != nil {
		return err
	}
	loaded, err := s.loadLease(ctx, layoutID, kind, targetID)
	if err != nil {
		return err
	}
	if !s.canRevokeLease(actor, eff, loaded.entry) {
		return ErrLeaseNotParty
	}
	s.brakeLeasedTarget(ctx, layoutID, kind, targetID)
	switch {
	case loaded.vehicle != nil:
		if err := s.vehicleLeases.Revoke(ctx, loaded.vehicle.VehicleID); err != nil {
			return err
		}
	case loaded.train != nil:
		if err := s.trainLeases.Revoke(ctx, loaded.train.TrainID); err != nil {
			return err
		}
	default:
		return ErrLeaseNotFound
	}
	s.cancelTimer(kind, targetID)
	if s.roster != nil {
		_ = s.roster.SyncLayoutRoster(ctx, layoutID)
	}
	s.publishEvent(ctx, layoutID, actor, contract.TypeLeaseRevoked, loaded.entry)
	return nil
}

// UpdateSpeedLimit changes the lessee speed cap (owner or admin).
func (s *Lease) UpdateSpeedLimit(
	ctx context.Context,
	layoutID uint,
	actor domain.User,
	eff domain.EffectiveRoles,
	kind domain.TakeoverTarget,
	targetID string,
	speedLimit uint8,
) (LeaseEntry, error) {
	if err := s.requireStores(); err != nil {
		return LeaseEntry{}, err
	}
	if speedLimit > 100 {
		return LeaseEntry{}, ErrLeaseInvalidSpeedLimit
	}
	loaded, err := s.loadLease(ctx, layoutID, kind, targetID)
	if err != nil {
		return LeaseEntry{}, err
	}
	if !s.canManageGrantedLease(actor, eff, loaded.entry.FromUserID) {
		return LeaseEntry{}, ErrLeaseNotOwner
	}
	switch {
	case loaded.vehicle != nil:
		loaded.vehicle.SpeedLimit = speedLimit
		if err := s.vehicleLeases.Update(ctx, loaded.vehicle); err != nil {
			return LeaseEntry{}, mapLeaseStoreErr(err)
		}
		loaded.entry.SpeedLimit = speedLimit
	case loaded.train != nil:
		loaded.train.SpeedLimit = speedLimit
		if err := s.trainLeases.Update(ctx, loaded.train); err != nil {
			return LeaseEntry{}, mapLeaseStoreErr(err)
		}
		loaded.entry.SpeedLimit = speedLimit
	default:
		return LeaseEntry{}, ErrLeaseNotFound
	}
	if s.roster != nil {
		_ = s.roster.SyncLayoutRoster(ctx, layoutID)
	}
	s.refreshTimerEntry(kind, targetID, loaded.entry)
	s.publishEvent(ctx, layoutID, actor, contract.TypeLeaseUpdated, loaded.entry)
	return loaded.entry, nil
}

// UpdateDuration sets a new expiry relative to now (owner or admin).
func (s *Lease) UpdateDuration(
	ctx context.Context,
	layoutID uint,
	actor domain.User,
	eff domain.EffectiveRoles,
	kind domain.TakeoverTarget,
	targetID string,
	duration time.Duration,
) (LeaseEntry, error) {
	if err := s.requireStores(); err != nil {
		return LeaseEntry{}, err
	}
	if duration < LeaseMinDuration || duration > LeaseMaxDuration {
		return LeaseEntry{}, ErrLeaseInvalidDuration
	}
	loaded, err := s.loadLease(ctx, layoutID, kind, targetID)
	if err != nil {
		return LeaseEntry{}, err
	}
	if !s.canManageGrantedLease(actor, eff, loaded.entry.FromUserID) {
		return LeaseEntry{}, ErrLeaseNotOwner
	}
	expires := time.Now().UTC().Add(duration)
	switch {
	case loaded.vehicle != nil:
		loaded.vehicle.ExpiresAt = expires
		if err := s.vehicleLeases.Update(ctx, loaded.vehicle); err != nil {
			return LeaseEntry{}, mapLeaseStoreErr(err)
		}
		loaded.entry.ExpiresAt = expires
	case loaded.train != nil:
		loaded.train.ExpiresAt = expires
		if err := s.trainLeases.Update(ctx, loaded.train); err != nil {
			return LeaseEntry{}, mapLeaseStoreErr(err)
		}
		loaded.entry.ExpiresAt = expires
	default:
		return LeaseEntry{}, ErrLeaseNotFound
	}
	s.scheduleExpiry(kind, targetID, expires, loaded.entry)
	if s.roster != nil {
		_ = s.roster.SyncLayoutRoster(ctx, layoutID)
	}
	s.publishEvent(ctx, layoutID, actor, contract.TypeLeaseUpdated, loaded.entry)
	return loaded.entry, nil
}

func (s *Lease) resolveTargetOwner(
	ctx context.Context,
	kind domain.TakeoverTarget,
	targetID string,
) (domain.User, error) {
	var ownerUserID uint
	switch kind {
	case domain.TakeoverTargetVehicle:
		v, err := s.vehicles.FindByID(ctx, domain.VehicleID(targetID))
		if err != nil {
			return domain.User{}, err
		}
		if v.DCCAddress == nil {
			return domain.User{}, ErrLeaseTargetNotDrivable
		}
		ownerUserID = v.OwnerUserID
	case domain.TakeoverTargetTrain:
		tr, err := s.trains.FindByID(ctx, domain.TrainID(targetID))
		if err != nil {
			return domain.User{}, err
		}
		ownerUserID = tr.OwnerUserID
	default:
		return domain.User{}, ErrLeaseTargetNotOnLayout
	}
	if s.users == nil {
		return domain.User{ID: ownerUserID}, nil
	}
	u, err := s.users.FindByID(ctx, ownerUserID)
	if err != nil {
		return domain.User{}, svcerrors.ErrUserNotFound
	}
	return u, nil
}

func (s *Lease) canManageGrantedLease(actor domain.User, eff domain.EffectiveRoles, fromUserID uint) bool {
	return actor.ID == fromUserID || eff.Has(domain.RoleAdmin)
}

func (s *Lease) canRevokeLease(actor domain.User, eff domain.EffectiveRoles, entry LeaseEntry) bool {
	return actor.ID == entry.FromUserID ||
		actor.ID == entry.ToUserID ||
		eff.Has(domain.RoleAdmin)
}

func validateLeaseInput(toUserID, ownerID uint, speedLimit uint8, duration time.Duration) error {
	if toUserID == ownerID {
		return ErrLeaseSelf
	}
	if speedLimit > 100 {
		return ErrLeaseInvalidSpeedLimit
	}
	if duration < LeaseMinDuration || duration > LeaseMaxDuration {
		return ErrLeaseInvalidDuration
	}
	return nil
}

func (s *Lease) ensureLessee(ctx context.Context, toUserID uint) error {
	if s.users == nil {
		return svcerrors.ErrUserNotFound
	}
	u, err := s.users.FindByID(ctx, toUserID)
	if err != nil {
		return svcerrors.ErrUserNotFound
	}
	if !u.Active {
		return svcerrors.ErrAccountDeactivated
	}
	return nil
}

func (s *Lease) ensureOnLayout(ctx context.Context, layoutID, ownerID uint, kind domain.TakeoverTarget, targetID string) error {
	if s.roster == nil {
		return ErrLeaseTargetNotOnLayout
	}
	switch kind {
	case domain.TakeoverTargetVehicle:
		entries, err := s.roster.ListVehicles(ctx, layoutID)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if e.Vehicle.ID.String() == targetID && e.Vehicle.OwnerUserID == ownerID {
				return nil
			}
		}
	case domain.TakeoverTargetTrain:
		entries, err := s.roster.ListTrains(ctx, layoutID)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if e.Train.ID.String() == targetID && e.Train.OwnerUserID == ownerID {
				return nil
			}
		}
	}
	return ErrLeaseTargetNotOnLayout
}

func (s *Lease) ensureNotLeased(ctx context.Context, kind domain.TakeoverTarget, targetID string) error {
	now := time.Now().UTC()
	switch kind {
	case domain.TakeoverTargetVehicle:
		vID := domain.VehicleID(targetID)
		rows, err := s.vehicleLeases.ListActive(ctx, []domain.VehicleID{vID}, now)
		if err != nil {
			return err
		}
		if len(rows) > 0 {
			return ErrLeaseConflict
		}
	case domain.TakeoverTargetTrain:
		tID := domain.TrainID(targetID)
		rows, err := s.trainLeases.ListActive(ctx, []domain.TrainID{tID}, now)
		if err != nil {
			return err
		}
		if len(rows) > 0 {
			return ErrLeaseConflict
		}
	}
	return nil
}

func (s *Lease) loadLease(ctx context.Context, layoutID uint, kind domain.TakeoverTarget, targetID string) (loadedLease, error) {
	if err := s.requireStores(); err != nil {
		return loadedLease{}, err
	}
	now := time.Now().UTC()
	idx, err := s.buildLayoutRosterIndex(ctx, layoutID)
	if err != nil {
		return loadedLease{}, err
	}
	logins := make(userLoginCache)
	switch kind {
	case domain.TakeoverTargetVehicle:
		row, ok, err := s.vehicleLeases.Get(ctx, domain.VehicleID(targetID))
		if err != nil {
			return loadedLease{}, err
		}
		if !ok || !row.IsActive(now) {
			return loadedLease{}, ErrLeaseNotFound
		}
		entry, ok := s.enrichVehicleLeaseIndexed(ctx, row, idx, logins)
		if !ok {
			return loadedLease{}, ErrLeaseTargetNotOnLayout
		}
		return loadedLease{entry: entry, vehicle: &row}, nil
	case domain.TakeoverTargetTrain:
		row, ok, err := s.trainLeases.Get(ctx, domain.TrainID(targetID))
		if err != nil {
			return loadedLease{}, err
		}
		if !ok || !row.IsActive(now) {
			return loadedLease{}, ErrLeaseNotFound
		}
		entry, ok := s.enrichTrainLeaseIndexed(ctx, row, idx, logins)
		if !ok {
			return loadedLease{}, ErrLeaseTargetNotOnLayout
		}
		return loadedLease{entry: entry, train: &row}, nil
	default:
		return loadedLease{}, ErrLeaseNotFound
	}
}

func (s *Lease) buildLayoutRosterIndex(ctx context.Context, layoutID uint) (layoutRosterIndex, error) {
	idx := layoutRosterIndex{
		vehicleIDs: make(map[string]struct{}),
		trainIDs:   make(map[string]struct{}),
	}
	if s.roster == nil {
		return idx, nil
	}
	vehicles, err := s.roster.ListVehicles(ctx, layoutID)
	if err != nil {
		return idx, err
	}
	for _, e := range vehicles {
		idx.vehicleIDs[e.Vehicle.ID.String()] = struct{}{}
	}
	trains, err := s.roster.ListTrains(ctx, layoutID)
	if err != nil {
		return idx, err
	}
	for _, e := range trains {
		idx.trainIDs[e.Train.ID.String()] = struct{}{}
	}
	return idx, nil
}

func (s *Lease) enrichVehicleLeaseIndexed(
	ctx context.Context,
	row domain.VehicleLease,
	idx layoutRosterIndex,
	logins userLoginCache,
) (LeaseEntry, bool) {
	if _, ok := idx.vehicleIDs[row.VehicleID.String()]; !ok {
		return LeaseEntry{}, false
	}
	return s.buildVehicleLeaseEntry(ctx, row, logins), true
}

func (s *Lease) enrichTrainLeaseIndexed(
	ctx context.Context,
	row domain.TrainLease,
	idx layoutRosterIndex,
	logins userLoginCache,
) (LeaseEntry, bool) {
	if _, ok := idx.trainIDs[row.TrainID.String()]; !ok {
		return LeaseEntry{}, false
	}
	return s.buildTrainLeaseEntry(ctx, row, logins), true
}

func (s *Lease) buildVehicleLeaseEntry(ctx context.Context, row domain.VehicleLease, logins userLoginCache) LeaseEntry {
	name := row.VehicleID.String()
	if s.vehicles != nil {
		if v, err := s.vehicles.FindByID(ctx, row.VehicleID); err == nil {
			name = v.Name
		}
	}
	return LeaseEntry{
		Kind:       domain.TakeoverTargetVehicle,
		TargetID:   row.VehicleID.String(),
		TargetName: name,
		FromUserID: row.FromUserID,
		FromLogin:  s.loginCached(ctx, logins, row.FromUserID),
		ToUserID:   row.ToUserID,
		ToLogin:    s.loginCached(ctx, logins, row.ToUserID),
		ExpiresAt:  row.ExpiresAt,
		SpeedLimit: row.SpeedLimit,
	}
}

func (s *Lease) buildTrainLeaseEntry(ctx context.Context, row domain.TrainLease, logins userLoginCache) LeaseEntry {
	name := row.TrainID.String()
	if s.trains != nil {
		if t, err := s.trains.FindByID(ctx, row.TrainID); err == nil {
			name = t.Name
		}
	}
	return LeaseEntry{
		Kind:       domain.TakeoverTargetTrain,
		TargetID:   row.TrainID.String(),
		TargetName: name,
		FromUserID: row.FromUserID,
		FromLogin:  s.loginCached(ctx, logins, row.FromUserID),
		ToUserID:   row.ToUserID,
		ToLogin:    s.loginCached(ctx, logins, row.ToUserID),
		ExpiresAt:  row.ExpiresAt,
		SpeedLimit: row.SpeedLimit,
	}
}

func (s *Lease) loginCached(ctx context.Context, cache userLoginCache, userID uint) string {
	if cache != nil {
		if login, ok := cache[userID]; ok {
			return login
		}
	}
	login := s.userLogin(ctx, userID)
	if cache != nil {
		cache[userID] = login
	}
	return login
}

func (s *Lease) userLogin(ctx context.Context, userID uint) string {
	if s.users == nil {
		return ""
	}
	u, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return ""
	}
	return u.Login
}

func (s *Lease) publishEvent(ctx context.Context, layoutID uint, actor domain.User, typ string, entry LeaseEntry) {
	if s.hub == nil {
		return
	}
	wire := contract.LeaseEventWire{
		Kind:       entry.Kind,
		TargetID:   entry.TargetID,
		TargetName: entry.TargetName,
		FromUserID: entry.FromUserID,
		FromLogin:  entry.FromLogin,
		ToUserID:   entry.ToUserID,
		ToLogin:    entry.ToLogin,
		ExpiresAt:  entry.ExpiresAt.UnixMilli(),
		SpeedLimit: entry.SpeedLimit,
	}
	s.hub.BroadcastLease(layoutID, entry.FromUserID, typ, wire)
	s.hub.BroadcastLease(layoutID, entry.ToUserID, typ, wire)
	if s.audit != nil {
		msg := "audit_vehicle_leased"
		if entry.Kind == domain.TakeoverTargetTrain {
			msg = "audit_train_leased"
		}
		switch typ {
		case contract.TypeLeaseRevoked:
			if entry.Kind == domain.TakeoverTargetTrain {
				msg = "audit_train_lease_revoked"
			} else {
				msg = "audit_vehicle_lease_revoked"
			}
		case contract.TypeLeaseExpired:
			if entry.Kind == domain.TakeoverTargetTrain {
				msg = "audit_train_lease_expired"
			} else {
				msg = "audit_vehicle_lease_expired"
			}
		}
		_ = s.audit.Publish(ctx, layoutID, AuditActor{UserID: actor.ID, Login: actor.Login}, msg, map[string]string{
			"target":      entry.TargetName,
			"to_login":    entry.ToLogin,
			"expires_at":  entry.ExpiresAt.Format(time.RFC3339),
			"speed_limit": formatUint8(entry.SpeedLimit),
		})
	}
}
