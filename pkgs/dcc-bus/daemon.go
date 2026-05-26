// Package dccbus is the entry point of the `dcc-bus` daemon. It
// wires the SQLite read-only handle, Redis client, command-station
// driver, command router and WebSocket server into a single
// Daemon.Run loop that the cobra subcommand drives.
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
)

// Config carries every runtime input the daemon needs. Populated
// from the cobra subcommand's flags.
type Config struct {
	LayoutID         uint
	CommandStationID uint

	BindAddr string
	Port     uint16

	DBPath    string
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

	sqlite *state.SQLite
	redis  *state.Redis
	rds    *redis.Client
	srv    *http.Server
	router *cmd.Router
}

// New validates cfg, opens dependencies (SQLite + Redis + command
// station) and returns a ready-to-Run daemon. The caller MUST call
// Close to release resources.
func New(ctx context.Context, log *logrus.Logger, cfg Config) (*Daemon, error) {
	if log == nil {
		log = logrus.New()
	}
	if cfg.LayoutID == 0 || cfg.CommandStationID == 0 {
		return nil, errors.New("dcc-bus: --layout-id and --command-station-id are required")
	}
	if cfg.DBPath == "" {
		return nil, errors.New("dcc-bus: --db-path is required")
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

	sqlite, err := state.OpenSQLite(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	cs, err := sqlite.CommandStation(ctx, cfg.CommandStationID)
	if err != nil {
		_ = sqlite.Close()
		return nil, fmt.Errorf("load command station: %w", err)
	}
	if _, err := sqlite.Layout(ctx, cfg.LayoutID); err != nil {
		_ = sqlite.Close()
		return nil, fmt.Errorf("load layout: %w", err)
	}
	attached, err := sqlite.LayoutAttached(ctx, cfg.LayoutID, cfg.CommandStationID)
	if err != nil {
		_ = sqlite.Close()
		return nil, fmt.Errorf("check layout attachment: %w", err)
	}
	if !attached {
		_ = sqlite.Close()
		return nil, fmt.Errorf("dcc-bus: command station %d is not attached to layout %d", cfg.CommandStationID, cfg.LayoutID)
	}

	rds := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	if err := rds.Ping(ctx).Err(); err != nil {
		_ = sqlite.Close()
		_ = rds.Close()
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	red := state.NewRedis(rds, cfg.LayoutID, cfg.CommandStationID)

	st, err := station.Open(cs)
	if err != nil {
		_ = sqlite.Close()
		_ = rds.Close()
		return nil, fmt.Errorf("open command station: %w", err)
	}

	hub := ws.NewHub()

	router, err := cmd.NewRouter(ctx, cmd.Config{
		Station:          st,
		Hub:              hub,
		Redis:            red,
		SQLite:           sqlite,
		Log:              log,
		LayoutID:         cfg.LayoutID,
		CommandStationID: cfg.CommandStationID,
		SpeedSteps:       cs.SpeedSteps,
	})
	if err != nil {
		_ = st.CleanUp()
		_ = sqlite.Close()
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
		sqlite: sqlite,
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

	invSub, err := d.redis.SubscribeInvalidations(ctx)
	if err != nil {
		return fmt.Errorf("subscribe invalidate channel: %w", err)
	}
	defer invSub.Close()

	go d.runCommandConsumer(ctx, cmdSub)
	go d.runInvalidateConsumer(ctx, invSub)

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

func (d *Daemon) runInvalidateConsumer(ctx context.Context, sub *redis.PubSub) {
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			d.log.WithField("payload", msg.Payload).Debug("dcc-bus invalidate")
			if err := d.router.ReloadRoster(ctx); err != nil {
				d.log.WithError(err).Warn("dcc-bus roster reload failed")
			}
		}
	}
}

// Close releases every dependency the daemon opened. Idempotent.
func (d *Daemon) Close() error {
	if d.srv != nil {
		_ = d.srv.Close()
	}
	if d.sqlite != nil {
		_ = d.sqlite.Close()
	}
	if d.rds != nil {
		_ = d.rds.Close()
	}
	return nil
}
