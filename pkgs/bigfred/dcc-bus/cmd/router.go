// Package cmd is the daemon's command router: it translates WS frames
// into pkgs/loco/commandstation calls, writes authoritative state to
// Redis, and applies the dead-man's safety plan when a client goes
// quiet.
package cmd

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/errors"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/security"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/state"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/station"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/ws"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

const (
	// stateTTL is the Redis TTL applied to loco:state:* entries. The
	// daemon refreshes the TTL on every observed change so an actively
	// driven loco never falls off the cache; a forgotten loco evicts
	// after ~5 minutes.
	stateTTL = 5 * time.Minute

	// maxDCCFunctionNum is the highest DCC function index fnCache
	// tracks (F0..F31 on Z21).
	maxDCCFunctionNum = 31
)

// Router implements ws.Router. It is the only component that ever
// talks to the underlying commandstation.Station — handlers from
// pub/sub or WS funnel through here so policy and audit fan-in stay
// in one place.
type Router struct {
	station          commandstation.Station
	hub              *ws.Hub
	redis            *state.Redis
	log              *logrus.Logger
	layoutID         uint
	commandStationID uint
	stationName      string
	stationKind      domain.CommandStationKind
	stationURI       string

	speedSteps   uint
	pollInterval time.Duration

	roster   *security.RosterGate
	trainsMu sync.RWMutex
	trains   []contract.DefinedTrain

	fnCache *FunctionsCache

	// pulseActive tracks in-flight timed function pulses per (addr, fn).
	// A new pulse is dropped while the previous one has not finished.
	pulseMu     sync.Mutex
	pulseActive map[fnKey]bool
}

// Config carries the few inputs Router needs at construction time.
type Config struct {
	Station          commandstation.Station
	Hub              *ws.Hub
	Redis            *state.Redis
	Log              *logrus.Logger
	AllowedVehicles  contract.AllowedVehicles
	DefinedTrains    contract.DefinedTrains
	LayoutID         uint
	CommandStationID uint
	StationName      string
	StationKind      domain.CommandStationKind
	StationURI       string
	SpeedSteps       uint
	// PollIntervalMs is the cadence of the state-feed polling fallback
	// used when the command-station driver cannot push state. 0 selects
	// a sane default.
	PollIntervalMs uint
}

// NewRouter assembles the router and seeds the layout roster cache
// from Redis snapshots published by loco-server.
func NewRouter(_ context.Context, cfg Config) (*Router, error) {
	log := cfg.Log
	if log == nil {
		log = logrus.New()
	}
	r := &Router{
		station:          cfg.Station,
		hub:              cfg.Hub,
		redis:            cfg.Redis,
		log:              log,
		layoutID:         cfg.LayoutID,
		commandStationID: cfg.CommandStationID,
		stationName:      cfg.StationName,
		stationKind:      cfg.StationKind,
		stationURI:       cfg.StationURI,
		speedSteps:       cfg.SpeedSteps,
		pollInterval:     time.Duration(cfg.PollIntervalMs) * time.Millisecond,
		roster:           security.NewRosterGate(cfg.LayoutID),
		fnCache:          NewFunctionsCache(),
		pulseActive:      make(map[fnKey]bool, 8),
	}
	r.ApplyAllowedVehicles(context.Background(), cfg.AllowedVehicles)
	r.ApplyDefinedTrains(cfg.DefinedTrains)
	r.log.WithFields(logrus.Fields{
		"layoutId":         r.layoutID,
		"commandStationId": r.commandStationID,
		"stationName":      r.stationName,
		"connection": station.Describe(domain.CommandStation{
			ID:            r.commandStationID,
			Name:          r.stationName,
			Kind:          r.stationKind,
			ConnectionURI: r.stationURI,
			SpeedSteps:    r.speedSteps,
		}),
		"rosterAddrs": len(r.roster.AllowedAddrs()),
	}).Info("dcc-bus router ready")
	return r, nil
}

