// Package dccbus is the entry point of the `dcc-bus` daemon. It
// wires Redis, the command-station driver, command router and
// WebSocket server into a single Daemon.Run loop that the cobra
// subcommand drives.
package dccbus

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/metric"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/auth"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/cmd"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/service/station"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/slotlease"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/state"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/withrottle"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/ws"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/z21server"
	bfotel "github.com/keskad/loco/pkgs/bigfred/otel"
	"github.com/keskad/loco/pkgs/bigfred/remotepairing"
	"github.com/keskad/loco/pkgs/bigfred/remotes"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
)

// Config carries every runtime input the daemon needs. Populated
// from the cobra subcommand's flags.
type Config struct {
	LayoutID         uint
	CommandStationID uint

	BindAddr string
	Port     uint16

	// CommandStation carries connection parameters passed from loco-server via CLI.
	CommandStation domain.CommandStation

	RedisAddr string

	JWTSecret []byte

	AllowedOrigins []string

	HeartbeatSecs float64
	DeadmanSecs   float64

	// PollIntervalMs is the cadence at which the daemon refreshes
	// state from the command station. 0 disables polling (events
	// only land via WS / control-cmd).
	PollIntervalMs uint

	// EnableTelemetry turns on the command-station latency histogram
	// wrapper when OTLPEndpoint is also set.
	EnableTelemetry bool
	// OTLPEndpoint is the gRPC host:port for metric export (Alloy).
	// Telemetry stays off when this is empty.
	OTLPEndpoint string

	// EnableZ21 starts the inbound Z21 LAN UDP server for physical handsets.
	EnableZ21 bool
	Z21Bind   string
	Z21Port   uint16
	// Z21IPStickiness keys handset sessions by client IP only.
	Z21IPStickiness bool

	// EnableWithrottle starts the inbound WiThrottle TCP server.
	EnableWithrottle        bool
	WithrottleBind          string
	WithrottlePort          uint16
	WithrottlePairingAddr   uint16
	WithrottleHeartbeatSecs float64

	MaxVehiclesPerUser int
	MaxLoconetSlots    int
	IdleTimeoutSecs    uint
}

// Daemon is the assembled dcc-bus instance.
type Daemon struct {
	cfg             Config
	log             *logrus.Logger

	redis           *state.Redis
	rds             *redis.Client
	srv             *http.Server
	router          *cmd.Router
	metricsShutdown func(context.Context) error
	lnMetricsReg    metric.Registration
	z21MetricsReg   metric.Registration
	slotMetricsReg  metric.Registration
	withrottleSrv   *withrottle.Server
	gatewayWg       sync.WaitGroup
}

