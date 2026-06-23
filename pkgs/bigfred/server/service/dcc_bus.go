package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/keskad/loco/pkgs/bigfred/contract"
	dccbuscli "github.com/keskad/loco/pkgs/bigfred/dcc-bus/cli"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
	"github.com/keskad/loco/pkgs/bigfred/server/metrics"
	svcerrors "github.com/keskad/loco/pkgs/bigfred/server/errors"
	"github.com/keskad/loco/pkgs/bigfred/server/repo"
	"github.com/keskad/loco/pkgs/bigfred/server/supervisord"
)

// DccBusGroupName is the supervisord [group:NAME] holding every
// `dcc-bus-<L>-<C>` program managed by DccBusService.
const DccBusGroupName = "dcc-bus"

// ErrNoDccBusPortsAvailable is returned by EnsureRunning when the
// configured port pool is exhausted.
var ErrNoDccBusPortsAvailable = svcerrors.ErrNoDCCBusPortsAvailable

// ErrDccBusUnavailable is returned when EnsureRunning could not
// confirm the daemon reached RUNNING + dial-able within the timeout.
// Surface this to the WS layer as `dcc_bus_unavailable`.
var ErrDccBusUnavailable = errors.New("dcc-bus daemon unavailable")

// DccBusConfig configures DccBusService. Defaults match §7e.2.
type DccBusConfig struct {
	// Executable is the absolute path of the loco-server binary used
	// to launch `<executable> dcc-bus …`. Defaults to os.Args[0].
	Executable string
	// RedisAddr is forwarded as the daemon's --redis-addr flag.
	RedisAddr string
	// JWTSecret is forwarded verbatim via --jwt-secret. The
	// supervisord render path applies shell quoting so spaces /
	// quotes survive.
	JWTSecret []byte
	// PortMin / PortMax bracket the TCP port pool. Default
	// 9200..9209 — see §7e.2.
	PortMin uint16
	PortMax uint16
	// SpawnTimeout is the budget EnsureRunning waits for a freshly-
	// spawned daemon to accept a WS dial. Default 10s — matches
	// supervisord's startsecs + a small slack window.
	SpawnTimeout time.Duration
	// AllowedOrigins is forwarded as --allowed-origin flags so a
	// dev frontend can dial the daemon directly (production proxies
	// through loco-server and the slice is empty).
	AllowedOrigins []string
	// ProxyEnabled controls whether session.opened reports the
	// reverse-proxy path (default true) or the raw daemon port.
	ProxyEnabled bool
	// EnableTelemetry forwards --enable-telemetry to spawned dcc-bus
	// daemons when OTLPEndpoint is set.
	EnableTelemetry bool
	// OTLPEndpoint is the OTLP/gRPC address dcc-bus exports metrics to
	// (typically the local Alloy receiver).
	OTLPEndpoint string
}

// DccBusService is the loco-server-side orchestrator for dcc-bus
// daemons. It owns the port pool, drives supervisord via
// SupervisordService, and exposes typed helpers to publish commands
// onto the daemon's Redis channels (§7e.6).
type DccBusService struct {
	cfg   DccBusConfig
	sup   Supervisor
	redis *RedisService
	cs    *repo.CommandStations
	log   *logrus.Logger

	mu      sync.Mutex
	ports   map[portKey]uint16 // (layoutID, commandStationID) -> port
	metrics *metrics.Metrics
}

type portKey struct {
	LayoutID         uint
	CommandStationID uint
}

// NewDccBusService returns a service ready to spawn daemons. The
// caller MUST run SupervisordService.Start before any EnsureRunning
// call. `redis` may be nil in tests that don't need the persistent
// port assignment; production wires it.
func NewDccBusService(cfg DccBusConfig, sup Supervisor, redis *RedisService, cs *repo.CommandStations, log *logrus.Logger) *DccBusService {
	if log == nil {
		log = logrus.New()
	}
	if cfg.Executable == "" {
		cfg.Executable = os.Args[0]
	}
	if cfg.PortMin == 0 {
		cfg.PortMin = 9200
	}
	if cfg.PortMax == 0 {
		cfg.PortMax = 9209
	}
	if cfg.SpawnTimeout == 0 {
		cfg.SpawnTimeout = 10 * time.Second
	}
	return &DccBusService{
		cfg:   cfg,
		sup:   sup,
		redis: redis,
		cs:    cs,
		log:   log,
		ports: make(map[portKey]uint16, 8),
	}
}

