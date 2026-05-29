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

	"github.com/keskad/loco/pkgs/dcc-bus/protocol"
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

	speedSteps uint

	allowedMu sync.RWMutex
	allowed   map[uint16]struct{} // drivable DCC addresses on the layout
	byAddr    map[uint16]layoutroster.AllowedVehicle
	trains    []layoutroster.DefinedTrain

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
		allowed:          make(map[uint16]struct{}, 16),
		byAddr:           make(map[uint16]layoutroster.AllowedVehicle, 16),
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
		"rosterAddrs": len(r.allowed),
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
	if snap.LayoutID != 0 && snap.LayoutID != r.layoutID {
		return
	}
	allowed := make(map[uint16]struct{}, len(snap.Vehicles))
	byAddr := make(map[uint16]layoutroster.AllowedVehicle, len(snap.Vehicles))
	addrs := make([]uint16, 0, len(snap.Vehicles))
	for _, v := range snap.Vehicles {
		allowed[v.Addr] = struct{}{}
		byAddr[v.Addr] = v
		addrs = append(addrs, v.Addr)
	}
	r.allowedMu.Lock()
	r.allowed = allowed
	r.byAddr = byAddr
	r.allowedMu.Unlock()
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
	r.allowedMu.Lock()
	r.trains = append([]layoutroster.DefinedTrain(nil), snap.Trains...)
	count := len(r.trains)
	r.allowedMu.Unlock()
	r.log.WithFields(logrus.Fields{
		"layoutId": r.layoutID,
		"count":    count,
	}).Info("dcc-bus defined trains updated")
}

func (r *Router) isLocoAllowedOnLayout(addr uint16) bool {
	r.allowedMu.RLock()
	defer r.allowedMu.RUnlock()
	_, ok := r.allowed[addr]
	return ok
}

// userCanControlLoco reports whether userID may drive the locomotive at
// addr, using controllerUserIds from the allowed_vehicles snapshot.
func (r *Router) userCanControlLoco(userID uint, addr uint16) bool {
	r.allowedMu.RLock()
	v, ok := r.byAddr[addr]
	r.allowedMu.RUnlock()
	if !ok {
		return false
	}
	for _, id := range v.ControllerUserIDs {
		if id == userID {
			return true
		}
	}
	return false
}

// denyDriveCommand returns a non-empty ack reason when the user may
// not issue drive commands for addr (roster membership or control rights).
func (r *Router) denyDriveCommand(userID uint, addr uint16) string {
	if !r.isLocoAllowedOnLayout(addr) {
		return "vehicle_not_on_layout"
	}
	if !r.userCanControlLoco(userID, addr) {
		return "not_authorized"
	}
	return ""
}

