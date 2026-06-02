// Package cli wires the cobra command that the `loco-server` binary
// runs. Keeping the cobra wiring out of `main` makes the command
// testable in isolation and mirrors the layout of `pkgs/loco/cli`.
package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	dccbuscli "github.com/keskad/loco/pkgs/dcc-bus/cli"
	httpapi "github.com/keskad/loco/pkgs/server/http"
	"github.com/keskad/loco/pkgs/server/repo"
	"github.com/keskad/loco/pkgs/server/repo/migrations"
	"github.com/keskad/loco/pkgs/server/service"
	"github.com/keskad/loco/pkgs/server/ws"
)

// Flags collects every command-line knob exposed by `loco-server`.
// Defaults are tuned for local development: SQLite file lives next to
// the binary, the API listens on :8080 and CORS allows the Vite dev
// server on :5173.
type Flags struct {
	HTTPAddr       string
	DBPath         string
	JWTSecret      string
	AllowedOrigins []string
	SecureCookie   bool
	NoSupervisor   bool

	// Redis. By default loco-server spawns its own redis-server via
	// supervisord on RedisBindAddr:RedisPort; pass --redis-external
	// to skip the managed daemon and dial RedisAddr instead.
	RedisBin      string
	RedisBindAddr string
	RedisPort     uint16
	RedisDataDir  string
	RedisAddr     string
	RedisExternal bool
	RedisPersist  bool

	// LogLevel is a logrus level name (debug, info, warn, error). The
	// BIGFRED_LOG_LEVEL env var overrides the flag when set.
	LogLevel string
}