// HydratePorts loads previously-allocated port mappings from Redis
// so a loco-server restart preserves daemon ↔ port pairings. Call
// once during bootstrap, after SupervisordService.Start. Safe to
// call when Redis is empty.
func (d *DccBusService) HydratePorts(ctx context.Context) error {
	if d.redis == nil {
		return nil
	}
	rows, err := d.redis.Client().HGetAll(ctx, contract.DccBusPortsKey).Result()
	if err != nil {
		return fmt.Errorf("hgetall %s: %w", contract.DccBusPortsKey, err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for k, v := range rows {
		var layoutID, commandStationID uint
		if _, err := fmt.Sscanf(k, contract.DccBusPortsFieldTmpl, &layoutID, &commandStationID); err != nil {
			continue
		}
		p, err := strconv.ParseUint(v, 10, 16)
		if err != nil {
			continue
		}
		d.ports[portKey{LayoutID: layoutID, CommandStationID: commandStationID}] = uint16(p)
	}
	return nil
}

// LayoutIDsWithProgramForCS returns layout ids that currently have a
// port assignment for commandStationID. Used when catalogue rows change
// so supervisord can restart daemons that are still running after the
// operator disconnects from the control plane.
func (d *DccBusService) LayoutIDsWithProgramForCS(commandStationID uint) []uint {
	d.mu.Lock()
	defer d.mu.Unlock()
	seen := make(map[uint]struct{})
	out := make([]uint, 0)
	for k := range d.ports {
		if k.CommandStationID != commandStationID {
			continue
		}
		if _, ok := seen[k.LayoutID]; ok {
			continue
		}
		seen[k.LayoutID] = struct{}{}
		out = append(out, k.LayoutID)
	}
	return out
}

// SetMetrics wires optional OpenTelemetry recorders for orchestration paths.
func (d *DccBusService) SetMetrics(m *metrics.Metrics) {
	d.metrics = m
}

// AllocatedPortCount returns how many dcc-bus daemons have a port assignment.
// Implements metrics.DccBusStatsReader.
func (d *DccBusService) AllocatedPortCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.ports)
}

// PortFor returns the TCP port assigned to (layoutID, commandStationID)
// or 0 when none has been allocated yet. Used by the reverse proxy
// to resolve `csId → port` without serializing on the daemon's mutex.
func (d *DccBusService) PortFor(layoutID, commandStationID uint) uint16 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.ports[portKey{LayoutID: layoutID, CommandStationID: commandStationID}]
}

// programName returns the supervisord program name for a daemon.
// The name MUST satisfy `[a-z][a-z0-9_-]*` to pass DesiredState
// validation, so we always lower-case + dash-join the numbers.
func programName(layoutID, commandStationID uint) string {
	return fmt.Sprintf("dcc-bus-%d-%d", layoutID, commandStationID)
}

// EnsureRunning guarantees a `dcc-bus-<L>-<C>` program exists in
// supervisord, RUNNING, and accepting WS connections. It returns
// the loopback port the daemon listens on plus the program name
// for audit / logging.
//
// The lazy-spawn flow per §7e.6:
//
//  1. If a port is already allocated, just verify the daemon is
//     RUNNING and dial-able; return its port.
//  2. Otherwise allocate a free port from the pool, upsert the
//     program into supervisord (autostart + autorestart), wait for
//     RUNNING and `tcp dial OK`, persist the port assignment.
func (d *DccBusService) EnsureRunning(ctx context.Context, layoutID, commandStationID uint) (uint16, string, error) {
	start := time.Now()
	port, name, err := d.ensureRunning(ctx, layoutID, commandStationID)
	if d.metrics != nil {
		d.metrics.RecordDccBusEnsureRunning(layoutID, commandStationID, time.Since(start), err)
	}
	return port, name, err
}