// New validates cfg, opens Redis + the command station driver and
// returns a ready-to-Run daemon. The caller MUST call Close to
// release resources.
func New(ctx context.Context, log *logrus.Logger, cfg Config) (*Daemon, error) {
	if log == nil {
		log = logrus.New()
	}
	if cfg.LayoutID == 0 || cfg.CommandStationID == 0 {
		return nil, errors.New("dcc-bus: --layout-id and --command-station-id are required")
	}
	if cfg.CommandStation.ID == 0 {
		cfg.CommandStation.ID = cfg.CommandStationID
	}
	if cfg.CommandStation.ID != cfg.CommandStationID {
		return nil, errors.New("dcc-bus: --command-station-id does not match station config")
	}
	if !cfg.CommandStation.Kind.IsValid() || cfg.CommandStation.ConnectionURI == "" {
		return nil, errors.New("dcc-bus: station connection flags are required (--station-kind, --station-uri, …)")
	}
	if cfg.Port == 0 {
		return nil, errors.New("dcc-bus: --port is required")
	}
	if len(cfg.JWTSecret) == 0 {
		return nil, errors.New("dcc-bus: --jwt-secret is required")
	}
	if cfg.RedisAddr == "" {
		cfg.RedisAddr = "127.0.0.1:6379"
	}
	if cfg.BindAddr == "" {
		cfg.BindAddr = "127.0.0.1"
	}
	if cfg.HeartbeatSecs <= 0 {
		cfg.HeartbeatSecs = 2
	}
	if cfg.DeadmanSecs <= 0 {
		cfg.DeadmanSecs = 6
	}

	cs := cfg.CommandStation

	rds := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	if err := rds.Ping(ctx).Err(); err != nil {
		_ = rds.Close()
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	red := state.NewRedis(rds, cfg.LayoutID, cfg.CommandStationID)

	allowedSnap := contract.AllowedVehicles{LayoutID: cfg.LayoutID}
	if snap, ok, err := red.LoadAllowedVehicles(ctx); err != nil {
		_ = rds.Close()
		return nil, fmt.Errorf("load allowed vehicles: %w", err)
	} else if ok {
		allowedSnap = snap
	}
	trainSnap := contract.DefinedTrains{LayoutID: cfg.LayoutID}
	if snap, ok, err := red.LoadDefinedTrains(ctx); err != nil {
		_ = rds.Close()
		return nil, fmt.Errorf("load defined trains: %w", err)
	} else if ok {
		trainSnap = snap
	}
	fnSnap := contract.VehicleFunctions{LayoutID: cfg.LayoutID}
	if snap, ok, err := red.LoadVehicleFunctions(ctx); err != nil {
		_ = rds.Close()
		return nil, fmt.Errorf("load vehicle functions: %w", err)
	} else if ok {
		fnSnap = snap
	}

	log.WithFields(logrus.Fields{
		"commandStationId": cs.ID,
		"name":             cs.Name,
		"kind":             cs.Kind,
		"connection":       station.Describe(cs),
		"speedSteps":       cs.SpeedSteps,
	}).Info("dcc-bus opening command station driver")

	st, err := station.Open(cs)
	if err != nil {
		log.WithError(err).WithFields(logrus.Fields{
			"commandStationId": cs.ID,
			"connection":       station.Describe(cs),
		}).Error("dcc-bus command station driver open failed")
		_ = rds.Close()
		return nil, fmt.Errorf("open command station: %w", err)
	}

	var metricsShutdown func(context.Context) error
	if cfg.EnableTelemetry && cfg.OTLPEndpoint != "" {
		var shutdownErr error
		metricsShutdown, shutdownErr = bfotel.InitMetrics(ctx, "dcc-bus", cfg.OTLPEndpoint)
		if shutdownErr != nil {
			_ = st.CleanUp()
			_ = rds.Close()
			return nil, fmt.Errorf("init station metrics export: %w", shutdownErr)
		}
		log.WithField("endpoint", cfg.OTLPEndpoint).Info("dcc-bus station metrics enabled")

		st, err = station.Wrap(st, station.InstrumentConfig{
			Enabled:          true,
			LayoutID:         cfg.LayoutID,
			CommandStationID: cfg.CommandStationID,
			Kind:             cs.Kind,
			SpeedSteps:       cs.SpeedSteps,
		})
		if err != nil {
			if metricsShutdown != nil {
				_ = metricsShutdown(context.Background())
			}
			_ = st.CleanUp()
			_ = rds.Close()
			return nil, fmt.Errorf("wrap command station metrics: %w", err)
		}
	} else if cfg.EnableTelemetry {
		log.Debug("dcc-bus station metrics disabled: no OTLP endpoint configured")
	}

	log.WithFields(logrus.Fields{
		"commandStationId": cs.ID,
		"connection":       station.Describe(cs),
	}).Info("dcc-bus command station driver ready")

	// Register driver metrics (RX/TX, per-message-type, queues, saturation).
	// Observable instruments read a snapshot at each OTLP export; no extra
	// goroutine. Only wired when the driver exposes counters and telemetry is on.
	var lnMetricsReg metric.Registration
	var z21MetricsReg metric.Registration
	if cfg.EnableTelemetry && cfg.OTLPEndpoint != "" {
		if src, ok := station.AsMetricsSource(st); ok {
			reg, regErr := station.StartLocoNetMetrics(src, station.LocoNetMetricsConfig{
				LayoutID:         cfg.LayoutID,
				CommandStationID: cfg.CommandStationID,
				Kind:             cs.Kind,
			})
			if regErr != nil {
				log.WithError(regErr).Warn("dcc-bus loconet metrics registration failed")
			} else {
				lnMetricsReg = reg
				log.Info("dcc-bus loconet driver metrics enabled")
			}
		}
		if src, ok := station.AsZ21MetricsSource(st); ok {
			reg, regErr := station.StartZ21Metrics(src, station.Z21MetricsConfig{
				LayoutID:         cfg.LayoutID,
				CommandStationID: cfg.CommandStationID,
				Kind:             cs.Kind,
			})
			if regErr != nil {
				log.WithError(regErr).Warn("dcc-bus z21 metrics registration failed")
			} else {
				z21MetricsReg = reg
				log.Info("dcc-bus z21 driver metrics enabled")
			}
		}
	}

	hub := ws.NewHub()

	slotMetrics, slotMetricsErr := slotlease.NewMetrics(slotlease.MetricsConfig{
		Enabled:          cfg.EnableTelemetry && cfg.OTLPEndpoint != "",
		LayoutID:         cfg.LayoutID,
		CommandStationID: cfg.CommandStationID,
	})
	if slotMetricsErr != nil {
		if metricsShutdown != nil {
			_ = metricsShutdown(context.Background())
		}
		_ = st.CleanUp()
		_ = rds.Close()
		return nil, fmt.Errorf("init slot metrics: %w", slotMetricsErr)
	}
	if cfg.EnableTelemetry && cfg.OTLPEndpoint != "" {
		log.Info("dcc-bus slot lease metrics enabled")
	}

	var remoteIdle time.Duration
	remoteIdleDisabled := cfg.IdleTimeoutSecs == 0
	if !remoteIdleDisabled {
		remoteIdle = time.Duration(cfg.IdleTimeoutSecs) * time.Second
	}

	router, err := cmd.NewRouter(ctx, cmd.Config{
		Station:          st,
		Hub:              ws.HubPort(hub),
		Redis:            red,
		Log:              log,
		LayoutID:         cfg.LayoutID,
		CommandStationID: cfg.CommandStationID,
		StationName:      cs.Name,
		StationKind:      cs.Kind,
		StationURI:       cs.ConnectionURI,
		SpeedSteps:       cs.SpeedSteps,
		PollIntervalMs:   cfg.PollIntervalMs,
		AllowedVehicles:  allowedSnap,
		DefinedTrains:    trainSnap,
		VehicleFunctions: fnSnap,
		MaxVehiclesPerUser:        cfg.MaxVehiclesPerUser,
		MaxLoconetSlots:           cfg.MaxLoconetSlots,
		RemoteIdleTimeout:         remoteIdle,
		RemoteIdleTimeoutDisabled: remoteIdleDisabled,
		SlotMetrics:               slotMetrics,
	})
	if err != nil {
		_ = st.CleanUp()
		_ = rds.Close()
		return nil, fmt.Errorf("build router: %w", err)
	}

	var slotMetricsReg metric.Registration
	if router.SlotLeaser() != nil {
		reg, regErr := slotMetrics.RegisterGauges(router.SlotLeaser())
		if regErr != nil {
			log.WithError(regErr).Warn("dcc-bus slot gauge registration failed")
		} else {
			slotMetricsReg = reg
		}
	}

	verifier := auth.NewVerifier(cfg.JWTSecret, cfg.LayoutID)

	var wsMetrics *ws.Metrics
	if cfg.EnableTelemetry && cfg.OTLPEndpoint != "" {
		var metricsErr error
		wsMetrics, metricsErr = ws.NewMetrics(ws.MetricsConfig{
			Enabled:          true,
			LayoutID:         cfg.LayoutID,
			CommandStationID: cfg.CommandStationID,
		})
		if metricsErr != nil {
			if metricsShutdown != nil {
				_ = metricsShutdown(context.Background())
			}
			_ = st.CleanUp()
			_ = rds.Close()
			return nil, fmt.Errorf("init ws metrics: %w", metricsErr)
		}
		log.Info("dcc-bus ws metrics enabled")
	}

	wsSrv := ws.NewServer(ws.ServerConfig{
		Verifier:       verifier,
		Hub:            hub,
		Router:         ws.NewRouterAdapter(router),
		Log:            log,
		LayoutID:       cfg.LayoutID,
		CommandStation: cfg.CommandStationID,
		SpeedSteps:     cs.SpeedSteps,
		HeartbeatSecs:  cfg.HeartbeatSecs,
		DeadmanSecs:    cfg.DeadmanSecs,
		AllowedOrigins: cfg.AllowedOrigins,
		Metrics:        wsMetrics,
		SlotsDiag: ws.NewSlotsDiagHandler(ws.SlotsDiagConfig{
			Leaser:         router.SlotLeaser(),
			Metrics:        slotMetrics,
			Log:            log,
			AllowedOrigins: cfg.AllowedOrigins,
			Verifier:       verifier,
		}),
	})

	srv := &http.Server{
		Addr:              net.JoinHostPort(cfg.BindAddr, strconv.Itoa(int(cfg.Port))),
		Handler:           wsSrv,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return &Daemon{
		cfg:             cfg,
		log:             log,
		redis:           red,
		rds:             rds,
		srv:             srv,
		router:          router,
		metricsShutdown: metricsShutdown,
		lnMetricsReg:    lnMetricsReg,
		z21MetricsReg:   z21MetricsReg,
		slotMetricsReg:  slotMetricsReg,
	}, nil
}

// Run starts the WS listener and the Redis subscribers. It blocks
// until ctx is cancelled or one of the inner loops returns a fatal
// error.
func (d *Daemon) Run(ctx context.Context) error {
	d.log.WithFields(logrus.Fields{
		"layoutId":         d.cfg.LayoutID,
		"commandStationId": d.cfg.CommandStationID,
		"bind":             d.srv.Addr,
		"redis":            d.cfg.RedisAddr,
	}).Info("dcc-bus starting")

	cmdSub, err := d.redis.SubscribeCommands(ctx)
	if err != nil {
		return fmt.Errorf("subscribe command channel: %w", err)
	}
	defer cmdSub.Close()

	vehSub, err := d.redis.SubscribeAllowedVehicles(ctx)
	if err != nil {
		return fmt.Errorf("subscribe allowed_vehicles channel: %w", err)
	}
	defer vehSub.Close()

	trainSub, err := d.redis.SubscribeDefinedTrains(ctx)
	if err != nil {
		return fmt.Errorf("subscribe defined_trains channel: %w", err)
	}
	defer trainSub.Close()

	fnSub, err := d.redis.SubscribeVehicleFunctions(ctx)
	if err != nil {
		return fmt.Errorf("subscribe vehicle_functions channel: %w", err)
	}
	defer fnSub.Close()

	radioStopSub, err := d.redis.SubscribeLayoutRadioStop(ctx)
	if err != nil {
		return fmt.Errorf("subscribe radio_stop channel: %w", err)
	}
	defer radioStopSub.Close()

	go d.runCommandConsumer(ctx, cmdSub)
	go d.runAllowedVehiclesConsumer(ctx, vehSub)
	go d.runDefinedTrainsConsumer(ctx, trainSub)
	go d.runVehicleFunctionsConsumer(ctx, fnSub)
	go d.runRadioStopConsumer(ctx, radioStopSub)

	if err := d.startRemoteGateways(ctx); err != nil {
		return err
	}

	go d.router.RunStoreFlush(ctx)
	go d.router.RunStateFeed(ctx)
	go d.router.RunIdleSweep(ctx)
	go d.router.RunReleaseWorker(ctx)

	serveErr := make(chan error, 1)
	go func() {
		if err := d.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
		close(serveErr)
	}()

	if err := d.redis.Publish(ctx, "daemon.started", map[string]any{
		"layoutId":         d.cfg.LayoutID,
		"commandStationId": d.cfg.CommandStationID,
		"port":             d.cfg.Port,
		"pid":              os.Getpid(),
		"at":               time.Now().UTC().UnixMilli(),
	}); err != nil {
		d.log.WithError(err).Debug("dcc-bus publish daemon.started")
	}

	select {
	case <-ctx.Done():
	case err := <-serveErr:
		if err != nil {
			return err
		}
	}

	// Stop inbound handset servers before emergency-stop so drive commands
	// cannot race with station cleanup.
	d.gatewayWg.Wait()

	if d.router != nil {
		d.router.Shutdown()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return d.srv.Shutdown(shutdownCtx)
}

func (d *Daemon) runCommandConsumer(ctx context.Context, sub *redis.PubSub) {
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			d.router.HandleControlCommand(ctx, []byte(msg.Payload))
		}
	}
}

func (d *Daemon) runAllowedVehiclesConsumer(ctx context.Context, sub *redis.PubSub) {
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			snap, err := contract.UnmarshalAllowedVehicles([]byte(msg.Payload))
			if err != nil {
				d.log.WithError(err).Warn("dcc-bus allowed_vehicles: bad payload")
				continue
			}
			d.router.ApplyAllowedVehicles(ctx, snap)
			if d.withrottleSrv != nil {
				d.withrottleSrv.UpdateAllowedVehicles(snap)
			}
		}
	}
}