func (r *Router) stationLogFields() logrus.Fields {
	return logrus.Fields{
		"commandStationId": r.commandStationID,
		"stationKind":      r.stationKind,
		"connection": station.Describe(domain.CommandStation{
			ID:            r.commandStationID,
			Name:          r.stationName,
			Kind:          r.stationKind,
			ConnectionURI: r.stationURI,
			SpeedSteps:    r.speedSteps,
		}),
	}
}

// ApplyAllowedVehicles replaces the in-memory drivable roster from a
// loco-server snapshot (boot GET or pub/sub on allowed_vehicles). Locos
// that were on the previous roster but missing from snap are stopped and
// have their functions turned off before the new roster is applied.
func (r *Router) ApplyAllowedVehicles(ctx context.Context, snap contract.AllowedVehicles) {
	if snap.LayoutID != 0 && snap.LayoutID != r.layoutID {
		return
	}

	// if any loco was removed from allowed vehicles, then stop it and turn off its functions
	removed := r.roster.DiffRemoved(snap)
	r.retireRemovedLocos(ctx, removed)

	// apply new list
	if !r.roster.ApplySnapshot(snap) {
		return
	}

	// logging
	addrs := make([]uint16, 0, len(snap.Vehicles))
	for _, v := range snap.Vehicles {
		addrs = append(addrs, v.Addr)
	}
	r.log.WithFields(logrus.Fields{
		"layoutId": r.layoutID,
		"addrs":    addrs,
		"count":    len(addrs),
	}).Info("dcc-bus allowed vehicles updated")
}

// ApplyDefinedTrains replaces the layout train roster cache.
func (r *Router) ApplyDefinedTrains(snap contract.DefinedTrains) {
	if snap.LayoutID != 0 && snap.LayoutID != r.layoutID {
		return
	}
	r.trainsMu.Lock()
	r.trains = append([]contract.DefinedTrain(nil), snap.Trains...)
	count := len(r.trains)
	r.trainsMu.Unlock()
	r.log.WithFields(logrus.Fields{
		"layoutId": r.layoutID,
		"count":    count,
	}).Info("dcc-bus defined trains updated")
}

// HandleSubscribe accepts a subscription request and immediately
// emits a state snapshot for each accepted address (or whatever is
// currently cached in Redis).
func (r *Router) HandleSubscribe(ctx context.Context, sess *ws.Session, payload protocol.LocoSubscribePayload, requestID string) ws.Outcome {
	accepted := make([]uint16, 0, len(payload.Addresses))
	rejected := make([]uint16, 0)
	for _, addr := range payload.Addresses {
		if !r.roster.IsLocoAllowedOnLayout(addr) {
			rejected = append(rejected, addr)
			_ = sess.SendTyped(ctx, protocol.TypeLocoError, protocol.LocoErrorPayload{
				Address: addr,
				Code:    security.ReasonVehicleNotOnLayout,
			})
			continue
		}
		accepted = append(accepted, addr)
	}
	fields := logrus.Fields{
		"sessionId": sess.ID,
		"requested": payload.Addresses,
		"accepted":  accepted,
		"rejected":  rejected,
	}
	if len(rejected) > 0 {
		r.log.WithFields(fields).Warn("dcc-bus loco.subscribe rejected")
	} else {
		r.log.WithFields(fields).Debug("dcc-bus loco.subscribe")
	}
	sess.Subscribe(accepted...)

	for _, addr := range accepted {
		if snap, ok, err := r.redis.LoadState(ctx, addr); err == nil && ok {
			r.fnCache.Seed(addr, snap.Functions)
			_ = sess.SendTyped(ctx, protocol.TypeLocoState, snap)
		}
	}
	if requestID != "" {
		return r.ackOutcome(ctx, sess, requestID, true, "")
	}
	return ws.OK()
}