func (d *DccBusService) ensureRunning(ctx context.Context, layoutID, commandStationID uint) (uint16, string, error) {
	if d.sup == nil {
		return 0, "", errors.New("dcc-bus: supervisord service is not wired")
	}
	name := programName(layoutID, commandStationID)
	key := portKey{LayoutID: layoutID, CommandStationID: commandStationID}

	d.mu.Lock()
	port, ok := d.ports[key]
	d.mu.Unlock()

	if ok {
		if err := d.waitDialable(ctx, port); err == nil {
			d.log.WithFields(logrus.Fields{
				"program": name, "layoutId": layoutID, "commandStationId": commandStationID, "port": port,
			}).Debug("dcc-bus ensure running: already up")
			return port, name, nil
		}
		d.log.WithFields(logrus.Fields{
			"program": name, "port": port,
		}).Warn("dcc-bus ensure running: port assigned but not dialable, re-upserting")
	}

	if !ok {
		var err error
		port, err = d.allocatePortLocked(layoutID, commandStationID)
		if err != nil {
			return 0, name, err
		}
		d.log.WithFields(logrus.Fields{
			"program": name, "layoutId": layoutID, "commandStationId": commandStationID, "port": port,
		}).Info("dcc-bus ensure running: allocated port")
	}

	spec, err := d.buildProgramSpec(ctx, name, layoutID, commandStationID, port)
	if err != nil {
		return 0, name, err
	}
	d.log.WithFields(logrus.Fields{"program": name, "port": port}).Info("dcc-bus ensure running: upserting supervisord program")
	if err := d.sup.UpsertProgram(ctx, DccBusGroupName, spec); err != nil {
		return 0, name, fmt.Errorf("upsert dcc-bus program: %w", err)
	}

	// supervisord autostarts the program; explicitly StartProgram
	// covers the "already declared, was stopped" path.
	if err := d.sup.StartProgram(ctx, name); err != nil {
		// supervisord returns an error when the program is already
		// running; swallow that case.
		if !strings.Contains(err.Error(), "ALREADY_STARTED") &&
			!strings.Contains(strings.ToUpper(err.Error()), "ALREADY") {
			d.log.WithError(err).WithField("program", name).Debug("dcc-bus start (treat as soft)")
		}
	}

	if err := d.waitDialable(ctx, port); err != nil {
		d.log.WithError(err).WithFields(logrus.Fields{"program": name, "port": port}).Error("dcc-bus ensure running: daemon not dialable")
		return 0, name, fmt.Errorf("%w: %v", ErrDccBusUnavailable, err)
	}
	d.persistPort(ctx, key, port)
	d.log.WithFields(logrus.Fields{"program": name, "port": port}).Info("dcc-bus ensure running: daemon ready")
	return port, name, nil
}

// Stop tears down one daemon (e.g. when the operator detaches the
// command station from the layout). Idempotent.
func (d *DccBusService) Stop(ctx context.Context, layoutID, commandStationID uint) error {
	name := programName(layoutID, commandStationID)
	d.log.WithFields(logrus.Fields{
		"program": name, "layoutId": layoutID, "commandStationId": commandStationID,
	}).Info("dcc-bus stop: removing supervisord program")
	_ = d.sup.StopProgram(ctx, name)
	if err := d.sup.RemoveProgram(ctx, DccBusGroupName, name); err != nil {
		if !errors.Is(err, supervisord.ErrProgramNotFound) {
			return err
		}
	}
	key := portKey{LayoutID: layoutID, CommandStationID: commandStationID}
	d.mu.Lock()
	delete(d.ports, key)
	d.mu.Unlock()
	if d.redis != nil {
		_ = d.redis.Client().HDel(ctx, contract.DccBusPortsKey, contract.DccBusPortsField(layoutID, commandStationID)).Err()
	}
	return nil
}

// PublishCommand emits a typed envelope on the daemon's command
// channel. Returns an error when the daemon has never been spawned
// (no port assigned yet) so callers can decide whether to spawn
// lazily or surface "no_command_station".
func (d *DccBusService) PublishCommand(ctx context.Context, layoutID, commandStationID uint, eventType string, payload any) error {
	if d.redis == nil {
		return errors.New("dcc-bus: redis is not wired")
	}
	env, err := protocol.Frame(eventType, payload)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(env)
	if err != nil {
		return err
	}
	channel := contract.DccBusCommandChannel(layoutID, commandStationID)
	return d.redis.Client().Publish(ctx, channel, raw).Err()
}

