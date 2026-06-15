package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/go-rel/rel"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/security"
	"github.com/keskad/loco/pkgs/bigfred/server/ws"
)

var (
	ErrTakeoverNotConfigured     = errors.New("takeover_not_configured")
	ErrTakeoverTargetNotOnLayout = errors.New("takeover_target_not_on_layout")
	ErrTakeoverNotOwner          = errors.New("takeover_not_owner")
	ErrTakeoverAlreadyPending    = errors.New("takeover_already_pending")
	ErrTakeoverNotFound          = errors.New("takeover_not_found")
	ErrTakeoverInvalidState      = errors.New("takeover_invalid_state")
	ErrTakeoverNotDriver         = errors.New("not_takeover_driver")
	ErrTakeoverNotSignalman      = errors.New("not_takeover_signalman")
	ErrNotInterlockingOccupant   = errors.New("not_interlocking_occupant")
)

// TakeoverService implements the takeover state machine (§4.3).
type TakeoverService struct {
	db              rel.Repository
	requests        *repo.TakeoverRequests
	vehicleLeases   repo.Leases[domain.VehicleLease]
	trainLeases     repo.Leases[domain.TrainLease]
	vehicles        *repo.Vehicles
	trains          *repo.Trains
	members         *repo.TrainMembers
	ilkSessions     *repo.InterlockingSessions
	users           *repo.Users
	roster          *LayoutVehicleService
	auth            *AuthService
	hub             *ws.Hub
	sec             security.TakeoverSecurityContext

	mu     sync.Mutex
	timers map[uint]*time.Timer
}

// TakeoverConfig wires TakeoverService dependencies.
type TakeoverConfig struct {
	DB            rel.Repository
	Requests      *repo.TakeoverRequests
	VehicleLeases repo.Leases[domain.VehicleLease]
	TrainLeases   repo.Leases[domain.TrainLease]
	Vehicles      *repo.Vehicles
	Trains        *repo.Trains
	TrainMembers  *repo.TrainMembers
	IlkSessions   *repo.InterlockingSessions
	Users         *repo.Users
	Roster        *LayoutVehicleService
	Auth          *AuthService
	Hub           *ws.Hub
}

// NewTakeoverService returns a ready orchestrator.
func NewTakeoverService(cfg TakeoverConfig) *TakeoverService {
	return &TakeoverService{
		db:            cfg.DB,
		requests:      cfg.Requests,
		vehicleLeases: cfg.VehicleLeases,
		trainLeases:   cfg.TrainLeases,
		vehicles:      cfg.Vehicles,
		trains:        cfg.Trains,
		members:       cfg.TrainMembers,
		ilkSessions:   cfg.IlkSessions,
		users:         cfg.Users,
		roster:        cfg.Roster,
		auth:          cfg.Auth,
		hub:           cfg.Hub,
		timers:        make(map[uint]*time.Timer),
	}
}

// RecoverPending reschedules auto-grant timers after restart.
func (s *TakeoverService) RecoverPending(ctx context.Context) error {
	if s.requests == nil {
		return nil
	}
	rows, err := s.requests.ListPending(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, row := range rows {
		if !row.AutoGrantAt.After(now) {
			if err := s.autoGrant(ctx, row.ID); err != nil {
				continue
			}
			continue
		}
		s.scheduleAutoGrant(row.ID, row.AutoGrantAt.Sub(now))
	}
	return nil
}

// RunJanitor revokes expired takeover leases and emits release events.
func (s *TakeoverService) RunJanitor(ctx context.Context) {
	if s.requests == nil {
		return
	}
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runJanitorOnce(ctx)
		}
	}
}

func (s *TakeoverService) runJanitorOnce(ctx context.Context) {
	rows, err := s.requests.ListGranted(ctx)
	if err != nil {
		return
	}
	now := time.Now().UTC()
	for _, row := range rows {
		if row.GrantedLeaseID == nil {
			continue
		}
		expired, err := s.leaseExpired(ctx, row, now)
		if err != nil || !expired {
			continue
		}
		_ = s.release(ctx, row, now, "lease_expired")
	}
}

func (s *TakeoverService) leaseExpired(ctx context.Context, row domain.TakeoverRequest, now time.Time) (bool, error) {
	if row.Target == domain.TakeoverTargetVehicle {
		lease, err := repo.FindVehicleLeaseByID(ctx, s.db, *row.GrantedLeaseID)
		if err != nil {
			return true, nil
		}
		return !lease.IsActive(now), nil
	}
	lease, err := repo.FindTrainLeaseByID(ctx, s.db, *row.GrantedLeaseID)
	if err != nil {
		return true, nil
	}
	return !lease.IsActive(now), nil
}

