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
	LayoutID         uint
	CommandStationID uint
	StationName      string
	StationKind      domain.CommandStationKind
	StationURI       string
	SpeedSteps       uint
	PollIntervalMs   uint
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
		dcc: &service.DCCWriter{
			Station:    cfg.Station,
			SpeedSteps: cfg.SpeedSteps,
			Log:        log,
		},
		cache:       service.NewFunctionsCache(),
		trainSpeed:  service.NewTrainSpeedScheduler(),
		pulseActive: make(map[service.FnKey]bool, 8),
		locoObservers: remotes.NewLocoStateNotifier(),
	}
	r.dcc.LogFields = r.stationLogFields
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

// broadcastLocoState fans a snapshot to WS sessions and registered remotes.
func (r *Router) broadcastLocoState(ctx context.Context, snap contract.LocoStateWire) {
	service.BroadcastLocoState(ctx, r.hub, snap)
	r.locoObservers.Notify(ctx, snap)
}

// locoSnapOrDefault returns the cached Redis snapshot or a stopped default.
func (r *Router) locoSnapOrDefault(ctx context.Context, addr uint16) contract.LocoStateWire {
	if r.redis != nil {
		if snap, ok, err := r.redis.GetLocoCurrentState(ctx, addr); err == nil && ok {
			return snap
		}
	}
	return contract.LocoStateWire{Address: addr, Forward: true}
}

// RunStateFeed mirrors external throttle changes into Redis and WS clients.
func (r *Router) RunStateFeed(ctx context.Context) {
	service.RunStateFeed(ctx, service.FeedDeps{
		Station:      r.station,
		Roster:       r.roster,
		Redis:        r.redis,
		Hub:          r.hub,
		HubSubs:      r.hub,
		FnCache:       r.cache,
		LocoObservers: r.locoObservers,
		Log:           r.log,
		PollInterval: r.pollInterval,
		StateTTL:     StateTTL,
	})
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