// NewRootCommand returns the top-level cobra command. It is invoked
// from `main()` of the standalone `loco-server` binary.
func NewRootCommand(log *logrus.Logger) *cobra.Command {
	var f Flags

	cmd := &cobra.Command{
		Use:   "loco-server",
		Short: "BigFred web application — Go backend (REST + WebSocket).",
		Long: `loco-server is the HTTP/WebSocket facade in front of the existing
LocoApp controller layer. It owns user authentication, session
management and (in later milestones) the WebSocket fan-out for
real-time throttle commands.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(cmd.Context(), log, f)
		},
	}

	cmd.Flags().StringVar(&f.HTTPAddr, "http", "0.0.0.0:8080",
		"address the HTTP server listens on (0.0.0.0 = all interfaces)")
	cmd.Flags().StringVar(&f.DBPath, "db", "bigfred.db", "path to the SQLite database file")
	cmd.Flags().StringVar(&f.JWTSecret, "jwt-secret", "",
		"hex/base64 secret used to sign session JWTs. Falls back to BIGFRED_JWT_SECRET "+
			"env var; a random per-run secret is generated when empty (sessions don't survive restarts).")
	cmd.Flags().StringSliceVar(&f.AllowedOrigins, "cors-origin",
		[]string{"http://localhost:5173", "http://127.0.0.1:5173"},
		"CORS allowed origins (Vite dev server on :5173 by default)")
	cmd.Flags().BoolVar(&f.SecureCookie, "secure-cookie", false,
		"set the Secure flag on the session cookie (REQUIRED in production, off for local http://)")
	cmd.Flags().BoolVar(&f.NoSupervisor, "no-supervisor", false,
		"skip supervisord process management (for local dev without the supervisor package)")
	cmd.Flags().StringVar(&f.LogLevel, "log-level", "info",
		"logrus level (debug, info, warn, error). BIGFRED_LOG_LEVEL env overrides this flag.")

	cmd.Flags().StringVar(&f.RedisBin, "redis-bin", "valkey-server",
		"redis-server binary path (PATH-relative or absolute) used by the managed daemon")
	cmd.Flags().StringVar(&f.RedisBindAddr, "redis-bind", "127.0.0.1",
		"interface the managed redis-server binds on; loopback by default")
	cmd.Flags().Uint16Var(&f.RedisPort, "redis-port", 6380,
		"TCP port the managed redis-server listens on (default 6380 to avoid colliding with a system redis on 6379)")
	cmd.Flags().StringVar(&f.RedisDataDir, "redis-data-dir", "",
		"working directory for redis-server (defaults to the supervisord log directory)")
	cmd.Flags().StringVar(&f.RedisAddr, "redis-addr", "",
		"redis dial address (host:port) used by loco-server and dcc-bus; defaults to redis-bind:redis-port")
	cmd.Flags().BoolVar(&f.RedisExternal, "redis-external", false,
		"do not spawn a managed redis-server; dial --redis-addr instead (operator runs Redis out-of-band)")
	cmd.Flags().BoolVar(&f.RedisPersist, "redis-persist", false,
		"keep RDB snapshots / AOF for the managed redis-server; off by default because dcc-bus rebuilds state cheaply")

	cmd.AddCommand(dccbuscli.NewCommand(log))

	return cmd
}

func run(ctx context.Context, log *logrus.Logger, f Flags) error {
	if err := configureLogLevel(log, f.LogLevel); err != nil {
		return err
	}

	if absPath, err := filepath.Abs(f.DBPath); err == nil {
		f.DBPath = absPath
	}

	secret, err := resolveJWTSecret(f.JWTSecret, log)
	if err != nil {
		return err
	}

	repository, sqlDB, err := repo.Open(f.DBPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer sqlDB.Close()

	log.WithField("path", f.DBPath).Info("database opened, applying migrations")
	migrations.MigrateUp(ctx, repository)

	users := repo.NewUsers(repository)
	layouts := repo.NewLayouts(repository)
	interlockings := repo.NewInterlockings(repository)
	layoutInterlockings := repo.NewLayoutInterlockings(repository)
	layoutSignalmen := repo.NewLayoutSignalmen(repository)
	sudoElevations := repo.NewSudoElevations(repository)
	interlockingSessions := repo.NewInterlockingSessions(repository)
	dccPools := repo.NewDCCAddressRanges(repository)
	vehicles := repo.NewVehicles(repository)
	trains := repo.NewTrains(repository)
	trainMembers := repo.NewTrainMembers(repository)
	layoutVehicles := repo.NewLayoutVehicles(repository)
	layoutTrains := repo.NewLayoutTrains(repository)
	commandStations := repo.NewCommandStations(repository)
	layoutCommandStations := repo.NewLayoutCommandStations(repository)
	_ = layoutTrains

	layoutSvc := service.NewLayoutService(layouts, interlockings, layoutInterlockings, commandStations, layoutCommandStations)
	commandStationSvc := service.NewCommandStationService(commandStations, layoutCommandStations, layouts)
	interlockingSvc := service.NewInterlockingService(interlockings, layoutInterlockings)
	authSvc := service.NewAuthService(users, layoutSvc, layoutSignalmen, sudoElevations, service.AuthConfig{JWTSecret: secret})
	dccPoolSvc := service.NewDCCPoolService(dccPools)
	vehicleSvc := service.NewVehicleService(vehicles, dccPoolSvc, trainMembers)
	trainSvc := service.NewTrainService(trains, trainMembers, vehicles)
	userSvc := service.NewUserService(users, vehicles, trains, dccPoolSvc)

	hub := ws.NewHub()
	sudoSvc := service.NewSudoService(sudoElevations, layoutSignalmen, layoutSvc, hub, service.DefaultSudoConfig)
	presenceSvc := service.NewPresenceService(hub, authSvc, users, interlockingSessions, interlockings, layoutInterlockings)
	hub.SetPresenceRefresher(presenceSvc)
	occupancySvc := service.NewInterlockingOccupancyService(
		interlockings, layoutInterlockings, interlockingSessions, users,
		authSvc, hub, presenceSvc,
	)
	layoutVehicleSvc := service.NewLayoutVehicleService(
		layoutVehicles, layoutTrains, vehicles, trains, trainMembers, users, hub,
	)

	go hub.Run(ctx)
	// Janitor for expired sudo elevations (§7a.7). Runs in its own
	// goroutine so a slow SQLite write doesn't block the WS hub.
	go sudoSvc.RunJanitor(ctx)

	redisAddr := f.RedisAddr
	if redisAddr == "" {
		redisAddr = fmt.Sprintf("%s:%d", f.RedisBindAddr, f.RedisPort)
	}
	redisSvc := service.NewRedisService(service.RedisServiceConfig{Addr: redisAddr})
	defer func() { _ = redisSvc.Close() }()
	layoutVehicleSvc.SetRedisRosterPublisher(redisSvc)

	var supSvc *service.SupervisordService
	if !f.NoSupervisor {
		initial := service.DefaultInfraProcesses(service.RedisConfig{
			Bin:                  f.RedisBin,
			BindAddr:             f.RedisBindAddr,
			Port:                 f.RedisPort,
			DataDir:              f.RedisDataDir,
			EphemeralPersistence: !f.RedisPersist,
			Disable:              f.RedisExternal,
		})
		supSvc, err = service.NewSupervisordService(service.SupervisordConfig{
			InitialState: initial,
			Log:          log,
		})
		if err != nil {
			return fmt.Errorf("supervisord init: %w", err)
		}
		if err := supSvc.Start(ctx); err != nil {
			return fmt.Errorf("supervisord start: %w", err)
		}
		supSvc.RunHealthLoop(ctx, 5*time.Second, func(states []service.ProgramState) {
			log.WithField("programs", states).Debug("supervisord status changed")
		})
	}

	// Block until Redis is reachable so anything that publishes
	// during the rest of bootstrap (presence broadcasts, future
	// dcc-bus enqueues) won't race the daemon's first byte. The
	// timeout is generous because supervisord's StartSecs already
	// gates "RUNNING" on a healthy boot.
	redisReady := true
	if !f.RedisExternal || f.NoSupervisor {
		readyCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		if err := redisSvc.WaitReady(readyCtx, 10*time.Second); err != nil {
			cancel()
			redisReady = false
			if f.NoSupervisor {
				log.WithError(err).Warn("redis unreachable; continuing because --no-supervisor was set")
			} else {
				return fmt.Errorf("redis wait ready: %w", err)
			}
		} else {
			cancel()
			log.WithField("addr", redisAddr).Info("redis ready")
		}
	}

	// dcc-bus orchestrator. Disabled when supervisord / redis are
	// off-line because both are hard dependencies.
	var dccBusSvc *service.DccBusService
	var dccLayoutSync *service.DccBusLayoutSync
	var sessionCtl *service.SessionControlService
	if supSvc != nil && redisReady {
		executable, _ := os.Executable()
		dccBusSvc = service.NewDccBusService(service.DccBusConfig{
			Executable:   executable,
			RedisAddr:    redisAddr,
			JWTSecret:    secret,
			PortMin:      9200,
			PortMax:      9209,
			SpawnTimeout: 10 * time.Second,
			ProxyEnabled: true,
		}, supSvc, redisSvc, commandStations, log)
		if err := dccBusSvc.HydratePorts(ctx); err != nil {
			log.WithError(err).Warn("dcc-bus hydrate ports")
		}
		dccLayoutSync = service.NewDccBusLayoutSync(dccBusSvc, layoutSvc, hub)
	}

	if dccBusSvc != nil {
		sessionCtl = service.NewSessionControlService(service.SessionControlConfig{
			Log:         log,
			DccBus:      dccBusSvc,
			CommandStns: commandStations,
			LayoutCS:    layoutCommandStations,
			Layouts:     layouts,
		})
		hub.SetControlHandler(sessionCtl)
		commandStationSvc.SetRuntime(service.CommandStationRuntime{
			DccSync:    dccLayoutSync,
			SessionCtl: sessionCtl,
		})

		// Fan dcc-bus daemon events back onto the control plane so
		// the dashboard / sudo UI reacts to estop audits, daemon
		// crashes and (future) takeover broadcasts.
		evtConsumer := service.NewDccBusEventConsumer(redisSvc, hub, log)
		if err := evtConsumer.Start(ctx); err != nil {
			log.WithError(err).Warn("dcc-bus event consumer start")
		}
		defer evtConsumer.Stop()
	}

	// Seed the bootstrap system layout BEFORE the admin account so
	// the very first login can pick it from the dropdown without
	// hitting a 422 layout_not_found.
	if seeded, err := layoutSvc.EnsureSystemLayout(ctx); err != nil {
		return fmt.Errorf("seed system layout: %w", err)
	} else if seeded {
		log.WithField("admin_pin", service.SystemLayoutDefaultAdminPIN).
			Warn("bootstrap system layout created — CHANGE THE ADMIN PIN AFTER FIRST LOGIN")
	}

	if redisReady {
		all, err := layoutSvc.ListAll(ctx)
		if err != nil {
			log.WithError(err).Warn("layout roster redis sync: list layouts")
		} else {
			for _, l := range all {
				if err := layoutVehicleSvc.SyncLayoutRosterToRedis(ctx, l.ID); err != nil {
					log.WithError(err).WithField("layoutId", l.ID).Warn("layout roster redis sync")
				}
			}
		}
	}

	seeded, err := service.SeedAdmin(ctx, users, service.SeedDefaults)
	if err != nil {
		return fmt.Errorf("seed admin: %w", err)
	}
	if seeded {
		log.WithFields(logrus.Fields{
			"login": service.SeedDefaults.Login,
			"pin":   service.SeedDefaults.PIN,
		}).Warn("bootstrap admin account created — CHANGE THE PIN AFTER FIRST LOGIN")
	}

	// Seed a generous default pool for the bootstrap admin so a
	// freshly initialised installation can register vehicles
	// without an extra round trip through the admin pool page.
	if admin, err := users.FindByLogin(ctx, service.SeedDefaults.Login); err == nil {
		if err := dccPoolSvc.SeedAdminPoolIfEmpty(ctx, admin.ID); err != nil {
			return fmt.Errorf("seed admin dcc pool: %w", err)
		}
	}

	diagSvc, err := service.NewDiagnosticsService(supSvc)
	if err != nil {
		return fmt.Errorf("diagnostics init: %w", err)
	}

	router := httpapi.NewRouter(httpapi.RouterConfig{
		Auth:             authSvc,
		Users:            userSvc,
		Layouts:          layoutSvc,
		Interlockings:    interlockingSvc,
		Occupancy:        occupancySvc,
		Presence:         presenceSvc,
		DccBusLayoutSync: dccLayoutSync,
		Vehicles:         vehicleSvc,
		Trains:           trainSvc,
		LayoutVehicles:   layoutVehicleSvc,
		DCCPool:          dccPoolSvc,
		Sudo:             sudoSvc,
		CommandStations:  commandStationSvc,
		Diagnostics:      diagSvc,
		Hub:              hub,
		DccBus:           dccBusSvc,
		AllowedOrigins:   f.AllowedOrigins,
		SecureCookie:     f.SecureCookie,
	})

	srv := &http.Server{
		Addr:              f.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		log.WithField("addr", f.HTTPAddr).Info("listening")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
		close(serveErr)
	}()

	// Cooperative shutdown on SIGINT/SIGTERM. We give in-flight
	// requests a brief grace period before forcing the server down.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.WithField("signal", sig.String()).Info("shutdown requested")
	case err := <-serveErr:
		if err != nil {
			return err
		}
		return nil
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if supSvc != nil {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 15*time.Second)
		if err := supSvc.Stop(stopCtx); err != nil {
			log.WithError(err).Warn("supervisord shutdown")
		}
		stopCancel()
	}
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}

func configureLogLevel(log *logrus.Logger, flagLevel string) error {
	levelName := flagLevel
	if env := os.Getenv("BIGFRED_LOG_LEVEL"); env != "" {
		levelName = env
	}
	level, err := logrus.ParseLevel(levelName)
	if err != nil {
		return fmt.Errorf("log level %q: %w", levelName, err)
	}
	log.SetLevel(level)
	log.WithField("level", level.String()).Debug("log level configured")
	return nil
}

// resolveJWTSecret picks the JWT signing key in the documented
// precedence order: explicit --jwt-secret > BIGFRED_JWT_SECRET env >
// random per-run secret (development only).
func resolveJWTSecret(flag string, log *logrus.Logger) ([]byte, error) {
	if flag != "" {
		return []byte(flag), nil
	}
	if env := os.Getenv("BIGFRED_JWT_SECRET"); env != "" {
		return []byte(env), nil
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("generate random jwt secret: %w", err)
	}
	log.Warn("no JWT secret configured — generated a random one. Existing sessions will not survive a restart. " +
		"Set --jwt-secret or BIGFRED_JWT_SECRET in production.")
	// Use the raw bytes (not hex) — the encoding doesn't matter for
	// HMAC, but the log message above is a strong hint that this is
	// development-only behaviour.
	_ = hex.EncodeToString(buf)
	return buf, nil
}
