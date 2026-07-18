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
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	frontend "github.com/keskad/loco/web"

	dccbuscli "github.com/keskad/loco/pkgs/bigfred/dcc-bus/cli"
	"github.com/keskad/loco/pkgs/bigfred/mdns"
	bfotel "github.com/keskad/loco/pkgs/bigfred/otel"
	"github.com/keskad/loco/pkgs/bigfred/remotepairing"
	"github.com/keskad/loco/pkgs/bigfred/server/cmd"
	"github.com/keskad/loco/pkgs/bigfred/server/domain"
	httpapi "github.com/keskad/loco/pkgs/bigfred/server/http"
	"github.com/keskad/loco/pkgs/bigfred/server/metrics"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/repo/migrations"
	"github.com/keskad/loco/pkgs/bigfred/server/service"
	"github.com/keskad/loco/pkgs/bigfred/server/supervisord"
	"github.com/keskad/loco/pkgs/bigfred/server/ws"
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
	// to skip the managed daemon and dial RedisAddr instead. When
	// RedisAutoDetect is true (default), an existing instance at
	// RedisAddr is reused without spawning.
	RedisBin        string
	RedisBindAddr   string
	RedisPort       uint16
	RedisDataDir    string
	RedisAddr       string
	RedisExternal   bool
	RedisAutoDetect bool
	RedisRDBSave    []string
	RedisNoPersist  bool

	EnableTelemetry bool
	TelemetryConfig string

	// MDNS advertises the HTTP UI on the LAN as <MDNSHost>.local (default
	// bigfred.local). Disable with --mdns=false for local-only binds or
	// when another process owns mDNS.
	MDNS     bool
	MDNSHost string

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
			if err := mergeConfigFile(cmd, &f, log); err != nil {
				return err
			}
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
	cmd.Flags().Uint16Var(&f.RedisPort, "redis-port", 6379,
		"TCP port the managed redis-server listens on")
	cmd.Flags().StringVar(&f.RedisDataDir, "redis-data-dir", "",
		"working directory for redis-server (defaults to the supervisord config directory)")
	cmd.Flags().StringVar(&f.RedisAddr, "redis-addr", "",
		"redis dial address (host:port) used by loco-server and dcc-bus; defaults to redis-bind:redis-port")
	cmd.Flags().BoolVar(&f.RedisExternal, "redis-external", false,
		"do not spawn a managed redis-server; dial --redis-addr instead (operator runs Redis out-of-band)")
	cmd.Flags().BoolVar(&f.RedisAutoDetect, "redis-auto-detect", true,
		"skip spawning managed redis-server when --redis-addr already accepts PING")
	cmd.Flags().StringSliceVar(&f.RedisRDBSave, "redis-rdb-save", nil,
		"RDB snapshot rules as seconds:changes (repeatable); default save 60 100; pass \"\" to disable")
	cmd.Flags().BoolVar(&f.RedisNoPersist, "redis-no-persist", false,
		"disable RDB snapshots for the managed redis-server (ephemeral mode)")
	cmd.Flags().BoolVar(&f.EnableTelemetry, "enable-telemetry", false,
		"start Grafana Alloy via supervisord and enable OTLP metric export for loco-server and dcc-bus")
	cmd.Flags().StringVar(&f.TelemetryConfig, "telemetry-config", service.DefaultTelemetryConfigPath,
		"path to the Alloy config file (used with --enable-telemetry)")
	cmd.Flags().BoolVar(&f.MDNS, "mdns", true,
		"advertise the HTTP UI via mDNS (bigfred.local by default)")
	cmd.Flags().StringVar(&f.MDNSHost, "mdns-host", "bigfred",
		"mDNS hostname without domain (e.g. bigfred → bigfred.local)")

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

	var metricsShutdown func(context.Context) error
	var serverMetrics *metrics.Metrics
	if f.EnableTelemetry {
		var shutdownErr error
		metricsShutdown, shutdownErr = bfotel.InitMetrics(ctx, "loco-server", service.DefaultOTLPEndpoint)
		if shutdownErr != nil {
			return fmt.Errorf("loco-server metrics: %w", shutdownErr)
		}
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = metricsShutdown(shutdownCtx)
		}()
		serverMetrics, err = metrics.New(metrics.Config{Enabled: true})
		if err != nil {
			return fmt.Errorf("loco-server metrics instruments: %w", err)
		}
		log.WithField("endpoint", service.DefaultOTLPEndpoint).Info("loco-server metrics enabled")
	}

	repository, sqlDB, err := repo.Open(f.DBPath, log, serverMetrics)
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
	interlockingSessions := repo.NewInterlockingSessions(repository)
	dccPools := repo.NewDCCAddressRanges(repository)
	vehicles := repo.NewVehicles(repository)
	dccFunctions := repo.NewDccFunctions(repository)
	vehicleTemplates := repo.NewVehicleTemplates(repository)
	trains := repo.NewTrains(repository)
	trainMembers := repo.NewTrainMembers(repository)
	layoutVehicles := repo.NewLayoutVehicles(repository)
	layoutTrains := repo.NewLayoutTrains(repository)
	takeoverRequestsRepo := repo.NewTakeoverRequests(repository)
	commandStations := repo.NewCommandStations(repository)
	layoutCommandStations := repo.NewLayoutCommandStations(repository)

	layoutSvc := cmd.NewLayout(layouts, interlockings, layoutInterlockings, commandStations, layoutCommandStations)
	commandStationSvc := cmd.NewCommandStation(commandStations, layoutCommandStations, layouts)
	interlockingSvc := cmd.NewInterlocking(interlockings, layoutInterlockings)
	dccPoolSvc := cmd.NewDCCPool(dccPools)
	vehicleSvc := cmd.NewVehicle(vehicles, dccPoolSvc, trainMembers, layoutVehicles, users)
	functionSvc := cmd.NewFunction(dccFunctions, vehicles, vehicleTemplates, users)
	vehicleTemplateSvc := cmd.NewVehicleTemplate(vehicleTemplates, users, dccFunctions)
	trainSvc := cmd.NewTrain(trains, trainMembers, vehicles, layoutTrains, users)
	userSvc := cmd.NewUser(users, vehicles, trains, dccPoolSvc)

	redisAddr := f.RedisAddr
	if redisAddr == "" {
		redisAddr = fmt.Sprintf("%s:%d", f.RedisBindAddr, f.RedisPort)
	}
	redisCfg := service.RedisServiceConfig{Addr: redisAddr}
	redisMgmt := service.ResolveRedisManagement(ctx, redisCfg, f.RedisExternal, f.RedisAutoDetect)
	if redisMgmt.Source == "auto-detected" {
		log.WithField("addr", redisAddr).Info("redis already running; skipping managed instance")
	} else if redisMgmt.Source == "explicit-external" {
		log.WithField("addr", redisAddr).Info("using external redis (--redis-external)")
	}
	redisSvc := service.NewRedisService(redisCfg)
	defer func() { _ = redisSvc.Close() }()

	redisRDBSavePoints, err := supervisord.ResolveRDBSavePoints(f.RedisNoPersist, f.RedisRDBSave)
	if err != nil {
		return err
	}

	var supSvc service.Supervisor
	if !f.NoSupervisor {
		supPaths, err := supervisord.DefaultPaths()
		if err != nil {
			return fmt.Errorf("supervisord paths: %w", err)
		}
		telemetryCfg := service.TelemetryConfig{
			Enable:       f.EnableTelemetry,
			ConfigPath:   f.TelemetryConfig,
			OTLPEndpoint: service.DefaultOTLPEndpoint,
		}
		initial := service.DefaultInfraProcesses(service.InfraConfig{
			Redis: service.RedisConfig{
				Bin:           f.RedisBin,
				BindAddr:      f.RedisBindAddr,
				Port:          f.RedisPort,
				DataDir:       f.RedisDataDir,
				RDBSavePoints: redisRDBSavePoints,
				Disable:       !redisMgmt.Managed,
			},
			Telemetry: telemetryCfg,
		})
		supSvc, err = service.NewSupervisordService(service.SupervisordConfig{
			ConfigDir:    supPaths.ConfigDir,
			ConfigPath:   supPaths.ConfigPath,
			SocketPath:   supPaths.SocketPath,
			PIDFile:      supPaths.PIDFile,
			LogDir:       supPaths.LogDir,
			InitialState: initial,
			Telemetry:    telemetryCfg,
			Log:          log,
		})
		if err != nil {
			return fmt.Errorf("supervisord init: %w", err)
		}
		if err := supSvc.Start(ctx); err != nil {
			return fmt.Errorf("supervisord start: %w", err)
		}
		if f.EnableTelemetry {
			log.WithFields(logrus.Fields{
				"config":    f.TelemetryConfig,
				"alloy_run": service.AlloyRunConfigPath(telemetryCfg),
				"generated": service.BigFredAlloyGeneratedPath(telemetryCfg),
			}).Info("telemetry enabled: supervisord will manage alloy")
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
		log.WithFields(logrus.Fields{
			"addr":   redisAddr,
			"source": redisMgmt.Source,
		}).Info("redis ready")
	}

	var sudoElevations repo.SudoElevationStore
	if redisReady {
		sudoElevations = repo.NewRedisSudoElevations(redisSvc.Client())
		log.Info("sudo elevations stored in Redis")
	} else {
		sudoElevations = repo.NewSudoElevations(repository)
		log.Warn("redis unavailable; sudo elevations stored in SQLite")
	}

	var vehicleLeases repo.VehicleLeaseStore
	var trainLeases repo.TrainLeaseStore
	var takeoverRequests repo.TakeoverRequestStore = takeoverRequestsRepo
	if redisReady {
		vehicleLeases = repo.NewRedisVehicleLeases(redisSvc.Client())
		trainLeases = repo.NewRedisTrainLeases(redisSvc.Client())
		takeoverRequests = repo.NewRedisTakeoverRequests(redisSvc.Client())
		log.Info("takeover requests and drive leases stored in Redis")
	} else {
		log.Warn("redis unavailable; takeover requests stored in SQLite, leases disabled")
	}

	authSvc := cmd.NewAuth(users, layoutSvc, layoutSignalmen, sudoElevations, cmd.AuthConfig{JWTSecret: secret})

	hub := ws.NewHub()
	hub.SetMetrics(serverMetrics)
	if serverMetrics != nil {
		serverMetrics.SetPresenceReader(hub)
	}
	sudoSvc := cmd.NewSudo(sudoElevations, layoutSignalmen, layoutSvc, hub, cmd.DefaultSudoConfig)
	presenceSvc := service.NewPresenceService(hub, authSvc, users, interlockingSessions, interlockings, layoutInterlockings)
	hub.SetPresenceRefresher(presenceSvc)
	// Inject the admin-role resolver so the hub can keep each WS
	// session's cached EffectiveAdmin flag fresh after sudo grants /
	// revokes, and scope admin-only broadcasts (e.g. handset clients
	// snapshot) without a per-broadcast Redis lookup.
	hub.SetRoleResolver(func(ctx context.Context, userID, layoutID uint) bool {
		eff, err := authSvc.EffectiveForUserID(ctx, userID, layoutID)
		return err == nil && eff.Has(domain.RoleAdmin)
	})
	occupancySvc := service.NewInterlockingOccupancyService(
		interlockings, layoutInterlockings, interlockingSessions, users,
		authSvc, hub, presenceSvc,
	)
	layoutVehicleSvc := service.NewLayoutVehicleService(
		layoutVehicles, layoutTrains, vehicles, trains, trainMembers,
		vehicleLeases, trainLeases, users, hub,
	)
	layoutVehicleSvc.SetRedisRosterPublisher(redisSvc)
	layoutVehicleSvc.SetFunctionLister(functionSvc)

	var z21RemoteSvc *cmd.Z21Remote
	var withrottleRemoteSvc *cmd.WithrottleRemote
	var remoteSvc *cmd.Remote
	if redisReady {
		pairStore := remotepairing.NewStore(redisSvc.Client())
		z21RemoteSvc = cmd.NewZ21Remote(
			pairStore,
			commandStations,
			layoutCommandStations,
			layoutVehicleSvc.LayoutRoster,
			layoutVehicleSvc.LayoutRosterSnapshot,
			users,
		)
		withrottleRemoteSvc = cmd.NewWithrottleRemote(
			pairStore,
			commandStations,
			layoutCommandStations,
			layoutVehicleSvc.LayoutRoster,
			layoutVehicleSvc.LayoutRosterSnapshot,
		)
		remoteSvc = cmd.NewRemote(z21RemoteSvc, withrottleRemoteSvc, pairStore, users)
	}

	go hub.Run(ctx)
	if sudoElevations.RequiresJanitor() {
		go sudoSvc.RunJanitor(ctx)
	}

	// dcc-bus orchestrator. Disabled when supervisord / redis are
	// off-line because both are hard dependencies.
	var dccBusSvc *service.DccBusService
	var dccLayoutSync *service.DccBusLayoutSync
	var sessionCtl *service.SessionControlService
	var radioSvc *service.RadioService
	var takeoverSvc *service.TakeoverService
	if supSvc != nil && redisReady {
		executable, _ := os.Executable()
		dccBusSvc = service.NewDccBusService(service.DccBusConfig{
			Executable:      executable,
			RedisAddr:       redisAddr,
			JWTSecret:       secret,
			PortMin:         9200,
			PortMax:         9209,
			SpawnTimeout:    10 * time.Second,
			ProxyEnabled:    true,
			EnableTelemetry: f.EnableTelemetry,
			OTLPEndpoint:    service.DefaultOTLPEndpoint,
		}, supSvc, redisSvc, commandStations, layouts, log)
		if err := dccBusSvc.HydratePorts(ctx); err != nil {
			log.WithError(err).Warn("dcc-bus hydrate ports")
		}
		dccBusSvc.SetMetrics(serverMetrics)
		if serverMetrics != nil {
			serverMetrics.SetDccBusStatsReader(dccBusSvc)
		}
		dccLayoutSync = service.NewDccBusLayoutSync(dccBusSvc, layoutSvc, hub)
	}

	var auditSvc *service.AuditService
	if redisReady {
		auditSvc = service.NewAuditService(service.AuditServiceConfig{Redis: redisSvc})
		log.Info("audit service ready (Redis Streams)")
	}

	var radioStopSvc *service.RadioStopService
	var estopTargetSvc *service.EStopTargetService
	var leaseBrake cmd.LeaseBrakePort
	if dccBusSvc != nil && redisReady {
		radioStopSvc = service.NewRadioStopService(service.RadioStopConfig{
			Hub:    hub,
			Redis:  redisSvc,
			Roster: layoutVehicleSvc,
			Auth:   authSvc,
			Audit:  auditSvc,
			Log:    log,
		})
		estopTargetSvc = service.NewEStopTargetService(service.EStopTargetConfig{
			DccBus:      dccBusSvc,
			Roster:      layoutVehicleSvc,
			Layouts:     layoutSvc,
			Auth:        authSvc,
			Audit:       auditSvc,
			IlkSessions: interlockingSessions,
			LayoutIlks:  layoutInterlockings,
			Log:         log,
		})
		leaseBrake = service.NewLeaseBrake(service.LeaseBrakeConfig{
			DccBus:  dccBusSvc,
			Roster:  layoutVehicleSvc,
			Layouts: layoutSvc,
			Log:     log,
		})
	}

	leaseSvc := service.NewLeaseService(service.LeaseConfig{
		VehicleLeases:  vehicleLeases,
		TrainLeases:    trainLeases,
		LayoutVehicles: layoutVehicles,
		LayoutTrains:   layoutTrains,
		Vehicles:       vehicles,
		Trains:         trains,
		Users:          users,
		Roster:         layoutVehicleSvc,
		Hub:            hub,
		Audit:          auditSvc,
		Brake:          leaseBrake,
	})
	if err := leaseSvc.RecoverPending(ctx); err != nil {
		log.WithError(err).Warn("lease recover pending")
	}

	sessionCtl = service.NewSessionControlService(service.SessionControlConfig{
		Log:         log,
		DccBus:      dccBusSvc,
		RadioStop:   radioStopSvc,
		EStopTarget: estopTargetSvc,
		CommandStns: commandStations,
		LayoutCS:    layoutCommandStations,
		Layouts:     layouts,
		Metrics:     serverMetrics,
	})

	if redisReady {
		radioStore := service.NewRadioStore(service.RadioStoreConfig{Redis: redisSvc})
		radioSvc = service.NewRadioService(service.RadioConfig{
			Store:         radioStore,
			Hub:           hub,
			Auth:          authSvc,
			Layouts:       layouts,
			Vehicles:      vehicles,
			Trains:        trains,
			IlkSessions:   interlockingSessions,
			LayoutIlks:    layoutInterlockings,
			Interlockings: interlockings,
		})
		takeoverSvc = service.NewTakeoverService(service.TakeoverConfig{
			Requests:      takeoverRequests,
			VehicleLeases: vehicleLeases,
			TrainLeases:   trainLeases,
			Vehicles:      vehicles,
			Trains:        trains,
			TrainMembers:  trainMembers,
			IlkSessions:   interlockingSessions,
			Users:         users,
			Roster:        layoutVehicleSvc,
			Auth:          authSvc,
			Hub:           hub,
			Audit:         auditSvc,
			Metrics:       serverMetrics,
		})
		occupancySvc.SetTakeoverService(takeoverSvc)
		if err := takeoverSvc.RecoverPending(ctx); err != nil {
			log.WithError(err).Warn("takeover recover pending")
		}
		if takeoverRequests.RequiresJanitor() {
			go takeoverSvc.RunJanitor(ctx)
		}
		hub.SetControlHandler(service.NewMetricsControlHandler(service.NewCompositeControlHandler(
			sessionCtl,
			service.NewRadioControlService(radioSvc),
			service.NewTakeoverControlService(takeoverSvc),
		), serverMetrics))
	} else {
		hub.SetControlHandler(service.NewMetricsControlHandler(sessionCtl, serverMetrics))
	}

	if dccBusSvc != nil {
		commandStationSvc.SetRuntime(cmd.CommandStationRuntime{
			DccSync:    dccLayoutSync,
			SessionCtl: sessionCtl,
		})

		evtConsumer := service.NewDccBusEventConsumer(redisSvc, hub, log)
		evtConsumer.SetMetrics(serverMetrics)
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
		log.WithField("admin_pin", cmd.DefaultAdminPIN).
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

	// In `-tags prod` builds the compiled SPA (web/dist) is embedded and
	// served at "/". Development builds return (nil, false) and rely on
	// the Vite dev server instead.
	staticFS, embedded := frontend.Dist()
	if embedded {
		log.Info("serving embedded frontend bundle at /")
	} else {
		log.Info("no embedded frontend bundle (dev build) — serve the SPA with `make web-dev`")
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
		Functions:        functionSvc,
		VehicleTemplates: vehicleTemplateSvc,
		Trains:           trainSvc,
		LayoutVehicles:   layoutVehicleSvc,
		DCCPool:          dccPoolSvc,
		Sudo:             sudoSvc,
		CommandStations:  commandStationSvc,
		Diagnostics:      diagSvc,
		Hub:              hub,
		DccBus:           dccBusSvc,
		Radio:            radioSvc,
		Audit:            auditSvc,
		Leases:           leaseSvc,
		Remote:           remoteSvc,
		AllowedOrigins:   f.AllowedOrigins,
		SecureCookie:     f.SecureCookie,
		StaticFS:         staticFS,
		Metrics:          serverMetrics,
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

	mdnsCtx, mdnsCancel := context.WithCancel(ctx)
	defer mdnsCancel()
	startHTTPMDNS(mdnsCtx, log, f)

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

	mdnsCancel()

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

// startHTTPMDNS advertises the loco-server HTTP UI on the LAN. Failures are
// logged and never abort the HTTP server.
func startHTTPMDNS(ctx context.Context, log *logrus.Logger, f Flags) {
	if !f.MDNS {
		log.Debug("mDNS advertisement disabled")
		return
	}
	host, portStr, err := net.SplitHostPort(f.HTTPAddr)
	if err != nil {
		log.WithError(err).WithField("addr", f.HTTPAddr).Warn("mDNS: parse http listen address")
		return
	}
	if mdns.IsLoopbackHost(host) {
		log.WithField("addr", f.HTTPAddr).Info("mDNS: skipping advertisement for loopback-only bind")
		return
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		log.WithField("port", portStr).Warn("mDNS: invalid http port")
		return
	}
	mdnsHost := f.MDNSHost
	if mdnsHost == "" {
		mdnsHost = "bigfred"
	}
	go func() {
		reg := mdns.NewRegistrar(log)
		err := reg.Register(ctx, mdns.RegisterInput{
			Instance: "BigFred",
			Service:  mdns.ServiceHTTP,
			Host:     mdnsHost,
			Port:     port,
			TXT: map[string]string{
				"path": "/",
			},
		})
		if err != nil && ctx.Err() == nil {
			log.WithError(err).Warn("mDNS: advertisement stopped with error")
		}
	}()
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
	// Hex-encode the secret so it is ASCII-safe. It is forwarded to each
	// dcc-bus daemon on the command line and written verbatim into
	// supervisord.conf, which supervisord parses strictly as UTF-8. Raw
	// random bytes routinely contain non-UTF-8 sequences (e.g. 0x96) that
	// make `supervisorctl reread` fail, so the daemon never starts and the
	// data-plane proxy returns 502. The encoding is irrelevant to HMAC.
	return []byte(hex.EncodeToString(buf)), nil
}
