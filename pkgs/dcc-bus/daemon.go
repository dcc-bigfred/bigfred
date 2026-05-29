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
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/dcc-bus/auth"
	"github.com/keskad/loco/pkgs/dcc-bus/cmd"
	"github.com/keskad/loco/pkgs/dcc-bus/state"
	"github.com/keskad/loco/pkgs/dcc-bus/station"
	"github.com/keskad/loco/pkgs/dcc-bus/ws"
	"github.com/keskad/loco/pkgs/layoutroster"
	"github.com/keskad/loco/pkgs/server/domain"
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
}

// Daemon is the assembled dcc-bus instance.
type Daemon struct {
	cfg Config
	log *logrus.Logger

	redis *state.Redis
	rds    *redis.Client
	srv    *http.Server
	router *cmd.Router
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
		cfg.RedisAddr = "127.0.0.1:6380"
	}
	if cfg.BindAddr == "" {
		cfg.BindAddr = "127.0.0.1"
	}
	if cfg.HeartbeatSecs <= 0 {
		cfg.HeartbeatSecs = 5
	}
	if cfg.DeadmanSecs <= 0 {
		cfg.DeadmanSecs = 12
	}

	cs := cfg.CommandStation

	rds := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	if err := rds.Ping(ctx).Err(); err != nil {
		_ = rds.Close()
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	red := state.NewRedis(rds, cfg.LayoutID, cfg.CommandStationID)

	allowedSnap := layoutroster.AllowedVehicles{LayoutID: cfg.LayoutID}
	if snap, ok, err := red.LoadAllowedVehicles(ctx); err != nil {
		_ = rds.Close()
		return nil, fmt.Errorf("load allowed vehicles: %w", err)
	} else if ok {
		allowedSnap = snap
	}
	trainSnap := layoutroster.DefinedTrains{LayoutID: cfg.LayoutID}
	if snap, ok, err := red.LoadDefinedTrains(ctx); err != nil {
		_ = rds.Close()
		return nil, fmt.Errorf("load defined trains: %w", err)
	} else if ok {
		trainSnap = snap
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
	log.WithFields(logrus.Fields{
		"commandStationId": cs.ID,
		"connection":       station.Describe(cs),
	}).Info("dcc-bus command station driver ready")

	hub := ws.NewHub()

	router, err := cmd.NewRouter(ctx, cmd.Config{
		Station:          st,
		Hub:              hub,
		Redis:            red,
		Log:              log,
		LayoutID:         cfg.LayoutID,
		CommandStationID: cfg.CommandStationID,
		StationName:      cs.Name,
		StationKind:      cs.Kind,
		StationURI:       cs.ConnectionURI,
		SpeedSteps:       cs.SpeedSteps,
		AllowedVehicles:  allowedSnap,
		DefinedTrains:    trainSnap,
	})
	if err != nil {
		_ = st.CleanUp()
		_ = rds.Close()
		return nil, fmt.Errorf("build router: %w", err)
	}

	verifier := auth.NewVerifier(cfg.JWTSecret, cfg.LayoutID)
	wsSrv := ws.NewServer(ws.ServerConfig{
		Verifier:       verifier,
		Hub:            hub,
		Router:         router,
		Log:            log,
		LayoutID:       cfg.LayoutID,
		CommandStation: cfg.CommandStationID,
		SpeedSteps:     cs.SpeedSteps,
		HeartbeatSecs:  cfg.HeartbeatSecs,
		DeadmanSecs:    cfg.DeadmanSecs,
		AllowedOrigins: cfg.AllowedOrigins,
	})

	srv := &http.Server{
		Addr:              net.JoinHostPort(cfg.BindAddr, strconv.Itoa(int(cfg.Port))),
		Handler:           wsSrv,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return &Daemon{
		cfg:    cfg,
		log:    log,
		redis:  red,
		rds:    rds,
		srv:    srv,
		router: router,
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

	go d.runCommandConsumer(ctx, cmdSub)
	go d.runAllowedVehiclesConsumer(ctx, vehSub)
	go d.runDefinedTrainsConsumer(ctx, trainSub)

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
			snap, err := layoutroster.UnmarshalAllowedVehicles([]byte(msg.Payload))
			if err != nil {
				d.log.WithError(err).Warn("dcc-bus allowed_vehicles: bad payload")
				continue
			}
			d.router.ApplyAllowedVehicles(snap)
		}
	}
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
			snap, err := layoutroster.UnmarshalDefinedTrains([]byte(msg.Payload))
			if err != nil {
				d.log.WithError(err).Warn("dcc-bus defined_trains: bad payload")
				continue
			}
			d.router.ApplyDefinedTrains(snap)
		}
	}
}

// Close releases every dependency the daemon opened. Idempotent.
func (d *Daemon) Close() error {
	if d.srv != nil {
		_ = d.srv.Close()
	}
	if d.rds != nil {
		_ = d.rds.Close()
	}
	return nil
}
