package cmd

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/security"
)

var (
	ErrTakeoverNotConfigured     = svcerrors.ErrTakeoverNotConfigured
	ErrTakeoverTargetNotOnLayout = svcerrors.ErrTakeoverTargetNotOnLayout
	ErrTakeoverNotOwner          = svcerrors.ErrTakeoverNotOwner
	ErrTakeoverAlreadyPending    = svcerrors.ErrTakeoverAlreadyPending
	ErrTakeoverNotFound          = svcerrors.ErrTakeoverNotFound
	ErrTakeoverInvalidState      = svcerrors.ErrTakeoverInvalidState
	ErrTakeoverNotDriver         = svcerrors.ErrTakeoverNotDriver
	ErrTakeoverNotSignalman      = svcerrors.ErrTakeoverNotSignalman
	ErrNotInterlockingOccupant   = svcerrors.ErrNotInterlockingOccupant
)

// Takeover implements the takeover state machine (§4.3).
type Takeover struct {
	requests      repo.TakeoverRequestStore
	vehicleLeases repo.VehicleLeaseStore
	trainLeases   repo.TrainLeaseStore
	vehicles      *repo.Vehicles
	trains        *repo.Trains
	members       *repo.TrainMembers
	ilkSessions   *repo.InterlockingSessions
	users         *repo.Users
	roster        TakeoverRosterPort
	auth          TakeoverAuthPort
	hub           TakeoverHubPort
	sec           security.TakeoverSecurityContext

	mu             sync.Mutex
	grantTimers    map[uint]*time.Timer
	releaseTimers  map[uint]*time.Timer
}

type TakeoverRosterPort interface {
	ListVehicles(ctx context.Context, layoutID uint) ([]RosterVehicleEntry, error)
	ListTrains(ctx context.Context, layoutID uint) ([]RosterTrainEntry, error)
	SyncLayoutRoster(ctx context.Context, layoutID uint) error
}

type TakeoverAuthPort interface {
	Effective(ctx context.Context, user domain.User, layoutID uint) (domain.EffectiveRoles, error)
}

type TakeoverHubPort interface {
	BroadcastTakeover(layoutID, userID uint, typ string, payload any)
}

// TakeoverConfig wires Takeover dependencies.
type TakeoverConfig struct {
	Requests      repo.TakeoverRequestStore
	VehicleLeases repo.VehicleLeaseStore
	TrainLeases   repo.TrainLeaseStore
	Vehicles      *repo.Vehicles
	Trains        *repo.Trains
	TrainMembers  *repo.TrainMembers
	IlkSessions   *repo.InterlockingSessions
	Users         *repo.Users
	Roster        TakeoverRosterPort
	Auth          TakeoverAuthPort
	Hub           TakeoverHubPort
}

// NewTakeover returns a ready orchestrator.
func NewTakeover(cfg TakeoverConfig) *Takeover {
	return &Takeover{
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
		grantTimers:   make(map[uint]*time.Timer),
		releaseTimers: make(map[uint]*time.Timer),
	}
}