// HandleSetSpeed forwards a throttle move to the command station,
// updates the Redis cache and fans the new state out to every other
// subscribed session.
func (r *Router) HandleSetSpeed(ctx context.Context, sess *ws.Session, p contract.LocoSetSpeedWire, requestID string) ws.Outcome {
	if reason := r.roster.DenyDriveCommand(sess.UserID, p.Address); reason != "" {
		return r.ackOutcome(ctx, sess, requestID, false, reason)
	}
	if err := r.applyMemberSetSpeed(ctx, sess, p.Address, p.Speed, p.Forward, p.Emergency, "throttle"); err != nil {
		_ = sess.SendTyped(ctx, protocol.TypeLocoError, protocol.LocoErrorPayload{
			Address: p.Address,
			Code:    errors.CodeCommandStationError,
			Detail:  err.Error(),
		})
		return r.ackOutcome(ctx, sess, requestID, false, errors.CodeCommandStationError)
	}
	return r.ackOutcome(ctx, sess, requestID, true, "")
}

func (r *Router) applyMemberSetSpeed(
	ctx context.Context,
	sess *ws.Session,
	addr uint16,
	speed uint8,
	forward bool,
	emergency bool,
	source string,
) error {
	if err := r.stationSetSpeed(addr, speed, forward, emergency); err != nil {
		fields := r.stationLogFields()
		fields["addr"] = addr
		fields["speed"] = speed
		fields["forward"] = forward
		fields["emergency"] = emergency
		r.log.WithError(err).WithFields(fields).Warn("dcc-bus command station SetSpeed failed")
		return err
	}
	r.log.WithFields(logrus.Fields{
		"addr":    addr,
		"speed":   speed,
		"forward": forward,
	}).Debug("dcc-bus command station SetSpeed ok")
	snap := contract.LocoStateWire{
		Address:            addr,
		Speed:              speed,
		Forward:            forward,
		ControlledByUserID: sess.UserID,
		Source:             source,
		At:                 time.Now().UTC().UnixMilli(),
	}
	if env, ok, err := r.redis.LoadState(ctx, addr); err == nil && ok {
		snap.Functions = env.Functions
	}
	if err := r.redis.StoreState(ctx, snap, stateTTL); err != nil {
		r.log.WithError(err).Debug("dcc-bus redis store")
	}
	r.broadcastLocoStateToObservers(ctx, snap)
	return nil
}

const (
	trainNotOnLayoutCode        = "train_not_on_layout"
	trainNoPoweredMembersCode   = "train_no_powered_members"
	notAuthorizedToDriveCode    = "not_authorized_to_drive"
	trainPartialFailureCode     = "partial_failure"
)

func (r *Router) findDefinedTrain(trainID uint) (contract.DefinedTrain, bool) {
	r.trainsMu.RLock()
	defer r.trainsMu.RUnlock()
	for _, t := range r.trains {
		if t.TrainID == trainID {
			return t, true
		}
	}
	return contract.DefinedTrain{}, false
}

