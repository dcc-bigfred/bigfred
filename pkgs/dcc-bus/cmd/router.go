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

	"github.com/keskad/loco/pkgs/dcc-bus/errors"
	"github.com/keskad/loco/pkgs/dcc-bus/protocol"
	"github.com/keskad/loco/pkgs/dcc-bus/security"
	"github.com/keskad/loco/pkgs/dcc-bus/state"
	"github.com/keskad/loco/pkgs/dcc-bus/station"
	"github.com/keskad/loco/pkgs/dcc-bus/ws"
	"github.com/keskad/loco/pkgs/layoutroster"
	"github.com/keskad/loco/pkgs/loco/commandstation"
	"github.com/keskad/loco/pkgs/server/domain"
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
	trains   []layoutroster.DefinedTrain

	// fnCache mirrors the last sent function bit per (addr, fn) so a
	// rapid toggle doesn't reissue the same DCC packet.
	fnCache   map[fnKey]bool
	fnCacheMu sync.Mutex
}

// Config carries the few inputs Router needs at construction time.
type Config struct {
	Station          commandstation.Station
	Hub              *ws.Hub
	Redis            *state.Redis
	Log              *logrus.Logger
	AllowedVehicles  layoutroster.AllowedVehicles
	DefinedTrains    layoutroster.DefinedTrains
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

type fnKey struct {
	Addr uint16
	Fn   uint8
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
		fnCache:          make(map[fnKey]bool, 32),
	}
	r.ApplyAllowedVehicles(cfg.AllowedVehicles)
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
// loco-server snapshot (boot GET or pub/sub on allowed_vehicles).
func (r *Router) ApplyAllowedVehicles(snap layoutroster.AllowedVehicles) {
	if !r.roster.ApplySnapshot(snap) {
		return
	}
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
func (r *Router) ApplyDefinedTrains(snap layoutroster.DefinedTrains) {
	if snap.LayoutID != 0 && snap.LayoutID != r.layoutID {
		return
	}
	r.trainsMu.Lock()
	r.trains = append([]layoutroster.DefinedTrain(nil), snap.Trains...)
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
func (r *Router) HandleSubscribe(ctx context.Context, sess *ws.Session, payload protocol.LocoSubscribePayload, requestID string) {
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
			r.seedFnCache(addr, snap.Functions)
			_ = sess.SendTyped(ctx, protocol.TypeLocoState, snap)
		}
	}
	if requestID != "" {
		_ = sess.SendAck(ctx, requestID, true, "")
	}
}

// HandleSetSpeed forwards a throttle move to the command station,
// updates the Redis cache and fans the new state out to every other
// subscribed session.
func (r *Router) HandleSetSpeed(ctx context.Context, sess *ws.Session, p protocol.LocoSetSpeedPayload, requestID string) {
	if reason := r.roster.DenyDriveCommand(sess.UserID, p.Address); reason != "" {
		_ = sess.SendAck(ctx, requestID, false, reason)
		return
	}
	speed := p.Speed
	if p.Emergency {
		speed = 1 // DCC EMG-stop is "speed step 1" in 128-step mode
	}
	if err := r.station.SetSpeed(commandstation.LocoAddr(p.Address), speed, p.Forward, uint8(r.speedSteps)); err != nil {
		fields := r.stationLogFields()
		fields["addr"] = p.Address
		fields["speed"] = p.Speed
		fields["forward"] = p.Forward
		fields["emergency"] = p.Emergency
		r.log.WithError(err).WithFields(fields).Warn("dcc-bus command station SetSpeed failed")
		_ = sess.SendTyped(ctx, protocol.TypeLocoError, protocol.LocoErrorPayload{
			Address: p.Address,
			Code:    errors.CodeCommandStationError,
			Detail:  err.Error(),
		})
		_ = sess.SendAck(ctx, requestID, false, errors.CodeCommandStationError)
		return
	}
	r.log.WithFields(logrus.Fields{
		"addr":    p.Address,
		"speed":   p.Speed,
		"forward": p.Forward,
	}).Debug("dcc-bus command station SetSpeed ok")
	snap := protocol.LocoStatePayload{
		Address:            p.Address,
		Speed:              p.Speed,
		Forward:            p.Forward,
		ControlledByUserID: sess.UserID,
		Source:             "throttle",
		At:                 time.Now().UTC().UnixMilli(),
	}
	if env, ok, err := r.redis.LoadState(ctx, p.Address); err == nil && ok {
		snap.Functions = env.Functions
	}
	if err := r.redis.StoreState(ctx, snap, stateTTL); err != nil {
		r.log.WithError(err).Debug("dcc-bus redis store")
	}
	r.broadcastLocoStateToObservers(ctx, snap)
	if requestID != "" {
		_ = sess.SendAck(ctx, requestID, true, "")
	}
}

// HandleSetFunction sets a single function on or off. The desired
// state is sent to the command station and mirrored in Redis.
func (r *Router) HandleSetFunction(ctx context.Context, sess *ws.Session, p protocol.LocoSetFunctionPayload, requestID string) {
	if reason := r.roster.DenyDriveCommand(sess.UserID, p.Address); reason != "" {
		_ = sess.SendAck(ctx, requestID, false, reason)
		return
	}
	if err := r.setLocoFunction(ctx, p.Address, sess.UserID, p.Function, p.On, "throttle"); err != nil {
		_ = sess.SendTyped(ctx, protocol.TypeLocoError, protocol.LocoErrorPayload{
			Address: p.Address,
			Code:    errors.CodeCommandStationError,
			Detail:  err.Error(),
		})
		_ = sess.SendAck(ctx, requestID, false, errors.CodeCommandStationError)
		return
	}
	if requestID != "" {
		_ = sess.SendAck(ctx, requestID, true, "")
	}
}

// seedFnCache aligns the in-memory function dedup cache with a Redis
// snapshot (or any authoritative function vector). Called on subscribe
// so a TTL-expired Redis row cannot leave fnCache believing a function
// is still on while the UI reads an empty/off snapshot.
func (r *Router) seedFnCache(addr uint16, functions []bool) {
	r.fnCacheMu.Lock()
	defer r.fnCacheMu.Unlock()
	for fn, on := range functions {
		if fn > maxDCCFunctionNum {
			break
		}
		r.fnCache[fnKey{Addr: addr, Fn: uint8(fn)}] = on
	}
}

// checkFnStateMatches reports whether addr/fn is already in the desired
// state in both fnCache and Redis. A mismatch in either layer means
// the DCC command must be issued (and the UI refreshed).
func (r *Router) checkFnStateMatches(ctx context.Context, addr uint16, fn uint8, on bool) bool {
	key := fnKey{Addr: addr, Fn: fn}
	r.fnCacheMu.Lock()
	previous, hadPrev := r.fnCache[key]
	r.fnCacheMu.Unlock()
	if !hadPrev || previous != on {
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

// setLocoFunction issues one DCC function command and mirrors the
// result in Redis. Used by the throttle and the dead-man's switch.
func (r *Router) setLocoFunction(ctx context.Context, addr uint16, userID uint, fn uint8, on bool, source string) error {
	key := fnKey{Addr: addr, Fn: fn}
	unchanged := r.checkFnStateMatches(ctx, addr, fn, on)
	r.fnCacheMu.Lock()
	previous, hadPrev := r.fnCache[key]
	if !unchanged {
		r.fnCache[key] = on
	}
	r.fnCacheMu.Unlock()

	if unchanged {
		return nil
	}
	if err := r.station.SendFn(commandstation.MainTrackMode, commandstation.LocoAddr(addr), commandstation.FuncNum(fn), on); err != nil {
		fields := r.stationLogFields()
		fields["addr"] = addr
		fields["function"] = fn
		fields["on"] = on
		r.log.WithError(err).WithFields(fields).Warn("dcc-bus command station SendFn failed")
		r.fnCacheMu.Lock()
		if hadPrev {
			r.fnCache[key] = previous
		} else {
			delete(r.fnCache, key)
		}
		r.fnCacheMu.Unlock()
		return err
	}

	snap := protocol.LocoStatePayload{
		Address:            addr,
		ControlledByUserID: userID,
		Source:             source,
		At:                 time.Now().UTC().UnixMilli(),
	}
	if env, ok, err := r.redis.LoadState(ctx, addr); err == nil && ok {
		snap.Speed = env.Speed
		snap.Forward = env.Forward
		snap.Functions = make([]bool, max(len(env.Functions), int(fn)+1))
		copy(snap.Functions, env.Functions)
	} else {
		snap.Functions = make([]bool, int(fn)+1)
	}
	snap.Functions[fn] = on
	if err := r.redis.StoreState(ctx, snap, stateTTL); err != nil {
		r.log.WithError(err).Debug("dcc-bus redis store")
	}
	r.broadcastLocoStateToObservers(ctx, snap)
	return nil
}

// HandleEStop slams every loco the requesting session subscribes to
// down to speed step 1 (DCC EMG-stop) and emits a system.estop
// audit event on the Redis bus.
func (r *Router) HandleEStop(ctx context.Context, sess *ws.Session, p protocol.SystemEStopPayload, requestID string) {
	r.applyEmergencyForSession(ctx, sess, p.Reason, false)
	if requestID != "" {
		_ = sess.SendAck(ctx, requestID, true, "")
	}
}

// HandleSessionClose runs the dead-man's plan and fires audit. Called
// from the WS server when a browser session goes away (any reason).
func (r *Router) HandleSessionClose(ctx context.Context, sess *ws.Session, reason string) {
	if r.isLastSessionForUser(sess) {
		addrs := r.collectDriveTargetsForUser(ctx, sess.UserID)
		r.log.WithFields(logrus.Fields{
			"sessionId": sess.ID,
			"userId":    sess.UserID,
			"reason":    reason,
			"addrs":     addrs,
		}).Info("dcc-bus last user session closed — emergency stop on drive targets")
		r.applyEmergencyStop(ctx, sess.UserID, sess.ID, addrs, reason, true)
		return
	}
	if reason == errors.WsCodeSessionDeadman {
		r.applyEmergencyForSession(ctx, sess, reason, true)
	}
}

// isLastSessionForUser reports whether sess is the only live WS
// session for its user on this daemon (§7e.5 per-daemon rule).
func (r *Router) isLastSessionForUser(sess *ws.Session) bool {
	for _, s := range r.hub.SessionsForUser(sess.UserID) {
		if s.ID != sess.ID {
			return false
		}
	}
	return true
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

// applyEmergencyForSession brakes every loco the session subscribed
// to. We emit one EMG-stop per address so a partial failure (the
// command station rejected one address) doesn't abort the rest.
func (r *Router) applyEmergencyForSession(ctx context.Context, sess *ws.Session, reason string, movingOnly bool) {
	r.applyEmergencyStop(ctx, sess.UserID, sess.ID, sess.SubscribedAddrs(), reason, movingOnly)
}

// applyEmergencyStop issues EMG-stop for each address and publishes
// the audit frame. Shared by per-session and per-user last-session
// paths.
//
// When movingOnly is true (dead-man's switch paths), a loco is acted
// on only when its cached speed is above 1. When movingOnly is false
// (manual estop), locos already at normal stop (speed 0) are skipped
// so a benign page navigation does not surface wire speed 1 in the UI.
func (r *Router) applyEmergencyStop(ctx context.Context, userID uint, sessionID string, addrs []uint16, reason string, movingOnly bool) {
	affected := make([]uint16, 0, len(addrs))
	for _, addr := range addrs {
		if !r.shouldEmergencyStopLoco(ctx, addr, movingOnly) {
			continue
		}
		if err := r.station.SetSpeed(commandstation.LocoAddr(addr), 1, true, uint8(r.speedSteps)); err != nil {
			r.log.WithError(err).WithField("addr", addr).Warn("dcc-bus estop failed")
			continue
		}
		affected = append(affected, addr)
		snap := protocol.LocoStatePayload{
			Address:            addr,
			Speed:              1,
			Forward:            true,
			ControlledByUserID: userID,
			Source:             "estop",
			At:                 time.Now().UTC().UnixMilli(),
		}
		if cached, ok, err := r.redis.LoadState(ctx, addr); err == nil && ok {
			snap.Functions = cached.Functions
			snap.Forward = cached.Forward
		}
		if err := r.redis.StoreState(ctx, snap, stateTTL); err != nil {
			r.log.WithError(err).Debug("dcc-bus estop redis store")
		}
		r.broadcastLocoStateToObservers(ctx, snap)
		if v, ok := r.roster.AllowedVehicle(addr); ok {
			r.applyDeadManSwitchForLoco(context.Background(), addr, userID, v)
		}
	}
	if len(affected) == 0 {
		return
	}

	if err := r.redis.Publish(ctx, "system.estop.audit", map[string]any{
		"reason":    reason,
		"userId":    userID,
		"sessionId": sessionID,
		"addrs":     affected,
		"at":        time.Now().UTC().UnixMilli(),
	}); err != nil {
		r.log.WithError(err).Debug("dcc-bus estop publish")
	}
}

func (r *Router) shouldEmergencyStopLoco(ctx context.Context, addr uint16, movingOnly bool) bool {
	cached, ok, err := r.redis.LoadState(ctx, addr)
	if err != nil || !ok {
		return !movingOnly
	}
	if movingOnly {
		return cached.Speed > 1
	}
	return cached.Speed != 0
}

func (r *Router) applyDeadManSwitchForLoco(ctx context.Context, addr uint16, userID uint, v layoutroster.AllowedVehicle) {
	switch v.DeadManSwitchOption {
	case string(domain.DeadManSwitchStopHorn):
		r.pulseLocoFunction(addr, userID, v.Rp1Function)
	case string(domain.DeadManSwitchStopHornEmergencyLights):
		r.pulseLocoFunction(addr, userID, v.Rp1Function)
		if err := r.setLocoFunction(ctx, addr, userID, v.EmergencyLightsFunction, true, "deadman"); err != nil {
			r.log.WithError(err).WithFields(logrus.Fields{
				"addr":     addr,
				"function": v.EmergencyLightsFunction,
			}).Warn("dcc-bus dead-man emergency lights failed")
		}
	default:
		// "stop" and unknown values: brake only.
	}
}

func (r *Router) pulseLocoFunction(addr uint16, userID uint, fn uint8) {
	ctx := context.Background()
	if err := r.setLocoFunction(ctx, addr, userID, fn, true, "deadman"); err != nil {
		r.log.WithError(err).WithFields(logrus.Fields{
			"addr":     addr,
			"function": fn,
		}).Warn("dcc-bus dead-man rp1 on failed")
		return
	}
	time.AfterFunc(time.Second, func() {
		if err := r.setLocoFunction(context.Background(), addr, userID, fn, false, "deadman"); err != nil {
			r.log.WithError(err).WithFields(logrus.Fields{
				"addr":     addr,
				"function": fn,
			}).Warn("dcc-bus dead-man rp1 off failed")
		}
	})
}

// broadcastLocoStateToObservers broadcasts a state snapshot to every WS session that
// subscribed to the affected loco.
func (r *Router) broadcastLocoStateToObservers(ctx context.Context, snap protocol.LocoStatePayload) {
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
	var env protocol.Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		r.log.WithError(err).Debug("dcc-bus control cmd: bad envelope")
		return
	}
	r.log.WithField("type", env.Type).Debug("dcc-bus control cmd")

	switch env.Type {
	case protocol.TypeLocoSetSpeed:
		var p protocol.LocoSetSpeedPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return
		}
		r.applyControlSetSpeed(ctx, p)

	case protocol.TypeLocoSetFunction:
		var p protocol.LocoSetFunctionPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return
		}
		r.applyControlSetFunction(ctx, p)

	case protocol.TypeSystemEStop:
		r.applyEStopAll(ctx, "system")
	}
}

func (r *Router) applyControlSetSpeed(ctx context.Context, p protocol.LocoSetSpeedPayload) {
	if !r.roster.IsLocoAllowedOnLayout(p.Address) {
		return
	}
	speed := p.Speed
	if p.Emergency {
		speed = 1
	}
	if err := r.station.SetSpeed(commandstation.LocoAddr(p.Address), speed, p.Forward, uint8(r.speedSteps)); err != nil {
		r.log.WithError(err).WithField("addr", p.Address).Warn("dcc-bus control setSpeed failed")
		return
	}
	snap := protocol.LocoStatePayload{
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

func (r *Router) applyControlSetFunction(ctx context.Context, p protocol.LocoSetFunctionPayload) {
	if !r.roster.IsLocoAllowedOnLayout(p.Address) {
		return
	}
	if err := r.station.SendFn(commandstation.MainTrackMode, commandstation.LocoAddr(p.Address), commandstation.FuncNum(p.Function), p.On); err != nil {
		r.log.WithError(err).WithField("addr", p.Address).Warn("dcc-bus control setFn failed")
		return
	}
	key := fnKey{Addr: p.Address, Fn: p.Function}
	r.fnCacheMu.Lock()
	r.fnCache[key] = p.On
	r.fnCacheMu.Unlock()
	snap := protocol.LocoStatePayload{Address: p.Address, Source: "server", At: time.Now().UTC().UnixMilli()}
	if cached, ok, err := r.redis.LoadState(ctx, p.Address); err == nil && ok {
		snap.Speed = cached.Speed
		snap.Forward = cached.Forward
		snap.ControlledByUserID = cached.ControlledByUserID
		snap.Functions = make([]bool, max(len(cached.Functions), int(p.Function)+1))
		copy(snap.Functions, cached.Functions)
	} else {
		snap.Functions = make([]bool, int(p.Function)+1)
	}
	snap.Functions[p.Function] = p.On
	if err := r.redis.StoreState(ctx, snap, stateTTL); err != nil {
		r.log.WithError(err).Debug("dcc-bus control redis store")
	}
	r.broadcastLocoStateToObservers(ctx, snap)
}

// applyEStopAll brakes every roster locomotive — the cs-scoped
// emergency stop fired by loco-server (e.g. on takeover failure).
func (r *Router) applyEStopAll(ctx context.Context, reason string) {
	addrs := r.roster.AllowedAddrs()
	for _, addr := range addrs {
		_ = r.station.SetSpeed(commandstation.LocoAddr(addr), 1, true, uint8(r.speedSteps))
		snap := protocol.LocoStatePayload{
			Address: addr,
			Speed:   0,
			Forward: true,
			Source:  "estop",
			At:      time.Now().UTC().UnixMilli(),
		}
		if cached, ok, err := r.redis.LoadState(ctx, addr); err == nil && ok {
			snap.Functions = cached.Functions
		}
		_ = r.redis.StoreState(ctx, snap, stateTTL)
		r.broadcastLocoStateToObservers(ctx, snap)
	}
	_ = r.redis.Publish(ctx, "system.estop.audit", map[string]any{
		"reason": reason,
		"scope":  "all",
		"addrs":  addrs,
		"at":     time.Now().UTC().UnixMilli(),
	})
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