// RecoverPending reschedules auto-grant timers after restart.
func (s *Takeover) RecoverPending(ctx context.Context) error {
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

// RunJanitor revokes expired takeover leases when the store requires
// polling (SQLite fallback). Redis uses per-request release timers.
func (s *Takeover) RunJanitor(ctx context.Context) {
	if s.requests == nil || !s.requests.RequiresJanitor() {
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

func (s *Takeover) runJanitorOnce(ctx context.Context) {
	rows, err := s.requests.ListGranted(ctx)
	if err != nil {
		return
	}
	now := time.Now().UTC()
	for _, row := range rows {
		if !s.grantLeaseExpired(ctx, row, now) {
			continue
		}
		_ = s.release(ctx, row, now, "lease_expired")
	}
}

func (s *Takeover) broadcast(layoutID, userID uint, typ string, payload any) {
	if s.hub == nil {
		return
	}
	s.hub.BroadcastTakeover(layoutID, userID, typ, payload)
}

func (s *Takeover) effectiveRoles(ctx context.Context, user domain.User, layoutID uint) (domain.EffectiveRoles, error) {
	if s.auth == nil {
		return domain.NewEffectiveRoles(), nil
	}
	return s.auth.Effective(ctx, user, layoutID)
}

func (s *Takeover) userLogin(ctx context.Context, userID uint) string {
	if s.users == nil {
		return ""
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return ""
	}
	return user.Login
}

func (s *Takeover) grantLeaseExpired(ctx context.Context, row domain.TakeoverRequest, now time.Time) bool {
	if row.State != domain.TakeoverStateGranted {
		return false
	}
	switch row.Target {
	case domain.TakeoverTargetVehicle:
		rows, err := s.vehicleLeases.ListActive(ctx, []uint{row.TargetID}, now)
		if err != nil || len(rows) == 0 {
			return true
		}
		return !rows[0].IsActive(now)
	case domain.TakeoverTargetTrain:
		rows, err := s.trainLeases.ListActive(ctx, []uint{row.TargetID}, now)
		if err != nil || len(rows) == 0 {
			return true
		}
		return !rows[0].IsActive(now)
	default:
		return true
	}
}

// Request starts a pending takeover for a roster target.
func (s *Takeover) Request(
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
	eff, err := s.effectiveRoles(ctx, signalman, layoutID)
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

	signalmanLogin := s.userLogin(ctx, signalman.ID)
	requested := contract.TakeoverRequestedWire{
		RequestID:   row.ID,
		Signalman:   contract.TakeoverUserWire{UserID: signalman.ID, Login: signalmanLogin},
		Target:      target,
		TargetID:    targetID,
		AutoGrantAt: row.AutoGrantAt.UnixMilli(),
	}
	s.broadcast(layoutID, driverID, contract.TypeTakeoverRequested, requested)
	// Echo to the requesting signalman so their interlocking view can
	// show the "waiting for takeover" dialog with the same countdown
	// and a cancel action while the driver's 15 s window runs.
	s.broadcast(layoutID, signalman.ID, contract.TypeTakeoverRequested, requested)

	s.scheduleAutoGrant(row.ID, domain.TakeoverWindow)
	return row, nil
}

// Reject cancels a pending request from the driver side.
func (s *Takeover) Reject(ctx context.Context, requestID, driverID uint) error {
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
	s.cancelGrantTimer(requestID)
	s.broadcast(row.LayoutID, row.SignalmanUserID, contract.TypeTakeoverRejected, contract.TakeoverRejectedWire{RequestID: requestID})
	return nil
}

// Cancel backs out a pending request from the signalman side.
func (s *Takeover) Cancel(ctx context.Context, requestID, signalmanID uint) error {
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
	s.cancelGrantTimer(requestID)
	s.broadcast(row.LayoutID, row.DriverUserID, contract.TypeTakeoverCancelled, contract.TakeoverCancelledWire{RequestID: requestID})
	return nil
}

// Release ends a granted takeover and revokes the lease.
func (s *Takeover) Release(ctx context.Context, requestID, signalmanID uint, reason string) error {
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
func (s *Takeover) ReleaseAllForSignalman(ctx context.Context, signalmanID uint, reason string) error {
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

func (s *Takeover) release(ctx context.Context, row domain.TakeoverRequest, now time.Time, reason string) error {
	s.cancelGrantTimer(row.ID)
	s.cancelReleaseTimer(row.ID)
	if row.State == domain.TakeoverStateGranted {
		switch row.Target {
		case domain.TakeoverTargetVehicle:
			_ = s.vehicleLeases.Revoke(ctx, row.TargetID, now)
		case domain.TakeoverTargetTrain:
			_ = s.trainLeases.Revoke(ctx, row.TargetID, now)
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
		_ = s.roster.SyncLayoutRoster(ctx, row.LayoutID)
	}
	payload := contract.TakeoverReleasedWire{
		RequestID: row.ID,
		Target:    row.Target,
		TargetID:  row.TargetID,
		Reason:    reason,
	}
	s.broadcast(row.LayoutID, row.DriverUserID, contract.TypeTakeoverReleased, payload)
	s.broadcast(row.LayoutID, row.SignalmanUserID, contract.TypeTakeoverReleased, payload)
	return nil
}

func (s *Takeover) autoGrant(ctx context.Context, requestID uint) error {
	row, err := s.requests.FindByID(ctx, requestID)
	if err != nil {
		return err
	}
	if row.State != domain.TakeoverStatePending {
		return nil
	}
	now := time.Now().UTC()
	expires := now.Add(domain.TakeoverLeaseDuration)

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
	default:
		return ErrTakeoverInvalidState
	}

	row.State = domain.TakeoverStateGranted
	row.DecisionAt = &now
	leaseID := row.TargetID
	row.GrantedLeaseID = &leaseID
	if err := s.requests.Update(ctx, &row); err != nil {
		return err
	}
	if s.roster != nil {
		_ = s.roster.SyncLayoutRoster(ctx, row.LayoutID)
	}

	s.scheduleLeaseRelease(row.ID, expires)

	signalmanLogin := s.userLogin(ctx, row.SignalmanUserID)
	granted := contract.TakeoverGrantedWire{
		RequestID:      row.ID,
		Target:         row.Target,
		TargetID:       row.TargetID,
		Signalman:      contract.TakeoverUserWire{UserID: row.SignalmanUserID, Login: signalmanLogin},
		LeaseExpiresAt: expires.UnixMilli(),
	}
	s.broadcast(row.LayoutID, row.DriverUserID, contract.TypeTakeoverGranted, granted)
	s.broadcast(row.LayoutID, row.SignalmanUserID, contract.TypeTakeoverGranted, granted)
	return nil
}

func (s *Takeover) scheduleAutoGrant(requestID uint, delay time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.grantTimers[requestID]; ok {
		t.Stop()
	}
	timer := time.AfterFunc(delay, func() {
		_ = s.autoGrant(context.Background(), requestID)
		s.mu.Lock()
		delete(s.grantTimers, requestID)
		s.mu.Unlock()
	})
	s.grantTimers[requestID] = timer
}

func (s *Takeover) scheduleLeaseRelease(requestID uint, expiresAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.releaseTimers[requestID]; ok {
		t.Stop()
	}
	delay := time.Until(expiresAt)
	fire := func() {
		row, err := s.requests.FindByID(context.Background(), requestID)
		if err != nil || row.State != domain.TakeoverStateGranted {
			return
		}
		_ = s.release(context.Background(), row, time.Now().UTC(), "lease_expired")
		s.mu.Lock()
		delete(s.releaseTimers, requestID)
		s.mu.Unlock()
	}
	if delay <= 0 {
		go fire()
		return
	}
	s.releaseTimers[requestID] = time.AfterFunc(delay, fire)
}

func (s *Takeover) cancelGrantTimer(requestID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.grantTimers[requestID]; ok {
		t.Stop()
		delete(s.grantTimers, requestID)
	}
}

func (s *Takeover) cancelReleaseTimer(requestID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.releaseTimers[requestID]; ok {
		t.Stop()
		delete(s.releaseTimers, requestID)
	}
}

func (s *Takeover) loadPending(ctx context.Context, requestID uint) (domain.TakeoverRequest, error) {
	row, err := s.requests.FindByID(ctx, requestID)
	if err != nil {
		return domain.TakeoverRequest{}, ErrTakeoverNotFound
	}
	if row.State != domain.TakeoverStatePending {
		return domain.TakeoverRequest{}, ErrTakeoverInvalidState
	}
	return row, nil
}

func (s *Takeover) resolveDriverOnLayout(
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

func (s *Takeover) vehicleOwnerID(ctx context.Context, vehicleID uint) (uint, error) {
	if s.vehicles == nil {
		return 0, ErrTakeoverNotConfigured
	}
	v, err := s.vehicles.FindByID(ctx, vehicleID)
	if err != nil {
		return 0, err
	}
	return v.OwnerUserID, nil
}

func (s *Takeover) trainOwnerID(ctx context.Context, trainID uint) (uint, error) {
	if s.trains == nil {
		return 0, ErrTakeoverNotConfigured
	}
	t, err := s.trains.FindByID(ctx, trainID)
	if err != nil {
		return 0, err
	}
	return t.OwnerUserID, nil
}

func (s *Takeover) vehicleOnLayout(ctx context.Context, layoutID, vehicleID uint) bool {
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

func (s *Takeover) trainOnLayout(ctx context.Context, layoutID, trainID uint) bool {
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