// Request starts a pending takeover for a roster target.
func (s *TakeoverService) Request(
	ctx context.Context,
	layoutID uint,
	signalman domain.User,
	target domain.TakeoverTarget,
	targetID uint,
) (domain.TakeoverRequest, error) {
	if s.requests == nil {
		return domain.TakeoverRequest{}, ErrTakeoverNotConfigured
	}
	sess, err := s.ilkSessions.FindActiveByUser(ctx, signalman.ID)
	if err != nil {
		if errors.Is(err, repo.ErrInterlockingSessionNotFound) {
			return domain.TakeoverRequest{}, ErrNotInterlockingOccupant
		}
		return domain.TakeoverRequest{}, err
	}
	eff, err := s.auth.Effective(ctx, signalman, layoutID)
	if err != nil {
		return domain.TakeoverRequest{}, err
	}
	if d := s.sec.CanRequest(eff, sess.SignalmanUserID, signalman.ID); !d.Allowed {
		return domain.TakeoverRequest{}, takeoverDenied(d.Reason)
	}

	driverID, err := s.resolveDriverOnLayout(ctx, layoutID, target, targetID)
	if err != nil {
		return domain.TakeoverRequest{}, err
	}
	if driverID == signalman.ID {
		return domain.TakeoverRequest{}, ErrTakeoverNotOwner
	}

	if _, err := s.requests.FindPendingForTarget(ctx, target, targetID); err == nil {
		return domain.TakeoverRequest{}, ErrTakeoverAlreadyPending
	} else if !errors.Is(err, repo.ErrTakeoverRequestNotFound) {
		return domain.TakeoverRequest{}, err
	}

	now := time.Now().UTC()
	row := domain.TakeoverRequest{
		LayoutID:        layoutID,
		InterlockingID:  sess.InterlockingID,
		SignalmanUserID: signalman.ID,
		DriverUserID:    driverID,
		Target:          target,
		TargetID:        targetID,
		RequestedAt:     now,
		AutoGrantAt:     now.Add(domain.TakeoverWindow),
		State:           domain.TakeoverStatePending,
	}
	if err := s.requests.Insert(ctx, &row); err != nil {
		return domain.TakeoverRequest{}, err
	}

	signalmanUser, _ := s.users.FindByID(ctx, signalman.ID)
	s.hub.BroadcastToUserInLayout(layoutID, driverID, contract.TypeTakeoverRequested, contract.TakeoverRequestedWire{
		RequestID:   row.ID,
		Signalman:   contract.TakeoverUserWire{UserID: signalman.ID, Login: signalmanUser.Login},
		Target:      target,
		TargetID:    targetID,
		AutoGrantAt: row.AutoGrantAt.UnixMilli(),
	})

	s.scheduleAutoGrant(row.ID, domain.TakeoverWindow)
	return row, nil
}

// Reject cancels a pending request from the driver side.
func (s *TakeoverService) Reject(ctx context.Context, requestID, driverID uint) error {
	row, err := s.loadPending(ctx, requestID)
	if err != nil {
		return err
	}
	if row.DriverUserID != driverID {
		return ErrTakeoverNotDriver
	}
	now := time.Now().UTC()
	repo.MarkTakeoverDecision(&row, domain.TakeoverStateRejected, now)
	if err := s.requests.Update(ctx, &row); err != nil {
		return err
	}
	s.cancelTimer(requestID)
	s.hub.BroadcastToUserInLayout(row.LayoutID, row.SignalmanUserID, contract.TypeTakeoverRejected, contract.TakeoverRejectedWire{RequestID: requestID})
	return nil
}

// Cancel backs out a pending request from the signalman side.
func (s *TakeoverService) Cancel(ctx context.Context, requestID, signalmanID uint) error {
	row, err := s.loadPending(ctx, requestID)
	if err != nil {
		return err
	}
	if row.SignalmanUserID != signalmanID {
		return ErrTakeoverNotSignalman
	}
	now := time.Now().UTC()
	repo.MarkTakeoverDecision(&row, domain.TakeoverStateCancelled, now)
	if err := s.requests.Update(ctx, &row); err != nil {
		return err
	}
	s.cancelTimer(requestID)
	s.hub.BroadcastToUserInLayout(row.LayoutID, row.DriverUserID, contract.TypeTakeoverCancelled, contract.TakeoverCancelledWire{RequestID: requestID})
	return nil
}

