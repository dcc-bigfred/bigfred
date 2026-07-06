// Package cmd is the daemon's use-case layer: throttle actions, roster
// updates, Redis control commands, and the dead-man safety plan.
package cmd

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/security"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/service"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/service/station"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/slotlease"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/state"
	"github.com/keskad/loco/pkgs/bigfred/remotes"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

const (
	// StateTTL is the Redis TTL applied to loco:state:* entries.
	StateTTL = 5 * time.Minute
)

// Router orchestrates dcc-bus use cases. It is the only component that
// talks to the underlying commandstation.Station.
type Router struct {
	station          commandstation.Station
	hub              HubPort
	redis            *state.Redis
	log              *logrus.Logger
	layoutID         uint
	commandStationID uint
	stationName      string
	stationKind      domain.CommandStationKind
	stationURI       string

	speedSteps   uint
	pollInterval time.Duration

	roster      *service.RosterCache
	functions   *service.FunctionCatalogueCache
	drive       security.DrivePolicy
	trainPolicy security.TrainPolicy

	trainsMu sync.RWMutex
	trains   []contract.DefinedTrain

	dcc        *service.DCCWriter
	cache      *service.FunctionsCache
	trainSpeed *service.TrainSpeedScheduler

	pulseMu     sync.Mutex
	pulseActive map[service.FnKey]bool

	locoObservers *remotes.LocoStateNotifier

	store *state.LocoStateStore

	leaser *slotlease.Leaser

	maxVehiclesPerUser int
	slotMetrics        slotlease.Recorder

	shutdownOnce sync.Once
	bootStopMu   sync.Mutex
	bootStopDone bool
}

// Config carries the inputs Router needs at construction time.
type Config struct {
	Station          commandstation.Station
	Hub              HubPort
	Redis            *state.Redis
	Log              *logrus.Logger
	AllowedVehicles  contract.AllowedVehicles
	DefinedTrains    contract.DefinedTrains
	VehicleFunctions contract.VehicleFunctions
	LayoutID         uint
	CommandStationID uint
	StationName      string
	StationKind      domain.CommandStationKind
	StationURI       string
	SpeedSteps       uint
	PollIntervalMs   uint
	MaxVehiclesPerUser int
	MaxLoconetSlots    int // 0 = default 80 when the station supports slots
	RemoteIdleTimeout         time.Duration
	RemoteIdleTimeoutDisabled bool
	SlotMetrics               slotlease.Recorder
}