func (d *Daemon) startRemoteGateways(ctx context.Context) error {
	if !d.cfg.EnableZ21 && !d.cfg.EnableWithrottle {
		return nil
	}
	pairStore := remotepairing.NewStore(d.rds)
	coordinator := remotes.NewCoordinator(remotes.CoordinatorConfig{
		LayoutID:         d.cfg.LayoutID,
		CommandStationID: d.cfg.CommandStationID,
		Store:            pairStore,
		Drive:            d.router,
		Publisher:        d.redis,
		Log:              d.log,
	})
	coordinator.RegisterOnEvict(func(key string) {
		d.router.ReleaseHandsetSession(remotes.HandsetSessionID(key))
	})
	type gatewayRunner struct {
		name   string
		reason string
		gw     remotes.RemoteProtocol
	}
	var runners []gatewayRunner
	discGate := d.newDiscoveryGate(ctx)

	if d.cfg.EnableZ21 {
		remotes.RegisterGatewayFactory(z21server.GatewayName, z21server.NewGateway)
		coordinator.RegisterPolicy(contract.RemoteProtocolZ21, remotes.ProtocolPolicy{
			IdleEvict:       z21server.IdleEvictAfter * time.Second,
			StickyIdleEvict: z21server.StickySessionIdleEvictAfter * time.Second,
			IPStickiness:    d.cfg.Z21IPStickiness,
		})
		gw, err := remotes.NewGateway(ctx, z21server.GatewayName, remotes.GatewayConfig{
			LayoutID:         d.cfg.LayoutID,
			CommandStationID: d.cfg.CommandStationID,
			Coordinator:      coordinator,
			Store:            pairStore,
			Drive:            d.router,
			Log:              d.log,
			Extra: map[string]any{
				"bind":         d.cfg.Z21Bind,
				"port":         d.cfg.Z21Port,
				"ipStickiness": d.cfg.Z21IPStickiness,
				"speedSteps":   d.cfg.CommandStation.EffectiveSpeedSteps(),
				"onListening":  d.protocolListeningCallback(discGate, contract.RemoteProtocolZ21),
			},
		})
		if err != nil {
			return fmt.Errorf("z21 gateway: %w", err)
		}
		d.router.RegisterLocoObserver(gw)
		runners = append(runners, gatewayRunner{name: "z21", reason: "z21_bind_failed", gw: gw})
	}

	if d.cfg.EnableWithrottle {
		heartbeatSecs := d.cfg.WithrottleHeartbeatSecs
		if heartbeatSecs <= 0 {
			heartbeatSecs = contract.DefaultWithrottleHeartbeatSecs
		}
		remotes.RegisterGatewayFactory(withrottle.GatewayName, withrottle.NewGateway)
		coordinator.RegisterPolicy(contract.RemoteProtocolWithrottle, remotes.ProtocolPolicy{
			IdleEvict:        withrottle.IdleEvictAfter * time.Second,
			HeartbeatTimeout: withrottle.HeartbeatTimeout(heartbeatSecs),
		})
		gw, err := remotes.NewGateway(ctx, withrottle.GatewayName, remotes.GatewayConfig{
			LayoutID:         d.cfg.LayoutID,
			CommandStationID: d.cfg.CommandStationID,
			Coordinator:      coordinator,
			Store:            pairStore,
			Drive:            d.router,
			Log:              d.log,
			Extra: map[string]any{
				"bind":             d.cfg.WithrottleBind,
				"port":             d.cfg.WithrottlePort,
				"pairingAddr":      d.cfg.WithrottlePairingAddr,
				"heartbeatSecs":    heartbeatSecs,
				"speedSteps":       d.cfg.CommandStation.EffectiveSpeedSteps(),
				"allowedVehicles":  d.router.AllowedVehiclesSnapshot(),
				"vehicleFunctions": d.router.FunctionsSnapshot(),
				"onListening":      d.protocolListeningCallback(discGate, contract.RemoteProtocolWithrottle),
			},
		})
		if err != nil {
			return fmt.Errorf("withrottle gateway: %w", err)
		}
		if srv, ok := gw.(*withrottle.Server); ok {
			d.withrottleSrv = srv
		}
		d.router.RegisterLocoObserver(gw)
		runners = append(runners, gatewayRunner{name: "withrottle", reason: "withrottle_bind_failed", gw: gw})
	}

	go coordinator.Run(ctx)
	for _, r := range runners {
		run := r
		d.gatewayWg.Add(1)
		go func() {
			defer d.gatewayWg.Done()
			if err := run.gw.Run(ctx); err != nil && ctx.Err() == nil {
				d.log.WithError(err).Errorf("%s inbound server stopped", run.name)
				_ = d.redis.Publish(ctx, "daemon.degraded", map[string]any{
					"layoutId":         d.cfg.LayoutID,
					"commandStationId": d.cfg.CommandStationID,
					"reason":           run.reason,
					"error":            err.Error(),
					"at":               time.Now().UTC().UnixMilli(),
				})
			}
		}()
	}
	return nil
}