// HandleTrainSetSpeed fans a throttle move to every powered member of
// a train on this layout roster.
func (r *Router) HandleTrainSetSpeed(ctx context.Context, sess *ws.Session, p contract.TrainSetSpeedWire, requestID string) ws.Outcome {
	train, ok := r.findDefinedTrain(p.TrainID)
	if !ok {
		return r.ackOutcome(ctx, sess, requestID, false, trainNotOnLayoutCode)
	}
	leading, hasLeading := train.LeadingMember()
	if !hasLeading {
		return r.ackOutcome(ctx, sess, requestID, false, trainNoPoweredMembersCode)
	}
	if !train.CanDrive(sess.UserID) {
		return r.ackOutcome(ctx, sess, requestID, false, notAuthorizedToDriveCode)
	}
	maxSpeed := contract.MaxSpeedForSpeedSteps(r.speedSteps)
	acks := make([]protocol.TrainSetSpeedMemberAck, 0, len(train.Members))
	allOK := true
	for _, m := range train.Members {
		if m.Addr == nil {
			continue
		}
		mult := m.SpeedMultiplier
		if m.VehicleID == leading.VehicleID && m.Position == leading.Position {
			mult = 1.0
		}
		speed := contract.EffectiveMemberSpeed(p.Speed, mult, maxSpeed)
		forward := p.Forward
		if m.Reversed {
			forward = !forward
		}
if err := r.applyMemberSetSpeed(ctx, sess, *m.Addr, speed, forward, false, "train"); err != nil {
			allOK = false
			acks = append(acks, protocol.TrainSetSpeedMemberAck{Addr: *m.Addr, OK: false, Error: errors.CodeCommandStationError})
			continue
		}
		acks = append(acks, protocol.TrainSetSpeedMemberAck{Addr: *m.Addr, OK: true})
	}
	if requestID != "" {
		ack := protocol.AckPayload{OK: allOK, Members: acks}
		if !allOK {
			ack.Error = trainPartialFailureCode
		}
		if err := sess.SendAckData(ctx, requestID, ack); err != nil {
			return ws.Fail(ws.ErrorCodeSendFailed)
		}
		if !allOK {
			return ws.Fail(trainPartialFailureCode)
		}
		return ws.OK()
	}
	if !allOK {
		return ws.Fail(trainPartialFailureCode)
	}
	return ws.OK()
}

// HandleSetFunction sets a single function on or off. The desired
// state is sent to the command station and mirrored in Redis.
func (r *Router) HandleSetFunction(ctx context.Context, sess *ws.Session, p contract.LocoSetFunctionWire, requestID string) ws.Outcome {
	if reason := r.roster.DenyDriveCommand(sess.UserID, p.Address); reason != "" {
		return r.ackOutcome(ctx, sess, requestID, false, reason)
	}
	if err := r.setLocoFunction(ctx, p.Address, sess.UserID, p.Function, p.On, "throttle"); err != nil {
		_ = sess.SendTyped(ctx, protocol.TypeLocoError, protocol.LocoErrorPayload{
			Address: p.Address,
			Code:    errors.CodeCommandStationError,
			Detail:  err.Error(),
		})
		return r.ackOutcome(ctx, sess, requestID, false, errors.CodeCommandStationError)
	}
	return r.ackOutcome(ctx, sess, requestID, true, "")
}

// ackOutcome sends an acknowledgement to the client.
func (r *Router) ackOutcome(ctx context.Context, sess *ws.Session, requestID string, ok bool, errCode string) ws.Outcome {
	if requestID == "" {
		if ok {
			return ws.OK()
		}
		return ws.Fail(errCode)
	}
	if err := sess.SendAck(ctx, requestID, ok, errCode); err != nil {
		return ws.Fail(ws.ErrorCodeSendFailed)
	}
	if ok {
		return ws.OK()
	}
	return ws.Fail(errCode)
}

// checkFnStateMatches reports whether addr/fn is already in the desired
// state in both fnCache and Redis. A mismatch in either layer means
// the DCC command must be issued (and the UI refreshed).
func (r *Router) checkFnStateMatches(ctx context.Context, addr uint16, fn uint8, on bool) bool {
	if !r.fnCache.Matches(addr, fn, on) {
		return false
	}
	env, ok, err := r.redis.LoadState(ctx, addr)
	if err != nil || !ok {
		return false
	}
	if int(fn) >= len(env.Functions) {
		return !on
	}
	return env.Functions[fn] == on
}

// collectDriveTargetsForUser returns every locomotive address the
// user is actively associated with on this command station: union of
// all tab subscriptions plus Redis snapshots they still control.
func (r *Router) collectDriveTargetsForUser(ctx context.Context, userID uint) []uint16 {
	seen := make(map[uint16]struct{}, 8)
	add := func(out *[]uint16, addr uint16) {
		if _, ok := seen[addr]; ok {
			return
		}
		seen[addr] = struct{}{}
		*out = append(*out, addr)
	}
	var addrs []uint16
	for _, s := range r.hub.SessionsForUser(userID) {
		for _, addr := range s.SubscribedAddrs() {
			add(&addrs, addr)
		}
	}
	allowed := r.roster.AllowedAddrs()
	for _, addr := range allowed {
		snap, ok, err := r.redis.LoadState(ctx, addr)
		if err != nil || !ok || snap.ControlledByUserID != userID {
			continue
		}
		add(&addrs, addr)
	}
	return addrs
}