// HandleSubscribe accepts a subscription request and immediately
// emits a state snapshot for each accepted address (or whatever is
// currently cached in Redis).
func (r *Router) HandleSubscribe(ctx context.Context, sess *ws.Session, payload protocol.LocoSubscribePayload, requestID string) {
	accepted := make([]uint16, 0, len(payload.Addresses))
	rejected := make([]uint16, 0)
	for _, addr := range payload.Addresses {
		if !r.isLocoAllowedOnLayout(addr) {
			rejected = append(rejected, addr)
			_ = sess.SendTyped(ctx, protocol.TypeLocoError, protocol.LocoErrorPayload{
				Address: addr,
				Code:    "vehicle_not_on_layout",
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
	if reason := r.denyDriveCommand(sess.UserID, p.Address); reason != "" {
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
			Code:    "command_station_error",
			Detail:  err.Error(),
		})
		_ = sess.SendAck(ctx, requestID, false, "command_station_error")
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
	r.fanState(ctx, snap)
	if requestID != "" {
		_ = sess.SendAck(ctx, requestID, true, "")
	}
}

// HandleSetFunction toggles a single function bit on the loco. The
// function bit is mirrored into the Redis cache so a re-connecting
// client sees the right state immediately.
func (r *Router) HandleSetFunction(ctx context.Context, sess *ws.Session, p protocol.LocoSetFunctionPayload, requestID string) {
	if reason := r.denyDriveCommand(sess.UserID, p.Address); reason != "" {
		_ = sess.SendAck(ctx, requestID, false, reason)
		return
	}
	key := fnKey{Addr: p.Address, Fn: p.Function}
	r.fnCacheMu.Lock()
	previous, hadPrev := r.fnCache[key]
	r.fnCache[key] = p.On
	r.fnCacheMu.Unlock()

	toggle := !hadPrev || previous != p.On
	if err := r.station.SendFn(commandstation.MainTrackMode, commandstation.LocoAddr(p.Address), commandstation.FuncNum(p.Function), toggle); err != nil {
		fields := r.stationLogFields()
		fields["addr"] = p.Address
		fields["function"] = p.Function
		fields["on"] = p.On
		r.log.WithError(err).WithFields(fields).Warn("dcc-bus command station SendFn failed")
		_ = sess.SendTyped(ctx, protocol.TypeLocoError, protocol.LocoErrorPayload{
			Address: p.Address,
			Code:    "command_station_error",
			Detail:  err.Error(),
		})
		_ = sess.SendAck(ctx, requestID, false, "command_station_error")
		return
	}

	snap := protocol.LocoStatePayload{
		Address:            p.Address,
		ControlledByUserID: sess.UserID,
		Source:             "throttle",
		At:                 time.Now().UTC().UnixMilli(),
	}
	if env, ok, err := r.redis.LoadState(ctx, p.Address); err == nil && ok {
		snap.Speed = env.Speed
		snap.Forward = env.Forward
		snap.Functions = make([]bool, max(len(env.Functions), int(p.Function)+1))
		copy(snap.Functions, env.Functions)
	} else {
		snap.Functions = make([]bool, int(p.Function)+1)
	}
	snap.Functions[p.Function] = p.On
	if err := r.redis.StoreState(ctx, snap, stateTTL); err != nil {
		r.log.WithError(err).Debug("dcc-bus redis store")
	}
	r.fanState(ctx, snap)
	if requestID != "" {
		_ = sess.SendAck(ctx, requestID, true, "")
	}
}

// HandleEStop slams every loco the requesting session subscribes to
// down to speed step 1 (DCC EMG-stop) and emits a system.estop
// audit event on the Redis bus.
func (r *Router) HandleEStop(ctx context.Context, sess *ws.Session, p protocol.SystemEStopPayload, requestID string) {
	r.applyEmergencyForSession(ctx, sess, p.Reason)
	if requestID != "" {
		_ = sess.SendAck(ctx, requestID, true, "")
	}
}

// HandleSessionClose runs the dead-man's plan and fires audit. Called
// exactly once per session by the WS server.
func (r *Router) HandleSessionClose(ctx context.Context, sess *ws.Session, reason string) {
	if reason == "deadman" {
		r.applyEmergencyForSession(ctx, sess, "deadman")
	}
}

// applyEmergencyForSession brakes every loco the session subscribed
// to. We emit one EMG-stop per address so a partial failure (the
// command station rejected one address) doesn't abort the rest.
func (r *Router) applyEmergencyForSession(ctx context.Context, sess *ws.Session, reason string) {
	addrs := sess.SubscribedAddrs()
	for _, addr := range addrs {
		if err := r.station.SetSpeed(commandstation.LocoAddr(addr), 1, true, uint8(r.speedSteps)); err != nil {
			r.log.WithError(err).WithField("addr", addr).Warn("dcc-bus estop failed")
			continue
		}
		snap := protocol.LocoStatePayload{
			Address:            addr,
			Speed:              0,
			Forward:            true,
			ControlledByUserID: sess.UserID,
			Source:             "estop",
			At:                 time.Now().UTC().UnixMilli(),
		}
		if cached, ok, err := r.redis.LoadState(ctx, addr); err == nil && ok {
			snap.Functions = cached.Functions
		}
		if err := r.redis.StoreState(ctx, snap, stateTTL); err != nil {
			r.log.WithError(err).Debug("dcc-bus estop redis store")
		}
		r.fanState(ctx, snap)
	}

	if err := r.redis.Publish(ctx, "system.estop.audit", map[string]any{
		"reason":    reason,
		"userId":    sess.UserID,
		"sessionId": sess.ID,
		"addrs":     addrs,
		"at":        time.Now().UTC().UnixMilli(),
	}); err != nil {
		r.log.WithError(err).Debug("dcc-bus estop publish")
	}
}

// fanState broadcasts a state snapshot to every WS session that
// subscribed to the affected loco.
func (r *Router) fanState(ctx context.Context, snap protocol.LocoStatePayload) {
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
	if !r.isLocoAllowedOnLayout(p.Address) {
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
	r.fanState(ctx, snap)
}

func (r *Router) applyControlSetFunction(ctx context.Context, p protocol.LocoSetFunctionPayload) {
	if !r.isLocoAllowedOnLayout(p.Address) {
		return
	}
	if err := r.station.SendFn(commandstation.MainTrackMode, commandstation.LocoAddr(p.Address), commandstation.FuncNum(p.Function), true); err != nil {
		r.log.WithError(err).WithField("addr", p.Address).Warn("dcc-bus control setFn failed")
		return
	}
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
	r.fanState(ctx, snap)
}

// applyEStopAll brakes every roster locomotive — the cs-scoped
// emergency stop fired by loco-server (e.g. on takeover failure).
func (r *Router) applyEStopAll(ctx context.Context, reason string) {
	r.allowedMu.RLock()
	addrs := make([]uint16, 0, len(r.allowed))
	for a := range r.allowed {
		addrs = append(addrs, a)
	}
	r.allowedMu.RUnlock()
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
		r.fanState(ctx, snap)
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