// NewRouter assembles the router and seeds roster caches from Redis.
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
		roster:           service.NewRosterCache(cfg.LayoutID),
		functions:        service.NewFunctionCatalogueCache(cfg.LayoutID),
		dcc: &service.DCCWriter{
			Station:    cfg.Station,
			SpeedSteps: cfg.SpeedSteps,
			Log:        log,
		},
		cache:         service.NewFunctionsCache(),
		trainSpeed:    service.NewTrainSpeedScheduler(),
		pulseActive:   make(map[service.FnKey]bool, 8),
		locoObservers: remotes.NewLocoStateNotifier(),
		store:         state.NewLocoStateStore(cfg.Redis, StateTTL, log),
		maxVehiclesPerUser: cfg.MaxVehiclesPerUser,
		slotMetrics:        slotlease.RecorderOrNoop(cfg.SlotMetrics),
	}
	r.dcc.LogFields = r.stationLogFields
	r.reconcileBootSlots(cfg)
	r.initLeaser(cfg)
	if cfg.AllowedVehicles.LayoutID == 0 || cfg.AllowedVehicles.LayoutID == cfg.LayoutID {
		if r.roster.ApplySnapshot(cfg.AllowedVehicles) {
			r.store.LoadMissingFromRedis(context.Background(), r.roster.AllowedAddrs())
		}
	}
	r.ApplyAllowedVehicles(context.Background(), cfg.AllowedVehicles)
	r.ApplyDefinedTrains(cfg.DefinedTrains)
	r.ApplyVehicleFunctions(cfg.VehicleFunctions)
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
// loco-server snapshot.
func (r *Router) ApplyAllowedVehicles(ctx context.Context, snap contract.AllowedVehicles) {
	if snap.LayoutID != 0 && snap.LayoutID != r.layoutID {
		return
	}
	removed := r.roster.DiffRemoved(snap)
	r.retireRemovedLocos(ctx, removed)
	if !r.roster.ApplySnapshot(snap) {
		return
	}
	r.store.LoadMissingFromRedis(ctx, r.roster.AllowedAddrs())
	addrs := make([]uint16, 0, len(snap.Vehicles))
	for _, v := range snap.Vehicles {
		addrs = append(addrs, v.Addr)
	}
	r.log.WithFields(logrus.Fields{
		"layoutId": r.layoutID,
		"addrs":    addrs,
		"count":    len(addrs),
	}).Info("dcc-bus allowed vehicles updated")
	r.ensureBootStop(ctx)
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

// ApplyVehicleFunctions replaces the layout function catalogue cache.
func (r *Router) ApplyVehicleFunctions(snap contract.VehicleFunctions) {
	if snap.LayoutID != 0 && snap.LayoutID != r.layoutID {
		return
	}
	if !r.functions.ApplySnapshot(snap) {
		return
	}
	r.log.WithFields(logrus.Fields{
		"layoutId": r.layoutID,
		"count":    len(snap.Vehicles),
	}).Info("dcc-bus vehicle functions updated")
}

// FunctionsForAddr returns resolved function metadata for one roster address.
func (r *Router) FunctionsForAddr(addr uint16) []contract.FunctionDefinition {
	if r == nil || r.functions == nil {
		return nil
	}
	return r.functions.FunctionsForAddr(addr)
}

func (r *Router) findDefinedTrain(trainID string) (contract.DefinedTrain, bool) {
	r.trainsMu.RLock()
	defer r.trainsMu.RUnlock()
	for _, t := range r.trains {
		if t.TrainID == trainID {
			return t, true
		}
	}
	return contract.DefinedTrain{}, false
}

// RegisterLocoObserver subscribes an inbound remote for locomotive state push.
func (r *Router) RegisterLocoObserver(obs remotes.LocoStateObserver) {
	if r == nil {
		return
	}
	r.locoObservers.Register(obs)
}

// AllowedVehiclesSnapshot returns the in-memory drivable roster for inbound
// protocol roster emission (e.g. WiThrottle RL lines).
func (r *Router) AllowedVehiclesSnapshot() contract.AllowedVehicles {
	if r == nil {
		return contract.AllowedVehicles{}
	}
	return r.roster.Snapshot()
}

// FunctionsSnapshot returns the in-memory function catalogue for inbound
// protocol acquire replies.
func (r *Router) FunctionsSnapshot() contract.VehicleFunctions {
	if r == nil || r.functions == nil {
		return contract.VehicleFunctions{}
	}
	return r.functions.Snapshot()
}

// broadcastLocoState fans a snapshot to WS sessions and registered remotes.
func (r *Router) broadcastLocoState(ctx context.Context, snap contract.LocoStateWire) {
	service.BroadcastLocoState(ctx, r.hub, snap)
	r.locoObservers.Notify(ctx, snap)
}

// locoSnapOrDefault returns the in-memory snapshot or a stopped default.
func (r *Router) locoSnapOrDefault(_ context.Context, addr uint16) contract.LocoStateWire {
	if r.store != nil {
		return r.store.Snapshot(addr)
	}
	return contract.LocoStateWire{Address: addr, Forward: true}
}

// RunStoreFlush mirrors dirty loco state to Redis on a fixed tick.
func (r *Router) RunStoreFlush(ctx context.Context) {
	if r.store != nil {
		r.store.FlushLoop(ctx, 100*time.Millisecond)
	}
}

// RunStateFeed mirrors external throttle changes into WS clients.
func (r *Router) RunStateFeed(ctx context.Context) {
	service.RunStateFeed(ctx, service.FeedDeps{
		Station:       r.station,
		Roster:        r.roster,
		Store:         r.store,
		Hub:           r.hub,
		HubSubs:       r.hub,
		FnCache:       r.cache,
		LocoObservers: r.locoObservers,
		Log:           r.log,
		PollInterval:  r.pollInterval,
		StateTTL:      StateTTL,
	})
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (r *Router) initLeaser(cfg Config) {
	var slotSta slotlease.SlotStation
	maxSlots := 0
	if sm, ok := station.AsSlotManager(cfg.Station); ok {
		slotSta = sm
		maxSlots = cfg.MaxLoconetSlots
		if maxSlots <= 0 {
			maxSlots = 80
		}
	}
	r.leaser = slotlease.New(
		slotSta,
		r.dcc,
		r.store,
		leaseHub{hub: r.hub},
		r.leaseDriveGate,
		slotlease.Config{
			MaxPerUser:          cfg.MaxVehiclesPerUser,
			MaxSlots:            maxSlots,
			IdleTimeout:         cfg.RemoteIdleTimeout,
			IdleTimeoutDisabled: cfg.RemoteIdleTimeoutDisabled,
			Metrics:             cfg.SlotMetrics,
		},
	)
	// Attach the leaser as a slot observer so the driver reports IN_USE /
	// release events for slots it acquires (and master-purges). This keeps the
	// leaser's lease table in sync with the physical slot table even when a
	// drive arrives via SetSpeed without an explicit loco.select.
	if obs, ok := station.AsSlotObservable(cfg.Station); ok {
		obs.SetSlotObserver(r.leaser)
	}
}

// SlotLeaser exposes the slot leaser for admin diagnostics.
func (r *Router) SlotLeaser() *slotlease.Leaser {
	if r == nil {
		return nil
	}
	return r.leaser
}

func (r *Router) reconcileBootSlots(cfg Config) {
	reconciler, ok := station.AsBootSlotReconciler(cfg.Station)
	if !ok {
		return
	}
	roster := make(map[commandstation.LocoAddr]struct{}, len(cfg.AllowedVehicles.Vehicles))
	for _, v := range cfg.AllowedVehicles.Vehicles {
		roster[commandstation.LocoAddr(v.Addr)] = struct{}{}
	}
	if len(roster) == 0 {
		return
	}
	if err := reconciler.ReconcileBootSlots(roster); err != nil {
		r.log.WithError(err).Warn("dcc-bus boot slot reconciliation failed")
		return
	}
	r.log.WithField("rosterAddrs", len(roster)).Info("dcc-bus boot slot reconciliation complete")
}

// RunIdleSweep periodically releases remote-only leases past idleTimeout.
func (r *Router) RunIdleSweep(ctx context.Context) {
	if r == nil || r.leaser == nil {
		return
	}
	interval := 15 * time.Second
	if idle := r.leaser.IdleTimeout(); idle > 0 {
		interval = idle / 4
		if interval > 15*time.Second {
			interval = 15 * time.Second
		}
		if interval < time.Second {
			interval = time.Second
		}
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			r.leaser.SweepIdle(now)
			r.leaser.SweepDeferred(now)
		}
	}
}

// RunReleaseWorker drains the leaser's background slot-release queue so
// latency-sensitive Reserve calls never block on a bus release.
func (r *Router) RunReleaseWorker(ctx context.Context) {
	if r == nil || r.leaser == nil {
		return
	}
	r.leaser.RunReleaseWorker(ctx)
}

func (r *Router) leaseDriveGate(userID uint, addr uint16) error {
	vehicle, onLayout := r.roster.AllowedVehicle(addr)
	if d := r.drive.CanDrive(userID, vehicle, onLayout); !d.Allowed {
		return slotlease.ErrNotAllowed
	}
	return nil
}

type leaseHub struct {
	hub HubPort
}

func (h leaseHub) BroadcastLocoState(ctx context.Context, snap contract.LocoStateWire) {
	service.BroadcastLocoState(ctx, h.hub, snap)
}