// broadcastLocoStateToObservers broadcasts a state snapshot to every WS session that
// subscribed to the affected loco.
func (r *Router) broadcastLocoStateToObservers(ctx context.Context, snap contract.LocoStateWire) {
	env, err := protocol.Frame(protocol.TypeLocoState, snap)
	if err != nil {
		return
	}
	r.hub.Broadcast(ctx, snap.Address, env)
}

// HandleControlCommand decodes a server-initiated command frame from
// the Redis dcc-bus:cmd channel and applies it. The payload format
// mirrors the WS protocol so loco-server can hand off train.setSpeed
// to dcc-bus by simply forwarding the typed envelope (§7e.6).
func (r *Router) HandleControlCommand(ctx context.Context, raw []byte) {
	var env contract.EnvelopeWire
	if err := json.Unmarshal(raw, &env); err != nil {
		r.log.WithError(err).Debug("dcc-bus control cmd: bad envelope")
		return
	}
	r.log.WithField("type", env.Type).Debug("dcc-bus control cmd")

	switch env.Type {
	case protocol.TypeLocoSetSpeed:
		var p contract.LocoSetSpeedWire
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return
		}
		r.applyControlSetSpeed(ctx, p)

	case protocol.TypeLocoSetFunction:
		var p contract.LocoSetFunctionWire
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return
		}
		r.applyControlSetFunction(ctx, p)

	case protocol.TypeSystemEStop:
		r.applyEStopAll(ctx, "system")

	case protocol.TypeSystemRadioStop:
		r.HandleRadioStop(ctx)

	case protocol.TypeSystemEStopTarget:
		var p contract.EStopTargetCommandWire
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return
		}
		r.applyEStopTarget(ctx, p.Addresses)
	}
}

// applyControlSetSpeed sets a single throttle move. The desired state is
// sent to the command station and mirrored in Redis.
func (r *Router) applyControlSetSpeed(ctx context.Context, p contract.LocoSetSpeedWire) {
	if !r.roster.IsLocoAllowedOnLayout(p.Address) {
		return
	}
	if err := r.stationSetSpeed(p.Address, p.Speed, p.Forward, p.Emergency); err != nil {
		r.log.WithError(err).WithField("addr", p.Address).Warn("dcc-bus control setSpeed failed")
		return
	}
	snap := contract.LocoStateWire{
		Address: p.Address,
		Speed:   p.Speed,
		Forward: p.Forward,
		Source:  "server",
		At:      time.Now().UTC().UnixMilli(),
	}
	if cached, ok, err := r.redis.LoadState(ctx, p.Address); err == nil && ok {
		snap.Functions = cached.Functions
		snap.ControlledByUserID = cached.ControlledByUserID
	}
	if err := r.redis.StoreState(ctx, snap, stateTTL); err != nil {
		r.log.WithError(err).Debug("dcc-bus control redis store")
	}
	r.broadcastLocoStateToObservers(ctx, snap)
}

// applyControlSetFunction sets a single function on or off. The desired
// state is sent to the command station and mirrored in Redis.
func (r *Router) applyControlSetFunction(ctx context.Context, p contract.LocoSetFunctionWire) {
	if !r.roster.IsLocoAllowedOnLayout(p.Address) {
		return
	}
	userID := uint(0)
	if cached, ok, err := r.redis.LoadState(ctx, p.Address); err == nil && ok {
		userID = cached.ControlledByUserID
	}
	_ = r.setLocoFunction(ctx, p.Address, userID, p.Function, p.On, "server")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