func (d *Daemon) runDefinedTrainsConsumer(ctx context.Context, sub *redis.PubSub) {
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			snap, err := contract.UnmarshalDefinedTrains([]byte(msg.Payload))
			if err != nil {
				d.log.WithError(err).Warn("dcc-bus defined_trains: bad payload")
				continue
			}
			d.router.ApplyDefinedTrains(snap)
		}
	}
}

func (d *Daemon) runVehicleFunctionsConsumer(ctx context.Context, sub *redis.PubSub) {
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			snap, err := contract.UnmarshalVehicleFunctions([]byte(msg.Payload))
			if err != nil {
				d.log.WithError(err).Warn("dcc-bus vehicle_functions: bad payload")
				continue
			}
			d.router.ApplyVehicleFunctions(snap)
			if d.withrottleSrv != nil {
				d.withrottleSrv.UpdateVehicleFunctions(snap)
			}
		}
	}
}

func (d *Daemon) runRadioStopConsumer(ctx context.Context, sub *redis.PubSub) {
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			d.router.HandleLayoutRadioStopMessage(ctx, []byte(msg.Payload))
		}
	}
}

// Close releases every dependency the daemon opened. Idempotent.
// Shutdown order: HTTP server → emergency stop + station cleanup → Redis → metrics.
func (d *Daemon) Close() error {
	if d.srv != nil {
		_ = d.srv.Close()
	}
	// Let the clean up happen - release all assigned slots for example
	if d.router != nil {
		d.router.Shutdown()
	}
	if d.lnMetricsReg != nil {
		_ = d.lnMetricsReg.Unregister()
	}
	if d.z21MetricsReg != nil {
		_ = d.z21MetricsReg.Unregister()
	}
	if d.slotMetricsReg != nil {
		_ = d.slotMetricsReg.Unregister()
	}
	if d.rds != nil {
		_ = d.rds.Close()
	}
	if d.metricsShutdown != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = d.metricsShutdown(ctx)
		cancel()
	}
	return nil
}