// ProxyEnabled reports whether the WS layer should advertise the
// reverse-proxy path (true) or the raw `127.0.0.1:<port>` address
// (false, dev only).
func (d *DccBusService) ProxyEnabled() bool { return d.cfg.ProxyEnabled }

// allocatePortLocked picks the next free port from the pool. Caller
// MUST NOT hold the mutex.
func (d *DccBusService) allocatePortLocked(layoutID, commandStationID uint) (uint16, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	used := make(map[uint16]struct{}, len(d.ports))
	for _, p := range d.ports {
		used[p] = struct{}{}
	}
	for p := d.cfg.PortMin; p <= d.cfg.PortMax; p++ {
		if _, taken := used[p]; taken {
			continue
		}
		// Optimistically claim. The OS-level bind happens later via
		// supervisord; on a race the daemon would fail to listen and
		// EnsureRunning would surface ErrDccBusUnavailable.
		d.ports[portKey{LayoutID: layoutID, CommandStationID: commandStationID}] = p
		return p, nil
	}
	return 0, ErrNoDccBusPortsAvailable
}

// persistPort mirrors the in-memory port assignment into Redis so a
// loco-server restart can hydrate the same map.
func (d *DccBusService) persistPort(ctx context.Context, k portKey, port uint16) {
	if d.redis == nil {
		return
	}
	field := contract.DccBusPortsField(k.LayoutID, k.CommandStationID)
	if err := d.redis.Client().HSet(ctx, contract.DccBusPortsKey, field, strconv.Itoa(int(port))).Err(); err != nil {
		d.log.WithError(err).Debug("dcc-bus persist port")
	}
}

// waitDialable polls the daemon's loopback port until a TCP dial
// succeeds or the SpawnTimeout budget elapses.
func (d *DccBusService) waitDialable(ctx context.Context, port uint16) error {
	deadline := time.Now().Add(d.cfg.SpawnTimeout)
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port)))
	for {
		dialCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		conn, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", addr)
		cancel()
		if err == nil {
			_ = conn.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("dial dcc-bus on %s: %w", addr, err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(150 * time.Millisecond):
		}
	}
}

// buildProgramSpec assembles the supervisord program spec used to
// spawn the daemon. The command line is rendered with shell escaping
// applied at supervisord's template level.
func (d *DccBusService) buildProgramSpec(ctx context.Context, name string, layoutID, commandStationID uint, port uint16) (supervisord.ProgramSpec, error) {
	if d.cs == nil {
		return supervisord.ProgramSpec{}, errors.New("dcc-bus: command station repository is not wired")
	}
	cs, err := d.cs.FindByID(ctx, commandStationID)
	if err != nil {
		return supervisord.ProgramSpec{}, fmt.Errorf("load command station %d: %w", commandStationID, err)
	}
	args := []string{
		d.cfg.Executable, "dcc-bus",
		"--layout-id", strconv.FormatUint(uint64(layoutID), 10),
		"--command-station-id", strconv.FormatUint(uint64(commandStationID), 10),
		"--port", strconv.Itoa(int(port)),
		"--bind", "127.0.0.1",
		"--redis-addr", d.cfg.RedisAddr,
		"--jwt-secret", string(d.cfg.JWTSecret),
	}
	args = dccbuscli.AppendStationFlags(args, cs)
	for _, origin := range d.cfg.AllowedOrigins {
		args = append(args, "--allowed-origin", origin)
	}
	args = appendDccBusTelemetryArgs(args, d.cfg)
	return supervisord.ProgramSpec{
		Name:         name,
		Command:      strings.Join(args, " "),
		Autostart:    true,
		Autorestart:  true,
		StartSecs:    1,
		StopWaitSecs: 5,
	}, nil
}

func appendDccBusTelemetryArgs(args []string, cfg DccBusConfig) []string {
	if !cfg.EnableTelemetry || cfg.OTLPEndpoint == "" {
		return args
	}
	return append(args, "--enable-telemetry", "--otel-endpoint", cfg.OTLPEndpoint)
}