// Release ends a granted takeover and revokes the lease.
func (s *TakeoverService) Release(ctx context.Context, requestID, signalmanID uint, reason string) error {
	row, err := s.requests.FindByID(ctx, requestID)
	if err != nil {
		return ErrTakeoverNotFound
	}
	if row.State != domain.TakeoverStateGranted {
		return ErrTakeoverInvalidState
	}
	if row.SignalmanUserID != signalmanID {
		return ErrTakeoverNotSignalman
	}
	return s.release(ctx, row, time.Now().UTC(), reason)
}

// ReleaseAllForSignalman ends every granted takeover when the signalman
// leaves the interlocking box.
func (s *TakeoverService) ReleaseAllForSignalman(ctx context.Context, signalmanID uint, reason string) error {
	rows, err := s.requests.ListGrantedBySignalman(ctx, signalmanID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, row := range rows {
		if err := s.release(ctx, row, now, reason); err != nil {
			continue
		}
	}
	return nil
}

func (s *TakeoverService) release(ctx context.Context, row domain.TakeoverRequest, now time.Time, reason string) error {
	if row.GrantedLeaseID != nil {
		if row.Target == domain.TakeoverTargetVehicle {
			_ = repo.RevokeVehicleLease(ctx, s.db, *row.GrantedLeaseID, now)
		} else {
			_ = repo.RevokeTrainLease(ctx, s.db, *row.GrantedLeaseID, now)
		}
	}
	row.State = domain.TakeoverStateReleased
	row.ReleasedAt = &now
	if row.DecisionAt == nil {
		row.DecisionAt = &now
	}
	if err := s.requests.Update(ctx, &row); err != nil {
		return err
	}
	if s.roster != nil {
		_ = s.roster.SyncLayoutRosterToRedis(ctx, row.LayoutID)
	}
	payload := contract.TakeoverReleasedWire{
		RequestID: row.ID,
		Target:    row.Target,
		TargetID:  row.TargetID,
		Reason:    reason,
	}
	s.hub.BroadcastToUserInLayout(row.LayoutID, row.DriverUserID, contract.TypeTakeoverReleased, payload)
	s.hub.BroadcastToUserInLayout(row.LayoutID, row.SignalmanUserID, contract.TypeTakeoverReleased, payload)
	return nil
}

func (s *TakeoverService) autoGrant(ctx context.Context, requestID uint) error {
	row, err := s.requests.FindByID(ctx, requestID)
	if err != nil {
		return err
	}
	if row.State != domain.TakeoverStatePending {
		return nil
	}
	now := time.Now().UTC()
	expires := now.Add(domain.TakeoverLeaseDuration)

	var leaseID uint
	switch row.Target {
	case domain.TakeoverTargetVehicle:
		ownerID, err := s.vehicleOwnerID(ctx, row.TargetID)
		if err != nil {
			return err
		}
		lease := domain.VehicleLease{
			VehicleID:  row.TargetID,
			FromUserID: ownerID,
			ToUserID:   row.SignalmanUserID,
			StartedAt:  now,
			ExpiresAt:  expires,
		}
		if err := s.vehicleLeases.Insert(ctx, &lease); err != nil {
			return err
		}
		leaseID = lease.ID
	case domain.TakeoverTargetTrain:
		ownerID, err := s.trainOwnerID(ctx, row.TargetID)
		if err != nil {
			return err
		}
		lease := domain.TrainLease{
			TrainID:    row.TargetID,
			FromUserID: ownerID,
			ToUserID:   row.SignalmanUserID,
			StartedAt:  now,
			ExpiresAt:  expires,
		}
		if err := s.trainLeases.Insert(ctx, &lease); err != nil {
			return err
		}
		leaseID = lease.ID
	default:
		return ErrTakeoverInvalidState
	}

	row.State = domain.TakeoverStateGranted
	row.DecisionAt = &now
	row.GrantedLeaseID = &leaseID
	if err := s.requests.Update(ctx, &row); err != nil {
		return err
	}
	if s.roster != nil {
		_ = s.roster.SyncLayoutRosterToRedis(ctx, row.LayoutID)
	}

	signalmanUser, _ := s.users.FindByID(ctx, row.SignalmanUserID)
	granted := contract.TakeoverGrantedWire{
		RequestID:      row.ID,
		Target:         row.Target,
		TargetID:       row.TargetID,
		Signalman:      contract.TakeoverUserWire{UserID: row.SignalmanUserID, Login: signalmanUser.Login},
		LeaseExpiresAt: expires.UnixMilli(),
	}
	s.hub.BroadcastToUserInLayout(row.LayoutID, row.DriverUserID, contract.TypeTakeoverGranted, granted)
	s.hub.BroadcastToUserInLayout(row.LayoutID, row.SignalmanUserID, contract.TypeTakeoverGranted, granted)
	return nil
}

func (s *TakeoverService) scheduleAutoGrant(requestID uint, delay time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.timers[requestID]; ok {
		t.Stop()
	}
	timer := time.AfterFunc(delay, func() {
		_ = s.autoGrant(context.Background(), requestID)
		s.mu.Lock()
		delete(s.timers, requestID)
		s.mu.Unlock()
	})
	s.timers[requestID] = timer
}

func (s *TakeoverService) cancelTimer(requestID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.timers[requestID]; ok {
		t.Stop()
		delete(s.timers, requestID)
	}
}

func (s *TakeoverService) loadPending(ctx context.Context, requestID uint) (domain.TakeoverRequest, error) {
	row, err := s.requests.FindByID(ctx, requestID)
	if err != nil {
		return domain.TakeoverRequest{}, ErrTakeoverNotFound
	}
	if row.State != domain.TakeoverStatePending {
		return domain.TakeoverRequest{}, ErrTakeoverInvalidState
	}
	return row, nil
}

func (s *TakeoverService) resolveDriverOnLayout(
	ctx context.Context,
	layoutID uint,
	target domain.TakeoverTarget,
	targetID uint,
) (uint, error) {
	switch target {
	case domain.TakeoverTargetVehicle:
		ownerID, err := s.vehicleOwnerID(ctx, targetID)
		if err != nil {
			return 0, err
		}
		if !s.vehicleOnLayout(ctx, layoutID, targetID) {
			return 0, ErrTakeoverTargetNotOnLayout
		}
		return ownerID, nil
	case domain.TakeoverTargetTrain:
		ownerID, err := s.trainOwnerID(ctx, targetID)
		if err != nil {
			return 0, err
		}
		if !s.trainOnLayout(ctx, layoutID, targetID) {
			return 0, ErrTakeoverTargetNotOnLayout
		}
		return ownerID, nil
	default:
		return 0, ErrTakeoverInvalidState
	}
}

func (s *TakeoverService) vehicleOwnerID(ctx context.Context, vehicleID uint) (uint, error) {
	if s.vehicles == nil {
		return 0, ErrTakeoverNotConfigured
	}
	v, err := s.vehicles.FindByID(ctx, vehicleID)
	if err != nil {
		return 0, err
	}
	return v.OwnerUserID, nil
}

func (s *TakeoverService) trainOwnerID(ctx context.Context, trainID uint) (uint, error) {
	if s.trains == nil {
		return 0, ErrTakeoverNotConfigured
	}
	t, err := s.trains.FindByID(ctx, trainID)
	if err != nil {
		return 0, err
	}
	return t.OwnerUserID, nil
}

func (s *TakeoverService) vehicleOnLayout(ctx context.Context, layoutID, vehicleID uint) bool {
	if s.roster == nil {
		return false
	}
	rows, err := s.roster.ListVehicles(ctx, layoutID)
	if err != nil {
		return false
	}
	for _, e := range rows {
		if e.Vehicle.ID == vehicleID {
			return true
		}
	}
	return false
}

func (s *TakeoverService) trainOnLayout(ctx context.Context, layoutID, trainID uint) bool {
	if s.roster == nil {
		return false
	}
	rows, err := s.roster.ListTrains(ctx, layoutID)
	if err != nil {
		return false
	}
	for _, e := range rows {
		if e.Train.ID == trainID {
			return true
		}
	}
	return false
}

type takeoverDeniedError struct{ code string }

func (e takeoverDeniedError) Error() string { return e.code }

func takeoverDenied(code string) error {
	return takeoverDeniedError{code: code}
}

// TakeoverDeniedCode maps service errors to WS ack codes.
func TakeoverDeniedCode(err error) string {
	if err == nil {
		return ""
	}
	var denied takeoverDeniedError
	if errors.As(err, &denied) {
		return denied.code
	}
	switch {
	case errors.Is(err, ErrTakeoverNotConfigured):
		return "takeover_not_configured"
	case errors.Is(err, ErrTakeoverTargetNotOnLayout):
		return "takeover_target_not_on_layout"
	case errors.Is(err, ErrTakeoverNotOwner):
		return "takeover_not_owner"
	case errors.Is(err, ErrTakeoverAlreadyPending):
		return "takeover_already_pending"
	case errors.Is(err, ErrTakeoverNotFound):
		return "takeover_not_found"
	case errors.Is(err, ErrTakeoverInvalidState):
		return "takeover_invalid_state"
	case errors.Is(err, ErrTakeoverNotDriver):
		return "not_takeover_driver"
	case errors.Is(err, ErrTakeoverNotSignalman):
		return "not_takeover_signalman"
	case errors.Is(err, ErrNotInterlockingOccupant):
		return "not_interlocking_occupant"
	default:
		return "takeover_failed"
	}
}
